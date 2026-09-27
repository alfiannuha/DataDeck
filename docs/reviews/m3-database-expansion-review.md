# M3 Database Expansion Review

## Executive Summary

DataDeck is genuinely functional across **PostgreSQL, MySQL, and SQLite**
against real servers/files, while the PostgreSQL behavior established in M1/M2
is preserved. All engines pass connection, schema introspection, query
execution, history, and the multi-database switching flows; BIGINT/unsigned
precision is safe; the result grid remains virtualized (22 DOM rows at 100k).
Credential handling is clean. No BLOCKER or HIGH findings remain — **M3 READY**.

## Validation Environment

| Item | Value |
|---|---|
| Date | 2026-09-27 |
| Platform | darwin / arm64 |
| Go | go1.26.5 (module `go 1.23.0`); CGO-free build verified |
| Node / npm | v24.14.0 / 11.9.0 |
| PostgreSQL | 17.11 (Homebrew), started for the review then stopped |
| MySQL | 26.7 (Homebrew; isolated temp datadir on `:3307`) |
| SQLite | target file via `modernc.org/sqlite` (temp files) |
| Repository | Not a Git repository (remote CI never ran) |

## Commands Executed

| Command | Result |
|---|---|
| `gofmt -l .` | PASS (empty) |
| `go vet ./...` / `go vet -tags=integration ./...` | PASS |
| `go test ./... -count=1` | PASS (10 packages) |
| `go test -race ./...` | PASS |
| `go test -tags=integration ./...` (real PG + MySQL; SQLite temp) | PASS (10 packages) |
| `CGO_ENABLED=0 go build ./...` / `GOTOOLCHAIN=go1.23.0 go build` | PASS |
| `npm run lint` / `typecheck` / `test` / `build` | PASS — **160 tests**, 26 files |
| `make check` / `make build` | PASS |
| Playwright multi-engine E2E | PASS (flows A–G) |

## PostgreSQL Review

**PASS.** Regression intact: connection/inspect/query/history and the integration
suite (`TestPostgres*`) all green. Live: schema tree `datadeck_m3z → public`
with `users`/`roles`/`user_roles`, PK/FK/index metadata; query returned BIGINT
`9007199254740993` (string) and `NULL`; tab-bound execution hit the PG
connection.

## MySQL Review

**PASS.** Real MySQL 26.7 integration green (`TestMySQLIntegration`,
`Introspection`, `Query` 8 subtests, API/history). Live UI: database-level tables
(`datadeck_test → users`, `q_types`, …) with PK/FK/index; `SELECT
CAST(18446744073709551615 AS UNSIGNED)` returned the exact string; `NULL`
handled; tabs/history showed the MySQL connection.

## SQLite Review

**PASS.** Target-file tests (unit, temp files) green: introspection
(`main → users`, flat), columns/types, PK/index, query types (INTEGER safe/unsafe,
REAL, TEXT, BLOB base64, NULL), mutations with `changes()`, syntax error, timeout,
truncation. Live UI: SQLite connection shows the file path, flat schema, and
`SELECT 9007199254740993` preserved exact precision. The target file is distinct
from DataDeck's internal store (dedicated test asserts separate pools/files).

## Driver Architecture Review

Capability-focused `database.Connector` (`Name/Open/Ping/Introspect/Execute/
Capabilities`) with a shared `ConnectionManager`; driver-neutral `SQLError`
(`Driver/Code/Message/Position/Syntax`). PostgreSQL is the reference; MySQL and
SQLite conform. No engine-specific result formats.

## Connection Management Review

One pool per connection id, de-duplicated concurrent opens, bounded 5s health
ping, failed pools cleaned up, graceful `CloseAll`. Verified for all three
drivers (unit + live).

## Schema Introspection Review

- PostgreSQL: `pg_catalog`/`information_schema`, schema-qualified.
- MySQL: `information_schema`, database-level tables (no fake schema).
- SQLite: PRAGMA table-valued functions, flat tables, `sqlite_*` filtered.
All return the neutral `model.Database` (Schemas or Tables), PK/FK/indexes
represented.

## Query Engine Review

Shared `POST /query/execute` for all engines; row vs non-row decided by the
database, not SQL text. Timeout, 50 MB truncation, and history apply uniformly.
PostgreSQL command-tag, MySQL `ROW_COUNT()`, SQLite `changes()` supply affected
rows.

## SQL Dialect Review

