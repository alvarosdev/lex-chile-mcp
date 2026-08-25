## Context

El repositorio ya fue renombrado en GitHub a `alvarosdev/lex-chile-mcp` y el remote `origin` local apunta a `git@github.com:alvarosdev/lex-chile-mcp.git`. Quedan 106 ocurrencias de `chile-bcn-mcp` en código, docs, workflows y specs que mantienen la identidad vieja. La arquitectura es modular (`internal/bcn` vs `internal/cgr`, `prompts/bcn` vs `prompts/cgr`, `api.resources.yaml` por recurso) y soporta agregar `internal/pjud` sin cambios estructurales; solo el nombre umbrella estaba desalineado. El proyecto está en `v0.0.9` (pre-1.0, sin `ghcr.io/...:latest` estable publicado según README), por lo que un rename con breaking de `go.mod` es de bajo costo.

Ver `proposal.md` para motivación y alcance.

## Goals / Non-Goals

**Goals:**
- Alinear identidad completa (module path, binario, imagen OCI, `mcp.Implementation`, docs, specs) a `lex-chile-mcp` en un único PR atómico.
- Preservar redirect de GitHub y añadir nota `formerly chile-bcn-mcp` para no romper SEO/discoverability.
- Mantener `internal/bcn` y `internal/cgr` sin renombrar (son nombres de dominio, no de repo).

**Non-Goals:**
- No agregar nuevas fuentes (PJUD) ni tools en este cambio.
- No cambiar comportamiento funcional (mismos endpoints, mismos prompts, mismos tests salvo imports).
- No migrar historial de GHCR viejo ni re-taggear imágenes ya publicadas.

## Decisions

### Decision: Rename completo vs rebrand solo docs

Elegido **rename completo** (`go.mod` + `cmd/` + `Makefile`/`Dockerfile`/`scripts` + `server.go` + docs + specs). Alternativa "solo rebrand docs" deja `go.mod` mentiroso para siempre y obliga a otro rename al consolidar PJUD. Costo de rename atómico hoy es 1 PR mecánico; costo diferido es perpetuo. Reversible por redirect de GitHub 90 días.

### Decision: `cmd/lex-chile-mcp` y binario `lex-chile-mcp`

Consistente con `go build ./cmd/lex-chile-mcp` y `ENTRYPOINT ["lex-chile-mcp"]`. Mantiene convención `<repo>/cmd/<bin>/main.go`. `scripts/build-dist.sh` genera `dist/<os>/<arch>/lex-chile-mcp[.exe]` y `SHA256SUMS.txt` filtra por nuevo nombre. Alternativa de mantener binario viejo con alias duplica artefactos sin valor.

### Decision: `mcp.Implementation` Name/Title

`Name: "lex-chile-mcp-server"` (identificador técnico, kebab-case, estable para `initialize`) y `Title: "Lex Chile MCP Server"` (display humano). `Instructions` pasa a `"Use the lex-chile-mcp tools..."`. No se mantiene alias del nombre viejo: el handshake MCP no tiene mecanismo de alias y el redirect de GitHub no aplica a MCP wire protocol.

### Decision: Actualizar `ldflags` en 3 lugares

`Makefile:LDFLAGS`, `Dockerfile: ldflags`, `scripts/build-dist.sh: ldflags` deben usar el nuevo path `github.com/alvarosdev/lex-chile-mcp/internal/version.Version`. Se validó que los 3 son los únicos sitios que inyectan versión; `.mockery.yml` y todos los `import` Go también migran.

### Decision: README simplificado y no-técnico

