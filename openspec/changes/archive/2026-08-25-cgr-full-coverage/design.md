## Context

Estado actual: `internal/cgr` expone `SearchDictamenes`/`GetDictamen`/`CountJurisprudencia` con `source:"dictamenes"` hardcodeado, `cgr_search` `POST /search/dictamenes` 10s y `cgr_count` `POST /count/dictamenes` 10s en `api.resources.yaml`. HAR de 34 requests (`www.contraloria.cl.har`, 13 MB) + sondas live gentiles (`Quillota`, `municipalidad`, `licencias médicas`, `toma de razón`, pacing 3-4s) reveló 8 fuentes reales (`dictamenes`, `instructivos`, `contable`, `auditoria`, `legislacion`, `cuentas`, `consolidados`, `web`) + `todos`, cada una con `_source` distinto y volúmenes bajo carga: `municipalidad` → `contable 622`, `cuentas 645`, `legislacion 9.557`, `auditoria/dictamenes 10k+ gte`, `instructivos 18`, `consolidados 11`; `licencias médicas` → `consolidados 8` (CIC21), `auditoria 2.831`; `toma de razón` → `legislacion 1.558`, `instructivos 13`. El sitio satura a las 13h: `count/todos` timeout 40s, `search/legislacion` y `search/dictamenes` timeout 12s en primer intento (requirieron retry con `order=score`). El usuario pidió explícitamente cubrir `contable` e `instructivos` aunque sean nicho. Ver proposal.md Why.

Stack: Go 1.26, `go-sdk v1.7.0`, `resty v3` con `*resty.Client` por resource (breaker count-based + retry per-request declarado en YAML), `yaml.v3`, `testify` + `mockery`, `go:embed` sin hot-reload. Convenciones: interfaz en inglés, datos crudos en español, LLM-first dual output, tests sin red con `httptest` + `mock.Anything` para ctx, sanitización en `sanitize.go`/`garbage.go`.

## Goals / Non-Goals

**Goals:**
- Pasar de 3 tools mono-source a cobertura full 8 fuentes sin romper `search_cgr_dictamenes`/`get_cgr_dictamen` existentes.
- Transporte resiliente por familia (timeouts 10/12/15s, retry diferenciado, breaker 5/2/30s) y pacing documentado para no bloquear IP bajo carga.
- Wire types y sanitización por tipo, con `StructuredContent` tipado por `get_*` (no `any`).
- `count` corregido a `/count/todos` con buckets completos y semántica opcional.

- **Non-Goals:**
- Tool `suggest` (roto `cgr-*` 400) ni `web` como gets dedicados (scraping, ruido) ni facets `dinamic/*` como tools v1 (solo referencia en prompt).
- 1 prompt por `get_*` (6→20 prompts, satura harness/LLM) — se cubre con 2 por familia.
- Fachada que unifique BCN+CGR en un solo package.

## Decisions

**Decisión 1 — `CgrClient` genérico con `Source` param (elegida):**
```go
type SearchParams struct { Query string; ExactSearch bool; Order string; Page int; Source string }
func (c *Client) Search(ctx context.Context, p SearchParams) (SearchResponse, error)
func (c *Client) GetInstructivo(ctx context.Context, id string) (InstructivoFull, error) // etc.
func validSource(s string) bool { return slices.Contains(allowedSources, s) } // allowedSources = 9
```
Search construye `cgrSearchRequest{Source: p.Source, ...}` y hace `Post("/"+p.Source)` sobre `cgr_search` base `https://www.contraloria.cl/apibusca/search` (un solo resource base). Alternativa descartada: 8 resources idénticos en YAML (`cgr_search_dictamenes`, `cgr_search_contable`...) — duplica 8× timeout/retry/breaker y obliga a 8 `*resty.Client`; alternativa `source` como query param — el servicio solo acepta path segment.

**Decisión 2 — Fix `cgr_count` a `/count/todos` (elegida):**
Cambia `api.resources.yaml:cgr_count.path` de `/count/dictamenes` a `/count/todos`. Wire `cgrCountResponse.aggregations.count_by_type.buckets` ya espera 8 buckets; con `/count/dictamenes` solo venía 1. Mantiene shape `CountResponse{Total, Buckets}` sin breaking. Si `count` timeout/breaker, el handler devuelve error recuperable y el texto sugiere “usa search_cgr directo”.

