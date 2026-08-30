# Instalación — Lex Chile MCP Server

Guía técnica de instalación y conexión de agentes. Esta es la referencia completa de configuración; el [README](README.md) ofrece la descripción general y [TOOLS.md](TOOLS.md) documenta las herramientas.

Hay tres maneras de obtener el servidor (imagen de contenedor, binario de un Release o compilación local) y dos modos de ejecución (HTTP persistente o STDIO). Elija primero el modo:

| Modo | Qué es | Cuándo elegirlo |
|---|---|---|
| **HTTP** | El servidor queda corriendo y escucha en `http://localhost:8000/mcp`. | Varios agentes o clientes lo usarán; quiere controlarlo con `systemd`/servicios; quiere un token de acceso. |
| **STDIO** | El agente lanza el servidor como proceso hijo y habla con él por entrada/salida estándar. | Un solo agente lo usa; no quiere dejar procesos ni puertos abiertos. |

## Requisitos previos

- **Contenedores**: Podman (recomendado) o Docker.
- **Binario de un Release**: nada; el binario es autocontenido.
- **Compilar**: Go 1.27 o superior, `make` y `git`. Para `make dist` se requieren además `python3` y `sha256sum`.

## 1. Obtener el servidor

### Opción A — Imagen de contenedor (recomendado)

```bash
podman pull ghcr.io/alvarosdev/lex-chile-mcp:latest
```

En el registro están las etiquetas `:latest` y de versión (por ejemplo `:0.0.9`), para `linux/amd64` y `linux/arm64`. Consulte las etiquetas vigentes en [Releases](../../releases). Con Docker, el comando es idéntico cambiando `podman` por `docker`.

### Opción B — Binario de un Release

```bash
unzip dist.zip
cd dist
sha256sum -c SHA256SUMS.txt   # verifica la integridad de todos los binarios
./linux/amd64/lex-chile-mcp   # elija su plataforma
```

El archivo `dist.zip` está en [Releases](../../releases) y contiene una carpeta por plataforma:

```
dist.zip
├── linux/{amd64,arm64}/lex-chile-mcp
├── darwin/{amd64,arm64}/lex-chile-mcp
├── windows/{amd64,arm64}/lex-chile-mcp.exe
└── SHA256SUMS.txt
```

Cada binario es autocontenido: los extremos de API y las guías van embebidos en el ejecutable; no necesita archivos adicionales.

### Opción C — Compilar desde el código

```bash
git clone https://github.com/alvarosdev/lex-chile-mcp
cd lex-chile-mcp
make build    # produce bin/lex-chile-mcp
```

`make build` compila un binario estático (`CGO_ENABLED=0`) con rutas recortadas (`-trimpath`) y símbolos depurados (`-s -w`); la versión se inyecta desde el archivo `VERSION`. Para generar el paquete multiplataforma completo (los 6 objetivos y sus sumas, el mismo script que usa CI):

```bash
make dist     # produce dist.zip; requiere python3 y sha256sum
```

## 2. Ejecutar el servidor

### Modo HTTP (servidor persistente)

```bash
./bin/lex-chile-mcp
# INFO ... msg=Listening addr=127.0.0.1:8000 path=/mcp
```

