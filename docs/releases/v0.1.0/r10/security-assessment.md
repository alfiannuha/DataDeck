# DataDeck v0.1.0 — R10 Security Assessment

Reviewed R04 (audit) → R09 (published smoke). No unresolved BLOCKER/HIGH.

| Control | Result | Evidence |
|---|---|---|
| Loopback-only default bind | PASS (127.0.0.1) | R04/R09 live |
| Non-loopback refused without `ALLOW_REMOTE=1` | PASS | R04 |
| Credential encryption (AES-256-GCM, random nonce, tag verify) | PASS | T15/R04 |
| Stable `ENCRYPTION_KEY`, fail-closed on missing/invalid | PASS | R04/R09 |
| No plaintext fallback / no key exposure | PASS | R04 |
| CORS explicit allowlist | PASS | R04 |
| CSRF/Origin middleware (cross-origin JSON+multipart → 403) | PASS | R04/R09 |
| SQL identifier validation + bound values | PASS | T19/R04 |
| Import allowlist (INSERT-only) / restore allowlist + empty target | PASS | T15/T17/R04 |
| Destructive token (single-use, TTL, bound) + protected DB policy + no FORCE | PASS | T18/R04 |
| Sensitive logging redaction | PASS | R04 |
| Browser storage: no credentials persisted; SW bypasses `/api` | PASS | R04 |
| Security headers (`no-store`/`nosniff`/`no-referrer`) | PASS | R09 |

## Dependency advisories (reconciled)
- Go `govulncheck`: **0 reachable** (R07).
- npm: `sharp`/`source-map-js`/`postcss` are build-time/optional — **not reachable**
  in the shipped static export/binary (MEDIUM, accepted). **No dependency change**
  was made (the R04 `npm audit fix` trial was reverted).

## Residual security findings
- **R04-FIND-001** DNS rebinding residual — MEDIUM, accepted: requires an unguessable
  128-bit connection id; responses unreadable cross-origin; browser JSON mutations
  preflight-blocked. No practical browser exploit demonstrated (R05/R07/R09).
- **R04-FIND-003** static HTML lacks CSP/X-Frame — LOW, pre-existing.

No credential leak, public exposure, or wrong-database security defect observed.
