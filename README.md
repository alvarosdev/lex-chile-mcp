# Lex Chile MCP Server

[![Podman](https://img.shields.io/badge/Podman-First-purple?logo=podman)](https://podman.io/)
[![Docker](https://img.shields.io/badge/Docker-Fallback-blue?logo=docker)](https://www.docker.com/)
[![MCP](https://img.shields.io/badge/MCP-Compatible-green)](https://modelcontextprotocol.io/)
[![Go](https://img.shields.io/badge/Go-blue?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Dale a tu IA acceso directo a las leyes chilenas. Pregunta en lenguaje natural y te responde citando la fuente oficial — **LeyChile (BCN)** y **Contraloría (CGR)**.

> ⚠️ Proyecto comunitario, no es de BCN/CGR ni del Estado. Es informativo, no es asesoría legal. Verifica siempre en la fuente oficial.

## ✨ Qué puede hacer

**Leyes y normas (LeyChile):**
* 🔍 Buscar leyes, decretos y resoluciones por palabras — "Ley 21.600", "arriendo"
* 📖 Leer la ley completa en texto claro, con títulos y artículos
* ⚡ Resumen rápido sin abrir todo el texto
* 🕰️ Ver cómo era una ley en una fecha pasada
* 🔗 Ver qué leyes la modificaron y a cuáles modificó

**Contraloría:**
* 🏛️ Buscar dictámenes como en Google


**Para tu IA:**
* 🧠 14 guías listas — analiza, explica simple, compara versiones, revisa si es constitucional
* 💬 Pregunta como hablas — "¿qué dice el artículo 1?" y te responde citando
* 🔌 Funciona con Claude, Codex, Grok y más

## Cómo correr

Elige contenedor (recomendado) o programa solo. Todos responden en `http://localhost:8000/mcp` y `curl http://localhost:8000/health`.

### Podman (preferido)

```bash
# desde tu código (local)
podman build -t lex-chile-mcp:local .
podman run -d -p 8000:8000 --name lex-chile-mcp lex-chile-mcp:local

# desde internet (sin compilar)
podman pull ghcr.io/alvarosdev/lex-chile-mcp:latest
podman run -d -p 8000:8000 --name lex-chile-mcp ghcr.io/alvarosdev/lex-chile-mcp:latest

curl http://localhost:8000/health  # -> {"status":"healthy"}
```

Con clave (opcional):
```bash
podman run -d -p 8000:8000 -e MCP_AUTH_TOKEN=tu-clave lex-chile-mcp:local
# o con GHCR
podman run -d -p 8000:8000 -e MCP_AUTH_TOKEN=tu-clave ghcr.io/alvarosdev/lex-chile-mcp:latest
```

**Si tu agente lo lanza directo** (sin dejarlo corriendo):
```bash
podman run --rm -i -e MCP_TRANSPORT=stdio lex-chile-mcp:local
# GHCR
podman run --rm -i -e MCP_TRANSPORT=stdio ghcr.io/alvarosdev/lex-chile-mcp:latest
# -i mantiene la conexión, sin -t, --rm se borra al cerrar
```

### Docker (alternativa)

```bash
docker build -t lex-chile-mcp:local .
docker run -d -p 8000:8000 --name lex-chile-mcp lex-chile-mcp:local
curl http://localhost:8000/health
# si tu agente lo lanza directo
docker run --rm -i -e MCP_TRANSPORT=stdio lex-chile-mcp:local
```

Con `docker-compose`:
```bash
make compose-up   # levanta con healthcheck
make compose-down
```

### Programa solo (sin contenedor)

No necesitas instalar nada más. Baja el archivo y ejecútalo.

**Descarga** el último `dist.zip` en [Releases](../../releases):
```bash
unzip dist.zip
./dist/linux/amd64/lex-chile-mcp              # queda corriendo en http://127.0.0.1:8000/mcp
MCP_TRANSPORT=stdio ./dist/linux/amd64/lex-chile-mcp  # si tu agente lo lanza directo
```

**O compílalo tú:**
```bash
make build          # crea bin/lex-chile-mcp
./bin/lex-chile-mcp
make dist           # crea dist.zip para los 6 sistemas
```

Trae todo adentro, no necesita otros archivos:
```
dist.zip
├── windows/{amd64,arm64}/lex-chile-mcp.exe
├── linux/{amd64,arm64}/lex-chile-mcp
├── darwin/{amd64,arm64}/lex-chile-mcp
└── SHA256SUMS.txt
```

## Conectar tu agente

**Si dejaste el programa corriendo (http):**
```json
{
  "mcpServers": {
    "lex-chile": {
      "type": "http",
      "url": "http://localhost:8000/mcp",
      "headers": { "Authorization": "Bearer tu-clave" }
    }
  }
}
```
Sin clave, borra la línea `headers`.

**Si tu agente lo abre cada vez (stdio):**
```json
// programa solo
{
  "mcpServers": {
    "lex-chile": {
      "command": "/ruta/absoluta/a/lex-chile-mcp",
      "env": { "MCP_TRANSPORT": "stdio" }
    }
  }
}

// con Podman (sin instalar el programa)
{
  "mcpServers": {
    "lex-chile": {
      "command": "podman",
      "args": ["run", "--rm", "-i", "-e", "MCP_TRANSPORT=stdio", "lex-chile-mcp:local"]
    }
  }
}
```
Para GHCR usa `ghcr.io/alvarosdev/lex-chile-mcp:latest`. Para Docker cambia `podman` por `docker`. La clave no se usa aquí.

**Atajos por asistente:**
```bash
# el agente abre el contenedor con Podman (stdio) — local
claude mcp add --transport stdio lex-chile -- podman run --rm -i -e MCP_TRANSPORT=stdio lex-chile-mcp:local
codex mcp add --transport stdio lex-chile -- podman run --rm -i -e MCP_TRANSPORT=stdio lex-chile-mcp:local
grok mcp add --transport stdio lex-chile -- podman run --rm -i -e MCP_TRANSPORT=stdio lex-chile-mcp:local

# con GHCR (sin compilar) — haz pull primero si quieres
podman pull ghcr.io/alvarosdev/lex-chile-mcp:latest
claude mcp add --transport stdio lex-chile -- podman run --rm -i -e MCP_TRANSPORT=stdio ghcr.io/alvarosdev/lex-chile-mcp:latest
codex mcp add --transport stdio lex-chile -- podman run --rm -i -e MCP_TRANSPORT=stdio ghcr.io/alvarosdev/lex-chile-mcp:latest
grok mcp add --transport stdio lex-chile -- podman run --rm -i -e MCP_TRANSPORT=stdio ghcr.io/alvarosdev/lex-chile-mcp:latest
# Docker: cambia podman → docker

# si ya lo dejaste corriendo (http)
claude mcp add --transport http lex-chile http://localhost:8000/mcp
codex mcp add --transport http lex-chile http://localhost:8000/mcp
grok mcp add --transport http lex-chile http://localhost:8000/mcp
```

## Ajustes básicos

| Qué | Valor inicial | Para qué |
|-----|---------------|----------|
| `MCP_TRANSPORT` | `http` | `http` = queda corriendo, `stdio` = tu agente lo abre |
| `MCP_PORT` | `8000` | Por dónde escucha (solo en http) |
| `MCP_AUTH_TOKEN` | *(vacío)* | Clave para http; en stdio se ignora |

No necesitas otros archivos.

## Qué puedes preguntar

```
Busca la Ley 21.600
→ encuentra el ID 1195666

¿Qué dice el Artículo 1 de la Ley 21.600?
→ te muestra el texto citando la ley

Busca dictámenes de Contraloría sobre "bonos"
→ lista con link al PDF oficial
```

El sistema trae 14 guías que le enseñan a tu IA cómo buscar y citar paso a paso.

## Dónde conseguirlo

Imágenes listas en `ghcr.io/alvarosdev/lex-chile-mcp:0.0.9` y `:latest` (para `linux/amd64` y `linux/arm64`). Si aún no hay release, usa `podman build` de arriba.

## Aviso y licencia

Uso informativo y educativo. No guarda tus datos (solo memoria temporal). No satures los servicios públicos y verifica siempre en la fuente oficial.

Licencia MIT — ver [LICENSE](LICENSE). Créditos de terceros en [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES).
