## Why

El repositorio fue renombrado en GitHub de `chile-bcn-mcp` a `lex-chile-mcp` (`git@github.com:alvarosdev/lex-chile-mcp.git`). El nombre anterior describe solo la fuente BCN/LeyChile pero el servidor ya expone BCN + CGR (Contraloría) y el roadmap incluye Poder Judicial, Tribunal Constitucional y otras fuentes/ capacidades jurídicas. `lex-chile-mcp` establece un umbrella de dominio (`lex` = derecho) que no enumera instituciones y no requerirá otro rename al agregar fuentes.

## What Changes

- **BREAKING**: `go.mod` `module` pasa de `github.com/alvarosdev/chile-bcn-mcp` a `github.com/alvarosdev/lex-chile-mcp` y se actualizan todos los imports Go (`cmd/`, `internal/...`, `.mockery.yml`).
- **BREAKING**: Directorio del binario `cmd/chile-bcn-mcp/` → `cmd/lex-chile-mcp/` y binario `chile-bcn-mcp`/`chile-bcn-mcp.exe` → `lex-chile-mcp`/`lex-chile-mcp.exe` en `Makefile`, `Dockerfile`, `scripts/build-dist.sh`, `docker-compose.yml`.
- **BREAKING**: Identidad MCP `mcp.Implementation.Name` de `chile-bcn-mcp-server` a `lex-chile-mcp-server` y `Title` a `Lex Chile MCP Server`; `Instructions` y mensajes de log actualizados.
- **BREAKING**: Imagen y artefactos `chile-bcn-mcp:local` / `ghcr.io/.../chile-bcn-mcp` → `lex-chile-mcp:local` / `ghcr.io/alvarosdev/lex-chile-mcp` (el workflow `publish.yml` ya usa `${{ github.repository }}` por lo que sigue al rename; se actualiza título del release).
- Actualizar y simplificar `README.md` al nuevo nombre y al estado actual del proyecto: título `Lex Chile MCP Server`, banner `> formerly chile-bcn-mcp`, descripción corta del scope (Leyes vía LeyChile/BCN + dictámenes CGR, roadmap lex/PJUD) y sección de uso básica no-técnica para **podman**, **docker** y **binarios** (`dist.zip` + `go build`); mover detalle técnico avanzado (env vars completas, agent matrix Claude/Codex/Hermes, prompts/table, release process) a docs secundarias o secciones colapsadas. Incluir disclaimer breve (proyecto comunitario, no afiliado, no producción).
- Actualizar toda la documentación restante (`openspec/specs/**`, `openspec/changes/archive/**` referencias históricas) y contratos de versión (`internal/version` ldflags `-X` path, `VERSION` handling) al nuevo module path.
- Actualizar `.git` remote `origin` a `git@github.com:alvarosdev/lex-chile-mcp.git` (ya realizado).
- Añadir nota de compatibilidad en `README.md` (`> formerly chile-bcn-mcp` y sección de migración `go get`) durante al menos 6 meses para SEO y usuarios existentes.

## Capabilities

### New Capabilities

Ninguna — este cambio no introduce comportamiento observable nuevo.

### Modified Capabilities

- `mcp-server`: cambia identidad reportada (`Name`/`Title`/`Instructions`) y referencias de proyecto.
- `versioning`: cambia el path de `ldflags` (`-X .../internal/version.Version`) y nombres de binarios verificados.
- `container-deployment`: cambia nombre de imagen/contenedor/servicio `docker-compose`, `Dockerfile` `ENTRYPOINT` y `ARG VERSION` build path.
- `leychile-search`, `cgr-search`, `cgr-dictamen`, `cgr-count` (y `law-prompts`, `cgr-prompts`): solo actualización de referencias de módulo/proyecto en specs, sin cambio de requisitos funcionales.
