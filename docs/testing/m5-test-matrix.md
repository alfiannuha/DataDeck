# M5 Test Matrix & Coverage Audit

Status: M5-T09. Purpose: confirm DataDeck's release-critical behavior is covered
and close high-value gaps. Coverage is used as a signal, not a pass/fail target
(the PRD specifies none).

## Environment & Method

| Item | Value |
|---|---|
| Backend tests | `go test ./...` (unit) and `go test -tags=integration ./...` (real PG 17.11 + MySQL 8.0.46 + SQLite) |
| Backend coverage | `go test -coverprofile` (`-covermode=atomic`), `go tool cover -func` |
| Frontend tests | Vitest 5 + React Testing Library (jsdom) |
| Frontend coverage | `@vitest/coverage-v8@5.0.2` (added as a dev-only tool for this audit) |
| Browser E2E | Playwright/Chromium, performed manually during M4/M5 tasks (not in CI) |

Commands:

```bash
cd backend && go test ./... -coverprofile=/tmp/cover.out -covermode=atomic
cd backend && go test -tags=integration ./... -coverprofile=/tmp/cover_it.out -covermode=atomic
cd frontend && npx vitest run --coverage --coverage.reporter=text-summary
```

Inventory: **171 backend test functions** (17 integration, 2 fuzz targets, 5
benchmarks) and **39 frontend test files / 276 tests**.

## 1. Test Matrix

| Area | Coverage location | Class |
|---|---|---|
| Config | `internal/config` | UNIT |
| Storage (embedded SQLite, migrations, perms) | `internal/storage` | UNIT |
| Encryption (AES-256-GCM) | `internal/security` | UNIT |
| Repositories (connections/history/saved) | `internal/repository` | UNIT (+ INTEGRATION for real store) |
| API envelope/router/middleware | `internal/api`, `.../middleware`, `.../response` | UNIT |
| ConnectionManager | `internal/database/manager_*_test.go` | UNIT (fake driver) + REAL DB |
| PostgreSQL | `internal/database/postgres*` , handler `query_integration_test.go` | REAL DB (tagged) |
| MySQL | `internal/database/mysql*`, handler `mysql_query_integration_test.go` | REAL DB (tagged) |
| SQLite (target) | `internal/database/sqlite*` | REAL FILE (unit-tagged) |
| Introspection | `*_integration_test.go`, `schema_test.go` | REAL DB + UNIT |
| Query execution | `query_integration_test.go`, `*_query_test.go`, `resilience_integration_test.go` | REAL DB + UNIT |
| History / saved queries / pagination | `repository/benchmark_test.go`, handler `*_test.go`, `pagination` | UNIT + REAL DB |
| Errors (sanitization, mapping, body limits) | `credential_security_test.go`, `request_hardening_test.go`, `errors` | UNIT |
| Frontend connections | `connection-list.test.tsx`, `new-connection-modal.test.tsx` | UNIT (jsdom) |
| Schema explorer | `schema-explorer.test.tsx`, `schema-tree.perf.test.tsx` | UNIT + PERF |
| Editor / shortcuts / dirty state | `editor-panel.test.tsx`, `sql-editor`, tabs | UNIT |
| Query execution (client) | `use-run-query.test.tsx` | UNIT |
| Grid / virtualization | `result-grid.test.tsx`, `result-grid.perf.test.tsx` | UNIT + PERF |
| History / saved dialogs (incl. failure states) | `history-dialog.test.tsx`, `saved-queries-dialog.test.tsx`, `save-query-dialog.test.tsx` | UNIT |
| Export (CSV/JSON, formula, truncation) | `lib/export/*.test.ts`, `export.bench.test.ts`, `results-panel.test.tsx` | UNIT + PERF |
| PWA / offline / update | `pwa-assets.test.ts`, `service-worker.test.ts`, `offline-banner.test.tsx`, `use-pwa-install.test.tsx`, `top-bar.test.tsx` | UNIT |
| Failure recovery | `app-shell.test.tsx`, `workspace.integration.test.tsx` | INTEGRATION (jsdom) |
| Security (creds/logs/caches) | `credential_security_test.go`, `cipher_test.go`, `pwa-assets.test.ts`, `dependency-audit.md` | UNIT + review |
| Concurrency | `manager_stress_test.go`, `concurrency_integration_test.go` | UNIT + REAL DB |
| Performance | `benchmark_test.go`, `*.perf.test.tsx`, `export.bench.test.ts` | PERF |
| End-to-end (product) | manual Playwright runs (M4) + M5-T10 review | BROWSER E2E |

## 2. Backend Coverage

Unit only (`go test ./...`): **60.8 %** statements. Integration-tagged:
**78.6 %**. Driver code is intentionally exercised only under the integration
tag, so the tagged figure is the meaningful one for `internal/database`.

| Package | Unit | +Integration |
|---|---|---|
| `internal/api` | 100.0 % | 100.0 % |
| `internal/api/handler` | 83.9 % | 85.3 % |
| `internal/api/middleware` | 82.6 % | 82.6 % |
| `internal/api/response` | 66.7 % | 66.7 % |
| `internal/config` | 87.5 % | 87.5 % |
| `internal/database` | 43.9 % | **79.6 %** |
| `internal/model` | 50.0 % | 50.0 % |
| `internal/repository` | 76.3 % | 76.3 % |
| `internal/security` | 92.7 % | 92.7 % |
| `internal/storage` | 69.1 % | 69.1 % |
| `cmd/server` | 0 % (`main` wiring) | 0 % |

Notable low-coverage-by-design functions: `cmd/server/main.go` (process wiring,
covered by the startup/RSS benchmark and integration runs, not unit tests),
`response.SuccessWithMeta`/`middleware.RequestID` (exercised through the router
in other tests), and driver `formatUUID`/`defaultEncode` branches (real-DB paths).

