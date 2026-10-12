# PRF02-T20 — API Contract & OpenAPI Audit

Verified at HEAD `e3f6c20` via `internal/api/contract_test.go`
(`TestSwaggerSpecMatchesImplementation`, `TestEnvelopeConsistencyAcrossEndpoints`)
and manual comparison of router ↔ `docs/api-contract.md` ↔ frontend clients.
Result: **PASS**.

## Route inventory (implemented == documented == OpenAPI)
| Route | Task | Notes |
|---|---|---|
| `GET /connections/{id}/table-data` | T03 | browse; explicit `database` |
| `POST/PATCH/DELETE /connections/{id}/table-data/rows` | T07/T08/T09 | VALUE/NULL/DEFAULT; optimistic concurrency |
| `GET /connections/{id}/table-structure` | T13 | dedicated; lightweight `/schemas` unchanged |
| `POST /connections/{id}/table-data/export` | T14 | CSV/XLSX/SQL |
| `POST /connections/{id}/table-import[/validate|/{id}/execute]` | T15 | INSERT-only, atomic |
| `POST /connections/{id}/reconnect` | T16 | database-scoped |
| `POST /connections/{id}/database-export` | T17 | streamed `.sql` |
| `POST /connections/{id}/database-import[/{token}/execute]` | T17 | manifest-bound restore |
| `POST /connections/{id}/databases/{database}/delete-preflight` | T18 | token |
| `POST /connections/{id}/databases/{database}/delete` | T18 | confirm + verify |

## Checks
- No undocumented endpoint (contract test enumerates the full set).
- Every response uses the standard envelope; error codes documented in §3
  (`IMPORT_*`, `RESTORE_*`, `DATABASE_DELETE_*`, `DATABASE_IN_USE`,
  `CONNECTION_TIMEOUT`, `ROW_*`, `EXPORT_*`).
- Explicit database binding enforced on all database-scoped handlers; no unsafe
  bootstrap/default fallback (server-level PG requires an explicit database).
- No stale/contradictory response types: frontend OpenAPI types are regenerated
  from `docs/swagger.json` (`npm run generate:api`).
- Capabilities advertised are backed by implementation; unsupported combinations
  return explicit errors (`RESTORE_UNSUPPORTED`, `DATABASE_DELETE_UNSUPPORTED`,
  `IMPORT_FORMAT_UNSUPPORTED`), never silent success.
- No undocumented destructive endpoint.

**PASS.**
