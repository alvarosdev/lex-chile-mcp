# Proposal: cgr-document-pagination

## Why

CGR dictámenes are delivered whole by `get_cgr_dictamen` (`documento_completo`
field, unbounded). Unlike norms, a dictamen is a flat document with no addressable
title/article tree, so the hierarchical drill-down of `budget-first-law-delivery`
cannot apply. Large dictámenes (and the larger consolidados/auditorías family)
need the linear counterpart: budget-bounded pages with explicit ranges, reusing
the shared `output-budget` capability.

## What Changes

- **`part` argument on `get_cgr_dictamen`**: 1-indexed page of the sanitized
  document. Part 1 (default) covers metadata + the first ~100K chars of document
  body; each subsequent part continues.
- **Paragraph-safe cut**: page boundaries round outward to paragraph breaks
  (`\n\n`) — never a mid-paragraph cut that could read as the document's end.
- **Range signaling on every part** (HTTP `206 Partial Content` style):
  `Document: part 2 of 3 · chars 90,001–180,000 of 240,000 · continue with
  part=3`. No part may masquerade as the complete document (output-budget
  no-silent-truncation invariant).
- **`char_count` reports the whole document** in every part; metadata travels on
  part 1 only (subsequent parts repeat only the range header).
- **Document immutability**: dictámenes do not change after publication, so the
  page cursor is a plain `part` number — no continuity tokens.
- **Measurement task first**: real corpus sizes for dictámenes and the
  consolidados/auditorías family are measured before extending the pattern to
  other `get_cgr_*` tools (declared follow-up, not scope).

## Capabilities

### New Capabilities
- (none)

### Modified Capabilities
- `output-budget`: adds the linear-pagination pattern for unstructured documents
  alongside the existing hierarchical map degradation.
- `cgr-dictamen`: `get_cgr_dictamen` gains `part`, page-bound delivery and range
  signaling.

## Impact

Dictámenes over ~100K chars (~25K tokens) become multi-page reads with explicit
progress instead of single unbounded responses. Typical dictámenes (~10–50K chars)
are unaffected: part 1 delivers the complete document with a
`part 1 of 1` signal.

## Non-goals

- No hierarchical decomposition of dictámenes (they have no tree to exploit).
- No extension to consolidados/auditorías/instructivos in this change — only the
  measurement that decides whether a follow-up is needed.
- No changes to search tools (`search_cgr_*` are already page-bounded by the
  upstream API).
- No `max_chars` override (inherited from `output-budget`).
