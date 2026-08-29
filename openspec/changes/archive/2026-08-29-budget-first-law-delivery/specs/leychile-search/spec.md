## MODIFIED Requirements

### Requirement: Contenido de norma por identificador

El servidor DEBE exponer una tool `get_law` que consulta el recurso de contenido de
LeyChile con el identificador de norma (requerido, mapeado al parámetro `idNorma`
del servicio) y devuelve la norma estructurada: metadatos seleccionados, la
estructura de la norma, los proyectos de ley relacionados y el contenido. La tool
DEBE aceptar `structure_only` (omite el contenido) y `section_id` opcional.

Sujeto al presupuesto de salida (`output-budget`): cuando el contenido solicitado
(norma completa o sección) excede el presupuesto, la tool DEBE degradar la
respuesta a un mapa navegable con la estructura plegada (identificadores y
tamaños por nodo, señal del total real) y NO DEBE entregar el contenido truncado.
La degradación aplica de forma recursiva a una sección que por sí sola excede el
presupuesto. `structure_only=true` DEBE usar el mismo TOC plegado (nunca la lista
completa de artículos de una meganorma).

En respuestas de sección (`section_id`), el texto DEBE listar el sub-TOC local —
los hijos de la sección con sus `section_id` y tamaños — y NO el índice global de
la norma; el índice global solo viaja en `get_law_summary` y `structure_only`. El
`structuredContent` de una respuesta de sección DEBE acotar `estructura` al
subárbol de la sección (la estructura completa aplanada solo en summary y
`structure_only`), y el markdown del contenido NO DEBE duplicarse entre el texto
y el `structuredContent`.

#### Scenario: Resultados en formato LLM-first con estructura opcional
- **WHEN** un cliente llama a `get_law` con éxito
- **THEN** el resultado incluye contenido de texto formateado para lectura del modelo (metadatos, estructura y contenido en Markdown)
- **AND** incluye contenido estructurado tipado (`structuredContent`) con los mismos campos; el campo de contenido se omite cuando `structure_only` es verdadero o el contenido excede el presupuesto

#### Scenario: Norma válida con contenido
- **WHEN** un cliente llama a `get_law` con `norm_id: 1226950`
- **THEN** la respuesta incluye los metadatos de la norma, su estructura, los proyectos relacionados y el contenido completo de la norma

#### Scenario: Norma que excede el presupuesto se degrada a mapa
- **WHEN** un cliente llama a `get_law` sin `section_id` con el `norm_id` de una norma cuyo contenido excede el presupuesto (p.ej. un código refundido de ~1.2M chars)
- **THEN** la respuesta incluye metadatos, proyectos y el TOC plegado con `section_id` y tamaños por nodo
- **AND** incluye una señal explícita del total real y la instrucción de profundizar con `section_id`
- **AND** no incluye un cuerpo de contenido parcial

#### Scenario: Sección que excede el presupuesto se degrada recursivamente
- **WHEN** un cliente llama a `get_law` con un `section_id` cuya sección excede por sí sola el presupuesto
- **THEN** la respuesta mapea los hijos de esa sección con sus `section_id` y tamaños, con la señal del total real de la sección

#### Scenario: Solo estructura
- **WHEN** un cliente llama a `get_law` con `norm_id: 1226950` y `structure_only: true`
- **THEN** la respuesta incluye metadatos, estructura plegada y proyectos pero omite el contenido

#### Scenario: Sección específica
- **WHEN** un cliente llama a `get_law` con `norm_id` de una norma larga y un `section_id` que corresponde a un título de su estructura
- **THEN** la respuesta incluye los metadatos y limita el contenido a los bloques de esa sección y sus descendientes, omitiendo el resto de la norma
- **AND** el texto de la respuesta indica qué sección se muestra (p.ej. "Section: Título III")
- **AND** el texto lista los hijos de la sección con sus `section_id` y tamaños, sin el índice global
- **AND** el `structuredContent` acota `estructura` al subárbol de la sección

