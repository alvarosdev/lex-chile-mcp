## Purpose

Búsqueda de normas jurídicas chilenas en LeyChile (Biblioteca del Congreso Nacional): búsqueda paginada por texto y acceso al contenido de una norma por su identificador, con endpoints declarados externamente y transporte resiliente configurable.

## Requirements

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

### Requirement: Paginación tolerante a tipos inconsistentes del servicio

La búsqueda DEBE decodificar el bloque de paginación que entrega el servicio aceptando indistintamente string (`"10"`) o número (`10`) en cada uno de sus campos numéricos (`npagina`, `itemsporpagina`, `totalitems`), incluyendo combinaciones de ambos formatos en una misma respuesta. La decodificación DEBE además tolerar números con espacios (`" 10 "`), decimales de parte entera (`10.0`), cadena vacía (`""`) y `null` — estos dos últimos interpretados como 0 — sin fallar la búsqueda. Un valor no numérico (p.ej. `"abc"`) DEBE fallar la búsqueda con un error de decodificación explícito, nunca silenciarse como 0. El contrato de la tool hacia el cliente NO cambia: `total_items`, `page`, `page_size` y `total_pages` del contenido estructurado SIGUEN siendo números.

#### Scenario: Paginación numérica
- **WHEN** el servicio responde la búsqueda con `npagina`, `itemsporpagina` y `totalitems` como números (p.ej. la búsqueda `Ley 21461`)
- **THEN** la búsqueda retorna los resultados y el total sin error de decodificación, con el `norm_id` esperado entre los resultados

#### Scenario: Paginación como string (regresión)
- **WHEN** el servicio responde la búsqueda con `npagina`, `itemsporpagina` y `totalitems` como strings (p.ej. la búsqueda `Ley 21.600`)
- **THEN** la búsqueda sigue retornando los resultados y el total sin error de decodificación

#### Scenario: Formatos mixtos y valores vacíos
- **WHEN** la respuesta combina formatos en el mismo bloque (p.ej. `"npagina": 1` junto a `"itemsporpagina": "10"`) o trae `""` o `null` en un campo numérico
- **THEN** la búsqueda no falla por decodificación y los campos vacíos o nulos se interpretan como 0

#### Scenario: Variantes de formato numérico
- **WHEN** un campo numérico llega como string con espacios (`" 10 "`) o como decimal de parte entera (`10.0`)
- **THEN** la búsqueda lo interpreta como 10 sin error

#### Scenario: Valor no numérico falla explícito
- **WHEN** un campo numérico llega como un string sin contenido numérico (p.ej. `"abc"`)
- **THEN** la búsqueda falla con un error de decodificación explícito en lugar de interpretarlo como 0

#### Scenario: Contrato de la tool sin cambios
- **WHEN** la búsqueda tiene éxito con cualquiera de los formatos anteriores
- **THEN** el contenido estructurado entrega `total_items`, `page`, `page_size` y `total_pages` como números

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

### Requirement: Contenido en Markdown

El contenido de la norma (los bloques HTML del servicio) DEBE entregarse convertido a **Markdown**, con las entidades HTML decodificadas y los enlaces conservados, para que el texto sea legible por el cliente.

#### Scenario: Contenido convertido
- **WHEN** la norma tiene bloques de contenido con entidades como `&#xED;` y enlaces a otras normas
- **THEN** la respuesta contiene el texto decodificado (`í`) en formato Markdown y los enlaces conservados como enlaces

### Requirement: Resumen legible para el cliente

El campo de resumen de cada resultado de búsqueda, cuando el servicio lo entrega como XML embebido con entidades HTML, DEBE devolverse decodificado, sin el marcado XML y sin el whitespace de indentación del wrapper, para que el texto sea legible.

#### Scenario: Resumen con entidades y XML
- **WHEN** un resultado de búsqueda trae un `RESUMEN` con `<RESUMENES>`, entidades como `&#241;` e indentación del wrapper XML
- **THEN** la respuesta contiene el texto decodificado (`ñ`) y sin etiquetas XML ni indentación residual

### Requirement: Contenido de la norma sin basura de marcado

El contenido de la norma (tras la conversión a Markdown) DEBE estar normalizado: los espacios no separadores y sus variantes (`&nbsp;`, `&ensp;`, `&emsp;` → U+00A0, U+2002, U+2003) DEBEN convertirse a espacios normales, los espacios al inicio y fin de cada línea DEBEN recortarse, los espacios consecutivos DEBEN colapsarse, y los caracteres de control y de ancho cero DEBEN eliminarse. Las comillas de citas (`&quot;`) y los enlaces a otras normas DEBEN conservarse como parte del contenido.

