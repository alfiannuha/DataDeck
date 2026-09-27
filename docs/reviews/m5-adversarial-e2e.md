# M5 Adversarial Production-Readiness E2E

Status: M5-T11. Production-like build with real services. No mocks substituted
for failed real checks. All flows were exercised against a running backend +
database servers; browser flows used a production frontend build.

## Environment

| Item | Value |
|---|---|
| Backend | release build `CGO_ENABLED=0`, `ENVIRONMENT=test`, loopback `127.0.0.1:8080` |
| Internal store | temporary SQLite (`/tmp/dd_e2e_store.db`) |
| PostgreSQL | 17.11 (Homebrew), real service |
| MySQL | 8.0.46 (Homebrew), real service |
| SQLite target | real files under `/tmp` |
| Frontend | production `next start` on `127.0.0.1:3000` |
| Browser | Chromium via Playwright (real downloads, offline emulation) |
| Encryption key | benchmark-only 32-byte key |

Commands were the real API (`/api/v1`) over HTTP plus Playwright flows; no
behavior was simulated except network/offline emulation and service stop/start,
which are the actual failure modes under test.

## Flow Matrix

| Flow | Scenario | Result |
|---|---|---|
| A | Credential failures (wrong MySQL password, invalid PG user, corrupted stored ciphertext) | **PASS** |
| B | Database outage: PG and MySQL stop → query → restart → recovery | **PASS** |
| C | Real query timeout (`pg_sleep`), history status, subsequent queries | **PASS** |
| D | >50 MB result truncation, bounded rows, responsive grid, export warning | **PASS** |
| E | Concurrent queries across PG/MySQL/SQLite, isolation | **PASS** |
| F | Backend restart with dirty SQL in the browser | **PASS** |
| G | Offline → guarded actions → online, PWA cache safety | **PASS** |
| H | Malformed/oversized/invalid API input, unsupported method/content-type | **PASS** |
| I | SQLite target failures (inaccessible path, lock contention) | **PASS** |
| J | Export security (formula/HTML/quotes/newlines/BIGINT/binary) | **PASS** |
| K | Resource recovery: health + normal PG/MySQL/SQLite queries after stress | **PASS** |

