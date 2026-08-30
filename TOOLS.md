# Referencia de herramientas — Lex Chile MCP Server

Documento técnico de referencia. Describe las 15 herramientas MCP que expone el servidor (5 de LeyChile/BCN y 10 de Contraloría/CGR) y las 16 guías (prompts) incluidas (10 de BCN y 6 de CGR). Para la instalación y la conexión de agentes, consulte [INSTALL.md](INSTALL.md). Para una descripción general, consulte el [README](README.md).

## Convenciones generales

- **Identificadores**: cada buscador devuelve los identificadores que usan las herramientas de consulta (`get_*`). Nunca invente identificadores: obténgalos siempre del resultado de una búsqueda.
- **Respuestas dobles**: toda herramienta devuelve contenido de texto legible para el modelo **y** un `structuredContent` tipado con los mismos datos; el texto es una vista de los datos estructurados, sin divergencia.
- **Citas**: cite siempre los campos `url` y `pdf_url` (o `documento_cic_pdf_web` en consolidados) que acompañan a cada resultado de CGR, y verifique el contenido contra el documento oficial.
- **Pacing**: deje 3-4 segundos entre llamadas consecutivas hacia `contraloria.cl`. Los servicios consultados son públicos: evite saturarlos.

---

## Herramientas de LeyChile (BCN)

Flujo recomendado para una norma: `search_laws` → `get_law_summary` (primero siempre) → `find_in_norm` (si busca un artículo o frase concreta) → `get_law` con `structure_only=true` y luego por `section_id`. Lea la norma completa solo si el tamaño reportado lo permite.

### search_laws

Busca leyes, decretos y resoluciones por texto libre en LeyChile.

| Parámetro | Tipo | Requerido | Descripción |
|---|---|---|---|
| `query` | string | Sí | Texto de búsqueda, por ejemplo `"Ley 21.600"` o `"arriendo"`. |
| `page` | int | No | Número de página, desde 1 (por defecto `1`). |
| `page_size` | int | No | Resultados por página (por defecto `10`, máximo `50`). |

Devuelve por resultado: `norm_id` (identificador para el resto de herramientas), tipo, título, fecha de publicación, organismo y resumen oficial; además `total_items` y `total_pages` para paginar.

### get_law

Obtiene el contenido de una norma en Markdown, con metadatos y tabla de contenidos.

| Parámetro | Tipo | Requerido | Descripción |
|---|---|---|---|
| `norm_id` | int | Sí | Identificador devuelto por `search_laws`. |
| `version_date` | string | No | Versión vigente en esa fecha (`YYYY-MM-DD`; por defecto, la más reciente). |
| `structure_only` | bool | No | Devuelve solo metadatos y tabla de contenidos (por defecto `false`). |
| `section_id` | int | No | Devuelve únicamente esa sección (identificador obtenido de `get_law_summary` o de `get_law` con `structure_only=true`). |

Advertencia: las normas largas pueden superar cientos de miles de caracteres. Cuando el contenido completo excede el presupuesto de salida, la respuesta entrega un mapa navegable (marcado `degraded`, con la línea "Content withheld") y debe recortarse por `section_id`; no reintente la llamada completa. Los campos `char_count` y `article_count` describen siempre el alcance real solicitado.

### get_law_summary

Resumen ligero de una norma: título, origen, materias, categorías, resumen oficial de la BCN, tabla de contenidos con los `section_id` y tamaño del texto completo. Úsela **antes** que `get_law` para decidir qué leer.

| Parámetro | Tipo | Requerido | Descripción |
|---|---|---|---|
| `norm_id` | int | Sí | Identificador devuelto por `search_laws`. |
| `version_date` | string | No | Versión vigente en esa fecha (`YYYY-MM-DD`). |

### get_law_history

Historia legislativa de una norma: sus registros propios, las leyes que la modificaron (`modificatorias`) y las que ella modificó (`modificadas`). Cada registro incluye fecha, descripción, resumen y el enlace LeyChile. Para leer una de esas normas, use el valor `id_norma_hl` del registro — no el número de ley ni el campo `id_norma`.

| Parámetro | Tipo | Requerido | Descripción |
|---|---|---|---|
| `norm_id` | int | Sí | Identificador devuelto por `search_laws`. |

### find_in_norm

Busca texto literal (subcadena, sin distinción de mayúsculas, nunca expresión regular) dentro de una sola norma. Devuelve cada coincidencia con `section_id`, tamaño y un fragmento de contexto; las coincidencias por nombre de sección se priorizan. Use el `section_id` resultante con `get_law` para leer el contenido.

