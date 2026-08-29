# Design: budget-first-law-delivery

## Context

`get_law` renders a norm as header + global TOC + full Markdown body
(`formatNorma`); `get_law_summary` renders metadata + full structure listing
(`formatNormaSummary`). Neither bounds its output. The norm cache (ETag
revalidating LRU keyed by `(norm_id, version_date)`) already holds the converted
`NormaFull` in memory, and `ContentCharCount` already computes subtree sizes
without building strings — both are prerequisites this design builds on. The
structure tree (`EstructuraPart`, children via `H`, `T` classifies 1=título,
4=párrafo, 6=artículo) is the navigation substrate; every article already has its
own retrievable `section_id`.

## Goals / Non-Goals

**Goals:**
- One shared budget constant and one shared degradation path used by every
  content-bearing BCN tool response.
- Folded TOC rendering that is a pure function of the structure tree + sizes.
- Text and structuredContent describe the same scope in every response.

**Non-Goals:**
- No caller-facing budget parameter (no `max_chars`).
- No persistence, no new HTTP calls, no index structures — everything derives
  from the cached `NormaFull`.
- No changes to `get_law_history` or CGR tools (separate change reuses
  `output-budget`).

## Decisions

### D1 — Budget as a content-body constant, not a response-size check

A single constant `maxContentChars = 100_000` (~25K tokens) gates the *rendered
content body* of a response, not the assembled response length. TOC/header size is
controlled structurally by folding (D2), so the total stays well under budget
without a second mechanism.
*Alternative rejected*: post-hoc truncation of the assembled string — that is
exactly the silent truncation the change forbids.

### D2 — Adaptive TOC folding by child count, ranges are textual

`foldTOC(node)`: if a node has ≤ `maxListedChildren = 20` children, list each
child (`name · section_id · ~chars`); otherwise render one line per child
container with `name | section_id | first–last | ~chars | N articles`, where
first–last are the *textual labels* of the boundary articles of that container
(found by walking to its first and last `T==6` descendants). Textual because
`Artículo 58 BIS`, `TRANSITORIO` and `FINAL` break numeric min–max. The boundary
labels double as the localization mechanism: an agent looking for article 1749
matches it against container ranges and drills one level down.
*Alternative rejected*: hiding articles entirely behind an `expand` parameter —
ranges make expansion unnecessary; the section response already carries the next
level (D3).

**TOC budget chain (added during implementation, verified against the real
Código Civil)**: the by-children fold alone does not bound deep-narrow trees —
the CC nests book→title→§→article with every node under the limit and its
folded TOC still renders ~50K chars. The renderer applies a degradation chain
gated by `maxTOCChars = 12_000`: (1) folded TOC; (2) over budget → compact map
(one line per top-level container plus one ranged line per direct child); (3)
still over → top-level lines only. Measured: the CC map lands at ~2.4K chars
(~600 tokens) at level 2.

### D3 — Section responses carry only the local sub-TOC

`get_law(section_id)` replaces the global `renderStructure` block with the folded
listing of the section's direct children (with their `section_id` — today the text
TOC prints names only, leaving the agent blind). Re-orientation goes through
`get_law_summary` / `structure_only`, which are cache-derived and cheap.

### D4 — Recursive budget degradation on the delivery node

The handler computes `ContentCharCount(blocks)` for the requested scope (whole
norm or section). Over budget → render header + folded TOC of that scope's
children + signal line (`Content: ~1.28M chars — drill with section_id (ids
above)`); `Content` omitted from structuredContent exactly as `structure_only`
does. Under budget → today's behavior. Because the check runs per requested node,
an oversized section degrades identically without a special case. The fold limit
(D2) and the budget (D1) compose: a section that fits the budget but has 200
children still gets a folded (not exhaustive) sub-TOC with its full content.

### D5 — structuredContent scope alignment

`buildGetLawOutput` gains the same scope logic: `Estructura` is the flattened
*subtree* of the requested section (full flatten only for `structure_only=true`);
the `Content` markdown field is dropped from structuredContent (the text view is
the rendering; structured keeps counts and structure). Clean break, no
compatibility shim: clients are LLM agents.
*Alternative rejected*: keeping duplicated `Content` for API stability — it is
~180K chars of wire duplication on a Ley 21.600 and the project convention
("text is a view of the structured data") survives because both views now
describe the same scope.

### D6 — `char_count` semantics: real scope, always

`char_count`/`article_count` keep describing the requested scope (norm or
section), including in degraded responses where they state what was *withheld*.
The existing mirror contract between `ContentCharCount` (bcn) and `renderBlocks`
(tools) must stay in sync — the budget check depends on it.

### D7 — Hygiene piggybacked on the render path

Section-name whitespace collapse lives in the fold/listing renderer (one place).
`Related norms` cap (~10 + total signal) applies only to the text header; the
structured field keeps the complete list. `search_laws` aligns its structured
summary to the same `truncate` the text view uses.

## Risks / Trade-offs

- **Navigation latency**: reaching one article takes 2–3 calls (summary → section
  → article). Accepted: `find-in-norm` (separate change) collapses the hops.
- **Fold hides per-article ids on huge containers**: an agent that needs one
  specific article id must read the container section first. Mitigated by ranges
  (D2) and, later, `find_in_norm`.
- **Breaking structuredContent contract** (subtree `Estructura`, no `Content`):
  accepted per proposal; flagged BREAKING.
- **Constant tuning**: 100K/20 are initial values; both are named constants with
  benchmarks/tests guarding behavior, adjustable in one place.
- **Double articulado trees** (DFL refundidos like the Código Civil, where the
  whole code nests under `ARTÍCULO 2`): folding handles them by uniform rule —
  fidelity to the official tree, no cosmetic restructuring. The first fold level
  of such norms looks unusual; that is the corpus, not a bug.
