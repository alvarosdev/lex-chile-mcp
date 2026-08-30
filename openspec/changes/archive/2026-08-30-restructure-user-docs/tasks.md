## 1. Preparation
- [x] 1.1 Verify MCP CLI integration syntax for Codex CLI, Grok CLI, and pi from their official documentation (STDIO and HTTP modes where supported; record the exact `... mcp add` command forms for INSTALL.md). Claude Code (`claude mcp add`) and oh-my-pi (`~/.omp/agent/mcp.json`, no CLI) are already verified locally. Verify: each recorded command is traceable to an official docs page.

## 2. TOOLS.md

- [x] 2.1 Write TOOLS.md (formal neutral Spanish): every MCP tool with purpose, parameters with types and defaults, `get_*` identifier formats, full `search_cgr` `source` enum, truncation limits (30k CGR PDF content), and the 3-4s upstream pacing rule. Verify: the documented tool set matches the tools and prompts actually registered by the server (16 guides: 10 BCN + 6 CGR) by cross-checking `internal/tools/` and `internal/prompts/`.

## 3. INSTALL.md

- [x] 3.1 Write INSTALL.md setup sections (formal neutral Spanish): prerequisites (Go 1.27, make, podman/docker, dist-script tools); obtaining the server (Release binary + SHA256 check, `make build`/`make dist` with one-line flag explanations, GHCR pull); HTTP vs STDIO modes with when-to-choose guidance; Podman primary / Docker fallback / compose (via `make compose-up/down` and directly); hardened `make podman-run` flags noted next to the plain run. Verify: every command matches the current Makefile, Dockerfile, `scripts/build-dist.sh`, and `.env.example`.
- [x] 3.2 Write INSTALL.md integration sections: per-agent connection (Claude Code, Codex CLI, Grok CLI, pi via CLI; oh-my-pi via shortest `~/.omp/agent/mcp.json` snippet with no-CLI note), both modes per agent where supported; complete env var table (`MCP_TRANSPORT`, `MCP_HOST`, `MCP_PORT`, `MCP_PATH`, `MCP_AUTH_TOKEN`); health verification; note that the binary has no `--version`/`--help`; four-item troubleshooting (port in use, 401 auth, CGR load timeouts, compose tooling absent). Verify: each agent command matches task 1.1 research or local CLI (`claude mcp add --help`, omp `mcp.json` schema).

## 4. README.md

- [x] 4.1 Rewrite README.md (formal neutral Spanish): what the server is and is not (orientation, not legal advice, verify against official sources); high-level how-it-works diagram (agent → server → BCN/CGR with cache/sanitization/output budget); condensed feature list linking to TOOLS.md; data-volume precautions (pacing, truncation, do not saturate public services); example questions; quickstart of at most three commands; links to INSTALL.md and TOOLS.md. Verify: no duplicated setup/reference content (links only), quickstart ≤ 3 commands, links resolve to existing files/sections.

## 5. Verification

- [x] 5.1 End-to-end consistency pass: execute the README quickstart against the local podman runtime (build or pull, run, `curl http://localhost:8000/health` returns `{"status":"healthy"}`); execute one agent CLI command form locally (`claude mcp add` variant) and remove it after; confirm INSTALL.md's env var table against `.env.example`; run `openspec validate --change restructure-user-docs`. Verify: health check succeeds, no command in the docs fails against the repo's actual files.
