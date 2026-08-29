# cgr-prompts Specification

## Purpose
Prompts MCP curados para jurisprudencia de Contraloría (búsqueda, análisis, explicación e interpretación de dictámenes) que codifican el flujo `count → search → get` con citación `url`/`pdf_url`, método estructurado y anti-sesgo, de modo que cualquier LLM trate el MCP como fuente de la verdad y adapte la respuesta al idioma del usuario.

## Requirements

### Requirement: Prompts CGR curados expuestos

El servidor DEBE exponer 6 prompts curados vía `prompts/list` desde `internal/prompts/cgr`: `search_jurisprudence` (argumentos `query` requerido, `order` y `exact_search` y `lang` opcionales), `analyze_dictamen` (`dictamen_id` requerido y `lang` opcional), `explain_dictamen_simply` (`dictamen_id` requerido y `audience` y `lang` opcionales), `interpret_dictamen` (`dictamen_id` requerido y `lang` opcional), `analyze_contable_instructivo` (`contable_id` e `instructivo_id` opcionales, uno de los dos requerido en la práctica, y `lang` opcional) y `analyze_auditoria_consolidado` (`auditoria_id` o `consolidado_id` opcionales, y `lang` opcional). Cada prompt DEBE declarar su descripción y la obligatoriedad de sus argumentos, y DEBE estar bakeado vía `go:embed` en `internal/prompts/cgr/prompts.yaml`. Los prompts DEBEN estar en inglés pero incluir `{{if .lang}}{{.lang}}{{else}}el idioma del usuario (default español){{end}}`.

#### Scenario: Lista incluye prompts CGR
- **WHEN** un cliente MCP consulta `prompts/list`
- **THEN** la respuesta incluye los 6 prompts CGR con sus descripciones y argumentos (con `required` donde corresponde) además de los 10 prompts BCN (16 en total)

#### Scenario: Prompt CGR servido sin red
- **WHEN** un cliente pide `prompts/get` para `search_jurisprudence` con la API de Contraloría inaccesible
- **THEN** el prompt se sirve igualmente, sin error ni latencia de red

### Requirement: Templates CGR que guían el uso de las tools y fuente de la verdad

Cada prompt CGR DEBE devolver un template que (a) inyecta los valores de los argumentos recibidos, (b) referencia las tools reales por su nombre (`count_cgr_jurisprudencia`, `search_cgr_dictamenes`, `get_cgr_dictamen`), (c) embebe la regla "MCP es fuente de la verdad — no asumas derecho que no esté en el tool output; no inventes `dictamen_id` ni contenido" y (d) instruye adaptar la respuesta al idioma del usuario.

#### Scenario: search_jurisprudence inyecta query y guía flujo count→search→get
- **WHEN** un cliente pide `prompts/get` para `search_jurisprudence` con `query: "quillota"`
- **THEN** el mensaje incluye el `query` y las instrucciones de usar `count_cgr_jurisprudencia` para explorar buckets, luego `search_cgr_dictamenes` paginado con `order`/`exact_search`, y finalmente `get_cgr_dictamen` del `dictamen_id`, citando `url`/`pdf_url`

#### Scenario: analyze_dictamen cita fuentes legales
- **WHEN** un cliente pide `prompts/get` para `analyze_dictamen` con `dictamen_id: "E179593N25"`
- **THEN** el mensaje incluye el `dictamen_id` y las instrucciones de usar `get_cgr_dictamen` y citar `materia`/`descriptores`/`fuentes_legales` y `documento_completo` con `url`/`pdf_url`

### Requirement: Método estructurado, jerarquía y anti-sesgo en interpretación

El prompt `interpret_dictamen` DEBE codificar la guía completa de interpretación sin sesgo: naturaleza jurídica (informe obligatorio, jurisprudencia vinculante, no crea ley, impugnable), jerarquía corta (Constitución > ley/reglamento > dictamen — el dictamen es interpretación técnica subordinada, no fuente autónoma), método 4 pasos (problema jurídico → normas aplicables → fundamentación → resolutiva) y anti-sesgo (qué vs para qué, separar mensaje del mensajero, lecturas interesadas, hecho vs opinión). Los prompts `search_jurisprudence`, `analyze_dictamen` y `explain_dictamen_simply` DEBEN incluir un preamble corto con jerarquía, principios anti-sesgo y fuente de la verdad, referenciando `interpret_dictamen` como profundización.

#### Scenario: interpret_dictamen expone método 4 pasos
- **WHEN** un cliente pide `prompts/get` para `interpret_dictamen` con `dictamen_id: "E179593N25"`
- **THEN** el mensaje incluye los 4 pasos y los 4 principios anti-sesgo, citando `url`/`pdf_url` y la regla de no asumir derecho fuera del dictamen

### Requirement: Disclaimer y citación en prompts CGR

Los prompts `analyze_dictamen`, `explain_dictamen_simply` e `interpret_dictamen` DEBEN incluir en su template la instrucción de citar `url` y `pdf_url` por cada afirmación y el disclaimer fijo: "no es asesoría legal, solo fuente de información, la interpretación de la IA puede variar entre modelos y proveedores".

#### Scenario: explain_dictamen con disclaimer y citación
- **WHEN** un cliente pide `prompts/get` para `explain_dictamen_simply` con `dictamen_id: "E179593N25"`
- **THEN** el mensaje incluye la instrucción de citar `url`/`pdf_url` y el disclaimer fijo
