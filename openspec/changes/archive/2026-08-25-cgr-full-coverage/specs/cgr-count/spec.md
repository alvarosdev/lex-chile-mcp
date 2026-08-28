## MODIFIED Requirements

### Requirement: Conteo agregado cross-tipo por query (fix todos)

El servidor DEBE exponer `count_cgr_jurisprudencia` consultando `POST /apibusca/count/todos` (antes `/count/dictamenes`) con `query` (requerido, vacío permitido) y `exact_search` (bool, default false) y devolver `total` (`hits.total.value`) y `buckets` (`aggregations.count_by_type.buckets` con `key` y `doc_count` por tipo: `dictamenes`, `auditoria`, `legislacion`, `contable`, `instructivos`, `consolidados`, `cuentas`, `web`). `hits.hits` es siempre vacío y DEBE ignorarse. Cuando `relation:"gte"` y `value==10000` el texto DEBE advertir cap. El conteo DEBE ser opcional en el flujo del LLM: si timeout/breaker, el prompt DEBE indicar pasar directo a `search_cgr`.

#### Scenario: Conteo municipalidad con 8 buckets
- **WHEN** un cliente llama con `query:"municipalidad"`
- **THEN** la respuesta incluye `total: ~10000 gte` y `buckets` con `dictamenes`, `auditoria`, `legislacion 9557`, `contable 622`, `cuentas 645`, `instructivos 18`, `consolidados 11`, `web 106` (valores observados live)

#### Scenario: Conteo licencias médicas incluye consolidados
- **WHEN** un cliente llama con `query:"licencias médicas"`
- **THEN** la respuesta incluye `buckets` con `consolidados 8` y `auditoria 2831`

#### Scenario: Conteo tolerante a carga
- **WHEN** `POST /count/todos` responde timeout/breaker abierto
- **THEN** la tool devuelve error recuperable y el texto sugiere “count no disponible por carga, usa search_cgr directo con source”

### Requirement: Límites y no leak en conteo

El conteo DEBE validar `query` max 500 (truncar con aviso) y `source` no aplica (siempre `todos`). `old_url` nunca se expone. Logging trunca `query` a 80 chars. `ContentLength >1 MB` se rechaza antes de `Unmarshal`.

#### Scenario: Query larga en count truncada
- **WHEN** `query` de 800 chars
- **THEN** trunca a 500 con aviso y consulta