## 3. Frontend Coverage

`vitest --coverage` (v8): **Statements 89.81 %**, Branches 76.85 %,
Functions 90.26 %, Lines 91.87 % over 1158 statements.

## 4. Test-Quality Audit

- **No "assert no exception" tests** were found as the sole assertion in
  release-critical paths; tests assert concrete status/envelope/state.
- **Mocks:** frontend unit tests mock the API client (expected for component
  tests). Backend manager tests use a fake SQL driver to exercise
  pool/duplicate-open logic; **these cannot establish database compatibility** —
  that role belongs to the tagged integration tests against real PG/MySQL/SQLite,
  which are present and run in CI.
- **Duplicated low-value tests:** none identified that dominate; a few
  dialog tests repeat rendering setup but assert distinct behavior.
- **Timing-dependent tests:** `manager_stress_test.go`,
  `resource_safety_test.go`, `resilience_integration_test.go`,
  `concurrency_integration_test.go` (connection kill/terminate, cancellation,
  goroutine settling). They use retries/settle loops; see flakiness results.
- **Tests bypassing real databases:** only the manager fake-driver tests, which
  are explicitly pool-logic tests, not compatibility tests.
- **Hidden skips:** integration tests `t.Skip` when `DATADECK_TEST_*` env is
  unset — documented and expected. In CI the `integration` job sets PG+MySQL env,
  so the 17 integration tests execute. No unconditional/`t.SkipNow` hidden skips
  were found.
- **Environment-sensitive assumptions:** local runs require the MySQL 8.0
  datadir and PG service; CI uses containerized services.

## 5. Real vs Mock Classification

- **UNIT:** config, storage, security, repositories, API/middleware, manager
  (fake driver), most frontend components/hooks, export serializers, PWA
  install/update state.
- **INTEGRATION (in-process, real components, mocked transport):** frontend
  workspace/recovery flows (jsdom + TanStack Query with mocked endpoints).
- **REAL DATABASE:** all `-tags=integration` tests (PostgreSQL, MySQL, SQLite
  file) — introspection, query execution, history, saved queries, resilience,
  concurrency, endpoint timeout.
- **BROWSER E2E:** manual Playwright runs (multi-DB workflow, export downloads,
  PWA/offline, schema actions); not automated in CI.
- **PERFORMANCE:** Go benchmarks + Vitest grid/schema/export benchmarks.

## 6. Critical Gaps Found & Closed

High-value gaps found during the audit and closed in this task:

| Gap | Fix |
|---|---|
| History dialog: load failure/loading-termination not asserted | `history-dialog.test.tsx` error-state test |
| Saved-queries dialog: load failure not asserted | `saved-queries-dialog.test.tsx` error-state test |
| Save-query dialog: save failure path (dialog/state) untested | `save-query-dialog.test.tsx` failure test |
| Export failure feedback untested | `results-panel.test.tsx` "Export failed" test |
| Storage directory permissions (SEC-MED-2) not asserted | `storage_test.go` 0700 test + file-mode log |

Remaining **accepted** gaps (documented, not release-blocking): `cmd/server`
wiring has no unit test (covered by startup benchmark + integration); a handful
of driver encode branches are covered indirectly by real-DB tests.

## 7. Flakiness Results

| Suite | Repeats | Result |
|---|---|---|
| Backend unit `go test ./...` | ×5 | 0 failures |
| Backend `-race` (handler, database, repository) | ×3 | 0 failures, no data races |
| Backend integration (`-tags=integration ./...`) | ×3 | 0 failures |
| Backend resilience/concurrency (real DB) | ×5 | 0 failures |
| Frontend `npm run test` | ×3 | 276 passed each run |

No flaky failures were observed; the timing-sensitive suites were stable across
repeats. (Historically, MySQL kill/terminate tests were made retry-tolerant in
M5-T05; no failures recurred here.)

## 8. CI Alignment

`.github/workflows/test.yml` runs:

- **backend:** `gofmt -l`, `go vet`, `go test ./...`, `go build ./...`
- **frontend:** `npm ci`, `lint`, `typecheck`, `test`, `build`
- **integration:** PG 17 + MySQL 8 services, `go test -tags=integration ./...`

Release-critical items **not** in CI (recommendations for M6):
1. `go test -race` (currently developer-only).
2. `govulncheck` and `npm audit` (M5-T08 findings).
3. Browser E2E (Playwright) and the performance benchmarks — currently
   developer-only; add a nightly/release job.
4. Frontend coverage thresholds are not enforced (no PRD target; optional).

No essential test is left as a developer-only command **without** a CI path: the
integration job executes the real-DB suites, and the remaining items are
explicitly listed above as M6 additions.

## 9. Files Changed

- `docs/testing/m5-test-matrix.md` (new).
- `frontend/src/components/shared/history-dialog.test.tsx`,
  `saved-queries-dialog.test.tsx`, `save-query-dialog.test.tsx`,
  `results-panel.test.tsx` — added failure-state tests.
- `backend/internal/storage/storage_test.go` — permission test.
- `frontend/package.json`, `frontend/package-lock.json` — added dev-only
  `@vitest/coverage-v8@5.0.2` (coverage reporting; no runtime impact).

## Remaining Test Risks

- Driver compatibility is proven only by the tagged integration suite; CI runs
  it, but the local default `go test ./...` skips it by design.
- Browser E2E remains manual; automated E2E is an M6 item.
- No coverage threshold is enforced; new release-critical branches could regress
  silently without review.
- `cmd/server` process wiring is untested at the unit level.
- Timing-sensitive real-DB tests pass reliably here but could flake on slower
  CI; they already use retries/settle loops.