**Decisión 3 — Timeouts por familia y retry diferenciado (elegida):**
En `api.resources.yaml` añadir 3 resources o parametrizar por source en el client:
- `cgr_search_light` 10s `retry 3×500ms max 5s` para `contable/instructivos/consolidados/cuentas` (55KB-946KB)
- `cgr_search_medium` 12s `retry 3×500ms` para `dictamenes/web` (263KB-1.3MB)
- `cgr_search_heavy` 15s `retry 2×1s max 4s` para `auditoria/legislacion` (3-5MB gzip, 9.5k hits)
O mantener 1 resource 10s y override per-request con `SetTimeout`/`SetRetryCount` según `Source` (más simple, evita 3 resources). Elección final: 1 resource base + override en `searchOnce` por `Source` (justifica excepción a “todo en YAML”).

**Decisión 4 — Wire types por tipo y sanitización dedicada (elegida):**
No reutilizar `cgrSource` dictamen para todo. Nuevos `_source` structs:
- `cgrInstructivoSource` (carácter, documento_completo, n_dictamen IN23)
- `cgrContableSource` (tipo, número, normativa_contable, parte, texto, texto_raw)
- `cgrAuditoriaSource` (número, nombre, objetivo, conclusiones, contenido_pdf, pdf)
- `cgrConsolidadoSource` (numero CIC21, nombre, resena, contenido_extraido, documento_cic_pdf_web)
- `cgrCuentaSource` (numero_sentencia, texto, pdf/pdf2)
- `cgrLegislacionSource` (tipo, número, organismo, materias, texto_)
Sanitización en `internal/cgr/sanitize.go`: `SanitizeAuditoria` stripea `<?xml`+`pdf:PDFVersion` header, `SanitizeContable` ignora `texto_raw`, `SanitizeLegislacion` igual; todos delegan a `normalize()` existente y `garbage.go` (añadir `&ensp;` etc. si falta). Wire interno, proyección pública tipada `InstructivoFull`/`ContableFull`/etc.

**Decisión 5 — Tools: `search_cgr` genérica + 6 `get_*` tipados + alias compat (elegida):**
- `search_cgr` con args `query, exact_search, order, page, source` (enum, default `dictamenes`). `search_cgr_dictamenes` queda como wrapper que llama `Search(Source:"dictamenes")` (no deprecated aún, solo alias).
- `get_cgr_instructivo(instructivo_id)`, `get_cgr_contable(contable_id)`, `get_cgr_auditoria(auditoria_id)`, `get_cgr_consolidado(consolidado_id)`, `get_cgr_cuenta(cuenta_id)`, `get_cgr_legislacion(legislacion_id)` cada uno con validación regex propia y `Post /search/{source} exact_search:true page:0`. Alternativa `get_cgr_document(id, type)` descartada — pierde validación y `outputSchema` se vuelve unión.
**Decisión 6 — Caching y límites (elegida):**
LRU `searches` key = `source:query|exact|order|page`; `counts` key = `query|exact`. No cachear `auditoria/legislacion page:0` si `ContentLength > 2MB` para no llenar LRU; TTL 5m para search, 10m para count/get. `get_auditoria` trunca `contenido_pdf` a 30k chars con aviso + link `pdf`.

**Decisión 7 — Prompts: reescribir `search_jurisprudence` genérico + 2 por familia sin saturar (elegida, camino B):**
`search_jurisprudence` (query requerido, source/order/exact_search/lang opcionales) con tabla `source` (dictamenes=caso, instructivos=IN general IN23, contable=NICSP OFE..., auditoria=Informe Final 10k, consolidados=CIC21, cuentas=sentencia, legislacion=AFECTO) y flujo `count?(opcional, si timeout→search)` → `search_cgr(source,order)` → `get_*` según bucket; hint `order=score si total>500 else date`, pacing 3-4s, `source` default `dictamenes` para compat. `analyze_contable_instructivo` (contable_id/instructivo_id/lang) para NICSP+IN y `analyze_auditoria_consolidado` (auditoria_id/consolidado_id/lang) para fiscalización+CIC; ambos con `MCP fuente verdad`, `url`/`pdf_url`/`documento_cic_pdf_web`, hedge y preamble ref a `interpret_dictamen`. `legislacion`/`cuentas` sin prompt propio en v1 (viven en genérico).

