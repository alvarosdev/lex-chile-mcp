## Why

README.md mixes four audiences in one 235-line document (casual user, container operator, agent administrator, and API reference), so setup knowledge is buried in prose and there is no dedicated tool reference. The tone is informal while the target audience is Chilean users, who are better served by formal neutral Latin American Spanish. Nothing in the project conventions keeps the docs current, so they drift silently whenever behavior changes.

## What Changes

- Restructure user-facing documentation into three files with distinct roles:
  - **README.md** (rewritten, functional): what the server is, what it can do, how it works at a high level (diagram), what it is NOT (not legal advice — orientation only), data-volume precautions (upstream pacing, truncation, caching), a minimal quickstart, and links to the other two docs.
  - **INSTALL.md** (new, technical setup): prerequisites; three ways to obtain the server (release binary, compile from source, GHCR image); HTTP vs STDIO modes and when to choose each; Podman as primary runtime, Docker as fallback, compose orchestration; per-agent integration using each agent's CLI (Claude Code, Codex CLI, Grok CLI, pi, oh-my-pi); full environment-variable reference (including previously undocumented `MCP_HOST` and `MCP_PATH`); health verification. Style: lean but complete — every command explained in one line, no tangents.
  - **TOOLS.md** (new, technical reference): every MCP tool with parameters, defaults, ID formats, truncation limits, and pacing constraints.
- Rewrite all three docs in formal neutral Latin American Spanish.
- Resolve known doc gaps inside INSTALL.md: unify the container run story (plain `podman run` vs the hardened flags used by `make podman-run`), document compose usage both via `make compose-up/down` and directly, and note that the binary has no `--version`/`--help` flags.
- Add a documentation freshness convention to `openspec/config.yaml` context: every change touching user-observable behavior must review and update README/INSTALL/TOOLS in the same change. **Already applied during capture of this change.**

No code changes. **BREAKING**: none (docs-only; external links into README anchors may shift — accepted).

## Capabilities

### New Capabilities

- `user-documentation`: requirements governing the three user-facing docs — role split per file, formal neutral Latin American Spanish, content requirements (disclaimer, precautions, quickstart, setup, agent CLI integration, tool reference), the no-duplication/linking rule, and the freshness rule tied to the new `openspec/config.yaml` convention.

### Modified Capabilities

- None. Existing specs (`container-deployment`, `release-distributions`, `mcp-server`, etc.) describe runtime behavior, not documentation, and their requirements are unchanged.
