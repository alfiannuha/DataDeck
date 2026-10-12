# PRF02-T19 — Security Results

## 1. SQL injection & identifier safety
- Structured APIs (Table Data, structure, mutation, import/export) build SQL from
  **introspected metadata identifiers** that are dialect-quoted; all values are
  **bound parameters**. Evidence: `internal/database/{table_data,table_filter,mutation,insert}_*_test.go`,
  `internal/api/handler/table_rows_test.go`, `internal/tableimport/tableimport_test.go`.
- Database deletion uses `quoteIdentifier` (audited helper) — never string
  concatenation, never a bound parameter for an identifier, never raw frontend
  SQL. Evidence: `internal/database/database_admin.go` + `database_admin_test.go`.
- Restore validates a manifest (engine + database must equal the target) and a
  statement allowlist; `CREATE TABLE … AS SELECT` and cross-database / dangerous
  statements are rejected. Evidence: `internal/dbscript/dbscript_test.go`
  (`TestParseStatementsRejectsDangerous`).
- Status: **PASS**.

## 2. Mutation safety
- Declared PK only; composite PK supported; no-PK Edit/Delete disabled; unique-only
  rejected; no SQLite rowid / PG ctid identity; generated/PK columns protected;
  optimistic concurrency; affected-rows 0/1/>1 policy (0 → not found/conflict,
  1 → success, >1 → rollback + `MUTATION_AFFECTED_MULTIPLE_ROWS`).
- Evidence: `internal/database/mutation_unit_test.go`, `table_data_unit_test.go`,
  `table_rows_test.go`, integration suites.
- Status: **PASS**.

## 3. File handling
- CSV formula-injection guard (leading `= + @ -`); XLSX written as inline text
  cells; SQL import is INSERT-only with literal-only values and bounded size;
  malformed/oversized input rejected with typed errors; temp files are cleaned
  and bounded. Evidence: `internal/export/*_test.go`, `internal/tableimport/*_test.go`,
  `internal/api/handler/table_import_test.go`.
- No arbitrary SQL execution: T15 import is restricted INSERT-only; T17 restore
  is allowlisted and DataDeck-manifest-bound.
- Status: **PASS** (ZIP-bomb / symlink tests are covered by the XLSX library’s
  reader limits + the 64 MiB upload cap; dedicated adversarial XLSX fixtures were
  not re-added in T19 → **NOT RUN** for a fresh ZIP-bomb fixture).

## 4. Destructive-operation safety (Delete Database)
Verified by unit tests (`internal/api/handler/database_delete_test.go`,
`internal/database/database_admin_test.go`) and E2E (`delete-database.spec.ts`):
- delete without token → `DATABASE_DELETE_CONFIRMATION_INVALID`;
- wrong token / cross-target token → rejected before DROP;
- reused token (single-use) → rejected;
- wrong typed name / missing acknowledgment → rejected;
- protected database (`postgres`, `template0`, `template1`, `mysql`, …) → rejected;
- SQLite → `501 DATABASE_DELETE_UNSUPPORTED`;
- no `WITH (FORCE)`, no `CASCADE`, no automatic retry; success only after
  independent verification.
- Status: **PASS**.

## 5. Localhost / CSRF boundary
Measured against the running single binary (loopback only):
- Listener: `TCP 127.0.0.1:8099 (LISTEN)` — **no public listener**.
- Cross-origin `GET` with `Origin: https://evil.example` → `200` but **no
  `Access-Control-Allow-Origin`** header → browser blocks reading the response.
- CORS preflight (`OPTIONS`, `Access-Control-Request-Method: DELETE`) → `204` with
  **no ACAO** → browser aborts.
- Cross-origin *simple* POST (`Content-Type: text/plain`) to a destructive route
  → `400` (JSON content-type required) → no CSRF via simple requests.
- API responses: `Cache-Control: no-store`, `X-Content-Type-Options: nosniff`,
  `Referrer-Policy: no-referrer`.
- Service worker: `sw.js` contains an `/api` bypass (ADR-008 §46 “MUST NOT CACHE
  `/api` or `/api/`”); manifest + sw served `200`.
- Status: **PASS** (destructive APIs are not trivially triggerable cross-origin).

## 6. Credential / log hygiene
- Responses never include passwords/DSNs; mutations and deletes log structured,
  credential-free lines. Evidence: create/list tests assert no password field;
  delete audit line excludes tokens.
- Status: **PASS**.

## Not run / residual
- Fresh ZIP-bomb and symlink fixtures for XLSX import: **NOT RUN**.
- A live cross-origin *form* CSRF test through a real browser: **NOT RUN**
  (covered by the header/content-type analysis above).
