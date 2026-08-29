## 1. Separación de prompts por dominio

- [x] 1.1 Crear `internal/prompts/internal/render.go` compartido (package `internal`, `ParseAndValidate`, `placeholderRe`, `render`, `loadFromBytes` con `missingkey=error`) para no duplicar 80 líneas entre `bcn` y `cgr`
- [x] 1.2 Mover con `git mv` `internal/prompts/prompts.go` + `prompts.yaml` + `prompts_test.go` a `internal/prompts/bcn/` (package `bcn`, `type PromptSet`, `ToolNames()` con 4 tools BCN, `expectedPromptNames[10]`, `allowedPlaceholders` con `lang` añadido, `//go:embed prompts.yaml`) y añadir `interpret_law` + preamble corto de 1 línea referenciando `interpret_law` en los 9 existentes (jerarquía, 5 elementos, anti-sesgo, fuente de la verdad, adaptación `{{if .lang}}`)
- [x] 1.3 Crear `internal/prompts/cgr/prompts.go` + `prompts.yaml` + `prompts_test.go` (package `cgr`, `type PromptSet`, `ToolNames()` con 3 tools CGR, `expectedPromptNames[4]`, `allowedPlaceholders` con `dictamen_id`/`query`/`audience`/`lang`, 4 prompts `search_jurisprudence`/`analyze_dictamen`/`explain_dictamen_simply`/`interpret_dictamen` con `url`/`pdf_url`, jerarquía corta, anti-sesgo, disclaimer fijo y preamble corto referenciando `interpret_dictamen`)

## 2. Wiring y validación

- [x] 2.1 Actualizar `cmd/lex-chile-mcp/main.go` para importar `bcnPrompts` y `cgrPrompts`, hacer `LoadEmbedded()` y `RegisterPrompts()` por dominio (2 singletons), añadir `logger.Info("Prompts loaded", "bcn", len(bcnPS.Names()), "cgr", len(cgrPS.Names()))` y borrar import legacy
- [x] 2.2 Ejecutar `make check` (build+vet+test), `make fmt-check` y `openspec validate --strict`; verificar `prompts/list` 14 (10+4), `ToolNames()` por dominio, `allowedPlaceholders` con `lang`, y `TestTemplatesReferenceOnlyRegisteredTools` cruzado

## 3. Documentación

- [x] 3.1 Actualizar `README.md` (tabla de prompts por dominio con 14) y `openspec/specs/cgr-prompts` / `law-prompts` tras archive si aplica

> Audit 2026-08-29: verified against the live server (`prompts/list` = 16:
> 10 BCN + 6 CGR). Deviation from plan: the CGR set ships **6** prompts
> (adds `analyze_contable_instructivo` + `analyze_auditoria_consolidado`),
> not 4 — the delta spec was updated accordingly before archive.
