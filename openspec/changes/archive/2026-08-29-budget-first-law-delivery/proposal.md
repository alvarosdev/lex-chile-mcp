# Proposal: budget-first-law-delivery

## Why

Law retrieval tools deliver unbounded payloads to the LLM. Measured worst case: the
Código Civil (norm_id 172986) renders ~1.28M chars ≈ 320K tokens for a single
`get_law` call — it does not fit any mainstream model context and costs ~$1 per call
at Sonnet pricing. `get_law_summary`, the tool designed as the cheap map, returns
~40K tokens for the same norm because it lists all 2,841 articles in the table of
contents. Section responses drag the full global TOC (~35K tokens) on every call.
These responses are not truncated today — they are simply unbounded, which is worse
for cost and unusable at the extreme.

## What Changes

- **Output budget (server-enforced)**: `get_law` responses never exceed ~100K chars
  (~25K tokens) of rendered content. Norms whose requested content fits the budget
  are returned complete (unchanged contract for the ~95% of norms smaller than that).
- **Signaled downgrade instead of silent truncation**: when the whole norm exceeds
  the budget, `get_law` (without `section_id`) returns header + folded TOC + an
  explicit line (`Content: 1.28M chars — drill with section_id`), never a partial
  body. The same rule applies recursively to an oversized individual section, which
  degrades to a sub-map of its children. The unit of delivery is always a node that
  fits whole.
- **Adaptive folded TOC**: nodes with ≤ ~20 children list them (section_id + size);
  larger nodes collapse into one line per container with `name | section_id |
  first–last textual range | ~chars | N articles`. Textual ranges (not numeric
  min–max) because labels like `Artículo 58 BIS`, `TRANSITORIO`, `FINAL` exist.
  Applies uniformly to `get_law_summary`, the oversized-norm map view and
  `get_law(structure_only=true)` (which today also returns the full article list).
- **Local sub-TOC in section responses**: `get_law(section_id)` lists the section's
  children with their section_ids (today the text TOC shows no ids at all) and stops
  shipping the global index.
- **BREAKING (structuredContent)**: `Estructura` in `get_law` responses is scoped to
  the requested section (subtree) instead of the whole norm; the complete flattened
  structure ships only in `get_law_summary` and `structure_only` responses. The
  rendered content markdown stops being duplicated between TextContent and
  structuredContent.
- **Header hygiene**: `Related norms` capped at ~10 visible entries with the total
  signaled (`Related norms: 342 total — showing first 10`); section names collapse
  repeated internal whitespace (`Título VII      DE LA FILIACIÓN` → single spaces);
  `search_laws` aligns its text and structured views (today: truncated text summary
  + complete structured summary = duplication).
- **Completeness invariants (transversal)**: any response that omits content must
  declare the omission in the same response, report `char_count` of the REAL total,
  and carry the path to the omitted part (section_id). Silent ellipsis is forbidden.

## Capabilities

### New Capabilities
- `output-budget`: transversal contract for bounded, honestly-signaled tool
  responses — the budget value, the signaling invariants, and the rules for
  degrading oversized payloads into navigable maps. CGR document pagination
  (separate change) will reuse this capability.

### Modified Capabilities
- `leychile-search`: `get_law` and `get_law_summary` requirements change — folded
  TOC, budget downgrade, section sub-TOC, structuredContent scoping, header
  hygiene. `search_laws` requirement changes only in view alignment (no API change).

## Impact

Full flow "read article 1749 of the Código Civil": ~125K → ~18K tokens (~7×), 4
round trips. Cost per investigation at Sonnet pricing: ~$0.38 → ~$0.05. The
currently-impossible case (a norm larger than the context window) becomes a ~2.5K
token navigable map.

## Non-goals

- `max_chars` caller override — fixed server budget first; override only if it
  hurts in practice.
- `find_in_norm` (intra-norm search tool) — separate change `find-in-norm`.
- CGR document pagination — separate change `cgr-document-pagination`.
- `get_law_history` budgeting — worst case ~24K tokens (estimated), tolerable;
  no change assigned.
- RAG/embeddings retrieval, LLM-generated digests, semantic compression of citable
  legal text — explored and rejected (literalism of legal retrieval favors
  structural exact-match; digests embed unofficial legal interpretation;
  citable text must arrive intact).
