# PRF02-T20 — Security Readiness

Consolidates T19 `security-results.md` with T20 independent re-verification.
Zero unresolved BLOCKER/HIGH.

## Cross-database isolation (release-blocking gate)
PASS. Proven by e2e (α/β) across browse, sort, filter, insert, update, delete,
duplicate, structure, table export, table import, reconnect, database export,
database restore, database delete. Every spec asserts backend state (not just UI).
No α/β crossover observed.

## Mutation safety
PASS. Declared-PK-only, composite PK, no-PK Edit/Delete disabled, unique-only
rejected, no SQLite rowid / PG ctid, generated/PK protection, optimistic
concurrency, affected-rows 0/1/>1 → not-found/conflict/rollback.

## Destructive-operation safety (Delete Database)
PASS. Verified by unit (`database_delete_test.go`, `database_admin_test.go`),
PG e2e (`delete-database.spec.ts`) and **new MySQL integration**
(`database_delete_mysql_integration_test.go`): no-token/wrong/cross-target/expired/
reused token rejected; protected names rejected; SQLite `501`; no `WITH (FORCE)`;
no auto-retry; success only after verified absence.

## SQL injection / identifier safety
PASS. Structured APIs use metadata-validated dialect-quoted identifiers + bound
values; delete uses the audited `quoteIdentifier`; restore uses a manifest-bound
allowlist (rejects `CREATE TABLE … AS SELECT`, cross-DB, DDL/DML outside the set).

## File handling
PASS for executed cases (CSV formula guard, XLSX inline-text cells, INSERT-only SQL,
bounded uploads, temp cleanup). Fresh XLSX ZIP-bomb/symlink fixtures: NOT RUN
(library limits + 64 MiB cap documented; assessed low risk).

## Localhost / CSRF boundary
PASS (measured against the running single binary): loopback-only listener;
cross-origin GET/preflight have no `Access-Control-Allow-Origin`; simple
cross-origin POST rejected (JSON content-type required); `Cache-Control: no-store`,
`X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`; service worker
bypasses `/api` (ADR-008 §46). No new auth introduced.

## Credential/log hygiene
PASS. No passwords/DSNs in responses or logs; destructive audit lines exclude tokens.

## Residual (non-blocking)
- Fresh adversarial XLSX fixtures: NOT RUN.
- Real-browser cross-origin form CSRF exercise: NOT RUN (covered by header/content-type analysis).
