# M5 Security Audit — Threat Model & Implementation Review

## Executive Summary

The M4 implementation matches the intended local-first threat model: no
credentials are returned or logged, the encryption boundary holds, the daemon
binds to loopback by default, target identifiers are quoted, results are capped,
the service worker caches no API data, and exports apply a formula policy.

No **BLOCKER** or **HIGH** findings were identified. The audit produced three
**MEDIUM** findings (binding opt-in guard, store file permission hardening,
resource governance), four **LOW** findings, and several **INFO** observations.
None makes M4 unsafe; they are hardening targets for the rest of M5.

## Validation Environment

- macOS arm64; Go 1.26.5; Node 24.14.0; Next.js 15.5.26
- Real PostgreSQL 17.11 / MySQL 8.0.46 / SQLite (`modernc.org/sqlite` v1.38.2)
- Real Go backend (`datadeck-server`), production frontend build, Chromium
  (Playwright)
- Evidence from M4 E2E runs, M5 baseline (`docs/reviews/m5-baseline.md`), and a
  fresh source review of the security-relevant packages.

## Commands / Checks Executed During Audit

```text
grep/read: config.go, main.go, security/cipher.go, database/manager.go,
           api/router.go, api/middleware/{logger,cors}.go,
           api/handler/{connection,query,errors}.go, storage/storage.go,
           database/{postgres,postgres_query,mysql,sqlite}.go,
           frontend lib/pwa/*, lib/export/*, public/sw.js
runtime:   started real backend; requested /connections/test with a distinctive
           password; grep'd the server log -> 0 occurrences of the secret
runtime:   /connections response field audit -> no password/encrypted fields
runtime:   Cache Storage audit (M4) -> only shell assets, no /api entries
```

## Trust Boundaries Review

Documented in `docs/security/threat-model.md` §2. Boundaries are respected in
code: repositories are the only writer of the embedded store; the manager is the
only opener of target pools; the request logger records no headers/bodies; the
service worker excludes `/api`.

## Backend Audit

- **Loopback default:** `config.DefaultHost = "127.0.0.1"`; `envOr("HOST",
  DefaultHost)`; `Addr()` joins host:port. PASS. (SEC-MED-1 below on widening.)
- **Startup key requirement:** `security.NewCipher(cfg.EncryptionKey)` is called
  before the server starts; an invalid/missing key aborts startup. PASS.
- **Encryption:** AES-256-GCM via stdlib, `formatVersion` byte, fresh
  `crypto/rand` nonce per `Encrypt`, authentication on decrypt, errors never echo
  input or plaintext. PASS.
- **Request logging:** logs method, path, status, bytes, duration, request id
  only; headers/query strings/bodies deliberately excluded. PASS.
- **Panic recovery:** panics logged server-side; client gets a sanitized
  `INTERNAL_ERROR`. PASS.
- **Body limit:** every JSON endpoint uses `decodeJSON` →
  `http.MaxBytesReader(..., 1 MiB)`. PASS.
- **CORS:** explicit origin allowlist (default localhost/127.0.0.1:3000),
  restricted methods/headers, `Vary: Origin`. PASS.
- **Timeout / cap:** handler applies `context.WithTimeout` (default 30 s, max
  300 s); result buffer caps JSON payload at 50 MB and sets `truncated: true`.
  PASS.
- **Cross-connection isolation:** execution resolves driver/DSN from the
  connection profile and the pool is keyed by connection id; a PostgreSQL-only
  function sent to a SQLite connection failed with `SQL_ERROR` without touching
  PostgreSQL (M4 evidence). PASS.
- **Pool lifecycle:** pool closed on connection delete and on shutdown
  (`CloseAll`); test connections use a throwaway pool. PASS, with the residual
  in SEC-LOW-2.
- **SQLite path:** `sqliteDSN` normalizes to absolute; the app store is a
  separate file; no filesystem browsing API. PASS.

## Frontend Audit

- No `localStorage`/`sessionStorage`/`IndexedDB`/cookies anywhere in `src`. PASS.
- Password field is `type="password"` with `autoComplete="new-password"`; the
  value lives only in component state. PASS.
- No `dangerouslySetInnerHTML`; React escapes all rendered values (cells, SQL,
  errors). PASS.
