# find-in-norm Specification

## Purpose
Intra-norm exact search: given a norm and a plain-text query, return the sections
that match by name or content with their retrieval identifiers, so the agent can
jump directly to `get_law(section_id=...)` without walking the folded TOC.

## Requirements

### Requirement: Búsqueda exacta dentro de una norma

El servidor DEBE exponer una tool `find_in_norm` que acepta `norm_id` (requerido),
`query` (texto plano, requerido) y `version_date` opcional (formato `YYYY-MM-DD`,
validado estrictamente como en `get_law`), y DEBE buscar coincidencias
case-insensitive por substring sobre dos superficies de la norma identificada: los
nombres de las secciones de su estructura y el contenido Markdown de sus bloques.
La query DEBE tratarse como texto literal: el servidor NO DEBE interpretarla como
expresión regular ni patrón. La búsqueda DEBE ejecutarse en memoria sobre la norma
cacheada (revalidación ETag existente), sin llamadas HTTP adicionales al servicio
más allá de la obtención de la propia norma.

#### Scenario: Búsqueda por número de artículo
- **WHEN** un cliente llama a `find_in_norm` con `query: "1749"` sobre una norma que contiene un `Artículo 1749`
- **THEN** la respuesta incluye la sección con ese nombre junto a su `section_id` y tamaño

#### Scenario: Búsqueda por frase en el contenido
- **WHEN** un cliente llama a `find_in_norm` con `query: "sociedad conyugal"`
- **THEN** la respuesta incluye las secciones cuyo contenido Markdown contiene la frase (case-insensitive), cada una con `section_id`, tamaño y un snippet de contexto

#### Scenario: Query como texto literal
- **WHEN** un cliente llama a `find_in_norm` con `query: "art. 1.*"` o cualquier texto con sintaxis de expresión regular
- **THEN** la búsqueda trata la cadena como texto literal y no como patrón

#### Scenario: Argumentos inválidos
- **WHEN** un cliente llama a `find_in_norm` sin `norm_id`, sin `query`, con `norm_id` no positivo o con `version_date` inválida
- **THEN** la tool devuelve un error de argumentos sin consultar el servicio

#### Scenario: Norma inexistente
- **WHEN** un cliente llama a `find_in_norm` con un `norm_id` que no existe en LeyChile
- **THEN** la tool devuelve un error de "norma no encontrada"

### Requirement: Ranking y formato de resultados

Los resultados DEBEN ordenarse con las coincidencias por nombre de sección por
encima de las coincidencias por contenido, y dentro de cada grupo por el orden del
documento. La respuesta DEBE limitarse a ~20 resultados. Cada coincidencia de
nombre DEBE incluir nombre, `section_id`, `char_count` y `article_count` de la
sección; cada coincidencia de contenido DEBE añadir un snippet de ~200 chars
alrededor de la primera ocurrencia. La respuesta completa (texto y
`structuredContent` con los mismos datos) DEBE mantenerse en el orden de pocos
miles de tokens, conforme a la capability `output-budget`.

#### Scenario: Nombre rankea sobre contenido
- **WHEN** la query coincide con el nombre de una sección y con el contenido de otra
- **THEN** la coincidencia por nombre aparece primero en la respuesta

#### Scenario: Orden del documento dentro de cada grupo
- **WHEN** múltiples secciones coincen por contenido
- **THEN** se listan en el orden en que aparecen en la norma

#### Scenario: Límite de resultados
- **WHEN** la query produce más de ~20 coincidencias
- **THEN** la respuesta lista las primeras ~20 según el ranking y señala que el resultado está acotado

#### Scenario: Sin coincidencias
- **WHEN** ninguna sección coincide por nombre ni contenido
- **THEN** la tool devuelve una respuesta sin error indicando que no hay coincidencias, con el número de secciones recorridas
