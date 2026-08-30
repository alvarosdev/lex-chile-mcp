## Context

README.md (235 lines) serves four audiences at once; the project has no INSTALL or TOOLS reference. Setup facts are scattered or missing: `MCP_HOST`/`MCP_PATH` exist in `.env.example` but are undocumented; the README shows a plain `podman run` while `make podman-run` adds hardened flags (`--read-only --tmpfs /tmp --cap-drop ALL --security-opt no-new-privileges`); compose is only mentioned via `make`. The binary has no `--version`/`--help` flags (unknown flags are ignored; the server starts). Development environment has `claude` and `omp` CLIs installed; `codex`, `grok`, and `pi` are not installed, so their CLI syntax must come from official docs. oh-my-pi has no MCP management CLI — MCP servers are declared in `~/.omp/agent/mcp.json`. See proposal.md — Why for motivation.

## Goals / Non-Goals

**Goals:**

- Three documents at repo root with non-overlapping responsibilities and explicit cross-links.
- Formal neutral Latin American Spanish throughout the product docs; commands and identifiers stay literal.
- A reader can reach a verified, agent-connected server using only INSTALL.md.
- Every command in INSTALL.md carries a one-line "what this does" — detailed where it matters, no tangents.
- Anti-drift rule recorded in `openspec/config.yaml` (already applied during capture) and enforced by the `user-documentation` spec.

**Non-Goals:**

- No code changes (no `--version` flag, no server behavior changes).
- No English versions of the product docs.
- No full agent configuration-file documentation (toml/yaml/json formats) beyond the oh-my-pi exception.
- No migration of existing external README anchors/links (accepted breakage).

## Decisions

- **Three files at repo root** (`README.md`, `INSTALL.md`, `TOOLS.md`), not a `docs/` subdirectory: root-level docs are discoverable on GitHub without navigation and match the user's requested naming. Alternative (`docs/tools.md`) rejected for consistency with `README.md`/`LICENSE` at root.
- **Language split:** product docs in formal neutral Spanish (audience decision); OpenSpec artifacts stay English per existing config policy. The policy governs artifacts, not product docs — no conflict.
- **Content migration map** (current README section → destination):

  | Current README section | Destination |
  |---|---|
  | Tagline + disclaimer line | README (expanded into a "what it is / what it is not" section) |
  | "Qué puede hacer" | README (condensed; details link to TOOLS.md) |
  | "Cómo correr" (podman/docker/binary, ~80 lines) | INSTALL.md |
  | "Conectar tu agente" | INSTALL.md (per-agent CLI sections) |
  | "Ajustes básicos" (3 env vars) | INSTALL.md (5 env vars, complete table) |
  | "Qué puedes preguntar" | README |
  | CGR tools table + pacing | TOOLS.md (pacing also summarized in README precautions) |
  | "Dónde conseguirlo" (GHCR) | INSTALL.md (+ one-line pointer in README) |
  | "Aviso y licencia" | README |

- **README points, INSTALL/TOOLS decide:** quickstart of at most three commands (pull, run, health); every other setup topic is a link. This is the anti-drift mechanism at file level.
- **High-level "how it works"** in README as one small diagram (agent → MCP server → BCN/CGR, noting cache, sanitization, output budget) — orientation, not architecture documentation.
- **Agent integration is CLI-first:** one copy-paste command per agent per mode (`claude mcp add`, `codex mcp add`, `grok mcp add`, pi equivalent). oh-my-pi exception: shortest possible `~/.omp/agent/mcp.json` snippet with an explicit note that no CLI exists. Syntax for codex/grok/pi verified against official docs at implementation time.
- **Container story unified:** INSTALL.md shows the plain run first, then the hardened variant as "what `make podman-run` adds" — one paragraph, no security lecture.
- **Lean style rules:** flags explained in compact tables when more than three; troubleshooting limited to four real failure modes (port already in use, 401 with `MCP_AUTH_TOKEN` set, CGR timeouts under load, compose tooling absent with only podman installed); uninstall in two lines.

## Risks / Trade-offs

- **Agent CLI syntax drifts between versions.** Mitigation: verify each command against official docs during implementation; INSTALL.md states the verified source, not a version pin.
- **Docs now bilingual relative to the codebase** (Spanish docs, English code/issues). Accepted: audience is Chilean users; the split is explicit in config context.
- **Shorter README means less on-page setup detail** for GitHub visitors. Accepted: the quickstart covers the common path; links are one click away.
- **The freshness rule is only as good as its enforcement.** Mitigation: it lives in `openspec/config.yaml` context (inherited by every change) and as a spec requirement with a scenario, so both planning and review surfaces see it.
