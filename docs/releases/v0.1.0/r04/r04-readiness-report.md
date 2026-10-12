# DataDeck v0.1.0 — R04 Release Security Audit

## A. Executive Summary
- **Current HEAD:** prior baseline `ec444da` (R03) + this R04 change set.
- **Release baseline:** source `351444f` (PRF02-T20); R01 `d0e6461`, R02 `f29e0b3`, R03 `ec444da`.
- **Security decision:** **R04 READY** — zero unresolved BLOCKER/HIGH. Two in-scope
  hardening fixes applied (CSRF/Origin middleware; two cleared npm advisories).
  Documented residuals are MEDIUM/LOW with low applicability.

## B. Prerequisites
R01/R02/R03 READY (artifact dirs), PRF02-T20 READY. No BLOCKER/HIGH carried without
disposition.

## C. Threat Model
Assets, attackers, and trust boundaries in `threat-model.md`. Local browser →
loopback API → target DB; the primary attacker is a malicious web page.

## D. Security Control Matrix
`security-control-matrix.md` — 25 controls, all PASS (C1–C25). N/A: authentication
(by design).

## E. Credential Security
`credential-security.md`. AES-256-GCM, fresh random nonce, tag verified; required
key (32B/64hex), no plaintext fallback; missing/invalid key aborts startup (dynamic);
no rotation (documented); failure tests all reject with sanitized errors.

## F. API Security
`api-localhost-security.md`. Loopback-only listener (verified); non-loopback refused
without `ALLOW_REMOTE=1`; explicit CORS allowlist; **new CSRF middleware** blocks
cross-origin unsafe requests incl. multipart; same-origin/non-browser allowed;
`no-store`/`nosniff`/`no-referrer`.

## G. Database Security
`database-isolation.md`, `sql-and-crud-security.md`. Explicit binding, pool key
`(connection,database)`, α/β isolation across browse/CRUD/export/import/reconnect/
delete; PK-only identity; optimistic concurrency; affected-rows 0/1/>1; DB restore
transactional (PG/SQLite), MySQL restore disabled.

## H. Import/Export Security
`import-export-security.md`. INSERT-only allowlisted table import; manifest-bound
restore with statement allowlist + empty target; formula guard; size caps; temp
cleanup; no arbitrary SQL execution.

## I. Destructive Operation Security
`destructive-operation-security.md`. Two-phase token (random, 5 min, single-use,
bound), protected policy, typed confirmation + acknowledgment, maintenance connection,
no FORCE, verified deletion, unknown-outcome reporting, per-target serialization.

## J. Frontend Security
No credential persistence (Zustand holds transient UI state only; no auth tokens);
TanStack Query caches server data in memory; service worker bypasses `/api`
(ADR-008); untrusted content rendered as text (React escaping; constraint definition
text rendered as text, never HTML).

## K. Dependencies & Supply Chain
`dependency-audit.md`, `supply-chain-audit.md`. `govulncheck` = 0 reachable. npm:
`sharp`/`source-map-js`/`postcss` are build-time or optional-image-lib advisories,
not in the shipped runtime (MEDIUM). An `npm audit fix` trial was **reverted** because
it also downgraded `typescript`; **no dependency changed in R04**. Module checksums
verified; Docker non-root; release tags publish (not used).

## L. Security Test Results
| Type | Result |
|---|---|
| Unit (crypto, CSRF, delete token, import/restore allowlist, mutation) | PASS |
| Integration (isolation, CRUD, import/restore, delete — PG/MySQL) | PASS |
| Adversarial (injection corpus, dangerous statements, protected DB, token misuse) | PASS |
| Race (`go test -race` api/handler/database/middleware) | PASS |
| Dynamic (binding, key fail-closed, CSRF 403/400) | PASS |
| Browser E2E (α/β isolation incl. CSRF middleware) | PASS (10/10; one order-flake re-ran green) |
| Failure injection (disk full, network loss) | NOT RUN (design-documented) |

## M. Findings
No BLOCKER/HIGH. MEDIUM: R04-FIND-001 (DNS rebinding residual), R04-FIND-002
(postcss build-time), PRF02-FIND-001/002. LOW: R04-FIND-003/004/005. See
`findings-register.md`.

## N. Remediation
- R04-FIX-001: CSRF/Origin middleware (`middleware/csrf.go` + tests) — new baseline.
No dependency change was made (the `npm audit fix` trial was reverted). Re-validated:
backend unit/race/build; frontend lint/typecheck/472 tests/build; E2E green.
Baseline change: `router.go` + new CSRF middleware only.

## O. Release Impact
Eligible. Remaining risks are documented MEDIUM/LOW (DNS-rebinding residual;
build-time advisories) and do not undermine required acceptance.

## P. Final Decision
**R04 READY.**

## Q. Commit / Report
- **Files inspected:** `internal/security/cipher.go`; `internal/config/config.go`;
  `internal/api/middleware/{cors,security}.go`; `internal/api/router.go`;
  `internal/database/{manager,table_data,mutation,database_admin,restore}.go`;
  `internal/dbscript/*`; `internal/tableimport/*`; `internal/api/handler/{database_delete,database_import,table_import,connection}.go`; frontend `lib/api-client`, stores, SW; `docker/Dockerfile`; CI workflows; dependency manifests.
- **Files changed:** `backend/internal/api/middleware/csrf.go` (+test),
  `backend/internal/api/router.go`, and the R04 docs. (No dependency change.)
- **Commands:** `govulncheck ./...`; `npm audit --omit=dev`; `go build/vet/test/-race`;
  `npm ci`, `npm run lint/typecheck/test/build/build:static`; single-binary build +
  live CSRF/binding/key checks; `npx playwright test`.
- **Test results:** as §L.
- **Evidence paths:** `docs/releases/v0.1.0/r04/*`.
- **Original baseline commit:** `351444f`; **pre-R04 HEAD:** `ec444da`;
  **current/final report commit:** this commit.
- **Remaining findings:** see `findings-register.md` (no BLOCKER/HIGH).

**Stop — do not begin R05.**
