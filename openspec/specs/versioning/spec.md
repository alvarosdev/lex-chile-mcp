# versioning Specification

## Purpose
Centraliza el versionado del proyecto en una única fuente de verdad (archivo `VERSION` en la raíz) e inyecta esa versión en el binario en tiempo de compilación para que todo artefacto reporte la misma versión sin duplicar literales.

## Requirements

### Requirement: Fuente única de versión

El proyecto SHALL mantener la versión del release en un único archivo `VERSION` en la raíz del repositorio como SSOT. Ningún archivo de código DEBE contener un literal de versión duplicado.

#### Scenario: Edición de versión en un solo lugar
- **WHEN** el mantenedor actualiza la versión (ej. `echo "0.0.7" > VERSION`)
- **THEN** todos los builds posteriores (local, `dist.zip`, imagen OCI) reportan `0.0.7` sin editar otro archivo

#### Scenario: Detección de drift
- **WHEN** se ejecuta `grep -r "0\.1\.0" --include="*.go" --include="Makefile" --include="*.sh"`
- **THEN** no hay matches de literales de versión hardcodeados fuera de `VERSION` y tests

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

### Requirement: Normalización de prefijo v

El sistema SHALL tolerar `VERSION` con o sin prefijo `v` (`v0.0.6` y `0.0.6` son equivalentes) y SIEMPRE reportar la versión sin `v` en `mcp.Implementation.Version`. La normalización SHALL aplicarse tanto en `Makefile` (`sed 's/^v//'`) como en Go (`strings.TrimPrefix(version.Version, "v")`).

#### Scenario: VERSION con prefijo v
- **WHEN** `VERSION` contiene `v0.0.6` y se ejecuta `make build`
- **THEN** el servidor reporta `0.0.6` en el handshake MCP

#### Scenario: VERSION sin prefijo
- **WHEN** `VERSION` contiene `0.0.6`
- **THEN** el servidor reporta `0.0.6` sin alteración

### Requirement: Versionado consistente en distribuciones e imagen

Toda distribución (`dist.zip`) e imagen OCI publicadas desde el flujo de release SHALL usar la misma versión derivada del SSOT (`release/v*` → `VERSION` sin `v` → `ldflags` + tag OCI). Un tag `v*` manual o `workflow_dispatch` SHALL propagarse idénticamente a ambos artefactos.

#### Scenario: Release publica tag consistente
- **WHEN** un PR `release/v1.2.0` es mergeado
- **THEN** `dist.zip` contiene binarios que reportan `1.2.0`, la imagen `ghcr.io/...:1.2.0` contiene el mismo binario, y el GitHub Release draft es `v1.2.0`

#### Scenario: Dispatch manual
- **WHEN** el workflow se dispara con `version: 1.2.3`
- **THEN** los mismos artefactos se generan con versión `1.2.3`
