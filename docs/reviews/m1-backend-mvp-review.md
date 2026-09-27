# M1 Backend MVP Review

## Executive Summary

The DataDeck backend MVP is **complete and verified end to end**. All eight M1
endpoints are implemented and match `PRD.md` and `docs/api-contract.md`; the
standard envelope is uniform; storage, encryption, PostgreSQL connectivity,
schema introspection, query execution (including BIGINT precision, timeout and
the 50 MB result cap) and query history all pass.

Crucially, the PostgreSQL vertical slice was **actually executed against a real
PostgreSQL 17.11 server during this review** (both the `//go:build integration`
suite and a live curl walkthrough), not merely mocked. Integration tests PASS.

No BLOCKER findings. One HIGH risk was identified at the start of the review
("PostgreSQL vertical slice had never run against PostgreSQL") and was
**resolved by running it**. The remaining findings are MEDIUM/LOW and do not
make M2 unsafe.

## Validation Environment

| Item | Value |
|---|---|
| Date | 2026-09-26 |
| Platform | darwin / arm64 (macOS) |
| Go toolchain | go1.26.5; module `go 1.23.0` (also builds with `GOTOOLCHAIN=go1.23.0`) |
| Node.js / npm | v24.14.0 / 11.9.0 |
| PostgreSQL | 17.11 (Homebrew) — installed and started **during this review** for integration tests, then stopped; test DB dropped |
| Repository | Not a Git repository (unchanged from M0) |

## Commands Executed

| Command | Result |
|---|---|
| `gofmt -l .` (backend) | PASS — empty |
| `go vet ./...` | PASS |
| `go vet -tags=integration ./...` | PASS |
| `go test ./... -count=1` | PASS — all 11 packages |
| `go test -race ./...` | PASS |
| `go test -tags=integration ./...` | **PASS against PostgreSQL 17.11** |
| `CGO_ENABLED=0 go build ./...` | PASS |
| `GOTOOLCHAIN=go1.23.0 go build ./...` | PASS |
| Root `make check`, `make build`, `make clean` | exit 0 |
| Live curl vertical slice (server + real PG) | PASS (see PostgreSQL Review) |
| Credential-leakage pattern scan | No secrets found |

`go test ./...` (uncached, `-count=1`) — every package `ok`; integration run —
every integration test and subtest `PASS`.

## Application Bootstrap

- Server starts with config from env; safe default bind **127.0.0.1:8080**
  (verified via `lsof`: `TCP 127.0.0.1:...`). Startup fails fast on missing/
  invalid `ENCRYPTION_KEY`.
- `GET /api/v1/health` returns `{"success":true,"data":{"status":"healthy"},"error":null,"meta":{}}`.
- Structured JSON logging (`slog`); request logs carry method/path/status/bytes/
  duration/request-id only — no headers, query strings or bodies.
- Panic recovery returns a sanitized `INTERNAL_ERROR` envelope (tested).
- Graceful shutdown on SIGTERM: `shutdown_started` → `shutdown_complete`,
  exit 0, port released.

## Storage Review

- `internal/storage` opens the embedded SQLite app store, creates parent dirs,
  runs versioned embedded migrations, validates, and closes.
- **Migrations idempotent** (reopen keeps a single version-1 row), FK cascade /
  set-null behavior tested, `foreign_keys` ON, WAL, `busy_timeout`.
- Repositories (`Connection`, `QueryHistory`, `SavedQuery`) tested against
  isolated temp DBs; `ON DELETE CASCADE` (history) and `ON DELETE SET NULL`
  (saved queries) verified.
- Store closes cleanly; `storage_ready` logged with path.

## Security Review

- **AES-256-GCM** via stdlib; unique `crypto/rand` nonce per encryption;
  versioned payload; tamper/wrong-key/malformed detection tested.
- **Key handling:** `ENCRYPTION_KEY` (32 raw bytes or 64 hex) validated at
  startup; missing/invalid fails fast; key never logged (live log scan) and
  never returned.
