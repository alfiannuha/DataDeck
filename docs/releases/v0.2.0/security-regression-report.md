# DataDeck v0.2.0 — Security Regression Report

v0.2.0 changes are additive; no security control was weakened. Full v0.1.0 posture
retained (see `docs/releases/v0.1.0/r10/security-assessment.md`).

| Control | Status |
|---|---|
| Loopback-only default bind / non-loopback refusal | UNCHANGED |
| CORS allowlist + CSRF/Origin middleware | UNCHANGED (new GET endpoint is a safe method) |
| AES-256-GCM credential encryption; fail-closed key | UNCHANGED |
| No credentials/keys/DSNs in logs, API, or telemetry | VERIFIED — status payload has no secret fields; handler test asserts no forbidden tokens |
| SQL identifier validation + bound values | UNCHANGED (no new SQL construction) |
| Import/restore allowlists, destructive tokens | UNCHANGED |
| No cross-connection data leakage | POOL KEY unchanged `(connection,database)`; α/β isolation retained |

## New endpoint review (`GET /api/v1/connections/status`)
- Read-only, no side effects (never opens a pool).
- Returns only: registered-connection count + pool telemetry (identity, driver, state,
  database/sql counters, last-used). No passwords, DSNs, keys, or raw driver errors.
- Not a destructive route; CSRF not applicable (GET), but the Origin/CORS posture is
  unchanged for all routes.

## Residual (carried, accepted)
R04-FIND-001 DNS rebinding MEDIUM; R04-FIND-002 build-time advisories MEDIUM;
R07-F-001 reproducibility MEDIUM. No new BLOCKER/HIGH.

No dependency was added or upgraded in v0.2.0.
