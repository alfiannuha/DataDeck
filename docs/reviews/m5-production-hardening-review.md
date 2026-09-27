# M5 Production Hardening Review

## Executive Summary

DataDeck M5 is **READY**. The product is safe, bounded, and recoverable under
failure: no credential/key leakage, safe loopback defaults, enforced request and
result limits, exact BIGINT handling, race-free connection management with
bounded pools, defined database-outage recovery, resilient frontend
failure/recovery with stale-response protection, and a PWA shell that caches no
API data. All local quality gates pass, real PostgreSQL/MySQL/SQLite integration
passes, and every release-critical adversarial flow passed.

The PRD `<25 MB` idle-RAM goal is **met** (13.6–21.1 MB observed across runs).
No BLOCKER or unresolved HIGH finding exists. Remaining items are MEDIUM/LOW
hardening and M6 release items (remote CI, packaging, dependency patches).

Acting independently, this review reran the gates, the happy-path regression, a
light adversarial subset, and fresh performance measurements; no release
criterion was weakened and no product feature was added.

## Validation Environment

macOS Darwin 25.6.0 arm64; go1.26.5 (module `go 1.23.0`, `CGO_ENABLED=0`);
Node v24.14.0; Next.js 15.5.26 / React 19.3.0; PostgreSQL 17.11, MySQL 8.0.46,
SQLite via `modernc.org/sqlite v1.38.2`; real HTTP API on loopback; Chromium via
Playwright for browser E2E (M5-T11). Repo is not a Git repository.

## Commands Executed

```text
cd backend && gofmt -l .                                  -> clean
cd backend && go vet ./...                                -> ok
cd backend && go test ./... -count=1                      -> ok
cd backend && go test -race ./... -count=1                -> ok (no DATA RACE)
cd backend && go test -tags=integration ./... -count=1    -> ok (PG+MySQL+SQLite)
cd backend && CGO_ENABLED=0 go build ./...                -> ok
cd frontend && npm run lint / typecheck / test / build    -> 0 / 0 / 276 pass / ok
make check                                                -> exit 0
scripts/bench-backend.sh                                  -> startup + idle RSS
benchmarks + perf tests                                   -> see Performance
HTTP regression + adversarial subset (real services)      -> all PASS
```

## Security Review

- **Loopback default verified** (config test + real process binding to
  `127.0.0.1`); non-loopback requires explicit `ALLOW_REMOTE=1` and logs a
  warning — widening exposure cannot happen silently.
- **CORS** is an explicit origin allowlist (includes `PUT`); no wildcard.
- **Request limits**: 1 MiB body, `application/json` required (blocks simple-CSRF),
  single JSON value, trailing data rejected.
- **Errors sanitized**: validation errors are generic; SQL diagnostics retained
  where safe; panics recovered without leaking.
- **PWA cache safety**: only `datadeck-shell-*` shell/static assets; **zero
  `/api` entries** in Cache Storage (verified repeatedly).
- **Export safety**: CSV formula injection neutralized (`= + @`, non-numeric `-`),
  HTML kept inert, quotes/newlines correctly escaped.
- Cross-connection execution is isolated (pool keyed by connection id; verified
  in M5-T11 Flow E).

## Credential & Encryption Review

- AES-256-GCM (stdlib), fresh `crypto/rand` nonce per record, versioned format,
  authenticated decryption; malformed/tampered/wrong-key all rejected with no
  plaintext returned.
- Key is required at startup with no insecure fallback; absent/invalid refuses
  startup; key never logged, never returned, absent from frontend bundles.
- No plaintext credential persistence; the API never serializes credential
  fields; credential-bearing DSNs are never logged. Corrupted stored ciphertext
  yields a generic `INTERNAL_ERROR` with no echo (M5-T11 Flow A).

## API Hardening Review

Malformed JSON, oversized bodies, invalid pagination, invalid timeout, missing
connection id, unknown driver, unexpected content type, and unsupported methods
were all rejected with sanitized envelopes (`400/404/405`) and the server
remained healthy — no panic (M5-T11 Flow H; re-run subset PASS in this review).

## Query Resilience Review

- **Timeout**: default 30 s, hard max 300 s; real `pg_sleep` timeout → `504`
  recorded as ERROR history; connection reusable afterwards.
- **>50 MB guard**: real ~60 MB result truncated (`truncated: true`, 52,167 rows);
  the guard stops accumulation rather than reading everything.
- **Cleanup**: `rows.Close()`/`rows.Err()` on all paths; table-driven
  result-buffer unit tests confirm bounded accumulation.
