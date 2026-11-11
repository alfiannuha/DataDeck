# DataDeck v0.2.0 — Pool Lifecycle & Idle Janitor

## Lifecycle
`Open` → (dedup via `opening`) → `connect` + health `Ping` → register in `pools`
(+`lastUsed`, `poolDriver`). Failures are not registered. `evict` (unhealthy),
`Close` (one pool), `CloseConnection` (all pools of a profile), `CloseAll` (shutdown)
remove and close pools. `Reconnect` replaces one `(connection,database)` pool.

## Idle janitor (v0.2.0)
`Manager.SweepIdle(threshold)` closes a pool only when **all** hold:
1. no in-flight initialization for the key (`opening`),
2. `now - lastUsed >= threshold`,
3. `database/sql` `Stats().InUse == 0` — the **authoritative active-operation lease**.
Pools are removed under `m.mu` and closed outside it. Idempotent (a second sweep
closes nothing). `threshold <= 0` disables it.

`StartIdleJanitor(ctx, interval, threshold)` runs `SweepIdle` every `interval` until
`ctx` is cancelled — the only background worker introduced; it stops on shutdown.

**Safety rationale:** rather than threading a lease API through every handler,
`database/sql`'s own `InUse` counter is used as the active-operation reference; the
janitor cannot close a pool while any operation holds a connection. This guarantees
"never close an active pool" without per-call plumbing.

## Config
| Env | Default | Meaning |
|---|---|---|
| `POOL_IDLE_TIMEOUT` | `30m` | idle threshold |
| `POOL_JANITOR_INTERVAL` | `5m` | sweep interval (0 disables) |

## Shutdown
`signal` context cancels the janitor; `CloseAll` closes remaining pools. Clean
shutdown verified in v0.1.0 (R09); unchanged.

## Tests
`internal/database/manager_telemetry_test.go`: telemetry is side-effect-free + sorted;
`sweep` closes idle, **skips active (`InUse>0`)**, **skips initializing**, idempotent,
zero-threshold disables. Race detector passes.
