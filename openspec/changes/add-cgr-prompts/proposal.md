## Why

El servidor ya expone prompts curados para BCN (9 en `internal/prompts/prompts.yaml` con flujo `summary → section_id → get`), pero los dictámenes de Contraloría (CGR) tienen semántica distinta (`count → search → get`, sin `section_id` ni `version_date`, con `url`/`pdf_url` para citación) y no tienen prompts que enseñen a cualquier LLM cómo buscarlos y tratarlos. Mantener BCN y CGR en un mismo `prompts.yaml` y un mismo `PromptSet` mezcla dominios, rompe el patrón ya usado en `internal/bcn` vs `internal/cgr` y `RegisterTools` vs `RegisterCgrTools`, y hace que `ToolNames()` y `allowedPlaceholders` crezcan con condicionales.

## What Changes

- Separa `internal/prompts` en `internal/prompts/bcn` y `internal/prompts/cgr`, cada uno con su propio `prompts.go` y `prompts.yaml` bakeados vía `go:embed` (sin hot-reload, sin archivos externos), y su propio `prompts_test.go`, más `internal/prompts/internal/render.go` compartido para no duplicar `render`/`placeholderRe`/`loadFromBytes`. Se mueve con `git mv` para preservar `git log --follow` y se elimina `internal/prompts/prompts.go` original.
- Cada paquete expone interfaz descriptiva senior: `bcn.PromptSet` y `cgr.PromptSet` (con `LoadEmbedded()`, `ToolNames()`, `RegisterPrompts()`), con constantes de tools por dominio y `expectedPromptNames`/`allowedPlaceholders` cerrados (incluyendo `{{.lang}}` para adaptación de idioma). Sin fachada `internal/prompts` que reexporte ambos.
- `cmd/lex-chile-mcp/main.go` importa ambos paquetes y hace `bcn.LoadEmbedded()` + `cgr.LoadEmbedded()` y `bcn.RegisterPrompts(srv, bcnPS)` + `cgr.RegisterPrompts(srv, cgrPS)` (2 singletons, explícito, sin estado global).
- Se añaden 4 prompts CGR curados en `internal/prompts/cgr/prompts.yaml` (híbrido C): `search_jurisprudence` (flujo `count → search → get`), `analyze_dictamen` y `explain_dictamen_simply` (con cita `url`/`pdf_url`, preamble corto de 1 línea referenciando `interpret_dictamen` para jerarquía/anti-sesgo), y `interpret_dictamen` (guía completa: naturaleza jurídica, jerarquía corta Constitución>ley>dictamen, método 4 pasos, anti-sesgo). Todos en inglés pero con `{{if .lang}}{{.lang}}{{else}}el idioma del usuario (default español){{end}}` y disclaimer fijo "no es asesoría legal, solo fuente de información, la interpretación de la IA puede variar entre modelos y proveedores" y regla "MCP es fuente de la verdad".
- Se añade 1 prompt BCN `interpret_law` y se enriquece el preamble de los 9 prompts BCN existentes con la misma guía adaptada a leyes: jerarquía (Constitución, arts. 19-24 Código Civil), método 5 elementos (gramatical, histórico, lógico, sistemático, sociológico), anti-sesgo y adaptación de idioma con `{{.lang}}`.

## Capabilities

### New Capabilities

- `cgr-prompts`: prompts curados para jurisprudencia de Contraloría (búsqueda, análisis, explicación e interpretación de dictámenes con citación, anti-sesgo, jerarquía corta y adaptación de idioma).

### Modified Capabilities

- `law-prompts`: se mueve de `internal/prompts` a `internal/prompts/bcn` sin cambio de requisitos (9→10 prompts con `interpret_law`), pero con nueva ruta de package, `ToolNames()` por dominio, `allowedPlaceholders` con `lang` y preamble corto referenciando `interpret_law`; requiere delta spec para reflejar el move y el nuevo `RegisterPrompts` por dominio.

## Impact

- Código: `internal/prompts/bcn/*` (vía `git mv`, copia + `interpret_law`), `internal/prompts/cgr/*` (nuevo, 4 prompts), `internal/prompts/internal/*` (nuevo, compartido), `cmd/lex-chile-mcp/main.go` (2 imports, 2 loads, 2 registers).
- Tests: `internal/prompts/bcn/prompts_test.go` (10), `internal/prompts/cgr/prompts_test.go` (4) con `{{.lang}}`, validación de `ToolNames()` por dominio y `mock.Anything` para ctx.
- Docs: `README.md` (tabla de prompts por dominio), `openspec/specs/law-prompts/spec.md` y nuevo `openspec/specs/cgr-prompts/spec.md`.
- Sin breaking en tools (mismos 7 tools), sin nuevos deps; binario sigue autocontenido con ambos YAML embebidos.