- Errors surface `code`/`message`/`position` from the envelope; no raw responses.
  PASS.
- Tabs bind to their own connection; no silent rebinding (M4 integration). PASS.

## PWA / Service Worker Audit

- `public/sw.js` returns early for non-GET, cross-origin, and `/api*` requests;
  only shell assets are precached. `CACHE_PREFIX` cleanup is scoped. PASS.
- Cache Storage after real API traffic contained only `/manifest.json` + icons +
  `/` (M4). No connection/schema/query/history/saved/credential data cached. PASS.
- Update activation is user-controlled; dirty SQL protected (M4 + M5 baseline
  real-browser lifecycle). PASS.

## Storage Audit

- Migrations are embedded and transactional; schema validated at startup. PASS.
- Store uses WAL, `foreign_keys(1)`, `busy_timeout`; single writer connection.
  PASS.
- Directory forced to `0700`; **file modes are not explicitly set** (SEC-MED-2).
- Store content: encrypted passwords, connection metadata, history, saved SQL —
  no query results persisted. PASS.

## Export Audit

- Serializers are pure over `QueryResult`; no re-execution, no DOM scraping.
  PASS.
- CSV formula policy prefixes `= + @` and non-numeric `-`; numeric negatives and
  BIGINT (-9223372036854775808) untouched. PASS, with SEC-LOW-3 residual.
- Filename built from a sanitized connection part plus fixed pattern; no path
  separators; length-capped. PASS, with SEC-LOW-4 residual.
- JSON output preserves BIGINT strings, nested JSON, base64 binary; duplicate
  column names disambiguated. PASS.

## Network Binding Audit

Default `127.0.0.1` confirmed at config and runtime (`loopback:true` logged). The
only way to widen is setting `HOST`; there is no confirmation and no
authentication, so a widened bind exposes the API (SEC-MED-1).

## Findings

Severity counts: BLOCKER 0 · HIGH 0 · MEDIUM 3 · LOW 4 · INFO 4.

### SEC-MED-1 — Non-loopback binding has no explicit opt-in guard
- **Threat:** API exposure to the network. **Asset:** all.
- **Path:** set `HOST=0.0.0.0` → `ListenAndServe` on all interfaces; no auth.
- **Mitigation present:** default is loopback; `loopback` flag is logged; CORS
  restricts browser origins.
- **Evidence:** `config.go` accepts any non-empty `HOST`; `main.go` logs
  `slog.Bool("loopback", cfg.IsLoopback())` but does not block or warn.
- **Residual risk:** operator misconfiguration silently exposes the daemon.
- **Recommended task:** reject/confirm non-loopback binds (e.g., require an
  explicit `ALLOW_REMOTE=1` plus a prominent warning), and document the trust
  change (no auth, plaintext HTTP).

### SEC-MED-2 — Internal store file permissions rely on umask
- **Threat:** local plaintext-adjacent data exposure (encrypted secrets, history,
  saved SQL). **Asset:** app store.
- **Path:** another local user reads `datadeck.db`/`-wal`/`-shm` if permissions
  are group/other-readable.
- **Mitigation present:** parent directory created `0700`; SQLite file created by
  the driver with default umask.
- **Evidence:** `storage.Open` calls `os.MkdirAll(dir, 0o700)` but never
  `os.Chmod` on the DB files.
- **Residual risk:** with a permissive umask (e.g., 022), files are 0644; the
  `0700` directory mitigates traversal but not all backup/sync scenarios.
- **Recommended task:** `chmod 0600` the store and WAL/SHM files (and document
  `~/.datadeck/` as the recommended location).

### SEC-MED-3 — No per-connection execution governance
- **Threat:** resource exhaustion / denial of service from a local process.
- **Asset:** daemon, target DB.
- **Path:** many concurrent `/query/execute` calls; `timeout_seconds` allowed up
  to 300 s holds pool connections and memory.
- **Mitigation present:** bounded pool sizes, 30 s default timeout, 50 MB result
  cap.
- **Evidence:** router installs only RequestID/Logger/Recoverer/CORS; no rate or
  concurrency limiter; `maxQueryTimeoutSeconds = 300`.
