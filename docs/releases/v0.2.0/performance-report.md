# DataDeck v0.2.0 — Performance Report

No new hot-path work: v0.2.0 is additive (telemetry + idle janitor + badge). Measured
reference values are from the v0.1.0 baseline (R05/R07/R10); not re-benchmarked here.

| Metric | Value |
|---|---|
| Pool initialization | one connect + `PingTimeout` health check per `(connection,database)` (existing) |
| Raw 100k-row query | ~257 ms |
| Introspect 1000 tables | ~193 ms |
| Concurrency | `Open` dedups same-key; cross-key opens run independently |

## v0.2.0 overhead
- **Janitor**: a single `time.Ticker` goroutine (configurable interval, default 5m);
  each sweep is O(number of pools) under `m.mu`, calling `db.Stats()` (cheap) and
  removing eligible pools; `Close()` runs outside the lock. Bounded, no background
  workers per pool.
- **Telemetry**: `PoolStatuses` is a read-only O(pools) snapshot; the status endpoint
  lists registered profiles (one small SQLite query) + the snapshot. Never opens pools.
- **Bounds**: per-connection LRU cap (`MaxPoolsPerConnection`, default 16) bounds pool
  count; the janitor additionally reclaims idle pools after 30m.

## Memory / cleanup
- Idle pools (no active ops) are released after the idle timeout → memory returns.
- Shutdown: janitor stops on signal context; `CloseAll` closes remaining pools.
- No unbounded pool creation (LRU cap + idle sweep).

## Not measured
Explicit multi-connection load benchmark (N pools × concurrent queries) — not run in
this task; the existing manager benchmarks + race suite cover correctness. Recommend a
dedicated soak in a follow-up.
