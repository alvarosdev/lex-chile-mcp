# Tasks: find-in-norm

## 1. Client-side search primitive

- [x] 1.1 Add `NormaMatch`/`SectionMatch` types and `SearchNorma(q NormaQuery, query string)` to `internal/bcn` (LawClient interface + Client + regen mock): pre-order walk matching name (whitespace-collapsed, case-insensitive) and lazily-rendered subtree content; deterministic ranking (names first, then document order). Verify: unit tests with fixture norms covering name match, content match, both, none, and ranking order.
- [x] 1.2 Implement snippet extraction (~200 chars, first occurrence, paragraph-rounded window). Verify: unit test asserting window bounds and no mid-word hard cuts within the margin.

## 2. Tool surface

- [x] 2.1 Create `internal/tools/find_in_norm.go`: args validation (`norm_id` > 0, non-empty query, strict `version_date`), `FindInNormOutput{Results, MatchedTotal}` with omitempty snippet, LLM-first text view from the same data. Verify: handler tests with `MockLawClient` (ctx matched with `mock.Anything`) for happy path, each argument error, not-found, and empty results.
- [x] 2.2 Cap results at `maxResults = 20` with signaled truncation line when `MatchedTotal` exceeds the cap (output-budget invariant). Verify: handler test with a mock returning 25 matches asserting the signal and exactly 20 listed.
- [x] 2.3 Register the tool with a description that teaches the drill pattern (`search_laws` → `find_in_norm` → `get_law(section_id)`). Verify: server registration test (tool listed with input schema).

## 3. Verification

- [x] 3.1 Add a benchmark for `SearchNorma` over the largest fixture norm (guard-rail pattern per project conventions). Verify: benchmark runs without regression flags in `make check`.
- [x] 3.2 Full verification: `make check` (build + vet + test) and `make fmt-check` green.
