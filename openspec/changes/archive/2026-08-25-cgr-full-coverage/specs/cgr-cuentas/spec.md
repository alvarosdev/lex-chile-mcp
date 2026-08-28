## Purpose

Búsqueda y ficha de sentencias del Juzgado de Cuentas — juicios por reparos a funcionarios (ej. alcaldes) con `texto` de sentencia y PDFs de ejecutoria — expuesta como tools MCP LLM-first con citación.

## ADDED Requirements

### Requirement: Búsqueda de cuentas

El servidor DEBE exponer búsqueda de cuentas vía `search_cgr` con `source:"cuentas"`. Cada resultado DEBE incluir `doc_id`/`numeric_doc_id`, `numero_sentencia`, `numero_expediente`, `texto` (extracto), `fecha_sentencia`/`fecha_documento`, `pdf`/`pdf2` y `year_doc_id`.

#### Scenario: Municipalidad en cuentas
- **WHEN** `query:"municipalidad"`, `source:"cuentas"`
- **THEN** incluye `total:645` y `2982331 JC 39778 13 MAR 2026` contra alcalde Valparaíso

#### Scenario: Quillota en cuentas
- **WHEN** `query:"Quillota"`, `source:"cuentas"`
- **THEN** incluye `total:14` con sentencias del Juzgado de Cuentas

### Requirement: Ficha de cuenta por identificador

El servidor DEBE exponer `get_cgr_cuenta` con `cuenta_id` (requerido, `^[0-9]+$` ej. `2982331` o `89778` o `82331`). Consulta `POST /search/cuentas` `exact_search:true` y si `total==1` devuelve `doc_id`, `numeric_doc_id`, `numero_sentencia`, `numero_expediente`, `texto` sanitizado, `fecha_sentencia`, `fecha_expediente`, `pdf`/`pdf2`, `char_count`.
#### Scenario: LLM-first dual output
- **WHEN** la ficha tiene éxito
- **THEN** texto con header de números + `## Texto` y `structuredContent` tipado con `pdf` para citación

### Requirement: Sanitización y validación de cuenta

`texto` y `numero_sentencia`/`numero_expediente` DEBEN sanitizarse con `html.UnescapeString` + strip tags + `normalize()`. `old_url`/`172.30.x` nunca se expone. `cuenta_id` `len<=20` y regex `^[0-9]{1,6}$` con `TrimSpace`; rechazar si contiene `/ . %` o control chars. Validar `hit._index=="cgr-cuentas"`. `ContentLength>2 MB` se rechaza.

#### Scenario: ID con slash rechazado
- **WHEN** `cuenta_id:"2982331/../count"`
- **THEN** error `invalid cuenta_id` sin I/O