Por defecto escucha en `http://127.0.0.1:8000/mcp` y expone el estado de salud en `http://localhost:8000/health`. La dirección, el puerto, la ruta y la clave se ajustan con variables de entorno (sección [5](#5-variables-de-entorno)).

### Modo STDIO (proceso hijo del agente)

```bash
MCP_TRANSPORT=stdio ./bin/lex-chile-mcp
```

No escucha puertos: habla el protocolo MCP por stdin/stdout. En este modo **el agente lanza el servidor**; no es un proceso que usted deje corriendo a mano. Si lo ejecuta en una terminal verá el diálogo del protocolo: es lo esperado.

## 3. Ejecutar con contenedores

### Podman (preferido)

```bash
# construir la imagen desde el código
podman build -t lex-chile-mcp:local .

# modo HTTP: dejarlo corriendo en el puerto 8000
podman run -d --name lex-chile-mcp -p 8000:8000 lex-chile-mcp:local

# modo STDIO: no use este comando a mano; es el que configura en el agente
podman run --rm -i -e MCP_TRANSPORT=stdio lex-chile-mcp:local
```

| Opción | Qué hace |
|---|---|
| `-d` | Ejecuta en segundo plano. |
| `-p 8000:8000` | Publica el puerto del contenedor en su equipo. |
| `--name lex-chile-mcp` | Nombre fijo para administrarlo (`podman logs`, `podman rm`). |
| `--rm -i` (STDIO) | Elimina el contenedor al cerrarse y mantiene la entrada estándar abierta. No agregue `-t`: la terminal corrompería el protocolo. |
| `-e MCP_TRANSPORT=stdio` | Cambia el modo de ejecución. |

Con clave de acceso (solo tiene efecto en HTTP):

```bash
podman run -d --name lex-chile-mcp -p 8000:8000 -e MCP_AUTH_TOKEN=tu-clave lex-chile-mcp:local
```

Variante endurecida: `make podman-run` añade `--read-only --tmpfs /tmp --cap-drop ALL --security-opt no-new-privileges` — el servidor no escribe en disco (configuración embebida, caché en memoria), por lo que puede ejecutarse con el sistema de archivos en solo lectura y sin privilegios.

Para el modo STDIO con la imagen del registro, reemplace `lex-chile-mcp:local` por `ghcr.io/alvarosdev/lex-chile-mcp:latest`.

### Docker (alternativa)

Los comandos son los mismos cambiando `podman` por `docker`:

```bash
docker build -t lex-chile-mcp:local .
docker run -d --name lex-chile-mcp -p 8000:8000 lex-chile-mcp:local
docker run --rm -i -e MCP_TRANSPORT=stdio lex-chile-mcp:local
```

### Compose

```bash
make compose-up     # detecta podman-compose; si no existe, usa docker compose
make compose-down
```

Directo, sin `make`: `podman-compose up -d` o `docker compose up -d`. El archivo `docker-compose.yml` ya incluye las variables de entorno, el endurecimiento del contenedor y un healthcheck que consulta `/health` cada 30 segundos.

## 4. Conectar su agente

Elija la forma según el modo de ejecución:

- **Servidor ya corriendo (HTTP)**: apunte el agente a `http://localhost:8000/mcp`. Es la única variante que usa `MCP_AUTH_TOKEN`.
- **Agente lanza el servidor (STDIO)**: el comando del agente es `podman run --rm -i -e MCP_TRANSPORT=stdio <imagen>` (contenedor) o la ruta del binario con `MCP_TRANSPORT=stdio`.

Los ejemplos usan la imagen local `lex-chile-mcp:local`; sustitúyala por `ghcr.io/alvarosdev/lex-chile-mcp:latest` si no compiló, o por la ruta del binario descargado.

### Claude Code

```bash
# HTTP (servidor ya corriendo)
claude mcp add --transport http lex-chile http://localhost:8000/mcp

# HTTP con clave
claude mcp add --transport http lex-chile http://localhost:8000/mcp \
  --header "Authorization: Bearer tu-clave"

# STDIO con contenedor (Podman)
claude mcp add --transport stdio lex-chile -- \
  podman run --rm -i -e MCP_TRANSPORT=stdio lex-chile-mcp:local

# STDIO con binario
claude mcp add --transport stdio lex-chile -e MCP_TRANSPORT=stdio -- \
  /ruta/absoluta/lex-chile-mcp

# quitarlo
claude mcp remove lex-chile
```

### Codex CLI

```bash
# STDIO con contenedor
codex mcp add lex-chile -- \
  podman run --rm -i -e MCP_TRANSPORT=stdio lex-chile-mcp:local

# HTTP (servidor ya corriendo)
codex mcp add lex-chile --url http://localhost:8000/mcp

# listar servidores configurados
codex mcp list
```

Si su servidor HTTP usa `MCP_AUTH_TOKEN`, Codex envía el token desde una variable de entorno mediante la clave `bearer_token_env_var` en `~/.codex/config.toml`; es el único ajuste que no cubre la línea de comandos.

### Grok (Grok Build)

```bash
# STDIO con contenedor
grok mcp add lex-chile -- \
  podman run --rm -i -e MCP_TRANSPORT=stdio lex-chile-mcp:local

# HTTP (servidor ya corriendo)
grok mcp add --transport http lex-chile http://localhost:8000/mcp

# HTTP con clave
grok mcp add --transport http lex-chile http://localhost:8000/mcp \
  --header "Authorization: Bearer tu-clave"

# diagnóstico de conexión
grok mcp doctor lex-chile
```

### pi

pi no incorpora soporte MCP (decisión de diseño de su autor). La vía disponible es un adaptador comunitario, `pi-mcp-adapter`, que expone los servidores MCP como una herramienta proxy:

```bash
pi install npm:pi-mcp-adapter   # reinicie pi después de instalar
```

Luego declare el servidor en un archivo `.mcp.json` (proyecto) o `~/.config/mcp/mcp.json` (global):

```json
{
  "mcpServers": {
    "lex-chile": {
      "command": "podman",
      "args": ["run", "--rm", "-i", "-e", "MCP_TRANSPORT=stdio", "lex-chile-mcp:local"]
    }
  }
}
```

Para HTTP, reemplace `command`/`args` por `"url": "http://localhost:8000/mcp"`. El adaptador conecta de forma perezosa: la primera llamada a una herramienta del servidor dispara la conexión.

### Oh My Pi (omp)

omp no tiene comandos de línea para administrar MCP: los servidores se declaran en `~/.omp/agent/mcp.json`. Edite el archivo y reinicie omp:

```json
{
  "mcpServers": {
    "lex-chile": {
      "type": "stdio",
      "command": "podman",
      "args": ["run", "--rm", "-i", "-e", "MCP_TRANSPORT=stdio", "lex-chile-mcp:local"]
    }
  }
}
```

Con binario local, use `"command": "/ruta/absoluta/lex-chile-mcp"` y agregue `"env": {"MCP_TRANSPORT": "stdio"}`. Para un servidor HTTP ya corriendo:

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

Omita el bloque `headers` si el servidor no tiene clave.

## 5. Variables de entorno

Se leen una sola vez al arrancar. La URL completa del endpoint es `http://MCP_HOST:MCP_PORT/MCP_PATH`.

| Variable | Valor por defecto | Descripción |
|---|---|---|
| `MCP_TRANSPORT` | `http` | `http` = servidor persistente; `stdio` = proceso hijo del agente. |
| `MCP_HOST` | `127.0.0.1` | Dirección de escucha (solo HTTP). En contenedor la imagen fija `0.0.0.0`. |
| `MCP_PORT` | `8000` | Puerto de escucha (solo HTTP). |
| `MCP_PATH` | `/mcp` | Ruta del endpoint MCP (solo HTTP). |
| `MCP_AUTH_TOKEN` | *(vacío)* | Exige el encabezado `Authorization: Bearer <token>` en HTTP. En STDIO se ignora. |

## 6. Verificación

```bash
curl http://localhost:8000/health
# {"status":"healthy"}
```

Para una prueba de punta a punta (negociación MCP, listado de herramientas y consultas reales contra BCN, con el servidor corriendo):

```bash
make smoke                # con clave: SMOKE_TOKEN=tu-clave make smoke
```

Nota: el binario no implementa `--version` ni `--help`; las opciones desconocidas se ignoran y el servidor arranca con su configuración por defecto o la de sus variables de entorno.

## 7. Problemas frecuentes

| Síntoma | Causa probable | Solución |
|---|---|---|
| `bind: address already in use` al arrancar | El puerto 8000 está ocupado | Use otro puerto: `MCP_PORT=8080` (y `-p 8080:8000` en el contenedor). |
| `401 Unauthorized` desde el agente | Hay token configurado y falta el encabezado | Agregue `--header "Authorization: Bearer <token>"` o vacíe `MCP_AUTH_TOKEN`. |
| Consultas a Contraloría lentas o con timeout | Carga del servicio público o llamadas seguidas | Respete el pacing de 3-4 segundos; evite `count_cgr_jurisprudencia` bajo carga; use `order="score"` con muchos resultados. |
| `podman-compose: command not found` (y `docker compose` ausente) | Ninguna herramienta de compose instalada | Instale `podman-compose` o Docker Compose, o administre el contenedor con `podman run` directo. |

## 8. Desinstalar

```bash
# contenedor
podman rm -f lex-chile-mcp          # o docker rm -f lex-chile-mcp
podman rmi lex-chile-mcp:local

# binario
rm -rf dist bin                      # borre la carpeta donde lo descomprimió

# registro del agente
claude mcp remove lex-chile          # codex mcp remove lex-chile / grok mcp remove lex-chile
```

En omp y pi, elimine la entrada `lex-chile` del archivo `mcp.json` o `.mcp.json` correspondiente.
