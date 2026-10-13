# DataDeck v0.1.0 — R05 Database Isolation

Disposable fixtures: PostgreSQL `alpha`/`beta` (server-level profile), MySQL
disposable DB, SQLite temp file. No production data.

## PostgreSQL α/β (release-blocking gate)
| Operation | Result |
|---|---|
| Discovery + selection | databases listed; per-database lazy metadata |
| Independent QueryTabs | α and β tabs keep their own binding |
| Explorer switching | never retargets an open tab |
| TableData browse/sort/filter | α bound to α |
| CRUD (insert/update/delete/duplicate) | affects only the target DB |
| Table import/export | bound to the menu target; export contains α data only |
| Reconnect | targets the menu's database while Explorer is on β |
| DB export/restore | α-scoped; non-empty target → 409 |
| Delete database | α deleted, β intact; α token rejected for β |

No wrong-database read/mutation/deletion observed → **no BLOCKER**. Evidence:
`table-data.spec.ts`, `table-export.spec.ts`, `table-import.spec.ts`,
`table-menu.spec.ts`, `database-menu.spec.ts`, `database-script.spec.ts`,
`delete-database.spec.ts`; PG integration pool-key/reconnect tests.

## MySQL
Connection, browse, CRUD, import/export, reconnect covered by integration tests;
**live delete** verified by the T20 integration test. Restore disabled (policy).

## SQLite
Connection, browse, CRUD, import/export, reconnect, DB restore covered by unit/
round-trip tests. Delete disabled (policy).

## Cross-engine concurrency
Concurrent cross-profile execution (v0.2.0 Concurrent Multi-Connection Engine) is
**not implemented** → concurrency-specific cross-profile scenarios are **N/A**;
supported per-operation binding is verified above.
