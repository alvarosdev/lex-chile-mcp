## Purpose

Despliegue del servidor lex-chile-mcp en contenedores: imagen OCI multi-arquitectura publicada en GHCR, ejecutada como usuario no-root, configurable por entorno y con healthcheck, más validación automática de los cambios del repositorio.

## Requirements

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

### Requirement: Ejecución como usuario no-root

El proceso del servidor dentro del contenedor DEBE ejecutarse con un usuario sin privilegios, no con root.

#### Scenario: Verificación de usuario en contenedor
- **WHEN** se ejecuta un comando dentro del contenedor en ejecución
- **THEN** el usuario efectivo no es `root`

### Requirement: Configuración por entorno en el contenedor

El contenedor DEBE respetar las mismas variables de entorno que el binario nativo (`MCP_TRANSPORT`, `MCP_HOST`, `MCP_PORT`, `MCP_PATH`, `MCP_AUTH_TOKEN`). En el contenedor, el host de escucha por defecto DEBE ser `0.0.0.0` para permitir el acceso desde fuera del contenedor, y el puerto expuesto DEBE ser `8000`.

#### Scenario: Defaults de contenedor
- **WHEN** el contenedor se inicia sin variables de entorno
- **THEN** el servidor escucha en `0.0.0.0:8000` y responde en `/mcp`

#### Scenario: Override de entorno
- **WHEN** el contenedor se inicia con `MCP_PORT=9000` y `MCP_AUTH_TOKEN=secret`
- **THEN** el servidor escucha en el puerto 9000 y exige el token en las requests HTTP

### Requirement: Healthcheck del contenedor

El contenedor DEBE poder ser verificado por su endpoint de salud: el proceso DEBE responder `GET /health` con estado healthy y el contenedor DEBE incluir las herramientas necesarias para que un healthcheck por HTTP funcione sin herramientas externas.

#### Scenario: Healthcheck del contenedor
- **WHEN** se consulta `GET /health` en el puerto del contenedor
- **THEN** responde `200 OK` con estado `healthy`

### Requirement: Validación automática en CI

Todo push y pull request al repositorio DEBE ejecutar automáticamente la compilación y los tests del proyecto; si algún test o análisis estático falla, el estado del workflow DEBE ser de fallo y bloquear la integración.

#### Scenario: Cambios válidos
- **WHEN** un push o PR contiene cambios que compilan y pasan todos los tests
- **THEN** el workflow de CI finaliza con éxito

#### Scenario: Cambios que rompen tests
- **WHEN** un push o PR introduce un cambio que hace fallar algún test o el análisis estático
- **THEN** el workflow de CI finaliza en fallo

### Requirement: Contenedor endurecido
Las configuraciones de despliegue provistas (`docker-compose.yml` y los targets podman del Makefile) DEBEN ejecutar el contenedor endurecido: sistema de archivos raíz de solo lectura (con un tmpfs para `/tmp`), capacidades Linux mínimas (`cap_drop: ALL`) y `no-new-privileges`. El servidor NO DEBE necesitar escritura en disco ni archivos de configuración externos para operar.

#### Scenario: Rootfs de solo lectura
- **WHEN** el contenedor se ejecuta con la configuración provista y sin `config/` en el filesystem
- **THEN** el sistema de archivos raíz es read-only y el servidor arranca y responde sin escrituras a disco

#### Scenario: Capacidades mínimas
- **WHEN** el contenedor se ejecuta con la configuración provista
- **THEN** el contenedor corre con `cap_drop: ALL` y `no-new-privileges: true`

#### Scenario: Operación sin config externa
- **WHEN** el contenedor se ejecuta con `readOnlyRootfs: true` y sin montar `config/`
- **THEN** el servidor atiende `GET /health` y `prompts/list` correctamente

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