- **Plaintext passwords never persisted** — live check found no plaintext in the
  SQLite file; stored value is ciphertext that decrypts back in tests.
- **Passwords never returned** — `ConnectionResponse` omits all credential
  fields; contract test asserts the response body contains no `password`/
  `encrypted_password`/plaintext.
- **No hardcoded production secrets** — pattern scan (private keys, AWS/GitHub
  tokens, password logging) found none; the only match is the log label
  `"decrypt_password"`. `.env.example` holds clearly-labeled examples only.
- Query timeout (context) enforced; 50 MB result cap enforced by bounded
  accumulation; errors sanitized (generic to client, detailed server-side).

## PostgreSQL Review

Executed against PostgreSQL 17.11 (real server):

| Check | Result | Evidence |
|---|---|---|
| Connection test | PASS | `TestConnectionTestIntegration`, `TestPostgresIntegration` |
| Connection persistence | PASS | live create profile via API |
| Connection activation / pool reuse | PASS | shared pool per id (manager tests + live) |
| Close / CloseAll | PASS | `TestManagerClose`, `TestManagerCloseAll`, live shutdown |
| Health timeout | PASS | ping timeout honored (`TestManagerPingTimeout`) |
| Concurrency | PASS | 16-way concurrent open shares one pool (`-race` clean) |
| Connection failure | PASS | unreachable `127.0.0.1:1` → `502 CONNECTION_ERROR` (live) |

Live vertical slice (server + PG): create profile → `CREATE TABLE` (columns `[]`,
`rows_affected 0`) → `INSERT` (`rows_affected 2`) → `SELECT` (types
`INT8/TEXT/JSONB`, BIGINT as string `"1"`/`"2"`, JSONB embedded, `NULL` null) →
`GET /schemas` (`live_check` PK `id`, columns + nullability) → history (4
`SUCCESS`) → `SELECT WHERRE` → `400 SQL_SYNTAX_ERROR` `position 8`.

## Schema Introspection Review

`TestPostgresIntrospectionIntegration` PASS against real PG:
- Schemas/tables (system schemas filtered), columns + data types + nullability,
  composite primary keys, column-by-column foreign keys, index metadata
  (including `idx_users_email`), and `CREATE/DROP SCHEMA` lifecycle all verified
  on a deterministic `users`/`roles`/`user_roles` fixture.
- Live `GET /connections/{id}/schemas` returned the created table with PK and
  column metadata.

## Query Engine Review

| Check | Result |
|---|---|
| SELECT / zero rows | PASS (integration + live) |
| NULL | PASS |
| BIGINT precision (string) | PASS — `"9007199254740993"` |
| UUID / JSON(B) / timestamp / bytea | PASS |
| Non-row statements + rows affected | PASS — `INSERT` → 3 (integration), 2 (live) |
| Syntax error (+ position) | PASS |
| Timeout (`pg_sleep`) | PASS — `context.DeadlineExceeded` |
| Result truncation (50 MB) | PASS — 60k×1KB result truncated |
| Query history SUCCESS/ERROR | PASS — recorded; ERROR recorded on failure |

## API Contract Review

- All eight endpoints audited; every response uses the exact envelope keys
  (`success`, `data`, `error`, `meta`) — enforced by a contract test sweeping
  every endpoint (success and error paths).
- Status codes and error codes match `docs/api-contract.md`.
- Error codes: `VALIDATION_ERROR`, `SQL_SYNTAX_ERROR`, `SQL_ERROR`,
  `QUERY_TIMEOUT`, `QUERY_CANCELED`, `CONNECTION_ERROR`, `INTROSPECTION_ERROR`,
  `INTROSPECTION_TIMEOUT`, `NOT_FOUND`, `INTERNAL_ERROR`, `PAYLOAD_TOO_LARGE`.
- OpenAPI artifact present: `backend/docs/swagger.json` / `.yaml` (OpenAPI
  **3.1.0**, 7 paths, 20 schemas). A test asserts the spec's path set equals the
  implemented routes and that no credential identifiers appear.