El `README.md` actual tiene ~570 líneas, mezcla quick-start, deep-dive de `html-to-markdown`, matrix de 3 agentes (Claude/Codex/Hermes) con 4 variantes stdio/HTTP cada uno, tabla de 7 tools con caching ETag/LRU, 14 prompts y release process. Para `lex-chile-mcp` se elige **README corto orientado a "funcionar en 2 minutos"**:
- **Arriba**: badges + 1 párrafo qué es (MCP para derecho chileno: leyes BCN + dictámenes CGR, umbrella `lex` para futuro PJUD) + `> formerly chile-bcn-mcp` + disclaimer 2 líneas.
- **Cómo correr**: 3 bloques copiables sin explicación técnica profunda — **Podman** (`podman build/run`), **Docker** (`docker build/run`), **Binario** (`dist.zip` → `./lex-chile-mcp` y `go build`/`make build`). Un solo ejemplo HTTP `http://localhost:8000/mcp` y uno stdio `MCP_TRANSPORT=stdio ./lex-chile-mcp`; healthcheck `curl /health` como verificación.
- **Config mínima**: tabla de 3 env vars esenciales (`MCP_PORT`, `MCP_AUTH_TOKEN`, `MCP_TRANSPORT`) con defaults, link a `docs/configuration.md` si existe; no matriz completa de agentes ni detalles ETag/circuit-breaker en README.
- **Qué puedes preguntar**: 3 ejemplos de uso natural ("Busca la Ley 21.600", "Qué dice el Art. 1", "Busca dictámenes de CGR sobre...") sin tabla exhaustiva de tools/prompts; link a `docs/tools.md` y `docs/prompts.md` para referencia completa.
- Detalle técnico (env vars completas, prompts por dominio, caching, publish workflow, FAQ extenso) se mueve a `docs/` o secciones `<details>` colapsadas. Objetivo: README < 200 líneas, escaneable en 60s, sin perder capacidad de copy-paste.

 ### Decision: Specs delta por capacidad

## Risks / Trade-offs

- **Go import break**: consumidores con `require github.com/alvarosdev/chile-bcn-mcp` deberán migrar a `lex-chile-mcp`. Mitigación: nota de migración en README (`go get github.com/alvarosdev/lex-chile-mcp`), `formerly` banner, y bump a `v0.10.0`/`v0.1.0` para señalar breaking pre-1.0. GitHub redirect permite `git clone` viejo pero no `go get` con mismatch de `module`.
- **Docs/SEO split**: búsquedas "chile bcn mcp" perderán ranking temporal. Mitigación: `README.md` conserva `formerly chile-bcn-mcp` y keywords, y el repo redirect preserva backlinks.
- **CI/GHCR**: `publish.yml` usa `${{ github.repository }}` por lo que publica automáticamente en `ghcr.io/alvarosdev/lex-chile-mcp`; imágenes viejas `chile-bcn-mcp` quedan huérfanas sin re-tag. Aceptado (pre-1.0).
- **Scope creep**: aprovechar el PR para agregar PJUD. Excluido explícitamente en Non-Goals; este PR es solo rename mecánico verificable con `grep -r chile-bcn-mcp` = 0 (salvo `archive/` histórico opcional).

## Migration Plan

1. **Pre-check**: `grep -r chile-bcn-mcp --exclude-dir=.git --exclude-dir=openspec/changes/archive` lista 106 hits; clasificados por tipo (Go imports, ldflags, Makefile, README, specs).
2. **Codemod atómico** (1 commit): `go.mod` + `go mod tidy`, `git mv cmd/chile-bcn-mcp cmd/lex-chile-mcp`, editar `Makefile`/`Dockerfile`/`scripts/build-dist.sh`/`docker-compose.yml`/`.mockery.yml`/`.github/workflows/publish.yml`/`internal/server/server.go`/`internal/version/version.go` comment/`README.md`/specs vivas. Verificar `make check` (build+vet+test) y `grep -r chile-bcn-mcp` residual.
3. **Verificación**: `make build && bin/lex-chile-mcp --help`, `make dist` produce `dist.zip` con `lex-chile-mcp`, `podman build -t lex-chile-mcp:local . && podman run --rm lex-chile-mcp:local` healthcheck `/health`, `mcp inspector` lista tools sin cambios (7 tools).
4. **Release**: tag `v0.10.0` (o `v0.1.0` si se resetea minor) desde `release/v0.10.0` → `dist.zip` + `ghcr.io/alvarosdev/lex-chile-mcp:0.10.0` + `latest`.

## Open Questions

- ¿Mantener `openspec/changes/archive/**` con `chile-bcn-mcp` literal como registro histórico o reescribir también? Propuesta: no tocar `archive/` (historia inmutable), solo `openspec/specs/**` vivas.
- ¿Bump a `v0.10.0` (continuidad) o `v0.1.0` (señal de rebrand)? Decidir antes del merge de release.
