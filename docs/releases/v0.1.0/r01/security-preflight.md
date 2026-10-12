# DataDeck v0.1.0 — R01 Security Preflight

R04 performs the dedicated release security audit. R01 confirms no obvious
release-blocking exposure.

## Controls verified (evidence: T19/T20 security results + R01 inspection)
| Control | Result |
|---|---|
| Credential encryption (AES-256-GCM, `security` package) | PASS (unit) |
| Stable ENCRYPTION_KEY requirement | Documented (`AGENTS.md`, `.env.example`); wrong/missing key fails closed |
| No credentials in responses/logs | PASS |
| Localhost-only binding | PASS — `127.0.0.1:8099 (LISTEN)` measured |
| Origin/CSRF boundary | PASS — no ACAO for cross-origin; JSON content-type required for destructive POSTs |
| SQL injection (identifiers quoted, values bound) | PASS |
| Row mutation safety (PK identity, optimistic concurrency, affected-rows) | PASS |
| Upload/parser security (CSV/XLSX/SQL, size caps, temp cleanup) | PASS (executed cases) |
| Database restore restrictions (manifest-bound allowlist) | PASS |
| Destructive confirmation (single-use bound token, protected policy, no FORCE) | PASS |
| Sensitive API caching | PASS — `Cache-Control: no-store`; SW bypasses `/api` (ADR-008) |

## Secret & configuration audit
- Pattern scan for private keys / AWS keys / OpenAI-style keys / inline passwords:
  **no matches** in tracked source (values not printed).
- `.gitignore` excludes `.env*`, `data/`, `*.db`, `*.db-shm`, `*.db-wal`, `node_modules/`,
  build output. `.env.example` present with placeholders.
- No real production DSN or key committed. No CI secret substance exposed in workflows.

## Dependency vulnerabilities
- Go: `govulncheck` 0 reachable (last run, T15); re-run mandated in R04.
- npm: no critical (`npm audit --audit-level=critical` passes); high/moderate are the
  build-time postcss/Next chain (documented).

## Localhost deployment boundary
- Backend binds loopback only; no public listener.
- No new public API deployment introduced; destructive APIs require application/json
  (forces preflight) and are not reachable cross-origin without ACAO.
- Static frontend hosting limitation (no persistent Go backend on Vercel) remains documented.

## Release-blocking security findings
**None at R01.** Full adversarial re-test is delegated to R04.
