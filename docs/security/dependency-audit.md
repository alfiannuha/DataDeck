# DataDeck Dependency & Supply-Chain Audit

Status: M5-T08. Scope: Go modules, npm packages, lockfiles, lifecycle scripts,
CI actions, and a direct-dependency license inventory. No dependency was
upgraded or removed in this task (see "Deferred" items).

## Environment / Tooling

| Tool | Status |
|---|---|
| `go version` | go1.26.5 (module declares `go 1.23.0`) |
| `go mod verify` | **all modules verified** |
| `govulncheck` | installed ad hoc (`golang.org/x/vuln/cmd/govulncheck@latest`, v1.8.0) — network DB reachable |
| `npm audit` | ran against the npm registry (auditReportVersion 2) |
| `go-licenses` | not installed (license inventory done from module `LICENSE` files) |

Vulnerability data **was** retrieved for both ecosystems; results below are
real, not invented. No automatic fixes were applied.

## 1. Dependency Health

- **Go direct:** `github.com/go-chi/chi/v5 v5.3.2`, `github.com/go-sql-driver/mysql v1.9.3`,
  `github.com/jackc/pgx/v5 v5.7.6`, `modernc.org/sqlite v1.38.2` — all actively used;
  no duplicate functionality. Indirect graph is small (17 modules) and pulled by
  the drivers/chi.
- **npm direct:** 20 runtime + 16 dev dependencies; every direct dependency is
  referenced by non-test source (verified by grep). No unused direct dependency
  was found, so none was removed. `@tanstack/react-table` is used (type imports
  in `result-grid.tsx`), so the PRD-selected stack is intact.
- **Lifecycle scripts:** `package.json` has no `preinstall`/`postinstall`/`prepare`
  hooks; scripts are local (`next`, `tsc`, `eslint`, `vitest`,
  `openapi-typescript` reading a committed file). No unexpected network/download
  behavior; no new scripts added.

## 2. Vulnerability Audit Status

### Go — `govulncheck ./...` (exit 3: findings present)

"It found 6 vulnerabilities from 2 modules and the Go standard library" whose
symbols are reachable; a further 6 import-level and 22 module-level advisories
are not called by DataDeck.

| ID | Package | Found | Fixed | Reachable path | Applicability |
|---|---|---|---|---|---|
| GO-2026-5004 | `github.com/jackc/pgx/v5` | v5.7.6 | v5.9.2 | `executePostgresQuery` → `pgx.Conn.Query` → `sanitize.SanitizeSQL` | Upstream "placeholder confusion" SQL-injection advisory. DataDeck sends **no parameters** and intentionally executes arbitrary user SQL, so the privilege impact is limited; still a real upstream fix. |
| GO-2026-5970 | `golang.org/x/text` (indirect) | v0.24.0 | v0.39.0 | `storage.Open` → `sql.Open` → `norm.Form` | DoS on malformed input; indirect. |
| GO-2026-6090 | stdlib `crypto/tls` | go1.26.5 | go1.26.6 | `net/http` server, `crypto/tls` | Fixed by a **toolchain patch**. |
| GO-2026-6089 | stdlib `net/http` | go1.26.5 | go1.26.6 | `server.run` → `ListenAndServe` | Fixed by a toolchain patch. |
| GO-2026-6088 | stdlib `encoding/xml` | go1.26.5 | go1.26.6 | pgx `Values` → `xml.Unmarshal` | Fixed by a toolchain patch. |
| GO-2026-5972 | stdlib `encoding/asn1` | go1.26.5 | go1.26.6 | `sql.Open` → `asn1.Unmarshal` | Fixed by a toolchain patch. |

### npm — `npm audit`

`metadata.vulnerabilities = { low: 0, moderate: 1, high: 1, critical: 0 }`.

| Package | Severity | Direct | Chain | Nature | Applies to shipped product? |
|---|---|---|---|---|---|
| `next` | moderate | yes | via `postcss` | build/bundler advisory (range 9.3.4-canary.0 – 16.3.0-preview.10) | Runtime dependency, but the advisory concerns the bundled PostCSS build path; `15.5.26` is in range. Audit offers only a **major** fix (`next 16.3.6`). |
| `postcss` | high | no (transitive) | pulled by Next/Tailwind | XSS / arbitrary file read via attacker-controlled CSS `sourceMappingURL` (GHSA-6g55-p6wh-862q and follow-ups) | **Build-time only** — it processes DataDeck's own CSS during `next build`, not user CSS at runtime. High severity, low applicability. |

No runtime-exploitable high/critical finding was identified for the shipped
frontend bundle.

## 3. Critical / High Findings

- **High (severity), low applicability:** `postcss` transitive advisories —
  build-time only. No action required for release; fix arrives with a Next/Tailwind
  toolchain bump.
