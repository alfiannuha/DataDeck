# DataDeck v0.2.0 — Implementation Plan

Additive enhancement; the `(connectionId, database)` identity and immutable tab binding
already exist (PRF-01/PRF-02) and are preserved. No rewrite.

## Backend
| Change | File(s) |
|---|---|
| Per-pool driver record + telemetry (`PoolStatuses`) | `internal/database/manager.go` |
| `SweepIdle(threshold)` — close pools idle ≥ threshold with `Stats().InUse == 0`, never while `opening`; idempotent | `internal/database/manager.go` |
| `StartIdleJanitor(ctx, interval, threshold)` background goroutine (cancelled on shutdown) | `internal/database/manager.go` |
| Config `POOL_IDLE_TIMEOUT` (default 30m), `POOL_JANITOR_INTERVAL` (default 5m) | `internal/config/config.go`, `.env.example` |
| Start janitor in the server lifecycle | `cmd/server/main.go` |
| `GET /api/v1/connections/status` — pool telemetry + registered-connection count, no secrets | `internal/api/handler/pool_status.go`, `router.go`, `contract_test.go`, swagger |

**Lease model:** `database/sql`'s `Stats().InUse` is the authoritative active-operation
count; the janitor will not close a pool with `InUse > 0`. `opening` guards
in-flight initialization. This avoids threading a lease API through every handler while
still guaranteeing "never close an active pool".

## Frontend
- Environment badge in the top bar showing the **active tab's** connection name,
  database, and driver (from `useWorkspaceStore` active tab + `useConnections`).

## Tests
- Pool identity (existing) + **new**: `SweepIdle` closes idle / skips in-use / skips
  opening / idempotent; `PoolStatuses` reports without opening and exposes no secrets;
  status endpoint envelope; janitor start/stop safe.
- Tab isolation: existing α/β E2E + unit; keep green.
- Regression: full backend unit/race/integration + frontend unit/build; α/β E2E.

## Non-goals
No API contract changes beyond the additive status endpoint; no security weakening; no
new dependencies; no v0.2.0 tag/publish.
