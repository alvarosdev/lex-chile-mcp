## Purpose

Búsqueda y ficha de legislación con toma de razón de Contraloría (decretos, DFL, resoluciones afectas) — puente con LeyChile — con `texto_`, `materias` y `organismo`, para consultas por “toma de razón” y control de legalidad.

## ADDED Requirements

### Requirement: Búsqueda de legislación

El servidor DEBE exponer búsqueda de legislación vía `search_cgr` con `source:"legislacion"`. Cada resultado DEBE incluir `doc_id`/`numeric_doc_id`, `tipo` (ej. `DECRETO`, `DECRETO CON FUERZA DE LEY`, `RESOLUCION`), `número`, `organismo`, `materias`, `caracter` (`AFECTO`), `fecha_documento`/`fecha_publicación`.

#### Scenario: Toma de razón en legislacion
- **WHEN** `query:"toma de razón"`, `source:"legislacion"`
- **THEN** incluye `total:1558` y `RESOLUCION 569 CONTR — Somete a toma de razón decretos materia personal` con `caracter:AFECTO`

#### Scenario: Municipalidad en legislacion
- **WHEN** `query:"municipalidad"`, `source:"legislacion"`
- **THEN** incluye `total:9557` con `DECRETO 1739 Municipalidades` y aviso cap `gte` si aplica

### Requirement: Ficha de legislación por identificador

El servidor DEBE exponer `get_cgr_legislacion` con `legislacion_id` (requerido, `^(RZA|FRA|...)[0-9]+` ej. `RZA005691400`, `FRA000012000` o `número` con `tipo`). Consulta `POST /search/legislacion` `exact_search:true` y si `total==1` devuelve `tipo`, `número`, `organismo`, `materias`, `texto_` sanitizado, `fecha_promulgación`/`fecha_publicación`, `dl_asociada`, `caracter`, `char_count`.

#### Scenario: Legislación existente DFL
- **WHEN** `legislacion_id:"FRA000012000"`
- **THEN** devuelve `DFL 1/2020 SEPRE — Normas aplicación art 1 ley 21.180 Transformación Digital` con `texto_` completo

#### Scenario: Legislación inexistente
- **WHEN** `legislacion_id:"RZA999999999"`
#### Scenario: LLM-first dual output
- **WHEN** la ficha tiene éxito
- **THEN** texto con `tipo`+`número`+`organismo`+`materias` + `## Texto` y `structuredContent` tipado

### Requirement: Sanitización y validación de legislación

`texto_`/`materias`/`organismo` DEBEN sanitizarse con `html.UnescapeString` + strip tags + `normalize()`. `texto__raw`/`old_url`/`172.30.x` nunca se expone. `legislacion_id` `len<=40` y regex `^(RZA|FRA)[0-9]{9,12}$` con `TrimSpace+ToUpper`; rechazar control chars. Validar `hit._index=="cgr-legislacion"`. `ContentLength>6 MB` se rechaza antes de `Unmarshal`.

#### Scenario: No leak texto__raw
- **WHEN** `_source` trae `texto__raw:"<div><font>..."` y `old_url:"http://172.30.21.160/..."`
- **THEN** solo `texto_` sanitizado se expone, sin `texto__raw` ni `old_url`
