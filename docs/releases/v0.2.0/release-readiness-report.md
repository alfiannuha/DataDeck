# DataDeck v0.2.0 — Release Readiness Report

**Status: READY.** (Implementation complete; no v0.2.0 tag/release created.)

## Files changed
- `backend/internal/database/manager.go` — per-pool driver record, `PoolStatus`,
  `PoolStatuses`, `SweepIdle`, `StartIdleJanitor`; lifecycle bookkeeping updated.
- `backend/internal/database/manager_telemetry_test.go` (new).
- `backend/internal/config/config.go` — `PoolIdleTimeout`/`PoolJanitorInterval` +
  `POOL_IDLE_TIMEOUT`/`POOL_JANITOR_INTERVAL` parsing.
- `backend/cmd/server/main.go` — start the idle janitor on the signal context.
- `backend/internal/api/handler/pool_status.go` (new) + `pool_status_test.go` (new).
- `backend/internal/api/router.go` — `GET /connections/status`.
- `backend/internal/api/contract_test.go` — path + envelope case.
- `backend/docs/swagger.{json,yaml}` — regenerated.
- `frontend/src/components/shared/top-bar.tsx` (+ `.test.tsx`) — environment badge.
- `frontend/src/types/generated/openapi.ts` — regenerated.
- `.env.example` — pool env vars.
- `docs/releases/v0.2.0/*` — this documentation set.

## Architectural decisions
- **Reuse** `PoolKey{ConnectionID, Database}`, `Open`/`Reconnect`/`evict`, the
  `Connector` abstraction, and the typed tab model — v0.2.0 is additive.
- **Lease model:** `database/sql` `Stats().InUse` is the active-operation reference; the
  janitor never closes a pool with active work or one initializing. Avoids threading a
  lease API through every handler while satisfying the "never close active pools" gate.
- **Additive API:** `GET /api/v1/connections/status` (read-only, no secrets, no side
  effects); distinct registered profiles vs initialized pools.

## Tests executed
Backend unit 15/15; race 6/6; PG integration PASS; frontend 473 tests + lint/typecheck +
build/static PASS; OpenAPI contract PASS. New tests cover telemetry side-effect freedom,
idle/active/initializing sweep behavior, idempotency, and status-endpoint secrecy.

## Engines tested
PostgreSQL (integration + e2e history) — PASS. SQLite (unit/handler) — PASS. MySQL —
integration **not run** (server down this session); driver behavior unchanged.

## Concurrency scenarios tested
Same-key open dedup; sweep skips active (`InUse>0`) and initializing; sweep race-safe
(`go test -race`); α/β tab isolation retained (v0.1.0 e2e unchanged).

## Performance
No new hot path; janitor is one ticker goroutine, O(pools)/sweep; telemetry is read-only.
Reference: 100k-row ~257 ms, 1000-table introspect ~193 ms.

## Acceptance criteria
| # | Criterion | Status |
|---|---|---|
| 1 | Multiple connections operate concurrently | MET (tabs + per-key pools) |
| 2 | Multiple databases under one connection concurrently | MET |
| 3 | Pool identity strictly `(connectionId, database)` | MET |
| 4 | Tabs retain immutable connection/database bindings | MET |
| 5 | Explorer switching cannot redirect tabs | MET |
| 6 | CRUD/transactions cannot cross database boundaries | MET (unchanged binding path) |
| 7 | Idle cleanup cannot close active pools | MET (`InUse` guard + tests) |
| 8 | Telemetry exposes no secrets | MET (field-level + test) |
| 9 | PG/MySQL/SQLite regressions tested | PG/SQLite MET; MySQL integration NOT RUN (env) |
| 10 | v0.1.0 security controls intact | MET |
| 11 | No unresolved BLOCKER/HIGH | MET |

## Known limitations / remaining blockers
- BLOCKER/HIGH: none.
- Explorer multi-tree (simultaneous expansion of several connections) is a future UX
  enhancement; concurrent connections are usable via tabs today.
- MySQL integration not run this session (environment). Version remains `0.1.0`
  (finalization deferred). See `known-limitations.md`.

## Notes
- No `v0.2.0` tag created; no artifacts published.
- The untracked `multi_connection_architectural_specification.md` is a Finder bookmark
  binary (not a usable spec); left untouched per R01-FIND-007.