- **Residual risk:** local DoS and target-DB load; no fairness control.
- **Recommended task:** add a per-connection concurrency/queue limit and/or a
  modest global execution limiter with clear errors.

### SEC-LOW-1 — Connection test failures may log host/user/database
- **Threat:** low-value metadata leakage. **Asset:** connection metadata.
- **Path:** `connection_test_failed` logs `err.Error()`; driver errors can include
  host/user/db (never the password).
- **Mitigation present:** DSNs with credentials are not logged; verified 0
  occurrences of a probe password in logs.
- **Residual risk:** username/host visible in local logs.
- **Recommended task:** optionally redact user/host in test-failure logs.

### SEC-LOW-2 — Long-lived pools without idle eviction
- **Threat:** stale/abandoned resources. **Asset:** daemon/DB.
- **Path:** a profile used once keeps a pool until delete or shutdown.
- **Mitigation present:** `Close` on delete, `CloseAll` on shutdown.
- **Residual risk:** unused connections consume DB handles.
- **Recommended task:** idle TTL eviction for managed pools.

### SEC-LOW-3 — Export formula heuristic misses leading whitespace/control chars
- **Threat:** CSV formula injection via padded cells (e.g., `" =1+1"`).
- **Asset:** user's spreadsheet.
- **Path:** `guardFormulaExport` inspects only the first character.
- **Mitigation present:** `= + @` and non-numeric `-` are prefixed; numerics
  preserved.
- **Evidence:** `frontend/src/lib/export/values.ts`.
- **Residual risk:** niche bypass with leading whitespace/tab.
- **Recommended task:** normalize leading whitespace/control chars before the
  marker check, with a regression test.

### SEC-LOW-4 — Export filename reserved-device names not special-cased
- **Threat:** file write quirks on Windows-style reserved names.
- **Asset:** exported file.
- **Mitigation present:** sanitization removes separators/hostile chars; fixed
  prefix + timestamp make exact reserved names improbable.
- **Residual risk:** negligible on macOS/Linux; relevant only if Windows
  packaging is added.
- **Recommended task:** fold into Windows release validation.

### INFO findings

- **SEC-INFO-1:** No application authentication; loopback trust by design.
- **SEC-INFO-2:** Credentials traverse loopback HTTP in the JSON body (plaintext
  on the loopback interface). Acceptable for local-first; would matter if bound
  wider (see SEC-MED-1).
- **SEC-INFO-3:** SQL error messages are returned to the client (schema names
  visible); expected behavior for a database GUI.
- **SEC-INFO-4:** No `engines` field in `frontend/package.json`; Node 24 used
  locally (also baseline R5).

## Existing Protections (verified)

AES-256-GCM with per-record nonce + format version; startup key validation;
credential exclusion from responses and logs; request logger with no
headers/bodies; panic recovery; 1 MiB body limit; CORS allowlist; 30 s query
timeout; 50 MB result cap with `truncated`; BIGINT-as-string; driver identifier
quoting; cross-connection isolation; pool closure on delete/shutdown; no client
credential storage; service worker `/api` exclusion and scoped cache cleanup;
export formula policy; React escaping; loopback default binding.

## Recommended Remediation Mapping

| Finding | Recommended M5 hardening task |
|---|---|
| SEC-MED-1 | Binding guard: explicit `ALLOW_REMOTE` opt-in + startup warning + docs |
| SEC-MED-2 | Store permission hardening: `0600` on DB/WAL/SHM + location guidance |
| SEC-MED-3 | Execution governance: per-connection concurrency limit + timeout policy |
| SEC-LOW-1 | Redact host/user in connection-test failure logs |
| SEC-LOW-2 | Idle pool eviction (TTL) |
| SEC-LOW-3 | Export formula hardening for leading whitespace/control chars |
| SEC-LOW-4 | Windows reserved-name handling during release validation |
| SEC-INFO-4 | Add Node `engines` pin + dependency audit (baseline R5) |

## Scope Audit

This task performed review and classification only. No architectural change, no
product feature, and no test weakening. The two documents below are the only
changes.

## Final Status

Security audit complete for M5-T01. **0 BLOCKER, 0 HIGH.** Recommended
hardening continues in subsequent M5 tasks.
