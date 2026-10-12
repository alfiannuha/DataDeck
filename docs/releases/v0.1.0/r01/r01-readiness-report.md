# DataDeck v0.1.0 — R01 Release Freeze & Preflight

## A. Executive Summary
- **Candidate baseline:** `351444f` (PRF02-T20), branch `main`.
- **Release scope:** frozen (see `scope-manifest.md`) — full local-first DataDeck
  with connection profiles, multi-database explorer, table data/CRUD/structure,
  table + database import/export, reconnect, and policy-gated Delete Database.
- **Decision:** **R01 READY** — with two documented conditions to close before
  R06 (CI on the frozen commit; resolve the stray untracked file).

## B. Prerequisite Verification
- **M0–M6:** milestone reports present under `docs/reviews/`; local suites pass at HEAD.
- **PRF01:** PostgreSQL multi-database readiness reviewed; PRF01 suites + α/β isolation pass.
- **PRF02-T20:** `docs/validation/prf02-t20/final-readiness-report.md` explicitly states **PRF02 READY** (0 BLOCKER/HIGH). Verified.

## C. Repository State
- Branch `main`; HEAD `351444f`; tracked tree clean.
- 11 commits ahead of `origin/main` (`624defc` = T09); no tags; one untracked
  unrelated file (`multi_connection_architectural_specification.md`).
- Remote: `origin` = GitHub `alfiannuha/DataDeck` (sanitized).

## D. Scope Freeze
Included features, deferred features and engine limitations are enumerated in
`scope-manifest.md`. No scope change proposed. Freeze policy: release-blocking
fixes only.

## E. Dependency & Build Preflight
- `go mod verify` PASS; `CGO_ENABLED=0 go build ./...` PASS.
- `npm ci` PASS (532 packages, lockfile-reproducible); `npm run build` + `build:static` PASS.
- Single binary built (embedded frontend) ~32 MB; `--version` prints `0.1.0-dev`.
- Cross-compiles (compile-only) PASS: darwin/{amd64,arm64}, linux/{amd64,arm64}, windows/amd64.
See `dependency-audit.md`, `baseline-audit.md`.

## F. CI & Regression
- **CI for `351444f`: NOT VERIFIED** — unpushed, no run exists. Local equivalents
  of every CI job pass (unit/race/integration/format/vet/CGO/e2e). See `ci-preflight.md`.
- Critical gates pass: α/β isolation, mutation safety, import atomicity, export
  integrity, restore safety, reconnect lifecycle, delete confirmation/protection,
  localhost boundary, credential encryption, single-binary packaging.

## G. Security
No release-blocking finding. Credential encryption, localhost-only binding,
CSRF/Origin boundary, SQL-injection defenses, destructive safeguards, and secret
scan all PASS. Full adversarial audit delegated to R04. See `security-preflight.md`.

## H. Performance
Binary ~32 MB; startup ~312 ms (10 ms poll) to ~430 ms (50 ms poll); idle RSS
observed 4.5–24.8 MB depending on measurement timing (post-start vs settled).
Loopback-only listener. Baseline comparison and explanations in
`docs/validation/prf02-t20/performance-readiness.md`.

## I. Release Risk Register
14 risks documented in `risk-register.md`. Open (non-blocking): R-10 native
non-macOS smoke, R-13 CI-on-baseline. No open HIGH data/security risk.

## J. Freeze Record
See `release-freeze.md`. Candidate `351444f`, tree clean (tracked), CI NOT VERIFIED,
0 BLOCKER, 0 HIGH, tag NOT created, NOT published. R01 documentation commit is
separate from the source baseline.

## K. R02 Handoff
Version locations and required changes in `version-inventory.md`. Preconditions
for R02: (1) confirm freeze; (2) choose whether `frontend/package.json` tracks the
app version; (3) decide on a `/health` or About version surface. No version file
changed in R01.

## L. Final Decision
**R01 READY** — prerequisites verified; scope frozen; candidate baseline identified
and immutable; dependencies reproducible; build preflight passes; critical
regression evidence valid; zero BLOCKER/HIGH; no critical security concern; engine
matrix and UI gating aligned; release-pipeline side effects understood; version
inventory complete; freeze record created; no tag/version/publish performed.

Conditions carried into later stages (not R01 blockers):
1. **R01-FIND-006 / R-13:** push the baseline or open a PR and obtain green CI on
   the frozen commit **before R06**.
2. **R01-FIND-007:** resolve the untracked repo-root file before R06.

## M. Commit / Report
- **Files inspected:** AGENTS.md, PRD.md, ADR-010 (+amendments), `docs/api-contract.md`,
  M0–M6 `docs/reviews/*`, PRF01 readiness, `docs/validation/prf02-t19|t20/*`,
  `go.mod`/`go.sum`, `frontend/package.json`/`-lock`, build scripts, CI workflows,
  Dockerfile, `.gitignore`, `VERSION`.
- **Files changed (this task):** `docs/releases/v0.1.0/r01/*` (11 artifacts).
- **Commands executed:** `git status/branch/rev-parse/log/remote/tag`, `git rev-list --left-right`,
  `gh run list`, `go mod verify`, `CGO_ENABLED=0 go build`, `npm ci`, `npm run build`,
  `npm run build:static`, embedded single-binary build + start (health/RSS/listener), secret scan.
- **Tests executed:** unit/race/integration/e2e at HEAD (see `regression-summary.md`).
- **Evidence paths:** `docs/releases/v0.1.0/r01/`, `docs/validation/prf02-t19/`, `docs/validation/prf02-t20/`.
- **Candidate baseline hash:** `351444f`.
- **Documentation commit hash:** recorded in git (this commit).
- **Findings/limitations:** R01-FIND-006 (CI not verified for baseline), R01-FIND-007
  (stray untracked file), PRF02 NR-001…005.

**Stop — do not begin R02.**
