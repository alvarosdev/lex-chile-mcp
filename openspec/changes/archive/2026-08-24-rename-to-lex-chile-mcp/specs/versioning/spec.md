## MODIFIED Requirements

### Requirement: Inyección de versión en tiempo de compilación

El binario SHALL exponer la versión vía `internal/version.Version` (var `string` con valor por defecto `"dev"`), sobreescrita por `-ldflags "-X github.com/alvarosdev/lex-chile-mcp/internal/version.Version=<version>"` en todos los caminos de build (`Makefile:build`, `scripts/build-dist.sh`, `Dockerfile`).

#### Scenario: Build local usa VERSION
- **WHEN** se ejecuta `make build` sin variables extra
- **THEN** el binario `bin/lex-chile-mcp` contiene la versión leída de `VERSION` (sin prefijo `v`)

#### Scenario: Build sin flags reporta dev
- **WHEN** se ejecuta `go run ./cmd/lex-chile-mcp` sin `ldflags`
- **THEN** `internal/version.Version` vale `"dev"` y el servidor reporta `dev` en `mcp.Implementation.Version`

#### Scenario: Build cross-platform inyecta versión
- **WHEN** `scripts/build-dist.sh 1.2.3` compila los 6 targets
- **THEN** cada binario dentro de `dist.zip` reporta `1.2.3` (verificable con `strings` o `initialize`)

### Requirement: Versionado consistente en distribuciones e imagen

Toda distribución (`dist.zip`) e imagen OCI publicadas desde el flujo de release SHALL usar la misma versión derivada del SSOT (`release/v*` → `VERSION` sin `v` → `ldflags` + tag OCI). Un tag `v*` manual o `workflow_dispatch` SHALL propagarse idénticamente a ambos artefactos.

#### Scenario: Release publica tag consistente
- **WHEN** un PR `release/v1.2.0` es mergeado
- **THEN** `dist.zip` contiene binarios que reportan `1.2.0`, la imagen `ghcr.io/alvarosdev/lex-chile-mcp:1.2.0` contiene el mismo binario, y el GitHub Release draft es `v1.2.0`

#### Scenario: Dispatch manual
- **WHEN** el workflow se dispara con `version: 1.2.3`
- **THEN** los mismos artefactos se generan con versión `1.2.3`
