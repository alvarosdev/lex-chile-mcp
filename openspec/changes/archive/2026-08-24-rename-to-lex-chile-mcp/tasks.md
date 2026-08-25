## 1. Módulo Go y estructura de directorios

- [x] 1.1 Renombrar `go.mod` module de `github.com/alvarosdev/chile-bcn-mcp` a `github.com/alvarosdev/lex-chile-mcp` y ejecutar `go mod tidy`
- [x] 1.2 Mover `cmd/chile-bcn-mcp/` → `cmd/lex-chile-mcp/` con `git mv` para preservar `git log --follow` y actualizar todos los imports Go (`cmd/lex-chile-mcp/main.go`, `internal/bcn/*`, `internal/cgr/*`, `internal/tools/*`, `internal/prompts/*/prompts.go`, `internal/server/server.go`, `internal/config`, `internal/version`)
- [x] 1.3 Actualizar `.mockery.yml` packages de `github.com/alvarosdev/chile-bcn-mcp/internal/{bcn,cgr}` a `lex-chile-mcp` y regenerar mocks con `make mock` (verificar `*_mock.go` sin diff funcional)

## 2. Build, versión y contenedores

- [x] 2.1 Actualizar `Makefile`: `BINARY := bin/lex-chile-mcp`, `IMAGE := lex-chile-mcp:local`, `CONTAINER := lex-chile-mcp`, `LDFLAGS := -s -w -X github.com/alvarosdev/lex-chile-mcp/internal/version.Version=$(VERSION)` y targets `build`/`run-http`/`run-stdio`
- [x] 2.2 Actualizar `Dockerfile`: comentario header, `ldflags` path, `COPY --from=builder /out/lex-chile-mcp`, `ENTRYPOINT ["lex-chile-mcp"]` y `ARG VERSION` handling
- [x] 2.3 Actualizar `scripts/build-dist.sh`: `BINARY="lex-chile-mcp"`, `go build ... -X .../lex-chile-mcp/... -o "$out" ./cmd/lex-chile-mcp`, `SHA256SUMS.txt` y `dist.zip` layout (`dist/<os>/<arch>/lex-chile-mcp[.exe]`)
- [x] 2.4 Actualizar `docker-compose.yml`: servicio `lex-chile-mcp`, `container_name: lex-chile-mcp-server` y `build.context`
- [x] 2.5 Actualizar `.github/workflows/publish.yml`: `--title "lex-chile-mcp ${{ ... }}"` (IMAGE_NAME ya es dinámico vía `${{ github.repository }}`)

## 3. Identidad del servidor MCP

- [x] 3.1 Actualizar `internal/server/server.go`: comentario package, `mcp.Implementation{Name: "lex-chile-mcp-server", Title: "Lex Chile MCP Server"}`, `Instructions: "Use the lex-chile-mcp tools..."`, `logger.Info("Starting Lex Chile MCP Server")`
- [x] 3.2 Actualizar `internal/version/version.go` comentario `// Overridden via -ldflags "-X github.com/alvarosdev/lex-chile-mcp/..."`
- [x] 3.3 Actualizar `cmd/lex-chile-mcp/main.go` mensajes de log (`API resources loaded`, `Prompts loaded`) sin referencias `chile-bcn`

## 4. Documentación y specs

- [x] 4.1 Reescribir y simplificar `README.md` para `lex-chile-mcp` (< 200 líneas, no técnico): título `# Lex Chile MCP Server` + badges + banner `> formerly chile-bcn-mcp` + párrafo scope (BCN/CGR → lex/PJUD) + disclaimer corto; sección **Cómo correr** con 3 bloques copiables — Podman (`podman build -t lex-chile-mcp:local . && podman run -p 8000:8000 lex-chile-mcp:local`), Docker (`docker build -t lex-chile-mcp:local . && docker run -p 8000:8000 lex-chile-mcp:local`) y Binario (`dist.zip` → `lex-chile-mcp` + `make build`/`./bin/lex-chile-mcp` + `MCP_TRANSPORT=stdio`); healthcheck `curl localhost:8000/health`; tabla mínima 3 env vars (`MCP_PORT`, `MCP_AUTH_TOKEN`, `MCP_TRANSPORT`); 3 ejemplos de uso natural sin tabla exhaustiva de tools/prompts; links a `docs/` para detalle técnico
- [x] 4.2 Actualizar specs vivas `openspec/specs/mcp-server/spec.md`, `openspec/specs/versioning/spec.md`, `openspec/specs/container-deployment/spec.md` (purpose + ldflags path + binario + ghcr path) — deltas ya capturados en `specs/**` de este change, aplicar al sync
- [x] 4.3 Revisar `openspec/specs/leychile-search/`, `cgr-*`, `law-prompts`, `release-distributions` por menciones residuales de `chile-bcn-mcp` en narrativa (sin cambio de requisitos funcionales)
- [x] 4.4 Decidir y documentar version bump (`v0.10.0` continuidad vs `v0.1.0` rebrand) y actualizar `VERSION` en el PR de release

## 5. Verificación

- [x] 5.1 `grep -r "chile-bcn-mcp" --exclude-dir=.git --exclude-dir=openspec/changes/archive --exclude-dir=graphify-out --exclude-dir=bin --exclude-dir=dist` debe dar 0 hits (o solo `formerly chile-bcn-mcp` y `archive/` si se preserva historia)
- [x] 5.2 `make check` (build+vet+test) en verde; `make mock` sin diff; `go vet ./...` sin `missingkey`
- [x] 5.3 `make build && bin/lex-chile-mcp` (HTTP) y `MCP_TRANSPORT=stdio bin/lex-chile-mcp` reportan `lex-chile-mcp-server` + versión correcta en `initialize`; `curl localhost:8000/health` → `healthy`
- [x] 5.4 `bash scripts/build-dist.sh 0.0.0-test && unzip -l dist.zip` lista `linux/amd64/lex-chile-mcp` etc. y `strings dist/linux/amd64/lex-chile-mcp | grep 0.0.0-test`
- [x] 5.5 `podman build -t lex-chile-mcp:local . && podman run -d -p 8000:8000 lex-chile-mcp:local && curl localhost:8000/health` (o `make podman-build`/`make compose-up`)
