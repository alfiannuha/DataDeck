# M6 Pre-Release Risk Closure

Status: M6-T00. M5 was **READY**; this task resolves or formally dispositions the
M5 findings that could affect release artifacts. No packaging and no product
features were added.

## Summary of Dispositions

| # | Finding (M5 severity) | Disposition |
|---|---|---|
| 1 | Outage error mapping returned `INTERNAL_ERROR` (MEDIUM M1) | **FIXED** — connection loss now maps to `CONNECTION_ERROR` |
| 2 | Go dep advisories: pgx v5.7.6, x/text v0.24.0 (MEDIUM M2) | **FIXED** — upgraded; `govulncheck` clean |
| 3 | Go toolchain stdlib advisories (MEDIUM M2) | **FIXED** — patched toolchain pinned |
| 4 | Next.js moderate advisory (MEDIUM M3) | **ACCEPTED EXCEPTION** — no compatible patch; build-time only |
| 5 | 1000-table table-level render (MEDIUM M4) | **FIXED** — bounded incremental rendering |
| 6 | Release baseline regression | **PASS** — all gates re-run |

## 1. Connection Error Mapping

- **Finding (original severity):** established-pool connection loss returned
  `500 INTERNAL_ERROR` instead of a connection error (M5-T05/M5-T11 MEDIUM).
- **Remediation:** added a driver-neutral detector
  (`internal/database/errors.go`: `isConnectionLoss`, `asSQLError`) matching only
  transport failures (`driver.ErrBadConn`, `sql.ErrConnDone`, `net.Error`,
  `io.EOF`/`UnexpectedEOF`, `ECONNRESET/REFUSED/EPIPE/ECONNABORTED`), applied to
  both connection acquisition and statement execution in the PostgreSQL, MySQL
  and SQLite connectors. `SQLError` (statement errors) and context
  errors/deadlines are checked first and never reclassified.
- **Evidence:**
  - New integration tests `TestPostgresConnectionLossMapped` /
    `TestMySQLConnectionLossMapped` use a controlled TCP proxy in front of the
    real servers. They assert (a) a syntax error remains `*database.SQLError`
    and is **not** an `ErrConnection`, then (b) after cutting the connection the
    next execution returns `database.ErrConnection`.
  - Live API E2E: PG and MySQL stopped mid-session →
    `502 CONNECTION_ERROR` (previously `500 INTERNAL_ERROR`); after restart,
    queries succeed.
- **Remaining risk:** heuristic classification; unusual driver errors that are
  neither typed transport errors nor `SQLError` still fall through to
  `INTERNAL_ERROR` (safe).

## 2. Go Dependencies

| Module | Before | After |
|---|---|---|
| `github.com/jackc/pgx/v5` | v5.7.6 | **v5.9.2** |
| `golang.org/x/text` (indirect) | v0.24.0 | **v0.39.0** |
| `golang.org/x/sync` (indirect) | v0.15.0 | v0.21.0 |

- Minimal compatible upgrades: pgx to the exact fixed release (GO-2026-5004),
  x/text to the minimal fixed release (GO-2026-5970). No broad churn.
- Validated: `go mod tidy`, `go mod verify` (all modules verified), `go vet`,
  `go test ./...`, `go test -race ./...`, full integration (PG+MySQL+SQLite),
  `CGO_ENABLED=0 go build ./...` — all PASS.
- **govulncheck (after):** **“No vulnerabilities found.”** (was 6 reachable
  advisories; the pgx and x/text advisories are resolved).

## 3. Go Toolchain

- **Build toolchain vs module minimum:** the module `go` directive (language
  minimum) moved 1.23.0 → **1.25.0**, the minimum required by the patched
  pgx/x/text releases (both declare `go 1.25.0`). This still satisfies PRD
  §8.1 (“Go 1.23 or newer”).
- A patched **build toolchain is pinned**: `toolchain go1.26.6`. The four
  remaining stdlib advisories were fixed in go1.26.6; `govulncheck` run through
  the pinned toolchain reports no vulnerabilities. Local dev toolchain 1.26.5
  auto-selects the pinned 1.26.6 via the Go toolchain mechanism.
- CI updated to install **Go 1.26.6** explicitly (both backend and integration
  jobs) instead of inheriting the `go` directive.
- **Remaining risk:** builds in an offline/`GOTOOLCHAIN=local` environment must
  already have ≥1.26.6 available.

## 4. Next.js Advisory

