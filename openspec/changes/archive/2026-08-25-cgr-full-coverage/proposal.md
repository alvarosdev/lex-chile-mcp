## Why

El servidor expone solo `dictamenes` (3 tools) pero `contraloria.cl/apibusca` expone 8 fuentes reales (`dictamenes`, `instructivos`, `contable`, `auditoria`, `legislacion`, `cuentas`, `consolidados`, `web`) + `todos`. La captura HAR (34 entries, 13 MB, `Quillota` + gentiles `municipalidad`/`licencias médicas`/`toma de razón` con pacing 3-4s) mostró volúmenes que invalidan el scope actual: `municipalidad` → `contable 622`, `cuentas 645`, `legislacion 9.557`, `auditoria 10k+ gte`, `instructivos 18` (IN23 día funcionario), `consolidados 11` (CIC21 licencias); `licencias médicas` → `consolidados 8` (CIC21), `auditoria 2.831`; `toma de razón` → `legislacion 1.558`, `dictamenes 10k+`, `instructivos 13` (E462387 exención toma de razón 2024). El usuario pidió explícitamente no ignorar `contable` e `instructivos`. Además los prompts actuales (4 CGR) enseñan solo `dictamenes` y obligan `count` siempre; con `municipalidad` a las 13h `count/todos` hizo timeout 40s y 1 prompt por `get_*` (6 nuevos → 20 prompts) saturaría harness/LLM (12-14k tokens). Hay que cubrir las 8 fuentes y reescribir prompts por familia sin saturar, en un solo PR.

## What Changes

- Corrige `count_cgr_jurisprudencia` para usar `POST /count/todos` (buckets `count_by_type` reales) y documenta que `count` es opcional bajo carga (si timeout, pasar directo a `search`).
- Generaliza `CgrClient` a `Search(ctx, SearchParams{Source, Query, ExactSearch, Order, Page})` con `validSource()` whitelist de 8 + `todos`; `Get` por tipo con validadores de ID por formato (`dictamen/instructivo` `^[A-Z]*[0-9]+N[0-9]{2}$`, `contable` `^E[0-9]+$`/`OFE...`, `cuentas` `^[0-9]+$`, `consolidados` `^CIC[0-9]+/[0-9]{4}$`, `legislacion` `^RZA...|FRA...`, `auditoria` `^[0-9]+/[0-9]{4}$`).
- Declara en `internal/config/api.resources.yaml` timeouts por familia: `contable/instructivos/consolidados/cuentas` 10s, `dictamenes/web` 12s, `auditoria/legislacion` 15s; retry `2×1s` para pesados y `3×500ms` para livianos; un `*resty.Client` base con `Origin`/`Accept` y breaker `5/2/30s` (no 8 clients).
- Expone `search_cgr` genérica con `source` enum (`dictamenes|instructivos|contable|auditoria|legislacion|cuentas|consolidados|web|todos`, default `dictamenes` para compat; mantiene `search_cgr_dictamenes` como alias delgado) y `page`/`order`/`exact_search`.
- Añade `get` tipados por dominio: `get_cgr_instructivo`, `get_cgr_contable`, `get_cgr_auditoria`, `get_cgr_consolidado`, `get_cgr_cuenta`, `get_cgr_legislacion` (cada uno llama `POST /search/{source}` `exact_search:true` y proyecta su `_source` a `StructuredContent` tipado + texto derivado).
- Sanitización por tipo en `internal/cgr/sanitize.go` + `garbage.go`: `SanitizeAuditoria` (strip `<?xml pdf:PDFVersion` y preserva `objetivo`/`conclusiones`), `SanitizeContable`/`SanitizeLegislacion` (strip `texto_raw` HTML), reutiliza `normalize()` existente.
- Reescribe `search_jurisprudence` en `internal/prompts/cgr/prompts.yaml` para `search_cgr` genérico: tabla `source` (dictamenes, instructivos IN23, contable NICSP, auditoria 10k, consolidados CIC21, cuentas, legislacion AFECTO), flujo `count?(opcional)` → `search_cgr(source,order)` → `get_*`, hint `order=score si total>500`, pacing 3-4s; mantiene `source` default `dictamenes` para compat.
- Añade `analyze_contable_instructivo` (contable NICSP + instructivo IN) y `analyze_auditoria_consolidado` (auditoria 5MB trunc 30k + CIC) — 2 prompts por familia en vez de 6, sin saturar (4→6 CGR, total 16 con BCN 10); `legislacion`/`cuentas` viven en genérico + `interpret_dictamen`.
- Actualiza `internal/prompts/cgr/prompts.go`: `expectedPromptNames` 4→6, `allowedPlaceholders` con `source`/`order`/`contable_id`/etc., `ToolNames()` 3→10, `//go:embed` bakeado.
- Actualiza `README.md`: **Contraloría** 3 bullets → 8 fuentes con ejemplos `municipalidad`/`licencias médicas`/`toma de razón`; **Para tu IA** 14→16 guías; tabla `search_cgr` `source` enum + 6 `get_*`; aviso `count` opcional y pacing.
- **BREAKING**: ninguno para callers existentes (`search_cgr_dictamenes` y `get_cgr_dictamen` siguen igual); `count` cambia path pero mantiene shape; `search_jurisprudence` mantiene nombre y añade `source` opcional.