#### Scenario: Sangría visual normalizada
- **WHEN** un párrafo de la norma comienza con `&nbsp; &nbsp;  ` (sangría visual de la API)
- **THEN** la respuesta contiene el párrafo sin los espacios no separadores al inicio

#### Scenario: Comillas de citas conservadas
- **WHEN** el contenido incluye `&quot;Art&#xED;culo 1.- ...&quot;`
- **THEN** la respuesta conserva las comillas como caracteres normales alrededor del texto citado

#### Scenario: Enlaces conservados
- **WHEN** el contenido incluye un enlace a otra norma (`<a href="...idNorma=...">`)
- **THEN** la respuesta lo conserva como enlace en el texto Markdown

#### Scenario: Caracteres de control eliminados
- **WHEN** el contenido contiene caracteres de control (U+0000–U+001F fuera de salto de línea y tabulación) o de ancho cero (U+200B, U+FEFF)
- **THEN** la respuesta no los incluye

### Requirement: Endpoints declarados en archivo YAML

El servidor DEBE cargar la definición de sus recursos de LeyChile desde el contrato embebido en el binario via `go:embed` (`internal/config/api.resources.yaml`), sin ruta fija en filesystem ni override por variable de entorno, que declara para cada recurso: `url`, `path`, `method`, `timeout`, `retry` (intentos y backoff) y `circuit_breaker` (umbrales de fallo, éxito y cooldown). El contenido DEBE validarse al cargar (en startup via `LoadEmbedded`); una configuración inválida DEBE impedir el arranque del servidor. El binario es autocontenido y no requiere `config/` en el filesystem ni en la imagen de contenedor.

#### Scenario: Carga exitosa con ruta fija
- **WHEN** el servidor arranca (el contrato está embebido via `go:embed` en `internal/config/api.resources.yaml`, sin `config/api.resources.yaml` en el filesystem)
- **THEN** los recursos quedan disponibles y el servidor continúa el arranque
#### Scenario: Configuración inválida
- **WHEN** el YAML embebido tiene un recurso sin `path`, con timeout negativo o con retry/breaker incoherentes
- **THEN** el servidor no arranca e informa el error de validación

#### Scenario: Configuración en la imagen de contenedor
- **WHEN** se construye la imagen de contenedor desde el Dockerfile
- **THEN** la imagen no necesita incluir la carpeta `config/` y el servidor arranca sin configuración adicional (contrato embebido)

### Requirement: Transporte resiliente por recurso

Cada recurso DEBE aplicar su propio `timeout`, reintentos y circuit breaker declarados en el YAML: ante fallos transitorios (timeouts, respuestas 5xx) el cliente DEBE reintentar según la configuración del recurso, y ante fallos repetidos el circuit breaker DEBE abrirse y rechazar llamadas sin llegar al servicio hasta que se recupere.

#### Scenario: Falla transitoria
- **WHEN** el servicio responde con error 5xx o timeout en la primera llamada y responde bien en las siguientes
- **THEN** el cliente reintenta según la configuración del recurso y devuelve la respuesta exitosa

#### Scenario: Circuit breaker abierto
- **WHEN** el recurso acumula fallos por encima del umbral declarado
- **THEN** las llamadas al recurso se rechazan inmediatamente con un error de circuito abierto, sin intentar contactar el servicio

### Requirement: Caché de normas con revalidación ETag

El cliente DEBE cachear en memoria cada norma recuperada junto con su ETag (validado con la API real: `get_norma_json` responde `ETag` y `304 Not Modified` con cuerpo vacío ante un `If-None-Match` coincidente). En una llamada posterior al mismo `norm_id`, el cliente DEBE enviar el `If-None-Match` guardado; ante un `304` DEBE servir la copia cacheada sin re-descargar ni re-convertir el contenido; ante un `200` DEBE reemplazar la entrada cacheada. El caché vive en el proceso (se pierde al reiniciar, lo cual es aceptable) y NO aplica a la búsqueda.

#### Scenario: Hit de caché
- **WHEN** la misma norma se solicita dos veces y el servicio responde `304` en la segunda llamada
- **THEN** la segunda respuesta se sirve desde el caché con el mismo contenido, sin re-descargar el cuerpo ni re-convertirlo

#### Scenario: Norma actualizada
- **WHEN** el servicio responde `200` con un ETag distinto al guardado
- **THEN** la entrada cacheada se reemplaza con el contenido nuevo

#### Scenario: Reinicio sin caché
- **WHEN** el servidor se reinicia
- **THEN** el caché comienza vacío y la primera llamada a cada norma vuelve a descargarse

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

### Requirement: Categorías de norma en metadatos

Los metadatos de la norma entregados por `get_law` DEBEN incluir el campo `categorias_norma` tal como lo entrega el servicio (etiquetas temáticas de la norma).

