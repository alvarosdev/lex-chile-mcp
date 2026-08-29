# Tasks: cgr-document-pagination

## 1. Measurement (decides the follow-up)

- [x] 1.1 Measure real corpus sizes: sample dictámenes plus consolidados/auditorías via the CGR search tools; record the distribution (how many exceed ~100K chars). Verify: measurement notes committed with the change (or a small fixture set reflecting the distribution).

## 2. Pagination primitive

- [x] 2.1 Implement `Paginate(doc string) Pages` in `internal/cgr/pagination.go`: budget-sized pages, paragraph-boundary rounding with bounded margin, pure/deterministic; expose count, per-page ranges and slicing. Verify: unit tests covering single-page doc, exact-boundary doc, paragraph rounding, margin fallback (no-paragraph pathological doc), and determinism across calls.
- [x] 2.2 Share the `maxContentChars` budget constant with `internal/tools` (single source; no second constant). Verify: test asserting both packages reference the same value.

## 3. Tool surface

- [x] 3.1 Add `part` argument to `get_cgr_dictamen`: default 1; `part < 1` rejected before fetch; `part > total` rejected with valid range after the cached fetch; metadata + full body on part 1 (`part 1 of 1` signal when it fits), body + range header on continuation parts; `char_count` = whole document always; structuredContent mirrors the part scope. Verify: handler tests with `MockCgrClient` (ctx `mock.Anything`) for part 1 small, part 1 multi-page, part N continuation (no metadata, no overlap), out-of-range part, and existing-contract regressions.
- [x] 3.2 Update the tool description to teach continuation (`continue with part=N`). Verify: registration test.

## 4. Verification

- [x] 4.1 Completeness-invariant tests: every multi-part response carries the range signal and whole-document `char_count`; no part presents itself as complete. Verify: suite green in `make check`.
- [x] 4.2 Full verification: `make check` (build + vet + test) and `make fmt-check` green.