## Capabilities

### New Capabilities

- `cgr-instructivos`: búsqueda y ficha de instrucciones generales (IN) con `search_cgr` `source=instructivos` y `get_cgr_instructivo`.
- `cgr-contable`: búsqueda y ficha de oficios de normativa contable NICSP con `source=contable` y `get_cgr_contable`.
- `cgr-auditoria`: búsqueda y ficha de informes de auditoría/fiscalización con `source=auditoria` y `get_cgr_auditoria` (paginación 20, `order` score/date, aviso cap 10k).
- `cgr-consolidados`: búsqueda y ficha de Consolidados de Información Circularizada (CIC) con `source=consolidados` y `get_cgr_consolidado` (`contenido_extraido` + `documento_cic_pdf_web`).
- `cgr-cuentas`: búsqueda y ficha de sentencias del Juzgado de Cuentas con `source=cuentas` y `get_cgr_cuenta` (`texto` + `pdf`/`pdf2`).
- `cgr-legislacion`: búsqueda y ficha de legislación con toma de razón con `source=legislacion` y `get_cgr_legislacion` (`texto_` + `materias` + `organismo`).
- `cgr-contable-instructivo-prompt`: prompt curado que enseña a buscar y analizar normativa contable (`get_cgr_contable`) e instrucciones generales (`get_cgr_instructivo`) con citación `url`/`pdf_url` y `normativa_contable`.
- `cgr-auditoria-consolidado-prompt`: prompt curado que enseña a buscar y analizar auditoría (`get_cgr_auditoria` trunc 30k) y consolidados CIC (`get_cgr_consolidado` con `resena`+`contenido_extraido`) con `pdf`/`documento_cic_pdf_web`.

### Modified Capabilities

- `cgr-search`: de `search_cgr_dictamenes` solo-dictamenes a `search_cgr` multi-source con `source` param, `order` y `page` genéricos, resources por familia y pacing anti-bloqueo.
- `cgr-count`: de `POST /count/dictamenes` a `POST /count/todos` con buckets completos y semántica opcional bajo carga.
- `cgr-dictamen`: sin cambio de requisitos, pero `CgrClient` pasa a usar `Search` genérico internamente; mantiene validación y URLs `url`/`pdf_url`.
- `cgr-prompts`: de 4 prompts CGR mono-source a 6 con `search_jurisprudence` genérico `source` enum (dictamenes, instructivos IN23, contable NICSP, auditoria 10k, consolidados CIC21, cuentas, legislacion AFECTO) y 2 prompts por familia (`analyze_contable_instructivo`, `analyze_auditoria_consolidado`) sin saturar a 20.
