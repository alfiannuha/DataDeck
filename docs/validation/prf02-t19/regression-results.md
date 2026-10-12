# PRF02-T19 — Regression & Command Results

Commit `7958007`. All commands run from the repository root unless noted.

| # | Command | Result | Notes |
|---|---|---|---|
| 1 | `cd backend && gofmt -l .` | PASS | empty output |
| 2 | `cd backend && go vet ./...` | PASS | |
| 3 | `cd backend && go test -count=1 ./...` | PASS | 15 packages |
| 4 | `cd backend && go test -race -count=1 ./...` | PASS | 15 packages |
| 5 | `DATADECK_TEST_PG_HOST=127.0.0.1 DATADECK_TEST_PG_USER=alfiannuha DATADECK_TEST_MYSQL_HOST=127.0.0.1 go test -tags=integration -p 1 -count=1 ./...` | PASS | PG 17.11 + MySQL 8.0.46 |
| 6 | `CGO_ENABLED=0 go build ./...` | PASS | |
| 7 | cross-compile `darwin/amd64,arm64 linux/amd64,arm64 windows/amd64` | PASS | compile-only (~30–32 MB each) |
| 8 | `cd frontend && npm run lint` | PASS | 0 problems |
| 9 | `npm run typecheck` | PASS | |
| 10 | `npm run test` | PASS | 51 files / 471 tests |
| 11 | `npm run build` | PASS | production build |
| 12 | `npm run build:static` | PASS | static export |
| 13 | `npx playwright test` (single binary) | PASS | 10/10 |
| 14 | `go test -bench -benchmem …` | PASS | see performance-results.md |
| 15 | OpenAPI swagger ↔ handlers | PASS | `TestSwaggerSpecMatchesImplementation`, `TestEnvelopeConsistencyAcrossEndpoints` |
| 16 | `govulncheck ./...` (last run, T15) | PASS | 0 reachable |
| 17 | `npm audit --audit-level=critical` (last run) | PASS | no critical |

## Regression scope
- PRF-01 multi-database isolation: handler + integration + e2e (α/β) PASS.
- T01–T02 workspace/empty state: unit PASS.
- T03–T06 TableData browse/sort/filter/page: unit + integration PASS.
- T07–T11 CRUD: unit + integration + e2e PASS.
- T12 table menu, T13 structure, T14 export, T15 import: unit + integration + e2e PASS.
- T16 reconnect, T17 db export/restore, T18 delete: unit + integration + e2e PASS (engine gaps as documented).
- Query execution / history / saved queries: existing suites PASS.
- PWA/static export + single-binary: e2e release-flow PASS.

## Not run
- Remote CI (GitHub Actions): **NOT RUN** (no remote in this environment).
- Native runtime smoke on non-macOS targets: **NOT RUN** (cross-compile only).