- **Medium (real, actionable):** `pgx v5.7.6` (GO-2026-5004) and
  `golang.org/x/text v0.24.0` (GO-2026-5970); Go toolchain patch level
  (4 stdlib advisories).
- **Medium:** `next` moderate advisory (runtime dependency, build-path issue,
  only a major upgrade offered).
- No **critical** findings in either ecosystem.

## 4. Removed Dependencies

None. Every direct Go/npm dependency is used, and removing PRD-selected
technologies is out of scope. No dependency was added.

## 5. Version / Reproducibility Findings

- **Go:** `go.mod` pins exact versions for direct and indirect modules; `go.sum`
  is committed; `go mod verify` passes. Reproducible.
- **npm:** `package.json` uses caret ranges, but `package-lock.json` (lockfile v3)
  is committed and CI uses **`npm ci`** — installs are lockfile-reproducible.
- **CI Go version:** `actions/setup-go` uses `go-version-file: backend/go.mod`,
  i.e. the `go 1.23.0` directive → CI resolves the latest patch of that line.
  The local govulncheck findings reflect the local **1.26.5** toolchain; the
  exact CI patch level was not observable (remote CI not run), so stdlib
  applicability in CI is **NOT VERIFIED**.
- **Frontend API types** are generated from a committed OpenAPI file
  (`npm run generate:api`), not fetched at build time.

## 6. CI Action Findings

- Actions used: `actions/checkout@v7`, `actions/setup-go@v7`,
  `actions/setup-node@v7` — all three **v7 tags exist** (verified via the GitHub
  API: checkout v7.0.1, setup-go v7.0.0, setup-node v7.0.0).
- **Recommendation (M6):** pin actions to full commit SHAs (Dependabot/renovate
  managed) instead of floating major tags, and pin the Go toolchain patch
  version explicitly rather than inheriting it from `go.mod`.
- No untrusted third-party actions are used; the workflow has
  `permissions: contents: read` and uses ephemeral service credentials.

## 7. License Inventory (direct dependencies)

Go (from module LICENSE files):

| Module | License |
|---|---|
| github.com/go-chi/chi/v5 | MIT |
| github.com/jackc/pgx/v5 | MIT |
| github.com/go-sql-driver/mysql | **MPL-2.0** (see review flag) |
| modernc.org/sqlite | BSD-3-Clause |

npm (from `node_modules/*/package.json`):

| License | Packages |
|---|---|
| MIT | all CodeMirror/`@lezer`, Radix dialogs, TanStack (query/table/virtual), clsx, tailwind-merge, lucide-react, next, react, react-dom, zustand, and dev tooling (eslint, vitest, jsdom, testing-library, vite, …) |
| Apache-2.0 | `class-variance-authority`, `typescript` (dev) |
| ISC | `lucide-react` |

**Human-review flags (not legal conclusions):**
- `github.com/go-sql-driver/mysql` is **MPL-2.0** (weak, file-level copyleft).
  Permissive for distribution but requires retaining the license/notices for
  modified files. Confirm notice retention in the packaged artifact.
- `modernc.org/sqlite` is BSD-3-Clause over SQLite-derived code; ensure its
  BSD notice and any SQLite public-domain statement are included in the release.
- No GPL/AGPL/unknown/restrictive licenses were found among direct dependencies.

## 8. Recommendations / Deferred to M6

1. **Patch Go security advisories in a dedicated task:** bump the build
   toolchain to a patched release (≥ the version fixing GO-2026-6088/6089/6090/5972),
   update `golang.org/x/text` to ≥ v0.39.0, and update `pgx` to ≥ v5.9.2 — then
   re-run `go test ./...`, `-race`, integration, and `govulncheck` to confirm
   zero reachable findings. Verify the chosen versions still satisfy the
   `go 1.23.0` directive (or raise the directive deliberately).
2. **Frontend advisories:** plan a Next/Tailwind toolchain bump (postcss fix) —
   deferred because the audit only offers a Next **major** upgrade and the
   postcss issue is build-time only.
3. **Pin CI actions to commit SHAs** and pin the Go toolchain patch version.
4. **Add `govulncheck` and `npm audit` to the CI pipeline** (with a sensible
   severity policy) so regressions are caught automatically.
5. **Include license/notice files** for MPL-2.0 (mysql) and BSD-3-Clause
   (modernc sqlite) in the release packaging.

## Limitations

- Severity labels here are taken from `govulncheck`/`npm audit` plus an
  applicability assessment; they are not legal or CVSS-certified conclusions.
- Remote CI was not executed (no Git remote), so CI-specific toolchain patch
  levels are not observable; stdlib advisory applicability in CI is NOT VERIFIED.
- npm `postcss` findings were classified as build-time; a future change that
  processes untrusted CSS would change that assessment.
- No SBOM generator was run; the license inventory covers direct dependencies
  only.
