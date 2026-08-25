## MODIFIED Requirements

### Requirement: Búsqueda paginada multi-source de Contraloría

El servidor DEBE exponer una tool `search_cgr` genérica que consulta `POST /apibusca/search/{source}` con `query` (requerido, vacío permitido), `exact_search` (bool, default false), `order` (`date` DESC default, `dateasc`, `score`), `page` (1-indexed, default 1) y `source` (enum `dictamenes|instructivos|contable|auditoria|legislacion|cuentas|consolidados|web|todos`, default `dictamenes` para compat). La tool DEBE mantener compatibilidad con `search_cgr_dictamenes` como alias delgado que fija `source=dictamenes`. Para cualquier `source`, DEBE mapear `page` a 0-index del servicio, fijar `date_name:"fecha_documento"` y `options:[]`, y devolver `pagination{total, page, page_size:20, total_pages, has_more}` con `has_more = page*20 < total`. `total` DEBE leerse de `hits.total.value`; cuando `relation:"gte"` y `value==10000` el texto DEBE advertir “más de 10.000 resultados, refina tu búsqueda o usa order=score”. Ante `order` inválido o `page <=0` o `source` no whitelisteado DEBE devolver error de argumentos sin consultar el servicio.

#### Scenario: Búsqueda genérica en contable por municipalidad
- **WHEN** un cliente llama a `search_cgr` con `query:"municipalidad"`, `source:"contable"`, `page:1`
- **THEN** la respuesta incluye hasta 20 oficios contables, `total:622`, `page_size:20`, `total_pages:32`, `has_more:true`, cada resultado con `tipo`, `número`, `normativa_contable`, `parte` y `destinatarios`

#### Scenario: Búsqueda en instructivos por toma de razón
- **WHEN** un cliente llama con `query:"toma de razón"`, `source:"instructivos"`
- **THEN** la respuesta incluye `total:13` e incluye `IN E462387` (resolución exenta 2024) con `carácter` y `documento_completo`

#### Scenario: Búsqueda en consolidados por licencias médicas
- **WHEN** un cliente llama con `query:"licencias médicas"`, `source:"consolidados"`
- **THEN** la respuesta incluye `total:8` con `CIC21/2026` como primer resultado, con `numero`, `nombre`, `resena` y `documento_cic_pdf_web`

#### Scenario: Alias dictamenes mantiene compatibilidad
- **WHEN** un cliente llama a `search_cgr_dictamenes` con `query:"quillota"` (tool legacy)
- **THEN** la respuesta es idéntica a `search_cgr` con `source:"dictamenes"` y `total:312`, sin breaking change

#### Scenario: Ordenamiento por score para 10k
- **WHEN** un cliente busca `query:"municipalidad"`, `source:"auditoria"` con `order:"score"`
- **THEN** los resultados vienen ordenados por `_score` DESC y el texto advierte si `total` es `gte 10000`

#### Scenario: LLM-first dual output para cualquier source
- **WHEN** la búsqueda tiene éxito
- **THEN** la respuesta incluye `Content` texto con lista numerada (materia/parte/nombre, fecha, id para drill-down y URLs de citación según source) y `structuredContent` tipado con los mismos datos completos

### Requirement: Transporte resiliente por familia y pacing

La búsqueda DEBE usar resource `cgr_search` base `https://www.contraloria.cl` con path `/apibusca/search/{source}` (construido en el client) y headers `Accept: application/json`, `Content-Type: application/json`, `Origin: https://www.contraloria.cl`. Timeouts por familia: `contable/instructivos/consolidados/cuentas` 10s, `dictamenes/web` 12s, `auditoria/legislacion` 15s; retry `3×500ms` para livianos y `2×1s` para pesados (5xx, timeout, status 0). Ante breaker abierto DEBE fallar rápido. El prompt asociado DEBE documentar pacing de 3-4s entre requests y que `count` es opcional bajo carga (si timeout, pasar directo a `search`).

#### Scenario: Retry en legislacion bajo carga
- **WHEN** `POST /search/legislacion` con `query:"toma de razón"` responde timeout en el primer intento
- **THEN** el cliente reintenta una vez con backoff 1s y retorna éxito si el segundo intento responde 200

#### Scenario: Validación de source
- **WHEN** `source:"invalido"`
- **THEN** la tool devuelve error de argumentos sin consultar el servicio

### Requirement: Límites y validación de seguridad para búsqueda

La tool DEBE validar antes de I/O: `query` max 500 chars (si excede, truncar a 500 con aviso en texto, no error), `page` max 500 (si `page>500` error `page must be <= 500`), `order` enum cerrado `date|dateasc|score`, `source` whitelist exacta case-insensitive `dictamenes|instructivos|contable|auditoria|legislacion|cuentas|consolidados|web|todos` con `TrimSpace+ToLower` y rechazo si contiene `/ . % ? #` o `len>20` o `../`. `ContentLength` de respuesta `>6 MB` DEBE rechazarse antes de `Unmarshal` con error `response too large`. `old_url` y cualquier `http://172.30.x` nunca DEBE exponerse en `structuredContent` ni texto. Logging DEBE truncar `query` a 80 chars y nunca loguear `documento_completo`/`contenido_pdf`.

#### Scenario: Query larga truncada
- **WHEN** un cliente llama con `query` de 800 chars
- **THEN** la tool trunca a 500, incluye aviso `query truncated to 500` en texto y consulta con 500 chars

#### Scenario: Page fuera de rango
- **WHEN** `page:9999`
- **THEN** la tool devuelve error `page must be <= 500` sin consultar el servicio

#### Scenario: Source con path traversal
- **WHEN** `source:"../count/todos"` o `source:"web%2fadmin"`
- **THEN** la tool devuelve error `invalid source` sin I/O, sin concatenar al path

#### Scenario: Response demasiado grande
- **WHEN** `POST /search/auditoria` responde `ContentLength: 8 MB`
- **THEN** la tool devuelve error `response too large` sin `Unmarshal`, con retry si aplica, y no cachea

#### Scenario: No leak de old_url interna
- **WHEN** `_source` contiene `old_url:"http://172.30.21.160/..."`
- **THEN** el resultado no incluye `old_url` en `structuredContent` ni texto, solo `url`/`pdf_url` canónicas
