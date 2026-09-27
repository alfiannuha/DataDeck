# M5 Baseline — Pre-Hardening Risk & Baseline Lock

## Executive Summary

M5 begins from a locked, reproducible baseline. **M4 is READY** and every M1–M4
guarantee was re-verified in the current environment before any M5 change. All
local quality gates pass, real PostgreSQL/MySQL/SQLite integrations pass, the
performance baseline is re-captured fresh, and the two M4 environmental risks are
resolved or explicitly carried.

Two M4 findings are now substantially improved:

- **M4-F2 (service-worker update)** — RESOLVED for the baseline: a real
  install→update→activate lifecycle was exercised in a real browser by serving a
  version-bumped `sw.js`, confirming update detection, dirty-workspace
  protection (no forced reload), and cache migration on activation.
- **M4-F1 (remote CI)** — remains unresolved: the workspace is not a Git
  repository, so no hosted pipeline exists. Reported honestly as NOT VERIFIED.

No product features were added; no dependency upgrades were performed. The only
file added by this task is this document.

## Environment

| Component | Value |
|---|---|
| OS / architecture | macOS (darwin), arm64 (Apple Silicon) |
| Go toolchain | go1.26.5 (module declares `go 1.23.0`) |
| CGO setting | `CGO_ENABLED=0` for the release build; default for dev/tests |
| Node / npm | v24.14.0 / 11.9.0 |
| PostgreSQL | 17.11 (Homebrew), real server on 127.0.0.1:5432 |
| MySQL | 8.0.46 (Homebrew `mysql@8.0`), real server on 127.0.0.1:3306 |
| SQLite driver | `modernc.org/sqlite v1.38.2` (pure Go, no CGO) |
| Frontend | Next.js 15.5.26 (App Router), React 19.3.0, production build |
| Browser/E2E | Chromium via Playwright MCP; production `next start` on 127.0.0.1:3000 |
| Backend runtime | `datadeck-server` on 127.0.0.1:8080 (temp store + test key for review) |
| Git / hosting | NOT a Git repository; no Git remote |

## Commands Executed

All commands run from the repository root unless noted.

```text
# Root
make check                                   -> exit 0

# Backend
(cd backend) gofmt -l .                      -> clean (no output)
(cd backend) go vet ./...                    -> exit 0
(cd backend) go test ./... -count=1          -> exit 0
(cd backend) go test -race ./... -count=1    -> exit 0
(cd backend) CGO_ENABLED=0 go build ./...    -> exit 0
(cd backend) go test -tags=integration ./... -count=1 -> exit 0 (PG + MySQL + SQLite)

# Frontend
(cd frontend) npm run lint                   -> exit 0 (0 problems)
(cd frontend) npm run typecheck              -> exit 0 (0 errors)
(cd frontend) npm run test                   -> exit 0 (37 files / 264 tests)
(cd frontend) npm run build                  -> exit 0 (static prerender)

# Performance
(cd frontend) npm run test (perf test)       -> bounded DOM rows recorded below
```

## Baseline Results

| Gate | Result |
|---|---|
| `gofmt -l .` | PASS (clean) |
| `go vet ./...` | PASS |
| `go test ./...` | PASS |
| `go test -race ./...` | PASS |
| `go test -tags=integration ./...` | PASS (real PG, MySQL; SQLite real file) |
| `CGO_ENABLED=0 go build ./...` | PASS |
| frontend `lint` | PASS |
| frontend `typecheck` | PASS |
| frontend `test` | PASS (37 files / 264 tests) |
| frontend `build` | PASS |
| `make check` | PASS (exit 0) |
| M4 final status | **M4 READY** (verified in `docs/reviews/m4-product-complete-review.md`) |

No pre-existing failures were observed.

## Database Integration Matrix

| Engine | Status | Notes |
|---|---|---|
| PostgreSQL 17.11 | PASS | real server; introspection + query + handler integration suites ran (not skipped) |
| MySQL 8.0.46 | PASS | real server; introspection + query + handler integration suites ran |
| SQLite | PASS | real target file via `modernc.org/sqlite`; unit + query + introspection suites |

Integration tests are opt-in via `DATADECK_TEST_*` env vars and skip cleanly when
those hosts are unset; in this baseline they were set and executed.

## Performance Baseline (fresh, current environment)

Result-grid virtualization (`result-grid.perf.test.tsx`), DOM rows mounted at
each size:

| Rows | DOM rows | Mount ms |
|---|---|---|
| 1,000 | 23 | 671 |
| 10,000 | 23 | 326 |
| 50,000 | 23 | 259 |
| 100,000 | 23 | 464 |

**Bounded virtualization holds at 100k (23 DOM rows).** The absolute millisecond
values are harness-dependent (jsdom on this machine) and are **not** a universal
target; only the bounded DOM count is the regression invariant. M4's ~105 ms
figure was a different run on the same harness — treat both as noise around a
flat, bounded curve.

## CI Status

- **LOCAL CI-EQUIVALENT: PASS** — the three jobs in `.github/workflows/test.yml`
  (backend, frontend, integration) were reproduced locally and all pass.
- **REMOTE CI: NOT VERIFIED** — there is no Git repository or hosted pipeline;
  no remote run exists. No evidence was fabricated or inferred from local runs.

## Service-Worker Update Status

M4 finding **F2 is RESOLVED for baseline validation** using a real browser and the
production build:

1. Loaded the app; service worker active at `/sw.js` with `datadeck-shell-v2`.
2. Served a version-bumped worker (`CACHE_PREFIX + "v3-review"`) from the same
   origin and called `registration.update()`.
3. The client surfaced **Update available** with no auto-reload.
4. With **dirty SQL** present, clicking Update opened the confirmation
   ("unsaved SQL … will be lost"); **no reload occurred** (page marker preserved)
   and editor SQL remained intact; *Not now* kept the workspace.
5. Confirming triggered `SKIP_WAITING` → `controllerchange` → reload, and the
   active cache migrated to `datadeck-shell-v3-review` (old cache cleaned).

The temporary `sw.js` edit was reverted (md5 verified unchanged: the shipped file
targets `datadeck-shell-v2`). Dirty-workspace protection was not weakened.

## Dependency Snapshot (direct)

Backend (`backend/go.mod`):

| Module | Version |
|---|---|
| `github.com/go-chi/chi/v5` | v5.3.2 |
| `github.com/go-sql-driver/mysql` | v1.9.3 |
| `github.com/jackc/pgx/v5` | v5.7.6 |
| `modernc.org/sqlite` | v1.38.2 |
| Go directive | `go 1.23.0` |

Frontend (resolved from `package-lock.json`):

| Package | Version |
|---|---|
| next | 15.5.26 |
| react / react-dom | 19.3.0 |
| typescript | 5.9.3 |
| tailwindcss | 4.3.3 |
| @tanstack/react-query | 5.104.0 |
| @tanstack/react-table | 8.21.3 |
| @tanstack/react-virtual | 3.14.13 |
| zustand | 5.0.15 |
| @codemirror/view | 6.43.13 |
| @codemirror/lang-sql | 6.10.0 |
| @radix-ui/react-dialog | 1.1.23 |
| lucide-react | 1.48.0 |
| vitest | 5.0.2 |
| eslint | 9.39.5 |
| openapi-typescript | 7.13.0 |

No broad upgrades were performed. There is no `engines` field in
`frontend/package.json` (Node 24 is used locally; pinning policy is an M5 input).

## Carried Risks

| ID | Severity | Risk | Status |
|---|---|---|---|
| R1 | MEDIUM | Remote CI not verifiable (no Git repository/hosted pipeline) | Carried; M5 must not assume remote green |
| R2 | LOW | Performance timings harness-dependent; only DOM boundedness is the invariant | Carried; field measurement needed for certification |
| R3 | LOW | Service-worker cache version bump is manual | Carried; release/packaging must bump on shell change |
| R4 | LOW | MySQL `mysql@8.0` Homebrew datadir required local re-initialization | Environment-only; not a product defect |
| R5 | LOW | No Node `engines` pin in `package.json` | Carried; candidate M5 hardening item |
| R6 | LOW | JSON export keeps non-BIGINT integers as JSON numbers (documented) | Accepted canonical semantics |

## Exit Checklist

- [x] M4 READY verified
- [x] baseline quality gates recorded
- [x] three DB integrations status recorded
- [x] fresh performance baseline recorded
- [x] remote CI status honest (NOT VERIFIED)
- [x] SW update validation status honest (real lifecycle exercised)
- [x] dependency snapshot created
- [x] no broad dependency upgrade
- [x] no new product feature

## Final Status

**M5 BASELINE LOCKED** — ready to begin M5-T01.