- `POST /api/v1/queries/saved` (PRD-listed) is intentionally not implemented in
  M1.

## Test Coverage

- Backend: unit tests for config, response envelope, middleware (panic/log/CORS/
  request-id), security (9 cipher tests), storage (11), repositories (16),
  manager (fake driver; concurrency), Postgres DSN/encode/buffer, API handlers,
  and API contract tests.
- Integration (real PG): connection manager, introspection, query engine
  (8 subtests), connection-test API, query+history API.
- `go test -race ./...` clean.

## Scope Audit

- **MySQL target driver:** absent (only the `mysql` enum value + validation that
  rejects it).
- **User SQLite target driver:** absent (`sqlite` appears only for the embedded
  app store and the driver enum).
- **Frontend features** (connection UI, schema explorer, CodeMirror, result
  grid, PWA, Zustand/TanStack): absent — `frontend/` remains the M0 placeholder.
- No future endpoints fabricated.

## Findings

| ID | Severity | Finding |
|---|---|---|
| F1 | HIGH → RESOLVED | PostgreSQL integration had never been executed. Resolved: installed PostgreSQL 17.11 and ran the full integration suite + live slice — all PASS. |
| F2 | MEDIUM | CI (`.github/workflows/test.yml`) does not run PostgreSQL integration tests (no PG service); the vertical slice is only exercised manually/opt-in. |
| F3 | MEDIUM | PRD OpenAPI-version conflict: §3 says 3.0 but §8.2's `swaggo/swag` v1 emits Swagger 2.0; this repo emits OpenAPI 3.1.0 via `swaggo/swag/v2 --v3.1`. Needs human approval. |
| F4 | MEDIUM | OpenAPI request schemas do not mark required fields (validation is manual), so codegen cannot enforce requiredness. |
| F5 | LOW | `execution_time_ms` is 0 when a query fails during pool activation (connection failure). |
| F6 | LOW | Query engine supports a single statement per request (pgx extended protocol). |
| F7 | LOW | JSONB numbers are decoded via Go `any` (float64), so very large integers *inside* JSON payloads are not precision-protected. |
| F8 | LOW | PostgreSQL 17.11 was installed via Homebrew for this review (test-only); server stopped and test DB dropped. |
| F9 | LOW | `backend/docs/.gitkeep` now redundant beside the generated swagger files. |

## Blockers

**None.**

## Risks for M2

1. **CI does not exercise PostgreSQL (F2):** regressions in driver/introspection/
   query code would not be caught by CI. Recommend a PostgreSQL service (or a
   dedicated integration job) before/soon after M2 starts.
2. **OpenAPI version decision (F3) and required-field documentation (F4):**
   frontend codegen should proceed against OpenAPI 3.1.0; confirm this and,
   ideally, mark required request fields.
3. **Real-server coverage is strong but single-engine:** only PostgreSQL exists;
   MySQL/SQLite target paths are intentionally absent.
4. **JSONB numeric precision (F7)** may surface with very large integers inside
   JSON columns.
5. Repository is still not a Git repo, so CI has never actually run.
6. Unchanged security posture: no API authentication beyond loopback binding.

## Exit Checklist

- [x] Backend builds
- [x] Backend tests pass (fresh, `-count=1`)
- [x] `go vet` passes
- [x] Storage works (open/migrate/idempotent/FK/close)
- [x] Encryption works (AES-256-GCM, tamper detection)
- [x] PostgreSQL connection vertical slice works (**executed against PG 17.11**)
- [x] Schema introspection works (**integration PASS**)
- [x] Query execution works (**integration + live PASS**)
- [x] Timeout enforced
- [x] BIGINT precision protected
- [x] Result size bounded (50 MB)
- [x] Query history works
- [x] Credentials protected (not persisted plaintext, not logged, not returned)
- [x] API contract stabilized
- [x] OpenAPI available
- [x] No BLOCKER
- [x] No unresolved HIGH issue that makes M2 unsafe

## Final Status

**M1 READY**
