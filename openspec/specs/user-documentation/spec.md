# user-documentation Specification

## Purpose
Define what the three user-facing documents (README.md, INSTALL.md, TOOLS.md) must contain, in which language and tone, how they divide responsibility without duplicating content, and how they stay current as the server evolves.

## Requirements

### Requirement: Role split across the three documents

The user-facing documentation SHALL be split into exactly three files with distinct roles:

- **README.md** — functional documentation: what the server is, what it can do, how it works at a high level, its limits, a minimal quickstart, and links to the other two documents.
- **INSTALL.md** — technical setup documentation: prerequisites, obtaining and building the server, execution modes, container runtimes, agent integration, environment variables, and verification.
- **TOOLS.md** — technical reference: every MCP tool with its parameters, defaults, identifier formats, and output constraints.

The README quickstart SHALL contain at most three commands and SHALL link to INSTALL.md for every setup topic it does not itself cover.

#### Scenario: Reader finds each topic in its designated document
- **WHEN** a reader looks for (a) what the server can do, (b) how to connect Claude Code via its CLI, or (c) the parameters of `search_cgr`
- **THEN** they find (a) in README.md, (b) in INSTALL.md, and (c) in TOOLS.md

#### Scenario: Quickstart stays minimal
- **WHEN** the README shows how to get a running server
- **THEN** it does so in at most three commands and defers compile-from-source, agent configuration, and environment variables to INSTALL.md via links

### Requirement: Language and tone of user-facing documentation

README.md, INSTALL.md, and TOOLS.md SHALL be written in formal, neutral Latin American Spanish suitable for Chilean users. Commands, file names, environment variables, tool names, and identifier formats SHALL remain in their literal form. The documentation SHALL NOT use informal register, voseo, or country-specific slang.

#### Scenario: Tone check
- **WHEN** any of the three documents addresses the reader
- **THEN** it uses formal second person (usted-form or impersonal) with no regionalisms, while keeping all commands and identifiers literal

### Requirement: README communicates purpose, limits, and precautions

README.md SHALL state: what the server is and which upstream sources it queries (LeyChile/BCN and Contraloría/CGR); how it works at a high level (agent → MCP server → upstream sources, with sanitization, caching, and output budgeting); that it is an informational orientation tool and NOT legal advice, with an explicit instruction to verify against official sources; and precautions about data volume (recommended pacing of 3–4 seconds between CGR requests, truncation of large contents, and not saturating public services).

#### Scenario: Disclaimer present
- **WHEN** a reader opens README.md
- **THEN** they find a clearly marked section stating the tool provides orientation on how Chilean law works, is not legal advice, and results must be verified against LeyChile and Contraloría official sources

#### Scenario: Precautions present
- **WHEN** a reader opens README.md
- **THEN** they find the recommended request pacing, the existence of truncation limits for large contents, and the request to avoid saturating the public services

### Requirement: INSTALL.md is the single source of truth for setup

INSTALL.md SHALL enable a user to go from nothing to a working agent connection using only that document. It SHALL cover, in order: prerequisites (Go 1.27, make, podman or docker, and the tools required by the dist script); three ways to obtain the server (release binary from GitHub Releases with SHA256 verification, compiling locally with `make build` and `make dist`, pulling the GHCR image); both execution modes (persistent HTTP server and STDIO child process) with guidance on when to choose each; Podman as primary runtime and Docker as fallback, including compose orchestration both via `make compose-up/down` and directly; integration for Claude Code, Codex CLI, Grok CLI, pi, and oh-my-pi; the complete environment-variable reference (`MCP_TRANSPORT`, `MCP_HOST`, `MCP_PORT`, `MCP_PATH`, `MCP_AUTH_TOKEN`); and health verification (`/health`). It SHALL document the hardened container flags used by `make podman-run` alongside the plain run. Style SHALL be lean but complete: each command carries a one-line explanation; no tangents.

#### Scenario: Zero-to-connected using only INSTALL.md
- **WHEN** a user with no prior setup follows INSTALL.md top to bottom for their container runtime of choice
- **THEN** they end with a verified running server and an agent connected to it, without needing information from README.md or TOOLS.md

#### Scenario: Both transports explained
- **WHEN** a user reads the execution modes section
- **THEN** they can state when to run a persistent HTTP server versus letting the agent spawn a STDIO process, and have copy-paste commands for both

### Requirement: Agent integration documented through each agent's CLI

For each supported agent (Claude Code, Codex CLI, Grok CLI, pi, oh-my-pi), INSTALL.md SHALL provide copy-paste commands using the agent's own CLI (`claude mcp add`, `codex mcp add`, `grok mcp add`, `pi` equivalent) for both connection modes where the agent supports them: HTTP against an already-running server, and STDIO launching the container or binary directly. oh-my-pi has no MCP management CLI, so INSTALL.md SHALL show its minimal configuration file (`~/.omp/agent/mcp.json`) with the shortest possible snippet. INSTALL.md SHALL NOT reproduce full agent configuration-file formats (toml/yaml/json) beyond that exception. Command syntax for agents not installed in the development environment (Codex CLI, Grok CLI, pi) SHALL be verified against their official documentation.

#### Scenario: CLI-first agent setup
- **WHEN** a Claude Code user wants to connect the server over STDIO
- **THEN** INSTALL.md gives a single `claude mcp add` command they can paste, without requiring them to edit any JSON file

#### Scenario: Exception documented honestly
- **WHEN** an oh-my-pi user reads the agent integration section
- **THEN** they find the minimal `mcp.json` snippet and an explicit note that oh-my-pi has no MCP management CLI

### Requirement: TOOLS.md is the technical tool reference

TOOLS.md SHALL document every MCP tool the server exposes: purpose, all parameters with types and defaults, identifier formats for `get_*` lookups, output notes (truncation limits such as the 30k cap on CGR PDF content), the `search_cgr` `source` enum in full, and the upstream pacing constraint. It SHALL match the tools and prompts actually registered by the server (16 guides, 10 BCN + 6 CGR).

#### Scenario: Reference completeness
- **WHEN** a reader opens TOOLS.md and counts the documented tools
- **THEN** the set matches the tools registered by the server, each with parameters, defaults, and constraints

### Requirement: No duplication between documents

Each setup or reference topic SHALL be documented authoritatively in exactly one document (INSTALL.md for setup, TOOLS.md for tool reference). README.md SHALL link instead of restating. Where a fact appears in README.md (e.g., the listen port), the other documents SHALL remain the authoritative source.

#### Scenario: No divergent copies
- **WHEN** a command or environment variable is documented in README.md and also belongs to setup or reference
- **THEN** README.md links to the authoritative section instead of duplicating the full explanation

### Requirement: Documentation stays current with behavior changes

Every change that creates or modifies user-observable behavior (tools, prompts, environment variables, transports, build/publish workflow, release artifacts) SHALL review README.md, INSTALL.md, and TOOLS.md as part of the change and update whichever sections describe the changed behavior. Proposals for new behavior SHALL name the affected document files. This rule SHALL be recorded in `openspec/config.yaml` project context so it is inherited by every future change.

#### Scenario: New tool triggers doc review
- **WHEN** a change adds a new MCP tool
- **THEN** the change also updates TOOLS.md (and README.md feature list if user-visible) before completion

#### Scenario: Convention inherited
- **WHEN** a future change scaffolds its artifacts
- **THEN** the documentation freshness rule appears in the project context the agent receives