- **Finding (original severity):** `next` moderate (via bundled PostCSS) and
  `postcss` high transitive advisories (M5-T08 M3).
- **Re-audit:** unchanged; total moderate 1 / high 1 / critical 0. The latest
  15.x is **15.5.26**, which is already the installed version and is inside the
  affected range; the only offered fix is `next 16.3.6` (semver-major).
- **Applicability:** the advisories concern PostCSS processing of
  attacker-controlled CSS / `sourceMappingURL`, i.e. **build-time** handling of
  DataDeck's own CSS — not a runtime request path. No untrusted CSS or source
  maps are processed during builds.
- **Decision:** **explicit accepted exception** — do not perform a major Next.js
  upgrade in this task (regression cost across the app router/PWA build). Track
  for a planned toolchain upgrade.
- **Remaining risk:** low; re-evaluate at the Next 16 upgrade or if any future
  build step processes third-party CSS.

## 5. Large Schema Explorer

- **Finding (original severity):** engines without a schema level mounted all
  tables (1000 tables → 1001 DOM nodes, ~412 ms) (M5-T07 M4).
- **Remediation:** added `TableList` incremental rendering in
  `schema-tree.tsx` — a 100-table batch plus a “Show 100 more (N remaining)”
  control, used for both database-level and schema-level table lists.
  Expand/collapse, context actions, and keyboard accessibility are unchanged;
  introspection response size is untouched.
- **Evidence (1000 tables):**

| Mode | Before | After |
|---|---|---|
| Schema-level (PostgreSQL-style) | 2 DOM / 57 ms | 2 DOM / ~66 ms |
| Table-level (SQLite/MySQL-style) | **1001 DOM / 412 ms** | **102 DOM / ~200 ms** |

- New test asserts the batch control appears, that clicking it reveals the next
  batch, and that context actions remain available on rendered tables.
- **Remaining risk:** none material; the first 100 tables still render eagerly
  (intentional batch to preserve usability).

## 6. M5 Regression

Re-run and PASS:

- Backend: `gofmt`, `go vet`, `go test ./...`, `go test -race ./...`, full
  `-tags=integration ./...` (real PG/MySQL/SQLite), `CGO_ENABLED=0` build.
- Frontend: lint, typecheck, **277 tests**, production build.
- `make check`.
- Live API regression: health, three engines (create/test/schema), SQLite DDL +
  BIGINT, PG BIGINT, MySQL unsigned BIGINT, history pagination, saved-query
  create/update/delete, >50 MB truncation (`truncated: true`, 52,167 rows),
  timeout (`504`), SQL syntax error stays a SQL error, wrong-password safe
  failure, health after stress.
- Outage mapping E2E: PG and MySQL `502 CONNECTION_ERROR` during outage and
  recovery on restart.
- 100k grid virtualization: unchanged (23 DOM rows; enforced by the perf test).
- PWA cache security: unchanged (only shell assets; no `/api`) — covered by the
  existing suite.

## Files Changed

- `backend/internal/database/errors.go` — connection-loss detection helpers.
- `backend/internal/database/postgres_query.go`, `mysql_query.go`,
  `sqlite_query.go` — map transport failures to `ErrConnection`.
- `backend/internal/database/outage_mapping_integration_test.go` (new) — TCP-proxy
  integration tests.
- `backend/go.mod`, `backend/go.sum` — pgx v5.9.2, x/text v0.39.0, `go 1.25.0`,
  `toolchain go1.26.6`.
- `frontend/src/components/sidebar/schema-tree.tsx` — bounded `TableList`.
- `frontend/src/components/sidebar/schema-tree.perf.test.tsx` — bounded-render
  assertion + incremental-rendering test.
- `.github/workflows/test.yml` — Go 1.26.6 in backend/integration jobs.
- `README.md` — prerequisite note (module minimum 1.25+, pinned toolchain).
- `docs/reviews/m6-pre-release-risk-closure.md` (this document).

## Remaining Accepted Risks

1. **Next.js/PostCSS advisories** — build-time only; deferred to a planned
   Next 16 toolchain upgrade.
2. **Connection-loss heuristic** — untyped driver errors may still surface as
   `INTERNAL_ERROR`.
3. **Remote CI** — still NOT VERIFIED (no Git remote/pipeline); M6 must confirm
   once hosted.
4. **Toolchain download** — the pinned `go1.26.6` must be available in
   restricted/offline build environments.
