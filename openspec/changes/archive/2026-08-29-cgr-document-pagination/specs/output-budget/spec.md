## ADDED Requirements

### Requirement: Linear pagination for unstructured documents

Tools that deliver flat, unindexable documents (no title/article tree) MUST honor
the output budget through linear pagination: the document is split into 1-indexed
parts of at most the budget, with boundaries rounded outward to paragraph breaks
(`\n\n`) so no part ends mid-paragraph. Every partial delivery MUST carry a range
signal in the same response stating the part number, the total parts, the char
range shown and the document total, plus the continuation instruction. A part MUST
NOT present itself as the complete document. Reported `char_count` describes the
whole document in every part. Page cursors MAY be plain part numbers when the
document is immutable upstream (published dictámenes).

#### Scenario: Document within budget arrives whole
- **WHEN** the sanitized document fits the budget
- **THEN** part 1 delivers it complete and the signal states `part 1 of 1`

#### Scenario: Boundary rounds to paragraph
- **WHEN** a page boundary would fall inside a paragraph
- **THEN** the cut moves to the nearest paragraph break, even if the page runs slightly under the budget

#### Scenario: Every part carries the range signal
- **WHEN** a client requests any part of a multi-part document
- **THEN** the response states part number, total parts, char range and document total, and how to continue

#### Scenario: char_count stays the document total
- **WHEN** a part shows a fraction of the document
- **THEN** the reported `char_count` describes the complete document, not the part