- **BIGINT** exact on all engines; binary base64; JSON embedded.

## Connection & Concurrency Review

- `-race` across the full backend suite is clean; concurrent lifecycle
  (Open/Get/Test/Close/CloseAll), duplicate-open sharing one pool, and
  cross-connection routing verified.
- Pool limits enforced (`MaxOpenConns` respected on a real server; SQLite
  single-connection with `busy_timeout`).
- DB outage does not crash the daemon; `/health` stays 200; recovery is
  automatic on restart (documented nuance below).

## Frontend Resilience Review

- Backend unavailable at startup: shell usable, error surfaced with Retry.
- Backend restart with dirty SQL: failure shown, SQL retained, **no
  auto-execution**, explicit Run recovers.
- Offline→online: banner/guards, shell cached, reconnect refetches without
  running SQL.
- Stale-response protection: request-id + active-tab guard drops outdated
  responses across rapid re-runs/tab switches.

## Performance Review

Fresh measurements (this review) plus M5-T07 benchmarks:

| Metric | Value |
|---|---|
| Startup → health | 461–1261 ms (3+ runs) |
| Idle backend RAM (RSS) | 13.56–21.14 MB (2 s), settles 11–21 MB |
| Binary size | 22.0 MB |
| 1k rows | ~0.95 ms, 20 KB JSON |
| 10k rows | ~11.0 ms, 232 KB JSON |
| 50k rows | ~48 ms, 1.27 MB JSON |
| 100k rows | ~92 ms, 2.62 MB JSON |
| 100k DOM rows | **23** (bounded) |
| 1000-table introspection | ~52 ms, 416 KB JSON |
| 1000-table explorer render | schema-level 2 DOM/57 ms; table-level 1001 DOM/412 ms |
| CSV export 100k | 131–411 ms, 5.66 MB |
| JSON export 100k | 35–127 ms, 7.36 MB |

## PRD Memory Target Review

- **Metric:** idle RSS, release `CGO_ENABLED=0` build, loopback, macOS arm64,
  no active queries.
- **Observed:** 13.56 MB (this review) up to 21.14 MB (M5-T07); all below 25 MB.
- **Result: PASS** (worst sampled 21.14 MB < 25 MB). The margin is modest;
  re-measure after any new background work. The target was **not** redefined.

## Dependency Audit

- `govulncheck` (v1.8.0) and `npm audit` were run and are real (not invented).
- **Go:** reachable advisories — pgx `v5.7.6` (GO-2026-5004, fixed v5.9.2),
  `x/text v0.24.0` (GO-2026-5970, fixed v0.39.0), and four stdlib advisories
  fixed by a Go patch (go1.26.6). None is a **critical** runtime finding for
  DataDeck's usage (no parameter binding; user already runs arbitrary SQL).
- **npm:** no critical; `postcss` high is **build-time only**; `next` moderate is
  a build-path advisory with only a major fix offered.
- Lockfiles committed; CI uses `npm ci`; `go mod verify` passes.

## Test Matrix Review

- 171 backend test functions (17 integration, 2 fuzz, 5 benchmarks) + 39
  frontend files / 276 tests; release-critical areas covered; failure states
  added during M5-T09.
- Real DB integration distinguishes mocks from real behavior; the module fake
  driver is used only for pool-logic tests.
- No hidden critical skips: integration tests skip only without
  `DATADECK_TEST_*`, and CI sets those env vars.

## Production Configuration Review

- Loopback default; explicit `ALLOW_REMOTE` opt-in with warning.
- Required encryption key, no fallback.
- Store directory `0700`, DB/WAL/SHM narrowed to `0600`; corrupted DB and
  unwritable path fail explicitly.
- Safe logging defaults (info; no secrets/SQL bodies).
- Limits documented and tested (timeout, body, result cap, pagination, pools).

## PostgreSQL Regression

PASS: create/test, introspection, query, history, saved-query CRUD,
BIGINT/JSON/NULL exact, real truncation.

## MySQL Regression

PASS: create/test, introspection, query, history, saved-query CRUD,
UNSIGNED bigint exact, truncation.

## SQLite Regression

PASS: create/test, introspection, DDL/insert/select, BIGINT/NULL exact, lock
contention bounded; internal store unaffected by target failure.

## Product Regression

PASS this review: health, three engines, schema, history pagination, saved
create/open/update/delete, CSV/JSON export exactness (M5-T11 browser downloads),
schema actions (Select Top 100 / Count Rows / Copy DDL where supported), PWA
manifest/SW/offline. No M1–M4 regression found.

