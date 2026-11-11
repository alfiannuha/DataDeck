# DataDeck v0.2.0 — Connection Manager & Pool Design

## Identity
`PoolKey{ConnectionID, Database}` (unchanged from PRF-01). PostgreSQL server-level
profiles open one pool per database lazily; MySQL/SQLite degenerate to the single
profile database/file. Different profiles/databases never share a pool.

## Manager responsibilities
- Resolve profiles (via `repository`), open/close pools, and resolve database-specific
  pools. Handlers compute the target with `resolveTargetDatabase` and call
  `Manager.Open(ctx, id, cfg)` where `cfg.Database` is the resolved database.
- Concurrent initialization of the same key dedups through the `opening` map.
- Failures are not registered (retryable). `Reconnect` is database-scoped + fail-closed.
- Per-connection LRU cap (`MaxPoolsPerConnection`, default 16) bounds pool count.

## States (v0.2.0 telemetry)
`ready` (registered + serving) and `initializing` (in-flight open). Registered
connection **profiles** (from the repository) are distinct from initialized **pools**
(reported by `PoolStatuses`).

## Status API
`GET /api/v1/connections/status` → `{ registered_connections, pools[] }` where each pool
has `connection_id, database, driver, state, open_connections, in_use, idle, wait_count,
wait_duration_ms, max_open_connections, last_used`. **No credentials/DSNs/keys/raw
errors.** `PoolStatuses` never opens a pool (no side effects) and is sorted.

## Immutable tab binding
`TabBase{connectionId, database}` on all tab kinds. Handlers resolve the pool from the
request/tab binding — never from `activeDatabaseByConnection` or a global. Explorer
selection updates only explorer state. Frontend shows an environment badge for the
active tab (connection / database / driver).
