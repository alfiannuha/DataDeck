# PRF-01 — Final PostgreSQL Multi-Database Readiness Review

Independent release review (PRF01-T12). All results below were reproduced first
hand against the code at `0e35a6d`; previous task reports were not assumed.

## 1. Executive Summary

PRF-01 is release-ready. The PostgreSQL server-level model is implemented as
specified, discovery is lazy, pools are keyed by (connection, database), schema
introspection and query execution are database-bound, and a 90-execution
selection-safety stress run produced **zero** cross-database executions. The
M6 → PRF migration is additive and non-destructive, with IDs, encrypted
credentials, history and saved queries preserved; rollback to the M6 binary was
exercised. Backend (gofmt/vet/unit/race/integration/CGO), frontend
(lint/typecheck/314 tests/build/static), the embedded single binary, the browser
release flow and the 5-target release build all pass. No BLOCKER and no
unresolved HIGH remain.

## 2. Architecture Result

- PostgreSQL profile = server; `database_name` optional (empty = no default).
- `PoolKey{ConnectionID, Database}` with per-connection LRU cap
  (`MaxPoolsPerConnection`, default 16); discovery uses temporary,
  unregistered pools that are closed immediately (`manager.go` `ListDatabases`).
- Bootstrap candidates (`postgres` → login-named DB) are used **only** for
  discovery/connection test, never silently for user execution.
- ADR-009 recorded; layering (handlers → manager/connector → repository)
  respected; no unapproved dependency or technology change.

## 3. Connection Profile Result

`normalizeRequest` (`connection.go:404-423`) makes `database_name` optional for
`postgres` only; MySQL/SQLite still require it. A server-level profile with an
empty database connects and is used for discovery. `POST /connections/test`
succeeds with an empty database via bootstrap.

## 4. Database Discovery Result

`GET /connections/{id}/databases` returned exactly the seeded databases
(`datadeck_alpha`, `datadeck_beta`, `datadeck_gamma`) for a server-level profile.
A legacy profile bound to `datadeck_ccm` additionally exposes the sibling
databases without altering its stored `database_name`. `ALLOW_CONNECTIONS false`
databases are excluded. `GET /connections/{id}/databases` returns `501` for
MySQL/SQLite.

## 5. Pool Isolation Result

Unit `TestListDatabasesDoesNotRegisterPools` lists **200** databases and asserts
zero registered pools (lazy). Live discovery over ~33 databases completed in
~0.08 s. Pools are keyed per database, so executions against alpha/beta/gamma do
not share a connection; cross-database reads (`alpha`→`only_beta`,
`beta`→`only_alpha`) are blocked by PostgreSQL itself.

## 6. API Result

- `GET /connections/{id}/databases` (PostgreSQL; discovery).
- `POST /query/execute` accepts optional `database`; precedence request →
  profile default; neither ⇒ `400 DATABASE_REQUIRED`.
- `GET /connections/{id}/schemas?database=` is database-scoped.
- `database_name` persisted/returned on history and saved queries.
- Contract codes verified live: `DATABASE_REQUIRED`, `DATABASE_NOT_FOUND`,
  `DATABASE_CONNECT_DENIED`, `CONNECTION_ERROR`; MySQL mismatch stays
  `VALIDATION_ERROR`.

## 7. Explorer Result

`schema-explorer.tsx` writes the browsing selection to
`activeDatabaseByConnection` only (granted via `setActiveDatabase`); it never
calls `setTabDatabase`. Per-database schemas returned isolated metadata
(alpha `[only_alpha, probe]`, beta `[only_beta, probe]`, gamma `[probe]`),
with no cross-database leakage. Failure surfaces `Retry` and, for
`DATABASE_NOT_FOUND`/`CONNECTION_ERROR`/`BOOTSTRAP_DATABASE_UNAVAILABLE`,
`Refresh databases`.

## 8. Query-Binding Result — Critical Safety Scenario