| Parámetro | Tipo | Requerido | Descripción |
|---|---|---|---|
| `norm_id` | int | Sí | Identificador devuelto por `search_laws`. |
| `query` | string | Sí | Texto literal a buscar, por ejemplo `"Artículo 19"` o `"arrendador"`. |
| `version_date` | string | No | Versión vigente en esa fecha (`YYYY-MM-DD`). |

---

## Herramientas de Contraloría (CGR)

Las fuentes disponibles vía `source` en `search_cgr`:

| `source` | Contenido | Formato de identificador |
|---|---|---|
| `dictamenes` | Jurisprudencia administrativa vinculante | `E179593N25` (patrón `E…N…`) |
| `instructivos` | Instructivos generales | `IN23N26`, `E462387N24` |
| `contable` | Oficios contables (NICSP, donaciones, EEFF) | `E080961`, `OFE0809612600` |
| `auditoria` | Informes finales de fiscalización | `371/2026`, `371N26` |
| `legislacion` | Toma de razón (AFECTO) | `RZA005691400`, `FRA000012000` |
| `cuentas` | Sentencias del Juzgado de Cuentas | `2982331` (numérico) |
| `consolidados` | Consolidados CIC | `CIC21/2026` |
| `web` | Contexto web (no es jurisprudencia) | — |
| `todos` | Todas las anteriores | según fuente |

### search_cgr

Búsqueda general multi-fuente en Contraloría.

| Parámetro | Tipo | Requerido | Descripción |
|---|---|---|---|
| `query` | string | Sí | Texto de búsqueda; vacío lista lo más reciente. |
| `source` | string | No | Fuente del cuadro anterior (por defecto `dictamenes`). |
| `exact_search` | bool | No | Coincidencia exacta (por defecto `false`). |
| `order` | string | No | `date` (más nuevos), `dateasc` (más antiguos), `score` (relevancia). |
| `page` | int | No | Página `1..500`, 20 resultados por página. |

Sugerencia: con más de 500 resultados, use `order="score"` en lugar de paginar por fecha.

### search_cgr_dictamenes

Alias de compatibilidad equivalente a `search_cgr` con `source="dictamenes"`. Mantiene los parámetros `query`, `exact_search`, `order` y `page`.

### get_cgr_dictamen

Ficha completa de un dictamen: materia, descriptores, criterio, origen, destinatarios, abogados, fuentes legales, carácter y texto del documento. Los documentos largos llegan paginados: la parte 1 trae los metadatos y los primeros ~100 000 caracteres; continúe con `part=2`, `part=3`… siguiendo la señal de rango que indica la respuesta.

| Parámetro | Tipo | Requerido | Descripción |
|---|---|---|---|
| `dictamen_id` | string | Sí | Identificador `E…N…` obtenido de la búsqueda. |
| `part` | int | No | Parte del documento, desde 1 (por defecto `1`). |

### get_cgr_instructivo

Ficha de un instructivo general: materia, descriptores, carácter y `documento_completo` saneado, con `char_count` y enlaces HTML/PDF.

| Parámetro | Tipo | Requerido | Descripción |
|---|---|---|---|
| `instructivo_id` | string | Sí | Por ejemplo `IN23N26` o `E462387N24`. |

### get_cgr_contable

Ficha de un oficio contable: número, normativa contable, tipo, parte, destinatarios, origen, fecha y texto saneado.

| Parámetro | Tipo | Requerido | Descripción |
|---|---|---|---|
| `contable_id` | string | Sí | Por ejemplo `E080961` o `OFE0809612600`. |

### get_cgr_auditoria

Ficha de un informe final de auditoría: número, nombre, tipo, objetivo, conclusiones, universo, muestra, destinatarios, servicio, unidad CGR y fecha. El campo `contenido_pdf` se trunca a 30 000 caracteres cuando es mayor; la respuesta lo indica (`truncated`) junto con el enlace al PDF oficial para la lectura íntegra.

| Parámetro | Tipo | Requerido | Descripción |
|---|---|---|---|
| `auditoria_id` | string | Sí | Por ejemplo `371/2026` o `371N26`. |

### get_cgr_consolidado

Ficha de un consolidado CIC: número, nombre, tipo, reseña, fecha, unidad CGR, sector y `contenido_extraido` (también truncado a 30 000 caracteres cuando corresponde), con enlace al documento oficial.

