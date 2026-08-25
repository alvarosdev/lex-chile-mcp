## Purpose

Ficha y búsqueda de instrucciones generales de Contraloría (IN) — criterio vinculante de alcance general que imparte instrucciones a la Administración (ej. IN23 día funcionario municipal 2026, E462387 exención toma de razón 2024) — expuesta como tools MCP LLM-first con citación.

## ADDED Requirements

### Requirement: Búsqueda de instructivos

El servidor DEBE exponer búsqueda de instructivos vía `search_cgr` con `source:"instructivos"` (ver `cgr-search`) y además permitir `search_cgr` con `query` libre. La búsqueda usa `POST /search/instructivos` con los mismos args `query`/`exact_search`/`order`/`page`. Cada resultado DEBE incluir `doc_id`/`dictamen_id`, `n_dictamen` (ej. `IN23`), `fecha_documento`, `materia` y `carácter`.

#### Scenario: Búsqueda municipal devuelve IN23
- **WHEN** un cliente llama a `search_cgr` con `query:"municipalidad"`, `source:"instructivos"`
- **THEN** incluye `total:18` e `IN23` con `carácter:NNN` y materia sobre día funcionario municipal

### Requirement: Ficha de instructivo por identificador

El servidor DEBE exponer `get_cgr_instructivo` con `instructivo_id` (requerido, regex `^[A-Z]*[0-9]+N[0-9]{2}$` ej. `IN23N26`, `E462387N24`) que consulta `POST /search/instructivos` con `{search: instructivo_id, exact_search:true, source:"instructivos", page:0}` y si `total==1` devuelve `_source` con `documento_completo` sanitizado, `materia`, `descriptores`, `carácter`, `fecha_documento` y URLs `url`/`pdf_url` canónicas.

#### Scenario: Instructivo existente toma de razón
- **WHEN** un cliente llama a `get_cgr_instructivo` con `instructivo_id:"E462387N24"`
- **THEN** devuelve `documento_completo` que contiene “resolución N°1/2024 exenta de toma de razón” con `char_count` y citación

#### Scenario: Instructivo inexistente
- **WHEN** `instructivo_id:"IN999999N99"` no existe
- **THEN** devuelve error “instructivo no encontrado” sin contenido

#### Scenario: LLM-first dual output
- **WHEN** la ficha tiene éxito
- **THEN** incluye texto con header de metadatos + `## Documento` y `structuredContent` tipado idéntico

### Requirement: Sanitización instructivo y validación de ID

`documento_completo` y `materia`/`descriptores` DEBEN sanitizarse con `html.UnescapeString` + strip tags `<[^>]+>` + `normalize()`. `documento_completo_raw`/`old_url`/`172.30.x` nunca se exponen. `instructivo_id` DEBE validarse `len<=40` y `TrimSpace+ToUpper` antes de regex `^[A-Z]{0,3}[0-9]{1,6}N[0-9]{2}$` (ej. `IN23N26`, `E462387N24`); si contiene `/ . %` o control chars → error sin I/O. Validar `hit._index=="cgr-instructivos"` antes de proyectar.

#### Scenario: Sanitización de &nbsp; y tags al inicio de líneas
- **WHEN** `documento_completo` contiene `\u00A0` o `<td>` al inicio
- **THEN** se entrega colapsado sin indentación ni tags

#### Scenario: ID con slash rechazado
- **WHEN** `instructivo_id:"IN23/../count"`
- **THEN** error `invalid instructivo_id` sin I/O

#### Scenario: Hit de otro índice rechazado
- **WHEN** `POST /search/instructivos` con `exact_search:true` devuelve `_index:"cgr-dictamenes"`
- **THEN** la tool trata como `not found` (no proyecta cross-index)
