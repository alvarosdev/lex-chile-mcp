## Purpose

Búsqueda y ficha de Consolidados de Información Circularizada (CIC) — reportes de la División de Fiscalización que cruzan datos del Estado (ej. CIC21/2026 licencias médicas con honorarios) — con `resena` y `contenido_extraido` LLM-ready y PDF oficial.

## ADDED Requirements

### Requirement: Búsqueda de consolidados

El servidor DEBE exponer búsqueda de consolidados vía `search_cgr` con `source:"consolidados"`. Cada resultado DEBE incluir `numero` (ej. `CIC21/2026`), `nombre`, `tipo`/`cra_tipo`, `resena`, `fecha_documento`, `unidad_cgr`, `sector` y `documento_cic_pdf_web`.

#### Scenario: Licencias médicas devuelve CIC21
- **WHEN** `query:"licencias médicas"`, `source:"consolidados"`
- **THEN** incluye `total:8` con `CIC21/2026` `resena` sobre funcionarios con licencia que recibieron honorarios

#### Scenario: Municipalidad en consolidados
- **WHEN** `query:"municipalidad"`, `source:"consolidados"`
- **THEN** incluye `total:11` con CIC de licencias y otros

### Requirement: Ficha de consolidado por identificador

El servidor DEBE exponer `get_cgr_consolidado` con `consolidado_id` (requerido, `^CIC[0-9]+/[0-9]{4}$` ej. `CIC21/2026`). Consulta `POST /search/consolidados` `exact_search:true` y si `total==1` devuelve `numero`, `nombre`, `tipo`, `resena`, `contenido_extraido` sanitizado, `fecha_documento`, `documento_cic_pdf_web`, `unidad_cgr`, `char_count`.
#### Scenario: LLM-first dual output
- **WHEN** la ficha tiene éxito
- **THEN** texto con `resena` como lead + `## Contenido` extraído + link PDF y `structuredContent` tipado

### Requirement: Sanitización y validación de consolidado

`resena` y `contenido_extraido`/`nombre` DEBEN sanitizarse con `html.UnescapeString` + strip tags + `normalize()`. `old_url`/`172.30.x` nunca se expone. `consolidado_id` `len<=20` y regex `^CIC[0-9]{1,4}/[0-9]{4}$` con `TrimSpace+ToUpper`; si contiene control chars → error sin I/O. Validar `hit._index=="cgr-consolidados"`. `ContentLength>2 MB` se rechaza.

#### Scenario: No leak old_url
- **WHEN** `_source` trae `old_url:"http://172.30.21.160/..."`
- **THEN** no se incluye en structured ni texto
