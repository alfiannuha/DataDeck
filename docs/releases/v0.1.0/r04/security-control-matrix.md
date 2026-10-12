# DataDeck v0.1.0 — R04 Security Control Matrix

Evidence is source-inspected + dynamically verified at the R04 baseline (see
`r04-readiness-report.md` for commits/commands). Status: PASS / FAIL / N/A / NOT RUN.

| # | Control | Evidence | Status |
|---|---|---|---|
| C1 | Loopback bind by default (`127.0.0.1`) | `config.DefaultHost`; live `lsof` shows `127.0.0.1:8099` | PASS |
| C2 | Refuse non-loopback bind without `ALLOW_REMOTE=1` | startup message on `HOST=0.0.0.0` | PASS |
| C3 | Credentials encrypted at rest (AES-256-GCM, random nonce, tag verified) | `internal/security/cipher.go` + unit tests | PASS |
| C4 | Required `ENCRYPTION_KEY`; no plaintext fallback | startup refuses missing/invalid key | PASS |
| C5 | Credentials never returned/logged | handler responses mask; cipher errors sanitized | PASS |
| C6 | CORS explicit allowlist (no ACAO for others) | `middleware/cors.go`; dynamic cross-origin check | PASS |
| C7 | CSRF: cross-origin unsafe requests rejected (incl. multipart) | `middleware/csrf.go` + tests; dynamic 403 | PASS |
| C8 | Same-origin and non-browser clients unaffected | dynamic 200/400; e2e 10/10 | PASS |
| C9 | `no-store`/`nosniff`/`no-referrer` on API | `middleware/security.go`; curl | PASS |
| C10 | Explicit database binding; no explorer fallback | `resolveTargetDatabase`; α/β e2e | PASS |
| C11 | Pool key includes `(connection, database)` | `PoolKey`; PRF-01 integration | PASS |
| C12 | Metadata-validated, dialect-quoted identifiers; bound values | `quoteIdentifier`, filter/mutation code | PASS |
| C13 | Row identity = declared PK only (composite ok; no rowid/ctid) | `table_data.go`; mutation tests | PASS |
| C14 | Optimistic concurrency + affected-rows 0/1/>1 | `mutation.go`; tests | PASS |
| C15 | Table import INSERT-only, allowlisted, atomic | `tableimport`; tests; integration | PASS |
| C16 | DB restore manifest-bound + statement allowlist + empty target | `dbscript`; tests | PASS |
| C17 | Delete DB two-phase token (TTL/single-use/bound), protected policy, no FORCE | `database_delete.go`; tests; e2e | PASS |
| C18 | Delete DB independent verification; unknown-outcome reporting | `DatabaseExists`; handler | PASS |
| C19 | Service worker does not cache `/api` | ADR-008 §46; `sw.js` | PASS |
| C20 | No secrets in repo/build metadata | secret scan; version metadata review | PASS |
| C21 | Dependency vulnerabilities triaged; Go reachable = 0 | govulncheck; npm audit | PASS |
| C22 | Docker runs non-root, minimal image, no baked key | `docker/Dockerfile` | PASS |
| C23 | Build is `CGO_ENABLED=0`, no external runtime | build + cross-compile | PASS |
| C24 | Release scripts do not tag/push | script inspection | PASS |
| C25 | Audit logs credential-free; tokens excluded | delete/script handlers | PASS |

## Notes
- No general-purpose authentication (C-N/A): by design; the local boundary is the
  control. Not a gap for v0.1.0 scope.
- DNS rebinding residual tracked as a MEDIUM in `findings-register.md`.
