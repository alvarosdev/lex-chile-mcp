## Context

Estado actual: `internal/prompts/prompts.go` + `prompts.yaml` (1 package, 1 `PromptSet`, 9 prompts BCN con 2 constitucionales), `//go:embed prompts.yaml` bakeado, `LoadEmbedded()` fail-fast, `RegisterPrompts(srv, ps)` con 1 singleton en `main.go`. `internal/bcn` vs `internal/cgr` y `RegisterTools` vs `RegisterCgrTools` ya están separados por dominio con wiring explícito en `main.go`. Los prompts CGR no existen — el LLM no tiene guía para `count_cgr_jurisprudencia`/`search_cgr_dictamenes`/`get_cgr_dictamen` ni para `url`/`pdf_url`.

## Goals / Non-Goals

**Goals:**
- Separar prompts por dominio en `internal/prompts/bcn` y `internal/prompts/cgr`, cada uno con `prompts.go` + `prompts.yaml` + `prompts_test.go`, con interfaces descriptivas `bcn.PromptSet` / `cgr.PromptSet` (senior naming con siglas conservadas).
- Mantener misma lógica: `go:embed` por yaml, `ToolNames()` por dominio, `allowedPlaceholders` cerrado, `render()` con `missingkey=error`, validación de `expectedPromptNames` y `RegisterPrompts` puro sin tocar red.
- 4 prompts CGR curados (híbrido C) + 1 prompt BCN `interpret_law` nuevo (total 14): CGR enseña flujo `count → search → get`, citación `url`/`pdf_url`, método 4 pasos y anti-sesgo; BCN enriquece los 9 existentes + `interpret_law` con jerarquía, 5 elementos (art.19-24 Código Civil) y anti-sesgo. Ambos con regla "MCP es fuente de la verdad" + disclaimer fijo + adaptación de idioma (default español, prompts en inglés).
- Wiring explícito en `main.go` sin fachada `internal/prompts` (2 imports, 2 loads, 2 registers).
- Extraer boilerplate compartido a `internal/prompts/internal` para no duplicar `render`/`placeholderRe`/`loadFromBytes` (DRY sin acoplar dominios).

**Non-Goals:**
- Fachada `internal/prompts` que reexporte ambos (reacopla).
- Renombrar siglas `bcn`/`cgr` a `law`/`jurisprudence` (cambiaría demasiados imports).
- Cambiar tools (siguen 7) o contrato `api.resources.yaml`.

## Decisions

**Decisión 1 — Estructura `prompts/bcn` + `prompts/cgr` sin fachada (elegida):**
- `internal/prompts/bcn/prompts.go` (package `bcn`, `type PromptSet`, `func LoadEmbedded()`, `func ToolNames() []string` → 4 tools BCN, `const toolSearchLaws...`, `var expectedPromptNames[10]`, `//go:embed prompts.yaml`).
- `internal/prompts/cgr/prompts.go` (package `cgr`, `type PromptSet`, `func LoadEmbedded()`, `func ToolNames() []string` → 3 tools CGR `search_cgr_dictamenes`/`get_cgr_dictamen`/`count_cgr_jurisprudencia`, `const toolSearchCgr...`, `var expectedPromptNames[4]`, `//go:embed prompts.yaml`).
- `internal/prompts/internal/render.go` (package `internal`, `func ParseAndValidate(yaml []byte, allowed map[string]bool, expected []string) (map[string]*template.Template, error)` con `placeholderRe`/`render`/`loadFromBytes`) — usado por `bcn` y `cgr` para no duplicar 80 líneas.
- Se borra `internal/prompts/prompts.go` y `prompts.yaml` originales vía `git mv` (preserva `git log --follow`). Alternativa descartada: 1 package con 2 files (`bcn_prompts.go`/`cgr_prompts.go`) — deja `allowedPlaceholders` y `ToolNames()` compartidos y obliga a condicionales; alternativa `internal/bcn/prompts` mezcla cliente y prompts.

**Decisión 2 — Nombres `bcn.PromptSet` / `cgr.PromptSet` (elegido):**
- Siglas conservadas como pides; interfaz descriptiva por dominio con `PromptSet` como tipo principal (senior: `bcn.PromptSet` dice "prompt set de BCN" sin inventar `LawPromptSet`). Alias largo `jurisprudence.PromptSet` descartado — cambiaría `bcn`/`cgr` en todo el repo.

