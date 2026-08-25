## MODIFIED Requirements

### Requirement: Imagen OCI multi-arquitectura

El repositorio DEBE publicar una imagen OCI del servidor en el registro GHCR bajo el nombre del repositorio (`ghcr.io/alvarosdev/lex-chile-mcp`), compilada para `linux/amd64` y `linux/arm64` bajo el mismo tag. La publicación DEBE gatillarse únicamente por el merge de una rama `release/v<version>` a `main` (o por dispatch manual con versión): el tag de imagen DEBE ser la versión extraída (`<version>`) más `latest`. Un PR cerrado sin merge NO DEBE publicar imagen. El binario dentro de la imagen DEBE llevar embebido el contrato de endpoints (`internal/config/api.resources.yaml` via `//go:embed`) y los prompts (`internal/prompts/*/prompts.yaml` via `//go:embed`); la imagen NO DEBE incluir una capa `config/` ni requerir archivos externos para arrancar.

#### Scenario: Publicación por release versionado
- **WHEN** un PR de `release/v1.2.0` a `main` es mergeado
- **THEN** GHCR recibe las imágenes `ghcr.io/alvarosdev/lex-chile-mcp:1.2.0` y `ghcr.io/alvarosdev/lex-chile-mcp:latest`, ambas con manifest multi-arquitectura que incluye `linux/amd64` y `linux/arm64` y el binario arranca sin `config/` en el filesystem

#### Scenario: Sin merge no hay imagen
- **WHEN** un PR de `release/v1.2.0` a `main` es cerrado sin mergear
- **THEN** no se publica ninguna imagen

#### Scenario: Publicación manual
- **WHEN** un mantenedor dispara el workflow manualmente indicando una versión
- **THEN** las imágenes multi-arquitectura se publican con esa versión y `latest`

#### Scenario: Binario autocontenido en la imagen
- **WHEN** el contenedor se inicia sin volumen `config/` montado y sin `config/api.resources.yaml` en el filesystem
- **THEN** el servidor arranca correctamente usando el contrato embebido en el binario

### Requirement: Versión inyectada en build de imagen

El `Dockerfile` SHALL aceptar `ARG VERSION` y compilar el binario con `-ldflags "-X github.com/alvarosdev/lex-chile-mcp/internal/version.Version=${VERSION}"`. El workflow de publish SHALL pasar `VERSION=${{ needs.version.outputs.version }}` como `build-args` a `docker/build-push-action`. Cuando `VERSION` no se provee, el binario SHALL reportar `"dev"`.

#### Scenario: Build local de imagen usa VERSION
- **WHEN** se ejecuta `podman build --build-arg VERSION=$(cat VERSION | sed 's/^v//') .`
- **THEN** el contenedor resultante reporta esa versión en `initialize`

#### Scenario: Build CI publica versión correcta
- **WHEN** el workflow publica `linux/amd64,linux/arm64` con `VERSION=1.2.0`
- **THEN** ambas arquitecturas reportan `1.2.0`

#### Scenario: Build sin VERSION
- **WHEN** se ejecuta `podman build .` sin `build-arg`
- **THEN** el binario reporta `dev`