Server profile with `alpha`, `beta`, `gamma` (distinct markers 11/22/33) and
tabs A→alpha, B→beta, C→gamma. Explorer selection was cycled repeatedly
(schemas + discovery for alpha/beta/gamma) between executions.

- **90 executions across 5 rounds × 6 selections × 3 tabs**
- **Mismatches: 0** (every result came from its tab's database)
- Isolation: alpha cannot read `only_beta`, beta cannot read `only_alpha`
  (`SQL_SYNTAX_ERROR` — relation absent, i.e. blocked).

Client code confirms the invariant: `QueryTab.database` is the execution
authority (`editor-panel.tsx:119` passes `activeTab.database`), the explorer
cannot mutate it, and switching the active tab re-derives
`tabDatabase` (`:44`). The component test
`schema-explorer.test.tsx:595` ("marks the explorer selection without rebinding
existing tabs") and `workspace.integration.test.tsx:310` ("keeps
database-bound tabs independent") assert the same.

## 9. Autocomplete Result

`editor-panel.tsx:48` calls `useSchema(schemaConnectionId, tabDatabase)`, so
autocomplete follows the **tab's** connection and database, not the explorer
selection. `useSchema(connectionId, database, { enabled })` keys the cache by
database.

## 10. History Result

Executions recorded `database_name` matching the database actually used
(including blocked attempts); legacy/pre-PRF rows remain `NULL` and are never
inferred. Opening history performs no execution (GET only).

## 11. Saved-Query Result

Saved queries bound to alpha/gamma round-tripped their `database_name`;
reopening performs no execution. Legacy rows stay `NULL`. Update preserves the
binding.

## 12. Migration Result

Fixture built with the **M6 binary** (`d22d950`):
`schema_migrations=[(1)]`, 3 profiles (PG `database_name=datadeck_ccm`,
MySQL, SQLite), encrypted credential, history, 2 saved queries, PG decryptable.

PRF-01 build against the same store:
- starts, `schema_migrations` → `[(1),(2)]` (single additive migration);
- all three profile IDs preserved exactly;
- encrypted password still decrypts (`SELECT current_database()` →
  `datadeck_ccm`);
- legacy `database_name` unchanged in storage; default execution still uses it;
- discovery exposes `alpha/beta/ccm/gamma` without mutating the profile;
- MySQL and SQLite unchanged (SQLite BIGINT `9007199254740993` intact);
- history and saved queries preserved (legacy rows `database_name=NULL`).

Rollback: the M6 binary started against the upgraded store, read all profiles,
executed PG successfully and listed history/saved queries — additive migration,
no destructive step. Full downgrade compatibility beyond M6 is not claimed.

## 13. PostgreSQL Regression

gofmt clean, `go vet` clean, `go test ./...` (12 pkgs), `go test -race ./...`
(12 pkgs) and `go test -tags=integration ./... -p 1` (12 pkgs, real PG17) all
pass; `CGO_ENABLED=0 go build ./...` and `make check` pass.

## 14. MySQL Regression

MySQL 8.0 integration passes; single-database semantics preserved
(`MultipleDatabases=false`); mismatch returns `VALIDATION_ERROR`; discovery
returns `NOT_IMPLEMENTED`.

## 15. SQLite Regression

SQLite integration passes; file target and BIGINT-as-string serialization
unchanged.

## 16. Security Regression

- Credentials encrypted at rest; response payloads never include a password.
- No DSN/password/`password` occurrences in runtime logs; errors sanitized.
- DSN-injection attempts (`?sslmode=`, `;DROP`, `../`, `%00`) are rejected as
  `DATABASE_NOT_FOUND`; the DSN is built with `url.URL`, not string concatenation.
- Query timeout (default 30 s), 50 MB result cap + `truncated`, loopback bind
  default, and `BIGINT`-as-string preserved.

## 17. Frontend Regression

`npm run lint`, `npm run typecheck`, `npm run test` (**314 passed**),
`npm run build` and `npm run build:static` all succeed.

## 18. Distribution Regression

- `scripts/build-embedded.sh` → CGO-free binary serving the embedded shell
  (`/` HTML, `/_next/static/*.js` 200) and API health 200.
- Browser E2E (`frontend/e2e/release-flow.spec.ts`) against that binary:
  **2 passed** (shell, SQLite connection, query, virtualized grid,
  manifest/service worker, and confirmation that Cache Storage holds **no**
  `/api` responses).
- `scripts/build-release.sh` produced all five target binaries
  (darwin arm64/amd64, linux amd64/arm64, windows amd64); the darwin/arm64
  artifact served the shell and health.
- Docker runtime could not be exercised locally (no Docker daemon); the
  Dockerfile/packaging path is covered by CI workflows. See Known Limitations.

## 19. Performance

Discovery is lazy (200 DBs → 0 pools; ~33 DBs in ~0.08 s), pools are capped per
connection (default 16) with LRU eviction, results are byte-capped, and
execution is timeout-bounded.

## 20. Findings

- **HIGH (fixed in T11, re-verified):** a database dropped while a pool was warm
  previously surfaced as `INTERNAL_ERROR`/`SQL_ERROR`; now classified as
  `DATABASE_NOT_FOUND` (connection-class SQLSTATE mapping + acquire
  classification + one no-SQL connect probe after eviction). Re-tested:
  dropped-database warm-pool returns `DATABASE_NOT_FOUND`; syntax errors still
  return `SQL_SYNTAX_ERROR`.
- **MEDIUM (fixed in T11, re-verified):** server-level profile without a
  database now returns the documented `DATABASE_REQUIRED` (was
  `VALIDATION_ERROR`); contract and integration tests aligned.
- **LOW (new, UX):** changing a tab's *connection* selector
  (`editor-panel.tsx:201`, `setTabConnection`) does not clear the tab's
  `database`, so a stale database name can remain bound after switching
  connections; the backend rejects mismatches safely (MySQL
  `VALIDATION_ERROR`) or reports `DATABASE_NOT_FOUND`, and this is unrelated to
  the explorer-selection invariant. Recommendation (future, non-blocking):
  reset the tab database to the new connection's default on explicit connection
  change.
- **LOW (environment):** interrupted integration runs can leave local test
  databases visible to discovery on a developer machine; not a product defect.

## 21. Blockers

None. No cross-database execution was ever observed; no BLOCKER and no
unresolved HIGH.

## 22. Known Limitations

- Docker image build/run was not executed locally (Docker unavailable);
  CI/release workflows own that gate.
- Query cancellation is frontend-only; no server-side cancel endpoint (ADR/PRD
  scope).
- Tab bindings are transient UI state (by design) and are not restored across
  a full page reload.
- Full downgrade compatibility is only proven for M6 after migration 002;
  future data-transforming migrations would require a backup-restore path.

## 23. Release Impact

PRF-01 expands PostgreSQL from one database per profile to a server model with
discovery and per-tab database binding, with new optional API fields and an
additive migration. Existing M6 profiles, credentials and data remain valid;
MySQL/SQLite are unchanged. Net impact: additive and backward compatible.

## 24. Exit Checklist

| Requirement | Result |
|---|---|
| No BLOCKER | PASS |
| No unresolved HIGH | PASS |
| Database field optional | PASS |
| Discovery works | PASS |
| Correct database binding | PASS (90/90, 0 mismatches) |
| No cross-database execution | PASS |
| Migration proven | PASS (additive; rollback exercised) |
| Credential security preserved | PASS |
| PG regression | PASS |
| MySQL regression | PASS |
| SQLite regression | PASS |
| Frontend regression | PASS (314 tests) |
| Single binary | PASS (embedded + E2E + release artifact) |
| Release-critical CI | PASS (`0e35a6d`: Backend, Backend Integration, Frontend, Browser E2E) |

## 25. Final Status

All release criteria are met. No BLOCKER and no unresolved HIGH remain; the
LOW findings are non-blocking.

PRF01 READY