#### Scenario: Sección inexistente
- **WHEN** un cliente llama a `get_law` con un `section_id` que no corresponde a ninguna parte de la estructura de la norma
- **THEN** la tool devuelve un error de argumentos indicando que la sección no existe, sin devolver contenido

#### Scenario: Identificador inexistente
- **WHEN** un cliente llama a `get_law` con un `norm_id` que no existe en LeyChile
- **THEN** la tool devuelve un error de "norma no encontrada" sin contenido

#### Scenario: Identificador faltante
- **WHEN** un cliente llama a `get_law` sin `norm_id`
- **THEN** la tool devuelve un error de argumentos sin consultar el servicio

### Requirement: Resumen de norma por identificador

El servidor DEBE exponer una tool `get_law_summary` que consulta el mismo recurso
de contenido de LeyChile que `get_law` y devuelve una versión liviana de la norma:
`titulo_norma`, `fuente`, `materias`, `categorias_norma`, `resumenes`
(sanitizados) y la `estructura` plegada. La estructura DEBE renderizarse con el
TOC plegado adaptativo: nodos con hasta ~20 hijos listan cada hijo con su
`section_id` y tamaño; nodos con más hijos colapsan en una línea por contenedor
con `nombre | section_id | rango primero–último (textual) | ~chars | N artículos`.
Los rangos DEBEN ser textuales (primero–último), no numéricos, porque existen
etiquetas como `Artículo 58 BIS`, `TRANSITORIO` y `FINAL`. La tool NO DEBE incluir
el contenido de la norma.

#### Scenario: Resumen válido
- **WHEN** un cliente llama a `get_law_summary` con `norm_id: 1142880`
- **THEN** la respuesta incluye el título de la norma, la fuente, las materias, las categorías de norma, los resúmenes sanitizados y la estructura con los identificadores de cada parte listada
- **AND** no incluye el contenido de la norma

#### Scenario: Estructura plegada en meganormas
- **WHEN** un cliente llama a `get_law_summary` con el `norm_id` de una norma con miles de artículos (p.ej. un código refundido)
- **THEN** la estructura renderizada mantiene el orden del documento listando los contenedores con sus rangos de artículos y tamaños
- **AND** el tamaño de la respuesta de texto se mantiene en el orden de pocos miles de tokens (no la lista completa de artículos)

#### Scenario: Identificador inexistente
- **WHEN** un cliente llama a `get_law_summary` con un `norm_id` que no existe en LeyChile
- **THEN** la tool devuelve un error de "norma no encontrada" sin contenido

#### Scenario: Identificador faltante
- **WHEN** un cliente llama a `get_law_summary` sin `norm_id`
- **THEN** la tool devuelve un error de argumentos sin consultar el servicio

#### Scenario: Resultados en formato LLM-first con estructura opcional
- **WHEN** un cliente llama a `get_law_summary` con éxito
- **THEN** el resultado incluye contenido de texto formateado para lectura del modelo
- **AND** incluye contenido estructurado tipado (`structuredContent`) con los mismos campos

### Requirement: Búsqueda paginada de normas

El servidor DEBE exponer una tool `search_laws` que consulta el endpoint de
búsqueda de LeyChile con los parámetros `query` (texto a buscar, requerido),
`page` (número de página, por defecto 1) y `page_size` (resultados por página,
por defecto 10). La tool DEBE devolver los resultados de la página solicitada
junto con el total de resultados, para que el cliente pueda navegar las páginas.
El resumen de cada resultado DEBE llegar truncado de forma idéntica en la vista
de texto y en el `structuredContent` (sin duplicación texto-truncado +
JSON-completo).

#### Scenario: Resultados en formato LLM-first con estructura opcional

- **WHEN** un cliente llama a `search_laws` con éxito
- **THEN** el resultado incluye contenido de texto formateado para lectura del modelo (lista de resultados con paginación y total)
- **AND** incluye contenido estructurado tipado (`structuredContent`) con los campos de los resultados, el total y la paginación, con el resumen truncado igual que la vista de texto

