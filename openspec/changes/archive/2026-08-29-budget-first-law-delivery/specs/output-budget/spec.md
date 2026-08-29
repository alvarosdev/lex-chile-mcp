## Purpose

Transversal contract for bounded, honestly-signaled MCP tool responses. Tools that
retrieve large documents must never deliver a payload the model context cannot
absorb, and must never omit content without declaring the omission and the path to
recover it. Shared by LeyChile (BCN) law delivery and — via a separate change — CGR
document delivery.

## ADDED Requirements

### Requirement: Server-enforced output budget

Every tool response MUST keep its rendered text content under the server output
budget (~100K chars ≈ 25K tokens), a single fixed constant shared by all tools.
The budget bounds the content body of the response; metadata headers and tables of
contents are bounded by their own folding rules so the total stays well under the
budget. The budget is fixed server-side: no caller parameter can raise it.

#### Scenario: Norm within budget returns complete
- **WHEN** a tool retrieves content whose rendered size is under the budget
- **THEN** the content is delivered complete in that single response

#### Scenario: Budget is one constant for all tools
- **WHEN** any tool composes a response larger than the budget would allow
- **THEN** the same constant governs the degradation, regardless of tool or upstream source

### Requirement: No silent truncation

A tool response MUST NOT cut content without declaring it in the same response.
Any omission of retrievable content MUST be accompanied by (a) an explicit signal
line stating what was omitted and its total size, and (b) the mechanism to retrieve
it (identifier or cursor). A response that shows a fragment must never look like
the complete document.

#### Scenario: Fragment is always marked
- **WHEN** a response includes only part of an available document or subtree
- **THEN** the response contains a signal line naming the total size and how to continue

#### Scenario: No fragment masquerades as complete
- **WHEN** the model reads a degraded response
- **THEN** the response text distinguishes delivered content from omitted content without ambiguity

### Requirement: Degradation to a navigable map

When the requested content exceeds the budget, the tool MUST degrade the response
to a navigable map of the content instead of truncating it: structure outline with
retrieval identifiers and per-node sizes, plus the total size signal. The
degradation rule applies recursively: if a single node (e.g. one section) still
exceeds the budget, that node degrades to the map of its own children. The unit of
delivery is always a node delivered whole.

#### Scenario: Oversized document degrades to map
- **WHEN** a client requests content larger than the budget
- **THEN** the response contains the outline with identifiers and sizes, not a partial body
- **AND** states the total size and instructs how to drill down

#### Scenario: Oversized section degrades recursively
- **WHEN** a client requests a section whose content alone exceeds the budget
- **THEN** the response maps that section's children with their identifiers and sizes

#### Scenario: Small node still delivered whole
- **WHEN** the drill-down reaches a node that fits the budget
- **THEN** that node's content is delivered complete, never paginated or cut

### Requirement: Declared completeness

Any response that omits content MUST report `char_count` of the real total scope
requested (the whole norm or the whole section), not of the fragment shown. The
sum of everything the client has read must be auditable against the declared
totals. The path to omitted content (section identifiers or continuation cursors)
MUST travel inside the response that omits it.

#### Scenario: char_count reflects the real total
- **WHEN** a response degrades to a map because content exceeds the budget
- **THEN** the reported size describes the full requested scope

#### Scenario: Omitted content is recoverable from the response
- **WHEN** a response omits part of a document
- **THEN** the identifiers needed to retrieve the omitted part are present in that same response
