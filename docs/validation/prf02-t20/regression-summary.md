# PRF02-T20 — Regression Summary

All commands run at HEAD `e3f6c20` (code `7958007`).

| Gate | Command | Result |
|---|---|---|
| Formatting | `cd backend && gofmt -l .` | PASS (empty) |
| Vet | `go vet ./...` | PASS |
| Build | `go build ./...` | PASS |
| Unit (fresh) | `go test -count=1 ./...` | PASS (15 pkgs) |
| Race (fresh) | `go test -race -count=1 ./...` | PASS (15 pkgs) |
| Integration | `DATADECK_TEST_PG_HOST=127.0.0.1 DATADECK_TEST_PG_USER=alfiannuha DATADECK_TEST_MYSQL_HOST=127.0.0.1 go test -tags=integration -p 1 -count=1 ./...` | PASS (PG 17.11 + MySQL 8.0.46) |
| CGO-free | `CGO_ENABLED=0 go build ./...` | PASS |
| Cross-build | `GOOS/GOARCH in {darwin,linux}×{amd64,arm64}, windows/amd64` | PASS (compile-only) |
| Frontend lint | `npm run lint` | PASS (0) |
| Frontend types | `npm run typecheck` | PASS |
| Frontend tests | `npm run test` | PASS (51 files / 471 tests) |
| Frontend build | `npm run build` / `build:static` | PASS |
| Browser E2E | `npx playwright test` (single binary) | PASS (10/10) |
| OpenAPI contract | `go test ./internal/api` | PASS |
| T20 targeted MySQL delete | `go test -tags=integration -run TestMySQLDeleteDatabaseIntegration` | PASS |

## PRF01 regression
Server-level PostgreSQL profile, database discovery, lazy per-database pools, LRU
pool cap, tab-local binding, history/saved database context, cross-database
isolation, MySQL/SQLite handling — existing suites + PG α/β integration PASS.

## M0–M6 regression
Query editor/execution/results, history, saved queries, connection profiles,
encrypted credentials, schema explorer, PWA/static export, embedded single binary,
Docker packaging config, cross-builds — existing suites + e2e release-flow PASS.

## CI
Remote CI (GitHub Actions): **NOT VERIFIED** — no remote in this environment.
The local equivalents of every CI job were executed above. Previous-commit CI is
not treated as proof for current HEAD.

## Transient observation
One MySQL integration failure (`TestQueryExecutionMySQLIntegration`) was observed
once in 7 combined runs and did **not** reproduce in 5 subsequent attempts
(including 3 full runs with fresh `-count=1`). Classified LOW (flaky), flagged for
CI monitoring (finding PRF02-FIND-004).
