# Tasks: budget-first-law-delivery

## 1. Budget core

- [x] 1.1 Add `maxContentChars = 100_000` constant (single shared location in `internal/tools`) and a `withinBudget(blocks)` helper over `bcn.ContentCharCount`. Verify: unit test asserting the helper mirrors the renderer size for fixture subtrees.
- [x] 1.2 Add recursive degradation to the `get_law` handler: over-budget scope (norm or section) returns header + folded TOC + signal line, `Content` omitted from structuredContent (same shape as `structure_only`). Verify: handler test with a synthetic oversized norm fixture asserting no partial body, signal line present, `char_count` = real total.

## 2. Folded TOC renderer

- [x] 2.1 Implement `foldTOC` in `internal/tools`: children listed up to `maxListedChildren = 20` (`name · section_id · ~chars`), larger sets collapse to one line per container with textual first–last boundary labels, `~chars`, and article count. Verify: unit tests over a synthetic tree covering the three shapes (small, mixed, deeply nested double-articulado).
- [x] 2.2 Replace the exhaustive listing in `formatNormaSummary` (get_law_summary), in `get_law(structure_only=true)` and in the degradation map with `foldTOC`. Verify: `get_law_summary` test on the meganorm fixture asserting rendered text stays in the low-thousands of tokens and preserves document order + section ids.

## 3. Section response scoping

- [x] 3.1 In `formatNorma`, replace the global `renderStructure` block with the folded sub-TOC of the requested section's children, each with its `section_id`. Verify: handler test asserting a section response contains no global index and lists child ids.
- [x] 3.2 Scope `Estructura` in `buildGetLawOutput` to the section subtree (full flatten only for `structure_only=true`); drop the `Content` markdown field from `GetLawOutput`. Update all handler tests and mocks that assert the old shape. Verify: `make check` green.
- [x] 3.3 Keep `char_count`/`article_count` semantics on real requested scope across all response modes (complete, degraded, section). Verify: table-driven test over the three modes against fixture counts.

## 4. Hygiene

- [x] 4.1 Collapse repeated internal whitespace in section names at the fold/listing renderer. Verify: unit test with `"Título VII      DE LA FILIACIÓN"`.
- [x] 4.2 Cap `Related norms` in the text header at ~10 entries with total signal; keep the full list in structuredContent. Verify: handler test with a many-vinculaciones fixture.
- [x] 4.3 Align `search_laws` structured summary to the text-view truncation (no text-truncated + structured-complete duplication). Verify: handler test asserting both views match.

## 5. Contract tests and verification

- [x] 5.1 Add a synthetic meganorm fixture (nested tree, > budget at whole-norm and at one section, BIS/TRANSITORIO labels) used by the degradation tests. Verify: fixture committed under `internal/tools/testdata/`.
- [x] 5.2 Completeness-invariant test suite: every response mode that omits content carries the signal line, real-total `char_count`, and retrievable ids. Verify: suite green in `make check`.
- [x] 5.3 Full verification: `make check` (build + vet + test) and `make fmt-check` green.