**Decisión 3 — 4 prompts CGR MVP (híbrido C, elegido) + 1 BCN `interpret_law`:**
- `search_jurisprudence` — `count_cgr_jurisprudencia` para explorar buckets, luego `search_cgr_dictamenes` paginado con `order`/`exact_search`, luego `get_cgr_dictamen` del `dictamen_id`, citando `url`/`pdf_url`. Preamble corto (1 línea: "Aplica principios de interpret_dictamen: jerarquía Constitución>ley>dictamen, no asumas, anti-sesgo, cita url/pdf_url") + adaptación de idioma.
- `analyze_dictamen` — `get_cgr_dictamen` + síntesis de `materia`/`descriptores`/`criterio`/`fuentes_legales` + `documento_completo`, citando `url`/`pdf_url` por claim, con hedge y disclaimer "no es asesoría legal, solo fuente de información, la interpretación de la IA puede variar entre modelos y proveedores", preamble corto.
- `explain_dictamen_simply` — parafraseo de `documento_completo` para audiencia no legal, con `url`/`pdf_url`, disclaimer y preamble corto.
- `interpret_dictamen` — prompt dedicado con guía completa: naturaleza jurídica (informe obligatorio, jurisprudencia vinculante, no crea ley, impugnable), jerarquía corta (Constitución > ley/reglamento > dictamen), método 4 pasos (problema → normas → fundamentación → resolutiva), anti-sesgo (qué vs para qué, separar mensaje/mensajero, lecturas interesadas, hecho vs opinión) y fuente MCP como verdad. Es la referencia profunda que los otros 3 citan con 1 línea.
- `interpret_law` (BCN) — guía para leyes: jerarquía (Constitución, arts. 19-24 Código Civil), 5 elementos (gramatical art.20, histórico art.19 inc2, lógico art.22, sistemático art.22, sociológico art.19 inc1) y anti-sesgo, con disclaimer y adaptación de idioma.

**Decisión 4 — Wiring en `main.go` sin fachada (elegido):**
```go
import bcnPrompts "github.com/alvarosdev/lex-chile-mcp/internal/prompts/bcn"
import cgrPrompts "github.com/alvarosdev/lex-chile-mcp/internal/prompts/cgr"

bcnPS, err := bcnPrompts.LoadEmbedded() // fail-fast
cgrPS, err := cgrPrompts.LoadEmbedded()
bcnPrompts.RegisterPrompts(srv, bcnPS)
cgrPrompts.RegisterPrompts(srv, cgrPS)
```
- 2 singletons, 2 registers, explícito, testeable. Fachada `prompts.LoadEmbedded() → combined` descartada: oculta separación y obliga a manejar 2 `PromptSet` en 1 tipo.

**Decisión 5 — Validación por dominio y reglas transversales:**
- Cada `prompts.yaml` valida `expectedPromptNames` exacto (10 vs 4) y `allowedPlaceholders` cerrado (`dictamen_id`, `query`, `audience`, `lang` para CGR; `norm_id`/`topic`/`aspect`/`lang` para BCN). `TestTemplatesReferenceOnlyRegisteredTools` por dominio con `ToolNames()` correspondiente. `mock.Anything` para ctx sigue igual.
- Reglas transversales bakeadas en cada template (ambos dominios): "MCP es fuente de la verdad — no asumas derecho que no esté en el tool output", anti-sesgo, disclaimer fijo "no es asesoría legal, solo fuente de información, la interpretación de la IA puede variar entre modelos y proveedores", y "responde en {{if .lang}}{{.lang}}{{else}}el idioma del usuario (detecta de la consulta/sesión; default español){{end}}" — prompts en inglés pero respuesta adaptativa testeable via `{{.lang}}`.

**Decisión 6 — Boilerplate compartido `internal/prompts/internal` (elegido):**
- Extrae `render`, `placeholderRe`, `loadFromBytes`, `PromptSet` base a `internal/prompts/internal` (package `internal`, solo código puro sin estado). `bcn` y `cgr` importan `internal` y solo definen `ToolNames`, `expectedPromptNames`, `allowedPlaceholders` y `prompts.yaml`. Evita duplicar 80 líneas y que un fix de `missingkey=error` se olvide en un dominio, sin acoplar dominios (cambios en BCN no tocan CGR salvo el internal puro).

## Risks / Trade-offs

- **Borrar `internal/prompts`** rompe imports existentes (`internal/prompts` usado en `main.go` y tests). Mitigación: `git mv` + actualización de todos los call sites en el mismo commit; no se deja redirect deprecated.
- **Binario con 2 embeds** — 2 `go:embed` en 2 packages, ambos baked. Sin hot-reload, rebuild requerido para cambiar prompts (ya es así).
- **Spec `law-prompts` movida** — `openspec/specs/law-prompts/spec.md` existe; el delta debe reflejar move sin cambio de requisitos.
- **Preamble corto vs completo** — riesgo de que LLM no vea anti-sesgo si solo está en `interpret_*`. Mitigación: cada prompt operativo incluye 1 línea de referencia a `interpret_*` como profundización.

## Migration Plan

1. Crear `internal/prompts/internal/render.go` con lógica compartida.
2. Crear `internal/prompts/bcn/prompts.go|yaml|test.go` vía `git mv` de `internal/prompts/` + añadir `interpret_law` y `{{.lang}}`, actualizar `expectedPromptNames[10]`.
3. Crear `internal/prompts/cgr/prompts.go|yaml|test.go` con 4 prompts curados (preamble corto + referencia a `interpret_dictamen`), `expectedPromptNames[4]`, `allowedPlaceholders` con `lang`.
4. Borrar `internal/prompts/prompts.go|yaml|test.go` originales.
5. Actualizar `cmd/lex-chile-mcp/main.go` (2 imports, 2 loads, 2 registers) y `README.md` (tabla de prompts por dominio).
6. `go vet`, `make check`, `openspec validate --strict`; verificar `prompts/list` 14 y `TestTemplatesReferenceOnlyRegisteredTools` por dominio.
