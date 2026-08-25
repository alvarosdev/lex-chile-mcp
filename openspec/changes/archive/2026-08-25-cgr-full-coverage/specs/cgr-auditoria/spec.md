## Purpose

Búsqueda y ficha de informes de auditoría y fiscalización de Contraloría (Informes Finales, Seguimientos, Investigaciones Especiales) — expone `auditoria` con `pdf` y `contenido_pdf` extraído, para consultas por servicio/unidad y cap 10k manejable.

## ADDED Requirements

### Requirement: Búsqueda de auditoría

El servidor DEBE exponer búsqueda de auditoría vía `search_cgr` con `source:"auditoria"`. Cada resultado DEBE incluir `doc_id`/`numeric_doc_id`, `número` (ej. `371/2026`), `nombre`, `tipo`, `objetivo`, `unidad_cgr`, `región`/`sector_regional`, `fecha_documento` y `pdf`.

#### Scenario: Búsqueda Quillota en auditoria
- **WHEN** `query:"Quillota"`, `source:"auditoria"`
- **THEN** incluye `total:717` y `371/2026 INFORME FINAL JUZGADO DE GARANTÍA DE QUILLOTA...` con `tipo:Informe Final de Fiscalización de Tribunales`

#### Scenario: Búsqueda licencias médicas en auditoria
- **WHEN** `query:"licencias médicas"`, `source:"auditoria"`
- **THEN** incluye `total:2831` con informes de Investigación Especial (Carabineros 540/2025)

### Requirement: Ficha de auditoría por identificador

El servidor DEBE exponer `get_cgr_auditoria` con `auditoria_id` (requerido, acepta `371/2026` o `371N26` o `numeric_doc_id`; normaliza). Consulta `POST /search/auditoria` `exact_search:true` y si `total==1` devuelve `nombre`, `número`, `tipo`, `objetivo`, `conclusiones`, `universo`, `muestra`, `destinatarios`, `servicio_`, `unidad_cgr`, `fecha_documento`, `pdf`, `contenido_pdf` sanitizado (truncado si supera límite) y `char_count`.

#### Scenario: Auditoría existente
- **WHEN** `auditoria_id:"371/2026"`
- **THEN** devuelve `objetivo` sobre cuenta corriente Banco Estado Juzgado Quillota y `conclusiones` “corresponde aprobar el movimiento...”

#### Scenario: Auditoría con contenido_pdf grande
- **WHEN** `contenido_pdf` supera 30k chars (XML extraído del PDF)
- **THEN** la ficha devuelve texto sanitizado truncado con aviso “contenido truncado, ver PDF oficial” y `char_count` del total original, más `pdf` para descarga

#### Scenario: LLM-first dual output
- **WHEN** la ficha tiene éxito
- **THEN** texto con `objetivo`+`conclusiones` como lead + `## Contenido` y `structuredContent` tipado

### Requirement: Sanitización auditoría y límites y no leak

`contenido_pdf` y `objetivo`/`conclusiones`/`destinatarios`/`universo`/`muestra` DEBEN sanitizarse con `html.UnescapeString` + strip tags + `normalize()`. `<?xml` + `pdf:PDFVersion` header se stripa antes de `normalize`. `destinatarios` `<ul><li>` se convierte a lista texto plano con saltos. `old_url`/`172.30.x` nunca se expone. `auditoria_id` `len<=40` y regex `^([0-9]{1,4}/[0-9]{4}|[A-Z]*[0-9]+N[0-9]{2}|[0-9]{1,6})$` con `TrimSpace`. Si `ContentLength>6 MB` se rechaza antes de `Unmarshal`. `hit._index` debe ser `cgr-auditoria`.

#### Scenario: Strip XML header y tags
- **WHEN** `contenido_pdf` inicia con `<?xml version="1.0"...><html xmlns=` y contiene `<meta>`
- **THEN** la ficha no incluye el header ni tags, solo el texto del informe sanitizado

#### Scenario: Destinatarios HTML a lista plana
- **WHEN** `destinatarios:"<ul><li>PRESIDENTA...</li></ul>"`
- **THEN** se entrega como líneas con `"- PRESIDENTA..."`
### Requirement: Transporte específico auditoría y retry 429

Auditoría usa `POST /search/auditoria` con timeout 15s y retry `2×1s max 4s` con `Retry-After` para `429`. `retryConditions` DEBE incluir `429` además de `5xx`/`0`. `search_cgr` con `source:"auditoria"` y `relation:gte` DEBE advertir “más de 10k, usa order=score o refina por servicio”.

#### Scenario: 429 con Retry-After
- **WHEN** `POST /search/auditoria` responde `429 Retry-After: 2`
- **THEN** el cliente espera 2s y reintenta una vez; si persiste, devuelve error con `retry after` sin amplificar

#### Scenario: Cap 10k con aviso
- **WHEN** `query:"municipalidad"` retorna `total:{value:10000, relation:"gte"}`
- **THEN** el texto incluye aviso y `structured.total:10000`