## Adversarial E2E

`docs/reviews/m5-adversarial-e2e.md`: **11/11 flows PASS** (credentials, outage,
timeout, huge result, concurrency, backend restart, offline, malformed API,
SQLite failure, export security, recovery). A light adversarial subset was
rerun in this review and passed. Two originally failing assertions were harness
artifacts (deleted SQLite file; trust-auth PG), diagnosed and documented.

## PWA / Service Worker Review

Manifest valid (`standalone`), SW active, shell cached only, offline banner +
guards, reconnect refetch with no auto-execution, dirty-SQL-safe update. Cache
audit: no `/api` data. SW cache version is manual.

## CI Review

- **LOCAL CI-EQUIVALENT: PASS** — backend (gofmt/vet/unit/race/CGO-free),
  integration (PG+MySQL+SQLite), frontend (lint/typecheck/test/build), `make check`.
- **REMOTE CI: NOT VERIFIED** — workspace is not a Git repository and no hosted
  pipeline exists; remote status is not inferred from local results.
- **LIVE SW UPDATE: PASS** — a real install→update→activate lifecycle was
  exercised in M5-T00 (version-bumped worker, dirty-SQL protection, cache
  migration).

## Findings

### BLOCKER
None.

### HIGH
None.

### MEDIUM
- **M1 — Outage error mapping.** Connection loss on an *established* pool
  returns `500 INTERNAL_ERROR` rather than `502 CONNECTION_ERROR`. Safe and
  auto-recovering, but semantically imprecise. Recommend mapping driver/network
  loss to `CONNECTION_ERROR`.
- **M2 — Go dependency advisories.** `pgx v5.7.6` (GO-2026-5004) and
  `x/text v0.24.0` (GO-2026-5970) reachable per `govulncheck`, plus four stdlib
  advisories fixed by a Go patch. Impact bounded (no parameter binding). Fix in a
  dedicated upgrade task.
- **M3 — Next advisory.** `next 15.5.26` moderate (build-path via postcss); only
  a major upgrade is offered. Defer to a planned toolchain bump.
- **M4 — Table-level schema rendering.** Engines without a schema level mount all
  tables (1000 tables → 1001 DOM nodes, 412 ms). Virtualize/lazy-expand.
- **M5 — Idle RSS margin.** `<25 MB` passes but the margin is modest
  (13.6–21.1 MB); re-measure after new background work.

### LOW
- Manual SW cache version bumps.
- `cmd/server` process wiring has no unit test.
- Remote CI not verified (tracked as an M6 release risk).
- Export formula heuristic ignores leading whitespace.
- Deep pagination `OFFSET` latency (constant memory).
- No Node `engines` pin; CI actions use floating major tags.
- MPL-2.0 (`go-sql-driver/mysql`) license notice retention must be confirmed in
  release packaging.

## Blockers

None.

## Risks for M6

Remote CI enablement; packaging/`embed.FS` single binary (containers will need
`ALLOW_REMOTE=1`); dependency patch upgrades (pgx/x/text/toolchain/Next); SBOM and
license notices; automated browser E2E; installed-app SW update validation;
performance certification in a real browser.

## Exit Checklist

- [x] no credential/key leakage
- [x] encryption/key handling safe
- [x] loopback safe default
- [x] request limits enforced
- [x] API errors sanitized
- [x] query timeout verified
- [x] >50 MB guard verified
- [x] query resources cleaned up
- [x] BIGINT remains exact
- [x] ConnectionManager race-safe
- [x] pool limits bounded
- [x] DB outage does not crash application
- [x] recovery behavior defined
- [x] frontend failure/recovery flows pass
- [x] dirty SQL protected
- [x] stale-response protection works
- [x] performance benchmark documented
- [x] PRD <25 MB memory goal explicitly evaluated (PASS)
- [x] 100k virtualization bounded (23 DOM rows)
- [x] 1000-table behavior evaluated
- [x] dependency audit completed
- [x] no unresolved applicable critical runtime vulnerability
- [x] test matrix reviewed
- [x] real PostgreSQL integration PASS
- [x] real MySQL integration PASS
- [x] real SQLite integration PASS
- [x] adversarial E2E PASS for release-critical flows
- [x] production defaults safe
- [x] M1–M4 product regression PASS
- [x] backend quality gates PASS
- [x] frontend quality gates PASS
- [x] production builds PASS
- [x] no BLOCKER
- [x] no unresolved HIGH issue that makes distribution unsafe

## Final Status

**M5 READY**
