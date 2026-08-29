# CGR corpus measurement — 2026-08-29

Live sampling against the real CGR API (via this server, e2e transport).
Rigor constraints: ~8 sequential calls total, 3s pauses between fetches,
no retry loops (per user request — avoid hammering contraloria.cl).

## Dictámenes (search "municipalidad", first 4)

| dictamen_id | char_count |
|---|---|
| OF141961N26 | 8,083 |
| OF158343N26 | 3,897 |
| OF158337N26 | 5,971 |
| D400N26     | 13,342 |

Every sampled dictamen is far under the 100K budget → part 1 of 1 in all
cases; pagination is a no-op for typical dictámenes, exactly as designed.

## Consolidados (search_cgr source=consolidados, first item)

- CIC23/2026 → 5,253 chars. Under budget.

## Auditoría (search_cgr source=auditoria, numeric_doc_id 596)

- `get_cgr_auditoria` **fails with "response too large"** — the CGR client
  response cap rejects the upstream body before any pagination could
  apply. This is a DIFFERENT failure mode (upstream body cap, hard error)
  than oversized-but-retrievable documents.

## Follow-up decisions (out of scope here, per proposal non-goals)

1. Audit the `auditoria` resource response cap (api.resources.yaml / client
   bounded read): large audit documents hard-fail today. Raising the cap
   plus `PaginateStandard` is the natural extension — needs its own change.
2. Consolidados sample under budget → no pagination pressure observed;
   revisit only with a larger sample.
