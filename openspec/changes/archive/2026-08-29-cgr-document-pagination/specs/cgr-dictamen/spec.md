## MODIFIED Requirements

### Requirement: Ficha de dictamen por identificador

El servidor DEBE exponer una tool `get_cgr_dictamen` que obtiene la ficha de un
dictamen por `dictamen_id` (string requerido, formato `^[A-Z]*[0-9]+N[0-9]{2}$`
ej. `E179593N25`, `OF80660N26`). La tool DEBE consultar
`POST /apibusca/search/dictamenes` con `{search: dictamen_id, exact_search:true,
options:[], order:"date", date_name:"fecha_documento", source:"dictamenes",
page:0}` y, si `hits.total.value==1`, devolver el `_source` del hit. Si
`total==0` DEBE devolver error "dictamen no encontrado" sin contenido.

La tool DEBE aceptar un parámetro opcional `part` (entero, por defecto 1) que
selecciona la página lineal del documento conforme a la capability
`output-budget`: partes de a lo más el presupuesto con cortes en límite de
párrafo, señal de rango en cada parte y `char_count` del documento completo. Los
metadatos de la ficha viajan en la parte 1; las partes siguientes repiten solo el
encabezado de rango. Un `part` menor que 1 o mayor que el total de partes DEBE
devolver error de argumentos indicando el rango válido.

#### Scenario: Dictamen existente
- **WHEN** un cliente llama a `get_cgr_dictamen` con `dictamen_id:"E179593N25"`
- **THEN** la respuesta incluye `dictamen_id`, `n_dictamen`, `fecha_documento`, `materia`, `descriptores`, `criterio`, `origen_`, `destinatarios`, `abogados`, `fuentes_legales`, `carácter`, `documento_completo` del dictamen y las URLs canónicas `url` (`https://www.contraloria.cl/buscadorpdf/dictamenes/{id}/html`) y `pdf_url` (`https://www.contraloria.cl/buscadorpdf/dictamenes/{id}/pdf`) para citación y descarga

#### Scenario: Dictamen dentro del presupuesto llega completo
- **WHEN** un cliente llama a `get_cgr_dictamen` con un dictamen cuyo documento sanitizado cabe en el presupuesto
- **THEN** la parte 1 entrega el documento completo y la señal indica `part 1 of 1`

#### Scenario: Dictamen largo se pagina con señal de rango
- **WHEN** un cliente llama a `get_cgr_dictamen` con un dictamen cuyo documento excede el presupuesto
- **THEN** la parte 1 entrega los primeros ~presupuesto chars con corte en límite de párrafo
- **AND** la respuesta señala `part 1 of N · chars 1–X of Y` e indica continuar con `part=2`
- **AND** `char_count` describe el documento completo

#### Scenario: Parte siguiente continúa donde terminó la anterior
- **WHEN** un cliente llama con `part: 2` del mismo dictamen
- **THEN** la respuesta contiene el siguiente segmento desde el corte de párrafo de la parte 1, sin repetir metadatos ni solapar contenido

#### Scenario: Parte fuera de rango
- **WHEN** un cliente llama con `part` menor que 1 o mayor que el total de partes
- **THEN** la tool devuelve un error de argumentos indicando el rango válido, sin consultar el servicio de nuevo

#### Scenario: Dictamen inexistente
- **WHEN** un cliente llama con `dictamen_id:"E999999N99"` que no existe
- **THEN** la tool devuelve un error de "dictamen no encontrado" sin contenido, sin error de protocolo

#### Scenario: Identificador faltante o malformado
- **WHEN** un cliente llama sin `dictamen_id` o con `dictamen_id:""`
- **THEN** la tool devuelve error de argumentos sin consultar el servicio

#### Scenario: LLM-first dual output
- **WHEN** la ficha se obtiene con éxito
- **THEN** la respuesta incluye texto formateado (header de metadatos + `## Documento Completo` con la parte correspondiente y la señal de rango) y `structuredContent` tipado con los mismos campos y alcance
