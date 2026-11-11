# DataDeck v0.2.0 — Test Report

Environment: macOS arm64; Go `go1.26.9`; Node 24.14; PostgreSQL 17.11 (MySQL server
was down during this run → integration for MySQL skipped; MySQL behavior unchanged and
covered by prior PRF-02 suites).

## Results
| Suite | Command | Result |
|---|---|---|
| Backend unit | `go test -count=1 ./...` | PASS — 15/15 packages |
| Backend race | `go test -race -count=1 ./internal/database ./internal/config ./internal/api/...` | PASS — 6/6 |
| Backend format/vet | `gofmt -l`, `go vet` | clean |
| Integration (PG) | `-tags=integration -p 1 ./internal/database ./internal/api/handler` | PASS |
| Frontend unit | `npm run test` | PASS — 51 files / **473 tests** |
| Frontend lint/typecheck | `npm run lint`, `tsc --noEmit` | PASS |
| Frontend build/static | `npm run build`, `build:static` | PASS |
| OpenAPI contract | `go test ./internal/api` | PASS (new `/connections/status`) |

## New v0.2.0 tests
`internal/database/manager_telemetry_test.go`:
- `TestPoolStatusesReportsWithoutOpening` — telemetry is side-effect free, sorted,
  reports identity/driver/state.
- `TestPoolStatusesIncludesInitializing` — in-flight open reported as `initializing`.
- `TestSweepIdleClosesIdlePoolAndIsIdempotent` — closes idle; second sweep = 0.
- `TestSweepIdleSkipsActivePool` — `InUse>0` never closed; closes after release.
- `TestSweepIdleSkipsInitializing` — not swept during initialization.
- `TestSweepIdleZeroThresholdDisables`.

`internal/api/handler/pool_status_test.go` — `TestPoolStatusEndpoint`: registered
connections reported, empty pools before init, pool entry (driver/state) after init,
and no forbidden tokens (password/key/DSN) in the response.

Frontend: `top-bar.test.tsx` — environment badge reflects the active tab's binding.

## Concurrency / isolation
Pool identity `(connectionId,database)` (existing tests); concurrent same-key open
dedup; janitor race-safe with `Open` (`go test -race`). α/β isolation remains covered by
the E2E suite (v0.1.0 passes; unchanged by additive v0.2.0 changes).

## Not run
- MySQL integration (server down). Non-mac native runtime (no runners). These are
  environment limitations, not product failures.
