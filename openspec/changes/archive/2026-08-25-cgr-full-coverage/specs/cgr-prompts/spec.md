## MODIFIED Requirements

### Requirement: Prompts CGR genéricos con source y por familia (sin saturar)

El servidor DEBE exponer 6 prompts CGR vía `prompts/list` desde `internal/prompts/cgr` bakeado con `go:embed`: los 4 existentes (`search_jurisprudence` reescrito genérico, `analyze_dictamen`, `explain_dictamen_simply`, `interpret_dictamen`) + 2 nuevos por familia (`analyze_contable_instructivo`, `analyze_auditoria_consolidado`). `search_jurisprudence` DEBE aceptar `query` requerido y `source`/`order`/`exact_search`/`lang` opcionales y enseñar tabla `source` (dictamenes, instructivos IN23, contable NICSP OFE..., auditoria 10k, consolidados CIC21, cuentas sentencia, legislacion AFECTO) con flujo `count?(opcional)` → `search_cgr(source)` → `get_*` y hints `order=score si total>500 else date` y pacing 3-4s. Los 6 prompts DEBEN incluir `{{if .lang}}` i18n, disclaimer fijo y regla “MCP fuente verdad”.

#### Scenario: Lista incluye 6 CGR
- **WHEN** un cliente consulta `prompts/list`
- **THEN** ve 6 prompts CGR con descripciones y args (con `required` donde corresponde) además de 10 BCN (total 16)

#### Scenario: search_jurisprudence genérico con source
- **WHEN** `prompts/get` `search_jurisprudence` con `query:"municipalidad"`, `source:"contable"`
- **THEN** el mensaje incluye el `query` y `source:"contable"`, la tabla de sources y las instrucciones de usar `search_cgr` con `source:"contable"` y luego `get_cgr_contable`

#### Scenario: count opcional bajo carga
- **WHEN** el template de `search_jurisprudence` se renderiza
- **THEN** dice “si count timeout/breaker, pasa directo a search_cgr”

### Requirement: Prompts por familia contable-instructivo y auditoria-consolidado

`analyze_contable_instructivo` DEBE aceptar `contable_id`/`instructivo_id`/`lang` y guiar `get_cgr_contable`/`get_cgr_instructivo` con `parte`/`materia` como lead, cita `url`/`pdf_url`, hedge condicional y disclaimer. `analyze_auditoria_consolidado` DEBE aceptar `auditoria_id`/`consolidado_id`/`lang` y guiar `get_cgr_auditoria` (aviso trunc 30k) / `get_cgr_consolidado` (`resena`+`contenido_extraido`) con `pdf`/`documento_cic_pdf_web`. Ambos DEBEN incluir preamble corto ref a `interpret_dictamen`.

#### Scenario: analyze_contable_instructivo con OFE
- **WHEN** `prompts/get` `analyze_contable_instructivo` con `contable_id:"E509321"`
- **THEN** el mensaje incluye el id y las instrucciones de usar `get_cgr_contable` y citar `normativa_contable` y `parte`

#### Scenario: analyze_auditoria_consolidado con CIC21
- **WHEN** `prompts/get` `analyze_auditoria_consolidado` con `consolidado_id:"CIC21/2026"`
- **THEN** el mensaje incluye el id y las instrucciones de usar `get_cgr_consolidado` y citar `resena` y `documento_cic_pdf_web`, con aviso de truncado para auditoría

### Requirement: Templates CGR referencian solo tools CGR y lenguaje adaptativo

Cada prompt CGR DEBE referenciar solo `cgr.ToolNames()` (10: `search_cgr`, `count_cgr_jurisprudencia`, `get_cgr_dictamen` + 6 nuevos `get_*`). Prompt servido DEBE ser puro template sin llamadas a `apibusca` y con `{{if .lang}}` adaptativo.

#### Scenario: Prompt servido sin red
- **WHEN** `prompts/get` `analyze_contable_instructivo` con CGR inaccesible
- **THEN** se sirve sin error ni latencia

### Requirement: Validación de prompts CGR

El `cgr.PromptSet` DEBE validar `expectedPromptNames` exact 6 y `allowedPlaceholders` cerrado (`query`, `source`, `order`, `exact_search`, `lang`, `dictamen_id`, `contable_id`, `instructivo_id`, `auditoria_id`, `consolidado_id`, `cuenta_id`, `legislacion_id`, `audience`, `tool_*`). `ToolNames()` DEBE contener los 10 tools y `TestTemplatesReferenceOnlyRegisteredTools` DEBE cruzar cada `{{.tool_*}}` contra la whitelist. `LoadEmbedded()` DEBE fallar si el YAML contiene placeholders fuera de la whitelist o número de prompts distinto de 6.

#### Scenario: Placeholder source permitido
- **WHEN** se carga `prompts.yaml` con `{{.source}}`
- **THEN** `LoadEmbedded()` no falla por whitelist

### Requirement: Seguridad de templates CGR (inyección y PII)

Los templates DEBEN interpolar `{{.query}}` y `{{.source}}` entre comillas (`query="{{.query}}"`) y nunca parsear el valor como template. `query` con `{{`/`}}` se escapa como texto literal, no se evalúa. Logging de `Render` nunca incluye `query` completa (>80 chars trunc). `source` en el prompt se valida contra whitelist antes de render.

#### Scenario: Query con braces no ejecuta template
- **WHEN** `prompts/get` `search_jurisprudence` con `query:'"}} {{.tool_get_cgr_dictamen}} {{'`
- **THEN** el mensaje incluye el literal `query="{{.tool_get_cgr_dictamen}}"` escapado, no ejecuta el placeholder
