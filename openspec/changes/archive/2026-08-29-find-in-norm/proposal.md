# Proposal: find-in-norm

## Why

Budget-first navigation (see `budget-first-law-delivery`) reaches a target article
in 3–4 tool round trips (summary → section → section → article) because the agent
must walk the folded TOC to discover the section_id of what it is looking for. When
the target is known by number or phrase ("article 1749", "sociedad conyugal"), the
walk is pure latency and protocol overhead. A server-side search inside the
already-cached norm collapses those hops into one call and keeps the retrieval
deterministic (exact-match structural search, no semantic ranking drift).

## What Changes

- **New tool `find_in_norm`**: searches inside one norm (`norm_id`, optional
  `version_date`) for a plain-text query and returns the matching sections with
  their `section_id`, sizes and context snippets — the identifiers the agent needs
  to fetch content with `get_law(section_id=...)`.
- **Matching**: case-insensitive substring over two surfaces — section names and
  content markdown. Matches on section names rank above matches on content.
- **No regex**: queries are treated as literal text. The model never sends a
  regular expression to the server (ReDoS surface eliminated by construction).
- **Output budget applies** (shared `output-budget` capability): at most ~20
  results; content matches carry a ~200-char snippet around the first occurrence;
  the response stays in the low thousands of tokens.
- **Zero new infrastructure**: executes in memory over the `NormaFull` already
  held by the ETag cache (no extra HTTP call, no index, no persistence).

## Capabilities

### New Capabilities
- `find-in-norm`: intra-norm exact search returning navigable section identifiers.

### Modified Capabilities
- (none — `get_law`/`get_law_summary` are untouched by this change)

## Impact

Best-case article lookup flow: `search_laws` → `find_in_norm` → `get_law(section_id)`:
3 calls with ~8K total tokens, down from 4 calls / ~18K tokens after
budget-first-law-delivery (and from 4 calls / ~125K tokens before it). Article
lookup latency drops by ~2 round trips.

## Non-goals

- No cross-norm search (`search_laws` already covers corpus search).
- No regex, fuzzy, or semantic matching — deterministic substring only.
- No pagination of matches (the ~20 result cap plus narrower queries covers the
  practical cases; revisit only if real usage shows otherwise).
- No changes to CGR tools.
