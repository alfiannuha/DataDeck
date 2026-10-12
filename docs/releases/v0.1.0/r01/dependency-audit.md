# DataDeck v0.1.0 — R01 Dependency Audit

No dependency was changed in R01.

## Go (backend)
- `go.mod` module: `github.com/datadeck/datadeck/backend`; `go 1.26.0`; `toolchain go1.26.9`.
- Direct: `go-chi/chi/v5 v5.3.2`, `go-sql-driver/mysql v1.9.3`, `jackc/pgx/v5 v5.9.2`,
  `xuri/excelize/v2 v2.11.0`, `modernc.org/sqlite v1.38.2`.
- Transitive pinned in `go.mod`; `go.sum` committed.
- `go mod verify` → **all modules verified**.
- CGO: **none required** (`CGO_ENABLED=0` builds and cross-compiles).
- Toolchain `go1.26.9` was adopted in T15 to clear reachable stdlib advisories;
  `govulncheck` reported **0 reachable** at that point (re-run in R04).

## Frontend
- `package.json` `datadeck-frontend@0.0.0`; `package-lock.json` committed.
- Clean install reproducibility: `npm ci` → **532 packages installed (25 s), no lockfile errors**.
- Node: 24.14.0 (CI pins Node 24).
- Build: `npm run build` and `build:static` succeed after the clean install.
- Known advisories (T15/npm): high/moderate are the build-time `postcss`/Next chain;
  `npm audit --audit-level=critical` passes (no critical). Re-run in R04.

## Lockfile consistency
- Both `go.sum` and `package-lock.json` are committed and reproduce cleanly.
- No automatic upgrades performed; any dependency change for security/build must be
  raised as a controlled release-blocking fix with impact analysis.
