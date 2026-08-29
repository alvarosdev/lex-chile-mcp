# Design: find-in-norm

## Context

The ETag cache (`internal/bcn`) already holds the converted `NormaFull` (structure
tree + sanitized markdown per block) keyed by `(norm_id, version_date)`; a cache
miss goes through the normal fetch-and-convert path and populates it. This design
adds a read-only search over that in-memory tree. No new storage, no upstream
endpoint.

## Goals / Non-Goals

**Goals:**
- One tool call from "I know what I'm looking for" to "I have the section_id".
- Deterministic, literal matching — reproducible results for the same query.
- Response bounded by the shared output budget.

**Non-Goals:**
- No regex, fuzzy or semantic matching (proposal non-goal).
- No cross-norm search, no persistence, no snippet highlighting beyond the first
  occurrence.

## Decisions

### D1 — Two-surface substring match with deterministic ranking

The query is lowercased once; matching lowercases each surface via
`strings.Contains(strings.ToLower(surface), q)`. Surfaces: section name
(`EstructuraPart.N`, whitespace-collapsed as in budget-first rendering) and the
section's content markdown (the subtree render of the corresponding `HtmlBlock`,
reused from the existing renderer so the match sees exactly what the agent would
read). Ranking: name-matches first, then content-matches; within each group,
document order (pre-order walk of the structure). Ties never depend on input
order — the walk is deterministic.
*Alternative rejected*: ranking by match count/density — density invites
cargo-cult queries and is harder to reason about than "names first, then
document order".

### D2 — Snippet: first occurrence, ~200 chars, paragraph-safe

For content matches, locate the first occurrence index, expand a window to ~200
chars, and round the window edges outward to paragraph boundaries (`\n\n`) when
within a small margin. The snippet carries no highlight markers — the agent gets
clean text.

### D3 — Cap at 20 results with signaled truncation

`maxResults = 20`. If the walk found more matches than returned, the response
states it (`Matched 87 sections — showing first 20; narrow the query`). This is
the `output-budget` no-silent-truncation invariant applied to search results.

### D4 — Tool placement and shape

`internal/tools/find_in_norm.go`, registered alongside the other BCN tools with
`bcn.LawClient` injected — it calls `GetNorma` (the cached fetch) and walks the
result; adding `Client.SearchNorma` keeps the handler thin and testable via the
existing `MockLawClient`. Output type mirrors conventions:
`FindInNormOutput{Results []SectionMatch, MatchedTotal int}` with
`SectionMatch{Name, SectionID, CharCount, ArticleCount, Snippet (omitempty)}`;
text view rendered from the same data (LLM-first + structured, same scope).

**Interface note (settled during implementation)**: `SearchNorma` returns a
small result struct — `NormaSearchResult{Matches []NormaMatch, Walked int}` —
instead of a bare slice: the spec's no-match message requires the number of
sections walked, which only the search walk can report.

### D5 — Whitespace-collapsed name matching

Section names arrive with repeated internal whitespace (measured: `Título VII
DE LA FILIACIÓN`). Both the query and the name are whitespace-collapsed before
matching, consistent with the hygiene rendering in budget-first-law-delivery —
otherwise "Título VII DE LA" would fail to match the raw name.

## Risks / Trade-offs

- **Substring false positives** ("1749" matches "17495"): accepted for
  determinism; name matches dominate and the agent sees sizes/snippets to
  disambiguate. Exact-article lookups get help from D5 name matching.
- **Full-tree scan per call**: bounded by the cached norm size (~1.3M chars worst
  case) — a `strings.Contains` pass is sub-millisecond territory; benchmark guard
  in tests.
- **Content-match surface cost**: rendering subtrees for every node on every call
  would be wasteful; the walk renders lazily only for candidate verification.