#### Scenario: Norma con categorías
- **WHEN** se solicita una norma cuyo response incluye `categorias_norma` (p.ej. `["Ley \"Educación sin Dicom\""]`)
- **THEN** los metadatos de la respuesta incluyen esas categorías en el mismo orden

### Requirement: Versión histórica de norma por fecha

Las tools `get_law` y `get_law_summary` DEBEN aceptar un parámetro opcional `version_date` en formato `YYYY-MM-DD` que selecciona la versión de la norma vigente a esa fecha; sin el parámetro DEBEN devolver la última versión. El formato DEBE validarse estrictamente: un valor que no sea una fecha válida DEBE devolver un error de argumentos sin consultar el servicio. El contenido de la respuesta DEBE corresponder a la versión solicitada (verificado con la API real: el texto cambia según la fecha).

#### Scenario: Versión histórica
- **WHEN** un cliente llama a `get_law` con `norm_id` de una norma modificada y `version_date: "2010-01-01"`
- **THEN** la respuesta contiene el texto de la norma vigente a esa fecha, distinto del texto vigente actual

#### Scenario: Sin fecha = última versión
- **WHEN** un cliente llama a `get_law` sin `version_date`
- **THEN** la respuesta contiene la última versión de la norma

#### Scenario: Fecha inválida
- **WHEN** un cliente llama a `get_law` con `version_date: "2010-13-45"` o `"basura"`
- **THEN** la tool devuelve un error de argumentos sin consultar el servicio

#### Scenario: Versión indicada en la respuesta
- **WHEN** un cliente llama a `get_law` con `version_date`
- **THEN** el texto de la respuesta indica la versión mostrada (p.ej. "Version: as of 2010-01-01")

### Requirement: Historia legislativa de una norma

El servidor DEBE exponer una tool `get_law_history` que consulta el endpoint de historias legislativas con `norm_id` (requerido) y devuelve los tres grupos oficiales — historia de la ley, historias de las leyes modificatorias e historias de las leyes modificadas — cada uno con sus entradas (fecha, descripción, bajada, enlace e identificadores). Un `norm_id` inexistente DEBE devolver un mensaje amable de que no hay historia (la API responde una lista vacía).

#### Scenario: Historia con modificaciones
- **WHEN** un cliente llama a `get_law_history` con `norm_id: 1195666`
- **THEN** la respuesta incluye los tres grupos con sus entradas, incluyendo las leyes que modificaron a la norma con su fecha y enlace

#### Scenario: Norma sin historia
- **WHEN** un cliente llama a `get_law_history` con un `norm_id` que no existe
- **THEN** la tool devuelve un mensaje amable indicando que no hay historia, sin error de protocolo

#### Scenario: Identificador faltante
- **WHEN** un cliente llama a `get_law_history` sin `norm_id`
- **THEN** la tool devuelve un error de argumentos sin consultar el servicio

#### Scenario: Enlaces construidos con el id correcto
- **WHEN** la respuesta de `get_law_history` incluye entradas con `id_norma_hl` e `id_norma`
- **THEN** los enlaces a la ficha de LeyChile se construyen con `id_norma_hl` (el idNorma de la norma dueña del registro), nunca con `id_norma` ni con el número del enlace de historia

### Requirement: Caché con revalidación por versión e historia

El caché ETag DEBE distinguir versiones: la clave de las normas DEBE componer `norm_id` y `version_date` (sin fecha = última), de modo que una versión histórica nunca reciba la respuesta cacheada de otra versión. La historia legislativa DEBE cachearse con revalidación ETag por `norm_id` (el endpoint envía ETag y responde 304, verificado).

#### Scenario: Versiones no se mezclan en caché
- **WHEN** un cliente solicita la misma norma con y sin `version_date`
- **THEN** cada versión se descarga y cachea por separado, sin servirse contenido de la otra

#### Scenario: Historia revalidada
- **WHEN** la misma historia se solicita dos veces y el servicio responde 304
- **THEN** la segunda respuesta se sirve del caché sin re-descargar

### Requirement: Tipo de norma decodificado como dato anexo

Los metadatos de la norma DEBEN anexar a cada tipo de norma los campos `canonical_type` y `canonical_abbr` decodificados del catálogo oficial (p.ej. `tipo: "1"` → `canonical_type: "Ley"`, `canonical_abbr: "LEY"`), SIN reemplazar los valores crudos de la API.

#### Scenario: Código decodificado
- **WHEN** una norma trae `tipos_numeros` con `tipo: "1"`
- **THEN** la respuesta incluye `canonical_type: "Ley"` y `canonical_abbr: "LEY"` junto al valor crudo

#### Scenario: Código desconocido
- **WHEN** un tipo trae un código fuera del catálogo
- **THEN** los campos canonical se omiten (omitempty) y el valor crudo permanece

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
