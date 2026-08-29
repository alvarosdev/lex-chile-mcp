# Design: cgr-document-pagination

## Context

`get_cgr_dictamen` (internal/tools/get_cgr_dictamen.go) renders metadata header +
`## Documento Completo` with the sanitized flat document; `CharCount` already
reports its size. CGR responses are cached in plain LRU tiers (light=100/heavy=20,
no ETag). Dictámenes are immutable once published — a property the cursor design
relies on. The `output-budget` capability (from `budget-first-law-delivery`)
defines the shared budget constant and the signaling invariants; this change adds
its linear-pagination pattern.

## Goals / Non-Goals

**Goals:**
- One pagination primitive reused by any flat-document CGR tool later.
- Page computation as a pure function of the sanitized text (deterministic,
  cacheable per document).
- Typical dictámenes (≤ budget) completely unchanged experience except a
  `part 1 of 1` signal.

**Non-Goals:**
- No hierarchical splitting of dictámenes (no tree exists).
- No extension to other `get_cgr_*` tools (measurement decides a follow-up).
- No content transformation — pagination only slices the existing sanitized text.

## Decisions

### D1 — Page map computed once, pure function

`paginate(doc string) []page` where each page is a `[start,end)` rune range:
advance by budget chars, round the end outward to the next `\n\n` boundary (search
within a bounded margin — e.g. up to 2K chars past the cut; if none found, cut at
the budget anyway so a pathological no-paragraph document cannot produce unbounded
pages). Pure and deterministic: identical inputs produce identical parts, so a
`part=2` call after a `part=1` call always continues exactly where part 1 ended.
*Alternative rejected*: byte-offset continuation tokens — dictámen immutability
makes plain indices safe and the `part` number is friendlier for an LLM caller.

### D2 — Metadata on part 1 only; range header on all parts

Part 1: full metadata header + document body + trailing range signal. Parts > 1:
`# {dictamen_id} — part N of M` + body + signal. Rationale: metadata is ~1K tokens
of pure repetition across parts; the range header keeps context (which document,
where we are) in every part. `structuredContent` mirrors the same scope: full
metadata + `documento_completo` of the part on part 1; part number, range and body
on continuation parts.

### D3 — Validation and error contract

`part < 1` or `part > total` → argument error stating the valid range
(`part must be between 1 and 3`) without re-querying upstream (the cached
document already determines the total). Note: the fetch itself happens before
validation of the upper bound because the total is only known from the document —
validation of `part < 1` happens before any fetch.

### D4 — Where the primitive lives

`internal/cgr/pagination.go` (package-level, no client dependency): `Paginate(doc
string) Pages` with `Pages` exposing count, range lookup and slicing. The tool
handler uses it after sanitization. This keeps `internal/tools` thin and lets
consolidados/auditorías adopt it unchanged if the measurement follow-up says so.

### D5 — Reuses the shared budget constant

The page size is the same `maxContentChars` constant from `output-budget`
(exported from a shared location in `internal/tools` or a tiny shared package —
decided at implementation, keeping one constant across BCN and CGR is the
requirement).

## Risks / Trade-offs

- **Page size vs budget**: metadata + page body on part 1 can slightly exceed the
  content-only budget. Accepted: metadata is bounded (~1K tokens) and the budget
  is defined over the content body (same as BCN).
- **Paragraph margin drift**: rounding outward makes pages slightly under-budget;
  worst-case drift is the margin (2K chars). Fine.
- **Immutable assumption**: if CGN ever rewrites a published dictamen, page
  boundaries shift between calls. Mitigated by the LRU cache short-term; the range
  signal (`chars 1–X of Y`) makes any drift visible to the agent.
