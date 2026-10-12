# PRF02-T19 — API Contract & ADR Consistency Review

## API contract ↔ implementation
`internal/api/contract_test.go` (`TestSwaggerSpecMatchesImplementation`) asserts
the committed `docs/swagger.json` path set exactly matches the router, and
`TestEnvelopeConsistencyAcrossEndpoints` asserts every listed endpoint returns the
standard envelope (`success`/`data`/`error`/`meta`). Result: **PASS**.

Implemented PRF-02 endpoints (all present in `docs/api-contract.md` §9):
- Table Data: `GET /table-data`, `POST/PATCH/DELETE /table-data/rows`, `POST /table-data/export`.
- Table Structure: `GET /table-structure` (T13 dedicated endpoint — matches ADR §12).
- Table Import: `POST /table-import`, `…/{import_id}/validate`, `…/{import_id}/execute` (T15).
- Database reconnect: `POST /connections/{id}/reconnect` (T16).
- Database scripts: `POST /database-export`, `POST /database-import`, `…/{token}/execute` (T17).
- Delete Database: `POST /databases/{database}/delete-preflight`, `POST /databases/{database}/delete` (T18).

Error codes documented in §3 include `IMPORT_*`, `RESTORE_*`, `DATABASE_DELETE_*`,
`DATABASE_IN_USE`, `CONNECTION_TIMEOUT`, `ROW_*`, `EXPORT_*`. No undocumented
destructive endpoint exists. Explicit database binding (`resolveTargetDatabase`)
is enforced on every database-scoped handler.

Status: **PASS**.

## ADR-010 consistency
| Decision | Implementation | Status |
|---|---|---|
| Typed `WorkspaceTab` union (query/table-data/table-structure) | `useWorkspaceStore.ts` | CONSISTENT |
| Explicit database binding, never explorer/tab-driven | stores + handlers | CONSISTENT |
| RowIdentity: declared PK, composite, no-PK read-only, no rowid/ctid | `model.StructureColumn`, mutation layer | CONSISTENT |
| Mutation concurrency: optimistic + affected-rows 0/1/>1 | mutation layer + tests | CONSISTENT |
| T13 dedicated structure endpoint | `table_structure.go` + `dbscript` reuse | CONSISTENT |
| Import scope CSV+SQL (XLSX import deferred) + T17 DB import/export base-tables-only | handlers + `dbscript` | CONSISTENT |
| Reconnect database-scoped, fail-closed, no replay | `Manager.Reconnect` + ADR §11a | CONSISTENT |
| Delete Database two-phase, protected policy, no FORCE, SQLite disabled | `database_admin.go` + `delete_database.go` + ADR §17 | CONSISTENT |

No ADR/implementation contradictions found. Where scope was bounded (MySQL
restore disabled; SQLite delete disabled; base-tables-only export), the code, the
API contract and ADR-010 agree.

Status: **PASS**.