| Parámetro | Tipo | Requerido | Descripción |
|---|---|---|---|
| `consolidado_id` | string | Sí | Por ejemplo `CIC21/2026`. |

### get_cgr_cuenta

Ficha de una sentencia del Juzgado de Cuentas: número de sentencia y de expediente, texto saneado, fechas y enlaces PDF.

| Parámetro | Tipo | Requerido | Descripción |
|---|---|---|---|
| `cuenta_id` | string | Sí | Identificador numérico, por ejemplo `2982331`. |

### get_cgr_legislacion

Ficha de un registro de legislación (toma de razón): tipo, número, organismo, materias, texto saneado, fecha y carácter.

| Parámetro | Tipo | Requerido | Descripción |
|---|---|---|---|
| `legislacion_id` | string | Sí | Por ejemplo `RZA005691400` o `FRA000012000`. |

### count_cgr_jurisprudencia

Cuenta resultados por tipo (dictámenes, auditoría, legislación, etc.) sin descargar documentos: devuelve `total` y desglose por tipo. Útil para dimensionar una búsqueda antes de ejecutarla. Es opcional y sensible a la carga del servicio: si falla por tiempo de espera o por el circuit breaker, no lo reintente en bucle; pase directamente a `search_cgr`.

| Parámetro | Tipo | Requerido | Descripción |
|---|---|---|---|
| `query` | string | Sí | Texto de búsqueda; vacío cuenta todo. |
| `exact_search` | bool | No | Coincidencia exacta (por defecto `false`). |

---

## Guías incluidas (prompts)

El servidor registra 16 guías que enseñan al modelo el flujo de trabajo paso a paso (búsqueda, verificación y citas). Se invocan como *prompts* MCP desde el agente.

### Guías de BCN (10)

| Guía | Propósito |
|---|---|
| `analyze_law` | Análisis estructurado de una norma (propósito, obligaciones, sanciones, vigencia). |
| `search_legal_topic` | Búsqueda de normas sobre un tema, con refinamiento por tipo de norma. |
| `compare_law_versions` | Comparación de una norma entre dos fechas: qué cambió y cuándo. |
| `trace_law_history` | Línea de tiempo legislativa con las leyes modificatorias. |
| `check_law_validity` | Vigencia de una norma: vigente, derogada o vigente a una fecha. |
| `explain_law_simply` | Explicación en lenguaje llano, con audiencia opcional. |
| `law_research_workflow` | Investigación eficiente de una norma respondiendo una pregunta concreta. |
| `answer_constitutional_question` | Respuesta a preguntas sobre la Constitución (Decreto 100) por capítulo o artículo. |
| `check_norm_constitutionality` | Análisis de compatibilidad de una norma con la Constitución, en paralelo artículo a artículo. |
| `interpret_law` | Método de interpretación sin sesgo (elementos gramatical, histórico, lógico, sistemático y sociológico). |

### Guías de CGR (6)

| Guía | Propósito |
|---|---|
| `search_jurisprudence` | Búsqueda de jurisprudencia multi-fuente con dimensionamiento previo y selección del getter adecuado. |
| `analyze_dictamen` | Análisis estructurado de un dictamen (materia, criterio, fuentes legales). |
| `explain_dictamen_simply` | Explicación de un dictamen en lenguaje llano. |
| `interpret_dictamen` | Método de interpretación de dictámenes sin sesgo político. |
| `analyze_contable_instructivo` | Análisis de oficios contables e instructivos generales. |
| `analyze_auditoria_consolidado` | Análisis de informes de auditoría y consolidados CIC. |

---

## Límites y buenas prácticas

- **Presupuesto de salida**: `get_law` degrada a mapa navegable cuando la norma excede el presupuesto; recorte por `section_id` en lugar de reintentar.
- **Truncamientos**: `get_cgr_auditoria` y `get_cgr_consolidado` truncan el contenido a 30 000 caracteres; `get_cgr_dictamen` pagina por ~100 000 caracteres con el parámetro `part`. Use siempre los enlaces PDF oficiales para la lectura completa.
- **Pacing**: 3-4 segundos entre llamadas a `contraloria.cl`; no ejecute reintentos en bucle ante fallos por carga.
- **Verificación**: todo dato debe contrastarse contra el texto retornado por las herramientas y, para efectos formales, contra la fuente oficial (bcn.cl / contraloria.cl). Nunca cite artículos ni identificadores que no aparezcan en una respuesta.