**Decisión 8 — `ToolNames()` 3→10 y `allowedPlaceholders` cerrado (elegida):**
`cgr/prompts.go` añade consts `toolSearchCgr`, `toolGetContable/Instructivo/Auditoria/Consolidado/Cuenta/Legislacion`, `ToolNames()` 10, `expectedPromptNames` 4→6, `allowedPlaceholders` con `source`/`order`/`exact_search`/`contable_id`/`instructivo_id`/`auditoria_id`/`consolidado_id`/`cuenta_id`/`legislacion_id`/`lang`, `//go:embed` bakeado, validación `missingkey=error` y `TestTemplatesReferenceOnlyRegisteredTools` por dominio.

## Risks / Trade-offs

- **SSRF via source path:** Mitigación: `validSource()` whitelist exacta `TrimSpace+ToLower` + rechazo `/ . %` y `len>20` **antes** de `Post("/"+source)`. Sin validación el path `../count/todos` alcanzaría otros endpoints.
- **Exhaustion via query/page:** Mitigación: `query` max 500 (trunc+aviso), `page` max 500, `ContentLength>6 MB` rechazado antes de `Unmarshal`, `cacheMax` por familia (light 100, heavy 20).
- **XSS via HTML remanente:** Mitigación: `Sanitize*` para **todos** los campos con `html.UnescapeString` + strip tags + `normalize()`, nunca exponer `texto_raw`/`_raw`/`old_url`/`172.30.x`.
- **Retry storm 429:** Mitigación: `retryConditions` incluye `429` con `Retry-After`, `attempts 2 backoff 1s` para pesados, `breaker failure 3` para pesados, pacing 3-4s en prompt.
- **PII en logs:** Mitigación: logger trunca `query` a 80 chars, nunca loguea `documento_completo`/`contenido_pdf`.
- **Template injection:** Mitigación: `{{.query}}` entre comillas, validación `allowedPlaceholders` cerrada, valor con `{{` se escapa.

## Migration Plan

1. Refactor `api.resources.yaml` (path `cgr_count` → `/count/todos`, timeouts por familia 10/12/15s, `retryConditions` con `429` + `Retry-After`, `breaker 3` para pesados) + `internal/cgr/cgr_client.go` (generic `Search`, `validSource` estricta, per-source timeout override, nuevos wire types, `validQuery`/`validPage`, `ContentLength` check, `old_url` filter, `hit._index` check).
2. Añadir `SanitizeAuditoria/Contable/Legislacion/Consolidado/Cuenta` con strip tags y `garbage.go`, y `truncate` para `contenido_pdf`.
3. Implementar `internal/tools/search_cgr.go` (genérica) + mantener `search_cgr_dictamenes.go` como wrapper + 6 `get_*` handlers con regex ancladas `len<=40` y validación `hit._index`.
4. Actualizar `internal/prompts/cgr/prompts.go|yaml|test.go`: reescribir `search_jurisprudence` genérico + 2 prompts por familia, `expectedPromptNames` 4→6, `ToolNames()` 3→10, `allowedPlaceholders` con `source`, escape de `{{.query}}`.
5. Tests: `httptest` fixtures por source, `TestSearch_InvalidSource_NoNetwork`, `TestQueryTruncated`, `TestPageOverflow`, `TestResponseTooLarge`, `TestNoInternalURLLeak`, `TestSanitize_StripsTags`, `TestID_Validation_LenAndRegex`, `Test429_RetryAfter`, `TestPromptInjection_QueryWithBraces`, `TestHitIndex_Mismatch_NotFound`.
6. `make check` + `make fmt-check` + `openspec validate --strict` + `tools/list` 14 y `prompts/list` 16 (10 BCN + 6 CGR); verificar pacing y `count` opcional.
