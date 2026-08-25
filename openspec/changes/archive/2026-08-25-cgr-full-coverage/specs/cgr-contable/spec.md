## Purpose

Ficha y búsqueda de oficios de normativa contable NICSP-CGR Chile (sector municipal) — pronunciamientos sobre contabilización de transferencias, donaciones, EEFF municipales — expuesta como tools MCP LLM-first con `normativa_contable` y citación.

## ADDED Requirements

### Requirement: Búsqueda contable

El servidor DEBE exponer búsqueda contable vía `search_cgr` con `source:"contable"`. Cada resultado DEBE incluir `doc_id`, `número` (ej. `E080961`), `normativa_contable` (ej. `OFE0809612600`), `tipo` (`Oficio`), `parte` (resumen), `destinatarios`, `fecha_documento` y `origen`.

#### Scenario: Búsqueda municipalidad en contable
- **WHEN** `query:"municipalidad"`, `source:"contable"`
- **THEN** incluye `total:622` y `E080961` (“Municipalidad de La Pintana deberá dar cumplimiento al oficio N°24.377...”)

#### Scenario: Búsqueda donación hospital
- **WHEN** `query:"donación Hospital La Paz Limache"`, `source:"contable"`
- **THEN** incluye `E509321` (`OFE5093212400`) sobre contabilización de donación Embajada Japón

### Requirement: Ficha contable por identificador

El servidor DEBE exponer `get_cgr_contable` con `contable_id` (requerido, acepta `E080961` o `OFE0809612600` o `E080961N26`-like; normaliza a `doc_id`). Consulta `POST /search/contable` con `exact_search:true` y si `total==1` devuelve `_source` con `texto` sanitizado, `texto_raw` descartado, `parte`, `tipo`, `número`, `normativa_contable`, `destinatarios`, `origen`, `fecha_documento`, `char_count`.

#### Scenario: Contable existente
- **WHEN** `contable_id:"E509321"`
- **THEN** devuelve `texto` con “Ampliación Unidad Fonoaudiología Hospital Geriátrico La Paz de la Tarde en Limache” y `normativa_contable:"OFE5093212400"`

#### Scenario: Contable inexistente
- **WHEN** `contable_id:"E999999"`
- **THEN** error “contable no encontrado”

#### Scenario: LLM-first dual output
- **WHEN** la ficha tiene éxito
- **THEN** texto con `parte` como lead + `## Texto` + `char_count` y `structuredContent` tipado

### Requirement: Sanitización contable y no leak

`texto` y `parte`/`destinatarios`/`origen` DEBEN sanitizarse con `html.UnescapeString` + strip de tags `<[^>]+>` + `normalize()` (espacios `&nbsp;`, `&ensp;`, zero-width, control C0 colapsados). `texto_raw`, `old_url` y `172.30.x` nunca se exponen. `contable_id` DEBE validarse `len<=40` antes de regex `^(E[0-9]{1,6}|OFE[0-9]{10,13}|E[0-9]+N[0-9]{2})$` con `TrimSpace+ToUpper`. `ContentLength>2 MB` se rechaza.

#### Scenario: HTML strip en parte y destinatarios
- **WHEN** `_source` trae `parte` con `<font>` o `destinatarios` con `<td>`
- **THEN** la ficha devuelve texto plano sin tags

#### Scenario: No leak old_url
- **WHEN** `_source` trae `old_url:"http://172.30.21.160/..."`
- **THEN** no se incluye en structured ni texto

#### Scenario: ID con control chars rechazado
- **WHEN** `contable_id:"E080961\n"`
- **THEN** error `invalid contable_id` sin I/O
