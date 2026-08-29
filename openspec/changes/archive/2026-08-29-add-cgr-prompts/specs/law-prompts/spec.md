## MODIFIED Requirements

### Requirement: Prompts curados expuestos

El servidor DEBE exponer diez prompts curados vía `prompts/list` desde `internal/prompts/bcn`: `analyze_law` (argumentos `norm_id` requerido, `aspect` y `lang` opcionales), `search_legal_topic` (`topic` requerido y `lang` opcional), `compare_law_versions` (`norm_id`, `from_date` y `to_date` requeridos y `lang` opcional), `trace_law_history` (`norm_id` requerido y `lang` opcional), `check_law_validity` (`norm_id` requerido y `date` y `lang` opcionales), `explain_law_simply` (`norm_id` requerido y `audience` y `lang` opcionales), `law_research_workflow` (`norm_id` requerido y `question` y `lang` opcionales), `answer_constitutional_question` (`question` requerido, `article_hint`, `version_date` y `lang` opcionales), `check_norm_constitutionality` (`norm_id` requerido, `question`, `version_date` y `lang` opcionales) e `interpret_law` (`norm_id` requerido y `lang` opcional). Cada prompt DEBE declarar su descripción y la obligatoriedad de sus argumentos.

#### Scenario: Lista de prompts
- **WHEN** un cliente MCP consulta `prompts/list`
- **THEN** la respuesta incluye los diez prompts BCN con sus descripciones y argumentos (con `required` donde corresponde)

### Requirement: Templates que guían el uso de las tools

Cada prompt BCN DEBE devolver un template de mensaje que (a) inyecta los valores de los argumentos recibidos, (b) referencia las tools reales por su nombre (`search_laws`, `get_law`, `get_law_summary`, `get_law_history`, `find_in_norm`), (c) embebe la regla "MCP es fuente de la verdad — no asumas derecho que no esté en el tool output; no inventes números de artículo ni contenido; LeyChile en bcn.cl es la fuente primaria" y (d) instruye adaptar la respuesta al idioma del usuario. Los templates DEBEN referenciar solo tools BCN vía `bcn.ToolNames()`.

#### Scenario: Prompt con argumentos inyectados
- **WHEN** un cliente pide `prompts/get` para `analyze_law` con `norm_id: 1195666`
- **THEN** el mensaje devuelto incluye el `norm_id` en el texto y las instrucciones de usar `get_law_summary` y `get_law` en ese orden

#### Scenario: Prompt con fechas de comparación
- **WHEN** un cliente pide `prompts/get` para `compare_law_versions` con `from_date` y `to_date`
- **THEN** el mensaje instruye usar `get_law` con `version_date` para ambas fechas e incluye ambas fechas en el texto

### Requirement: Disclaimer en explicaciones simples

Los prompts `explain_law_simply`, `interpret_law` y `analyze_law` DEBEN incluir la instrucción de citar la fuente por cada afirmación y el disclaimer fijo: "no es asesoría legal, solo fuente de información, la interpretación de la IA puede variar entre modelos y proveedores", con referencia a recursos `LeyChile`/`Historia de la Ley`/`doctrina`/`jurisprudencia` (Corte Suprema) como profundización.

#### Scenario: Prompt de explicación con disclaimer
- **WHEN** un cliente pide `prompts/get` para `explain_law_simply`
- **THEN** el mensaje incluye la instrucción de citar fuentes y el disclaimer de no-asesoría legal

### Requirement: Prompts sin llamadas externas

Servir un prompt BCN DEBE ser una operación pura de templates: `prompts/get` NO DEBE realizar llamadas a la API de BCN ni depender del estado del cliente de normas. `bcn.PromptSet` DEBE ser independiente de `cgr.PromptSet`.

#### Scenario: Prompt servido sin red
- **WHEN** un cliente pide `prompts/get` para `analyze_law` con la API de BCN inaccesible
- **THEN** el prompt se sirve igualmente, sin error ni latencia de red

## ADDED Requirements

### Requirement: Método estructurado y anti-sesgo en interpretación de leyes

El prompt `interpret_law` DEBE codificar la guía completa de interpretación sin sesgo: jerarquía y contexto (ley en armonía con Constitución, arts. 19-24 Código Civil, interpretación auténtica), método 5 elementos (gramatical art.20, histórico art.19 inc2, lógico art.22, sistemático art.22, sociológico art.19 inc1) y anti-sesgo (sesgo de confirmación, ley deseable vs vigente, fin no justifica medios). Los prompts BCN existentes DEBEN incluir un preamble corto con anti-sesgo (no buscar lo que se quiere que diga, sino lo que efectivamente dice; separar técnico de preferencias) y fuente de la verdad, referenciando `interpret_law` como profundización.

#### Scenario: interpret_law expone 5 elementos
- **WHEN** un cliente pide `prompts/get` para `interpret_law` con `norm_id: 1195666`
- **THEN** el mensaje incluye los 5 elementos (gramatical, histórico, lógico, sistemático, sociológico) con referencia a arts. 19-24 Código Civil y la regla de no desatender tenor literal