#### Scenario: Búsqueda simple
- **WHEN** un cliente llama a `search_laws` con `query: "Ley 21.827"`
- **THEN** la respuesta incluye los resultados de la primera página (cada uno con tipo de norma, número, título, fecha de publicación e `IDNORMA`)

#### Scenario: Navegación de páginas
- **WHEN** un cliente llama a `search_laws` con `query: "Ley 21.827"`, `page: 2` y `page_size: 5`
- **THEN** la respuesta incluye la página 2 y el total de resultados disponible, permitiendo saber cuántas páginas existen

#### Scenario: Cadena vacía
- **WHEN** un cliente llama a `search_laws` sin `query` o con `query` vacía
- **THEN** la tool devuelve un error de argumentos sin consultar el servicio

### Requirement: Tamaño de la norma en el output

El `structuredContent` de `get_law` y `get_law_summary` DEBE incluir `char_count`
y `article_count` que describen el alcance REAL solicitado — la norma completa o
la sección pedida — independientemente de si el contenido se entregó completo o
se degradó a mapa por presupuesto. En respuestas degradadas, `char_count` es el
total que motivó la degradación, nunca el tamaño del mapa mostrado. El texto de
la respuesta DEBE incluir el tamaño visible (p.ej. "Size: 426K chars · 154
articles").

#### Scenario: Conteo en get_law
- **WHEN** un cliente llama a `get_law` con éxito sobre una norma con contenido
- **THEN** el `structuredContent` incluye `char_count` y `article_count` con valores mayores a cero

#### Scenario: Conteo en get_law_summary
- **WHEN** un cliente llama a `get_law_summary` con éxito
- **THEN** el `structuredContent` incluye `char_count` y `article_count`

#### Scenario: Conteo declara el total en respuestas degradadas
- **WHEN** la respuesta se degradó a mapa porque el contenido excede el presupuesto
- **THEN** `char_count` y `article_count` describen el contenido completo del alcance solicitado, no el mapa

#### Scenario: Conteo de la sección solicitada
- **WHEN** un cliente llama a `get_law` con `section_id`
- **THEN** `char_count` y `article_count` corresponden al contenido de esa sección, no al total de la norma

#### Scenario: Conteo corresponde a la versión solicitada
- **WHEN** un cliente llama a `get_law` con `version_date` de una versión histórica
- **THEN** `char_count` y `article_count` corresponden a la versión solicitada

#### Scenario: Tamaño visible en el texto
- **WHEN** un cliente llama a `get_law` o `get_law_summary` con éxito
- **THEN** el texto de la respuesta incluye el tamaño (p.ej. "Size: 426K chars · 154 articles")

## ADDED Requirements

### Requirement: Higiene del encabezado de norma

Los nombres de sección que viajan en TOC, sub-TOC y encabezados DEBEN colapsar los
espacios internos repetidos a un solo espacio (`Título VII      DE LA FILIACIÓN`
→ `Título VII DE LA FILIACIÓN`). Las `Related norms` del encabezado de `get_law`
DEBEN acotarse a ~10 entradas visibles con el total señalado (p.ej. `Related
norms: 342 total — showing first 10`); el `structuredContent` conserva la lista
completa.

#### Scenario: Espacios colapsados en nombres de sección
- **WHEN** la estructura de la norma trae nombres con espacios internos repetidos
- **THEN** el TOC renderizado los muestra con espacios simples

#### Scenario: Related norms acotadas con total
- **WHEN** una norma trae más de ~10 vinculaciones
- **THEN** el texto lista las primeras ~10 y señala el total; el `structuredContent` conserva la lista completa

#### Scenario: Related bills acotadas con total
- **WHEN** una norma en modificación permanente trae cientos de boletines (p.ej. el Código Civil lista ~160+)
- **THEN** el texto lista las primeras ~10 con el total señalado (`161 total — showing first 10`); el `structuredContent` conserva la lista completa
