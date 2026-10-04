# PRF-01 Multi-Database E2E, Security & Regression Review

Status: PRF01-T11. End-to-end validation of the PostgreSQL server-level model
against a real server, plus full backend/frontend/distribution regression.

Two contract defects were found and fixed within scope (see Findings):
`DATABASE_REQUIRED` was returned as `VALIDATION_ERROR`, and a dropped database
with a warm pooled connection returned `INTERNAL_ERROR` / `SQL_ERROR` instead of
`DATABASE_NOT_FOUND`.

## 1. Environment

- PostgreSQL 17.11 (local, trust auth), MySQL 8.0.46, SQLite (modernc).
- Distinguishable data per database: `datadeck_alpha.probe=('alpha',11)`,
  `datadeck_beta.probe=('beta',22)`, `datadeck_gamma.probe=('gamma',33)`, each
  with unique tables `only_alpha` / `only_beta` to detect cross-database leaks.
- Server-level profile (`database_name=""`), a profile with an encrypted
  password, a MySQL profile, and a SQLite file profile.
- Application run at current HEAD; credentials never printed.

## 2. Database Discovery

`GET /connections/{id}/databases` on the server-level profile returned exactly
the self-created databases (`datadeck_alpha`, `datadeck_beta`,
`datadeck_gamma`); no-connect databases are excluded. Connection test with an
empty database succeeded via bootstrap.

## 3. Explorer

`GET /connections/{id}/schemas?database=<db>` returned per-database metadata:
- alpha → `[only_alpha, probe]`
- beta → `[only_beta, probe]`
- gamma → `[probe]`

The alpha response contained no `only_beta` table and vice versa: no metadata
leaks across database cache keys. (The frontend keys schema queries by
`["connections", id, "schema", database]`; component tests cover the cache
separation.)

## 4. Query Binding

Executing `SELECT db,v FROM probe` on the server profile with each database
returned the matching marker: alpha→`11`, beta→`22`, gamma→`33`. Wrong-database
execution was not observed.

## 5. Selection Safety

Cross-database isolation held: alpha could not read `only_beta` and beta could
not read `only_alpha` (blocked). At the API boundary the execution authority is
exclusively the request's `database`, so a tab's binding cannot be overridden by
explorer selection. The client-side rule (explorer selection never rebinds an
existing tab; Mod+Enter uses the tab's database) is covered by the T09 component
tests: `schema-explorer.test.tsx`, `editor-panel.test.tsx`, `status-bar.test.tsx`.

## 6. History

Executions against alpha, beta and gamma recorded `database_name` correctly
(history attribution matches the database actually used, including blocked
attempts). Opening history performs no execution (GET only).

## 7. Saved Queries

Saved queries bound to alpha and gamma persisted and returned their original
`database_name`; reopening performs no execution. Legacy (pre-PRF) snippets
remain `database_name = NULL`, never inferred.

## 8. Failure Recovery

| Case | Result |
|---|---|
| Inaccessible database | `400 DATABASE_NOT_FOUND` |
| Dropped database (warm pool) | `400 DATABASE_NOT_FOUND` (fixed; was `INTERNAL_ERROR`) |
| Dropped database (no pool) | `400 DATABASE_NOT_FOUND` |
| Revoked CONNECT (`REVOKE ... FROM PUBLIC`) | `400 DATABASE_CONNECT_DENIED` |
| Server outage | `502 CONNECTION_ERROR` (sanitized) |
| Timeout | covered by `DISCOVERY_TIMEOUT` integration test |
| Stale database list | dropped db absent from discovery until recreated |
| DSN injection attempts (`?sslmode=`, `;DROP`, `../`, `%00`) | all `DATABASE_NOT_FOUND`, never altered the DSN |

Dirty SQL survives recoverable errors: executing is server-side and the client
keeps its editor buffer; no automatic re-execution occurs during recovery. The
fix used a connect probe (no user SQL replay) to classify dropped-database
failures.

## 9. Security

- Encrypted at rest: the plaintext password is absent from `datadeck.db`.
- Passwords never returned: `GET /connections` payloads have no `password`
  field (asserted by an existing credential test as well).
- No secrets in logs: zero occurrences of the test password, `postgres://`, or
  the word `password` in the runtime log; errors are sanitized generic messages.
- DSN injection neutralized (section 8).
- Connection/database isolation verified (sections 3, 5).
- PWA does not cache API responses: `public/sw.js` explicitly skips `/api/` and
  non-GET requests (ADR-008); only the app shell and static assets are cached.

## 10. PostgreSQL Regression

`gofmt` clean; `go vet` clean; unit, `-race`, and `-tags=integration` packages
all `ok` (12 packages); `CGO_ENABLED=0 go build ./...` OK. Real-PG integration
covers discovery, pool keys, dropped-database classification, and API binding.

## 11. MySQL Regression

MySQL 8.0 integration passes with unchanged single-database semantics
(`MultipleDatabases=false`); mismatched database still `VALIDATION_ERROR`.

## 12. SQLite Regression

SQLite integration passes; file-target execution and BIGINT-as-string
serialization unchanged.

## 13. Frontend Regression

`npm run lint`, `npm run typecheck`, `npm run test` (**314 passed**),
`npm run build`, and `npm run build:static` all succeed.

## 14. Distribution Regression

`scripts/build-embedded.sh` produced a CGO-free single binary that served the
embedded app shell (`GET /` → HTML, `/_next/static/*.js` → 200) and
`/api/v1/health` → 200. Packaging/SBOM/release workflows remain CI-verified.

## 15. Performance

- Lazy discovery: new unit test `TestListDatabasesDoesNotRegisterPools` lists
  200 databases and asserts **zero** pools are registered.
- Large list: discovery over ~33 databases completed in ~0.08 s; a
  `ALLOW_CONNECTIONS false` database was correctly excluded.

## 16. Findings

- **HIGH (fixed):** pooled connection to a database dropped with `FORCE`
  surfaced as `INTERNAL_ERROR` / `SQL_ERROR`. Root cause: `postgresSQLError`
  converted connection-class SQLSTATEs (57P01/57P02/57P03/08xxx) to `SQLError`,
  and `postgres_query.go` did not classify acquire failures. Fixed by mapping
  those SQLSTATEs to `ErrConnection`, classifying acquire failures with
  `postgresConnectError`, and having the manager evict and run one connect probe
  (never re-executing user SQL) to recover `ErrDatabaseNotFound` /
  `ErrDatabaseConnectDenied`. Regression test
  `TestPostgresDroppedDatabaseAfterPoolClassified`.
- **MEDIUM (fixed, contract drift):** a PostgreSQL server-level profile with no
  database returned `VALIDATION_ERROR` instead of the documented
  `DATABASE_REQUIRED` (`docs/api-contract.md` §5.6, ADR-009). Fixed and covered
  by HTTP-level tests for query and schema endpoints.
- **LOW (environment):** interrupted integration runs can leave test databases
  (`datadeck_pool_*`, `datadeck_noconn`) on the local server, which then appear
  in discovery. Local-only artifact; not a product defect.
- **LOW (test technique):** PostgreSQL grants `CONNECT` to `PUBLIC` by default,
  so a revoked-access fixture must first `REVOKE CONNECT ... FROM PUBLIC`;
  otherwise the role still connects.

## 17. Severity Summary

- BLOCKER: none (no wrong-database execution found).
- HIGH: 1 — fixed (dropped-database misclassification).
- MEDIUM: 1 — fixed (error-code contract drift).
- LOW: 2 — environment/test-technique notes only.

## 18. Verdict

**PRF01-T11 READY**

Not proceeding to PRF01-T12.
