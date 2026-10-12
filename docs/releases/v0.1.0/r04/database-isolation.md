# DataDeck v0.1.0 — Database Isolation

## Explicit binding
Every database-scoped handler resolves the target through `resolveTargetDatabase`:
an explicit request `database` wins, else the profile default; a server-level
PostgreSQL profile with neither is rejected (`DATABASE_REQUIRED`). There is **no**
fallback to the Explorer selection, the active tab, or a bootstrap database.

## Pool identity
`PoolKey{ConnectionID, Database}` — PostgreSQL pools are per `(connection,
database)` (PRF-01). A pool can never be reused for a different database. MySQL/
SQLite keys degenerate to the profile's single database/file.

## Verified isolation (α/β gate)
Disposable `alpha`/`beta` databases under one server profile; E2E asserts:

| Operation | Evidence |
|---|---|
| Browse α, switch Explorer to β | α tab unchanged (`table-data.spec.ts`) |
| Sort/filter α | bound to α |
| CRUD α (insert/update/delete/duplicate) | affects α only |
| Table export α | contains α data, not β (`table-export.spec.ts`) |
| Table import α | bound to menu target (`table-import.spec.ts`) |
| Structure α | bound to α (`table-structure.spec.ts`) |
| Reconnect α while Explorer on β | targets α (`database-menu.spec.ts`) |
| DB export α / restore α | α-scoped; β unchanged (`database-script.spec.ts`) |
| Delete α | β intact; α token rejected for β (`delete-database.spec.ts`) |

No wrong-database read/mutation/deletion was observed → **no BLOCKER**.

## Reconnect / pool lifecycle
- Database-scoped: only the target `(connection, database)` pool is replaced; other
  pools and in-flight queries are untouched (PG integration test).
- Fail-closed; serialized per key (concurrent reconnects share one attempt; no pool
  stacking — unit test proves `opens == 2` for 12 concurrent reconnects).
- No SQL/mutation replay.

## Cross-connection
Per-tab, per-operation binding is enforced; concurrent cross-profile execution is
not a v0.1.0 feature (documented). No supported operation targets the wrong
database.