Totals: 11/11 flows PASS. Two initial automated assertions reported FAIL and were
**diagnosed as test-harness artifacts, not product defects** (see "Failures
Discovered").

## Flow Detail

### A — Credential failure (PASS)
- Wrong MySQL password → `502 CONNECTION_ERROR`; response contains no password.
- Invalid PostgreSQL user → `502 CONNECTION_ERROR`; no password echoed.
  (Local PG uses trust auth, so a *wrong password* alone is accepted by the
  server; an invalid user exercises the same safe failure path. Noted honestly.)
- Corrupted stored credential (store `encrypted_password` overwritten with
  invalid ciphertext) → `500 INTERNAL_ERROR`; ciphertext never echoed; backend
  stays operational.

### B — Database outage (PASS)
- PG: `SELECT 1` OK → stop service → `500 INTERNAL_ERROR` (bounded) with
  `/health` still 200 → restart → next query OK (transparent pool reconnect).
- MySQL: identical behavior.
- **Documented behavior:** a connection loss on an *established* pool surfaces as
  sanitized `INTERNAL_ERROR` rather than `502 CONNECTION_ERROR` (existing M5-T05
  finding; no crash, automatic recovery).

### C — Query timeout (PASS)
- `SELECT pg_sleep(5)` with `timeout_seconds:1` → `504 QUERY_TIMEOUT`.
- History records the attempt as `ERROR` with `error_message: "query timeout"`.
- `SELECT 1` succeeds immediately after the timeout.

### D — Huge result (PASS)
- Real PG query producing ~60 MB returned `truncated: true` with 52,167 rows
  (bounded; the guard stops accumulation rather than reading everything).
- `/health` remained 200 after the large result.
- Browser: status showed `Success · … · 52167 rows` + `Partial result (50 MB
  limit)`; the grid rendered (`gridcell` visible, virtualized); Export CSV opened
  the **partial-result warning** dialog naming the returned row count.

### E — Concurrent work (PASS)
- Simultaneous PG (`9007199254740993::bigint`), MySQL
  (`CAST(9223372036854775807 AS SIGNED)`), SQLite (`SELECT 1`) executions each
  returned their own engine's correct data — no cross-connection contamination.

### F — Backend restart with dirty SQL (PASS)
- Browser had dirty SQL (`SELECT 'dirty-marker-adv' AS m;`).
- Backend killed → Run produced a visible failure; **editor SQL retained
  unchanged**.
- Backend restarted → **no grid/result before an explicit Run** (no
  auto-execution); editor SQL still present; clicking Run succeeded
  (`dirty-marker-adv` rendered).

### G — Network offline/online (PASS)
- Offline: `navigator.onLine=false`, backend-unavailable banner shown, app shell
  present, **Run disabled**.
- Online: banner cleared, Run enabled again.
- Cache audit: only `datadeck-shell-v2` with `/`, `/manifest.json`, and the two
  icons; **zero `/api` entries** cached.

### H — Malformed API (PASS, server survived)
Observed statuses, all safe, no panic, backend healthy afterward:
malformed JSON `400`, oversized body (>1 MiB) `400`, `page=0` `400`,
`page=abc` `400`, negative timeout `400`, missing connection id `400`,
unknown driver `400`, `text/plain` content type `400`, unsupported method
`PUT /connections` `405`, unknown connection `404`.

### I — SQLite failure (PASS)
- Inaccessible path (`.../nested/...` under a file) → bounded `502`.
- Lock contention: another process held `BEGIN EXCLUSIVE`; an INSERT returned a
  bounded `400 SQL_ERROR`; after the lock was released, INSERT and
  `COUNT(*)` succeeded. Internal app store stayed healthy throughout.

### J — Export security (PASS, real downloaded files)
CSV: `=FORMULA`/`+FORMULA`/`@FORMULA` prefixed with `'`; HTML-like text kept
inert as text; quotes doubled; newline preserved inside a quoted field;
BIGINT exact; binary base64; negative number `-5` untouched. JSON: parsed as an
array, formulas/HTML preserved as plain strings (no execution context),
BIGINT exact, binary base64, negative number as a number.

### K — Resource recovery (PASS)
After all failure/stress scenarios: `/health` 200, and normal PK/MySQL/SQLite
queries (`SELECT 1`, `SELECT id FROM t`) all succeeded.

## Failures Discovered

Two automated assertions failed initially and were **not product defects**:

1. **"SQLite works after lock released" / "normal SQLite query" FAIL** — the
   harness had deleted the live target SQLite file mid-test, so the recreated
   file lacked the `t` table (`no such table: t`). A clean re-run (fresh file,
   create table, lock, release) passed. Behavior on a deleted target file is
   defined: the pool recovers on ordinary statements; schema present only in the
   deleted inode is gone until recreated.
2. **Invalid PostgreSQL password** cannot be tested with a wrong password alone
   because the local PG uses trust auth; an invalid user was used instead and
   exercised the same safe failure path.

## Security Observations

- No credential leakage in responses or browser state across any failure path.
- Corrupted ciphertext yields a generic `INTERNAL_ERROR` with no ciphertext echo.
- Service-worker caches contain only app-shell/static assets; no API data.
- Export never executes formulas/HTML and preserves BIGINT exactly.
- Malformed input is rejected with sanitized envelopes; the server did not panic.

## Resource / Recovery Observations

- DB outage: bounded error, `/health` stays 200, transparent recovery on restart.
- Timeout: bounded, recorded in history, connection reusable.
- >50 MB: bounded truncation with responsive UI and informed export.
- Restart with dirty SQL: no data loss, no auto-execution.
- Offline: explicit unavailable state; no sensitive caching; clean recovery.

## Files Changed

- `docs/reviews/m5-adversarial-e2e.md` (new). No product code changes; all
  temporary servers/DBs/artifacts stopped and removed.

## Tests

This task is an E2E verification: real HTTP flows against real PG/MySQL/SQLite,
real browser flows (downloads, offline emulation, backend restart), and real
service stop/start. No mocks replaced any failing real check.

## Blockers for M5 Exit

None. One documented MEDIUM behavior (established-pool outage → `500
INTERNAL_ERROR` instead of `502 CONNECTION_ERROR`) remains a hardening item; it
is safe and recovers automatically. All 11 adversarial flows PASS.