Dialect follows the **tab's** connection (not the global active one); CodeMirror
switches PostgreSQL/MySQL/SQLite via a `Compartment`. Autocomplete is
connection-scoped (schema query keyed to the tab connection) and hierarchy-aware.
Option quoting is driver-aware (`"` PG/SQLite, `` ` `` MySQL). Live: Select Top
100 generated `FROM "public"."users"`, `` FROM `users` ``, `FROM "users"`
respectively.

## Query History Review

Live: entries for PG/MY/SQLite with correct connection names, SUCCESS, duration,
rows, and SQL; opening loads into a tab bound to the original connection and does
not execute.

## Saved Queries Review

Live: `Cmd+S` saved `E2E saved` (tags `e2e`) from the SQLite tab; the list shows
title/date/connection/tags/SQL; opening re-bound a tab to the SQLite connection
and did not execute. Backend CRUD validated (create/list/get/update/delete,
connection relationship, `ON DELETE SET NULL`).

## Multi-Database E2E

Three tabs bound to PG/MY/SQLite kept their bindings after the global active
connection was switched (no silent rebinding); each executed against its own
engine with correct values (`pg`/`my`/`sq`, BIGINT/unsigned/REAL/NULL). History
recorded each under the right connection. **PASS.**

## Performance Regression

Live 100,000-row query (`generate_series`): `aria-rowcount=100000`, **22 DOM
rows**, ~394 ms to first render. Matches the M2 baseline (~22). **No regression.**

## Security Review

- Frontend: no `localStorage`/`sessionStorage`, no Zustand `persist`, no
  `dangerouslySetInnerHTML`/`eval`, no `console.log`, no password in stores/hooks.
- Backend: no credential fields logged; DSN builders are used only inside
  `Open`; passwords encrypted at rest and never returned by APIs.
- SQLite target paths are explicit and normalized; the target connector never
  defaults to the internal DB; no filesystem-browsing API.
- Cross-connection execution prevented by tab-bound execution plus the run guard.
- Saved/history SQL rendered as text (no HTML).

## CI Review

`.github/workflows/test.yml` defines `backend`, `frontend`, and `integration`
jobs; the integration job runs a `postgres:17` **and** `mysql:8` service with
`test -tags=integration`. SQLite needs no service. **LOCAL PASS.**
**REMOTE CI: NOT VERIFIED** — the repository is not under Git hosting, so GitHub
Actions has never executed remotely.

## Scope Audit

No premature M4 work: no PWA/offline, export system, production packaging, single
binary release, Docker release packaging, or release pipeline were added. M3
stayed within connection/driver/schema/query/editor/history/saved-query features.

## Findings

| ID | Severity | Finding |
|---|---|---|
| F1 | MEDIUM | History/saved-query lists are unpaginated (history capped at backend default 100; saved queries unbounded). |
| F2 | MEDIUM | Remote CI never verified (not a Git repo); hosted execution of the PG/MySQL service jobs is unproven. |
| F3 | LOW | MySQL semantic errors (e.g. 1054 unknown column) map to `SQL_ERROR`, unlike PostgreSQL class-42 `SQL_SYNTAX_ERROR`. |
| F4 | LOW | MySQL `BIT` renders base64 and `TINYINT(1)` as `0/1` (no boolean coercion). |
| F5 | LOW | SQLite declared types are weak (often empty/affinity) and rowid PK has no index. |
| F6 | LOW | Status bar retains the previous result after opening a saved/history query (no auto-exec; status is stale until the next run). |
| F7 | LOW | Local MySQL 26.7 differs from CI `mysql:8`; version-specific behavior (JSON, functional indexes) not cross-checked. |

## Blockers

**None.**

## Risks for M4

1. **Pagination (F1):** add consistent paging for history/saved queries before
   datasets grow.
2. **CI activation (F2):** push to Git hosting and confirm the PG+MySQL
   integration jobs run remotely.
3. **Engine version skew (F7):** validate MySQL 8 specifics if M4 targets
   version-sensitive features.
4. **Export/PWA/packaging** are M4 scope; none exists yet.
5. **Boolean/type rendering (F4/F5):** refine cross-engine value display if the UI
   needs stricter typing.

## Exit Checklist

- [x] PostgreSQL regression PASS
- [x] MySQL real integration PASS
- [x] SQLite integration PASS
- [x] Connection management works for all three
- [x] Schema introspection works for all three
- [x] Query execution works for all three
- [x] BIGINT precision safe
- [x] Timeout/resource protections preserved
- [x] Connection-aware tabs safe
- [x] SQL dialect switching works
- [x] Autocomplete connection-scoped
- [x] Query history UI works
- [x] Saved queries work
- [x] Frontend quality gates pass
- [x] Backend quality gates pass
- [x] Multi-driver E2E passes
- [x] Credentials protected
- [x] No BLOCKER
- [x] No unresolved HIGH issue making M4 unsafe

## Final Status

**M3 READY**
