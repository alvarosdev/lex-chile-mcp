## MODIFIED Requirements

### Requirement: Versión reportada del servidor MCP

El servidor SHALL reportar su versión real en `mcp.Implementation.Version` durante el handshake `initialize`, tomada de `internal/version.Version` (inyectada por build desde `VERSION`). El valor reportado SHALL ser la versión sin prefijo `v`; cuando el binario fue compilado sin `ldflags` SHALL reportar `"dev"`. La identidad del servidor SHALL ser `mcp.Implementation.Name == "lex-chile-mcp-server"` y `Title == "Lex Chile MCP Server"`.

#### Scenario: Handshake reporta versión de release
- **WHEN** un cliente MCP envía `initialize` a un binario compilado con `VERSION=0.0.6`
- **THEN** la respuesta contiene `serverInfo.version == "0.0.6"` y `serverInfo.name == "lex-chile-mcp-server"`

#### Scenario: Binario dev reporta dev
- **WHEN** un cliente MCP envía `initialize` a un binario compilado con `go run` sin `ldflags`
- **THEN** la respuesta contiene `serverInfo.version == "dev"` y `serverInfo.name == "lex-chile-mcp-server"`

#### Scenario: Consistencia entre transportes
- **WHEN** el mismo binario se inicia en `stdio` y en `http`
- **THEN** ambos transportes reportan idéntico `serverInfo.version` y `serverInfo.name` para la misma compilación
