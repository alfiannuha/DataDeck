# DataDeck v0.2.0 — Architecture Audit

Audit of the repository at the v0.1.0 release (`6d99856`) before v0.2.0 work.

## Current architecture (verified)
- **Frontend:** Next.js App Router + React + TS + Tailwind + Shadcn + CodeMirror 6 +
  TanStack Query/Table/Virtual + Zustand. Typed `WorkspaceTab` union
  (Query/TableData/TableStructure); each tab carries `connectionId` + `database` in
  `TabBase` (`frontend/src/store/useWorkspaceStore.ts`).
- **Backend:** Go/Chi. `internal/database.Manager` owns pools; `internal/api/handler`
  resolves the target per request; `repository` owns the embedded SQLite store.
- **Drivers:** pgx (PostgreSQL), go-sql-driver/mysql, modernc.org/sqlite — all via
  `database/sql`; `Connector` abstraction in `internal/database/driver.go`.

## Existing connection lifecycle
`Manager.Open(ctx, id, cfg)` → `PoolKey{ConnectionID, Database}` → get-or-open with a
health check; concurrent callers for the same key share one in-flight attempt
(`opening` map). `Manager.Get`, `Close`, `CloseConnection`, `CloseAll` manage lifecycle;
per-connection LRU cap `MaxPoolsPerConnection` (default 16) evicts the least-recently-used
pool. `Reconnect` (T16) is database-scoped and fail-closed.

## Existing pool implementation (already `(connectionId, database)`)
- `PoolKey{ConnectionID, Database}`; lazy per-database pools; `lastUsed` timestamps.
- Different profiles / databases never share a pool; concurrent opens dedup.
- Startup `PingTimeout` health check; failed opens are not registered (retryable).
- **No telemetry API, no idle janitor goroutine, no per-pool driver record.**

## Existing tab-to-connection relationship
- Tabs store immutable `connectionId`/`database`; handlers resolve the pool from the
  **request/tab binding**, never from global/Explorer state (`resolveTargetDatabase`).
- Explorer selection updates only `activeDatabaseByConnection` (explorer state), never
  rebinds tabs. α/β isolation proven by E2E (T19/R05/R09).

## Identified gaps / risks for v0.2.0
1. **No pool telemetry / status API** — operators cannot inspect pool state.
2. **No idle-pool cleanup** — long-running multi-connection use can retain pools
   indefinitely (bounded only by the per-connection LRU cap of 16).
3. No per-pool driver record for status output.
4. **No environment indicator** in the UI to make a tab's connection/database visible.
5. Documentation lacks explicit v0.2.0 multi-connection lifecycle/pool sections.

## Isolation/race findings
- Pool identity is already composite and concurrent dedup exists → **no wrong-database
  path found**; α/β isolation holds.
- Idle cleanup must not close a pool with `Stats().InUse > 0` nor one currently in
  `opening`; cleanup must be idempotent and race-free with `Open`.
- The untracked `multi_connection_architectural_specification.md` is a **macOS Finder
  bookmark binary** (not a usable spec); ADR-009/010 are the authority. Left untouched.

## Reusable components
`PoolKey`, `Open`/`Reconnect`/`evict`, `Options`, the `Connector` abstraction, the
typed tab model, and `resolveTargetDatabase` — all reused; v0.2.0 is additive.
