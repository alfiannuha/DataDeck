# DataDeck v0.1.0 — R05 Final Regression & Release Acceptance

## A. Executive Summary
- **Current HEAD:** `02d3ed5` (R05) on `release/v0.1.0-rc` (PR #1). Release source baseline `351444f`; R04 `e81ca8e`.
- **Decision:** **R05 READY** — full local regression green, CI green on the release revision, zero unresolved BLOCKER/HIGH, one test-harness flake fixed.

## B. Prerequisites
R01 (`d0e6461`), R02 (`f29e0b3`), R03 (`ec444da`), R04 (`e81ca8e`), PRF02-T20 — all READY.

## C. Git & Freeze
Branch `release/v0.1.0-rc`; HEAD `02d3ed5`; tracked tree clean; no tags. Changes since `351444f` = version metadata + docs + approved security hardening + the R05 test fix. **No scope change, no dependency change, no product feature.** R01-FIND-007 (`multi_connection_architectural_specification.md`) kept outside the release (untracked, never committed, not referenced by build/CI).

## D. Test Coverage
`test-matrix.md` — 36 scenarios: 30 PASS, 0 FAIL, 2 NOT RUN (rebinding browser exploit, failure injection + perf), 1 N/A-ish (native non-macOS), 1 pending→resolved (CI). Critical gates all PASS.

## E. Backend
gofmt/vet/mod-verify clean; unit + race (15 pkgs) PASS; integration PG 17.11 + MySQL 8.0.46 PASS; API contract PASS. `backend-regression.md`.

## F. Frontend
`npm ci`, lint 0, typecheck, **472 tests**, build + static PASS; Playwright **10/10 twice**. `frontend-regression.md`.

## G. Database
α/β isolation across browse/sort/filter/CRUD/export/import/reconnect/delete PASS; MySQL CRUD + live delete PASS; SQLite CRUD/restore PASS. `database-isolation.md`.

## H. Data Management
TableData, CRUD (PK-only identity + optimistic concurrency), import/export, reconnect, delete — all PASS. `crud-regression.md`, `import-export-regression.md`, `destructive-operation-regression.md`.

## I. Security
CSRF matrix PASS (cross-origin JSON/multipart → 403; same-origin/no-origin allowed); DNS rebinding residual reproduced/characterized (MEDIUM, requires leaked unguessable id); credential encryption + key fail-closed PASS; injection defenses PASS; dependencies triaged (build-time MEDIUM). `security-regression.md`, `dependency-reassessment.md`.

## J. Performance
Binary ~32 MB; startup ~312–430 ms; idle RSS ~4.5 MB; 100k-row 257 ms; 1000-table introspect 193 ms. `performance-report.md`.

## K. Packaging
Static export, embedded single binary, `--version`, health version, loopback listener, key fail-closed, graceful shutdown, 5 cross-compiles PASS. Docker build NOT RUN (no daemon). `packaging-regression.md`.

## L. CI
**Green** on `02d3ed5`: CI run `38325379999` (success), E2E run `38325379969` (success), `gh pr checks` 4/4. `ci-verification.md`. R01-FIND-006 resolved.

## M. Findings
No BLOCKER/HIGH. MEDIUM: R04-FIND-001 (DNS rebinding residual), R04-FIND-002 (npm build-time advisories), PRF02-FIND-001/002. LOW: R04-FIND-003/004, R05-FIND-002/003/004/005. Fixed: R05-FIX-001 (e2e flake). `findings-register.md`.

## N. Release Impact
Eligible for R06. Required actions carried: Docker build (R06/R07), native non-macOS smoke, failure-injection + large-scale perf (optional).

## O. Final Decision
**R05 READY.**

## P. Commit / Report
- **Files inspected:** AGENTS.md, PRD, ADR-010, `docs/api-contract.md`, PRF02-T19/T20, R01–R04 artifacts, backend `internal/{security,config,api,api/middleware,database,dbscript,tableimport}`, frontend `src`+`e2e`, CI workflows, build scripts, Dockerfile.
- **Files changed:** `frontend/e2e/database-script.spec.ts`, `docs/releases/v0.1.0/r05/*` (+ this report + `ci-verification.md`).
- **Commands:** gofmt/vet/mod verify; `go test -count=1 ./...`; `-race`; `-tags=integration -p 1`; `npm ci/lint/typecheck/test/build/build:static`; single-binary + cross-compile x5; live CSRF/binding/key/rebinding checks; `npx playwright test` x2; `gh pr create`/`gh run watch`.
- **Evidence paths:** `docs/releases/v0.1.0/r05/`.
- **Final commit hash:** `02d3ed5` (+ docs follow-up; CI re-verified on the tip).

**Stop — do not begin R06.**
