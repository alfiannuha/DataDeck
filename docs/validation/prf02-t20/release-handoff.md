# PRF02 → v0.1.0 Release Handoff

This handoff **authorizes beginning release preparation (R01) only**. It does NOT
authorize creating a tag, bumping the version, publishing a release, uploading
artifacts, or deploying.

## Verified baseline
| Item | Value |
|---|---|
| Review commit | `e3f6c20` (T19 evidence); validated code `7958007` (PRF02-T18) |
| T20 report commit | recorded in git after this commit |
| Build mode | `CGO_ENABLED=0`, embedded static frontend, localhost binding |
| Platforms verified | macOS arm64 native; 5-target cross-compile (compile-only) |

## Feature scope (PRF-02)
Typed multi-tab workspace; empty-workspace lifecycle; server-side Table Data
browse/filter/sort/pagination; safe single-row CRUD (insert/update/delete/duplicate)
with declared-PK identity and optimistic concurrency; table structure; table
export (CSV/XLSX/SQL); table import (CSV + restricted INSERT-only SQL, atomic);
database context menu; database-scoped reconnect; database SQL export/restore
(base tables); two-phase Delete Database.

## Supported engines
- **PostgreSQL** — all features (restore + delete included).
- **MySQL** — all features **except restore** (disabled).
- **SQLite** — all features **except delete database** (disabled).

## Capability limitations (approved policy — ADR-010)
- XLSX **import** deferred (all engines).
- MySQL **restore** disabled (DDL implicit commit; no atomic guarantee).
- SQLite **delete database** disabled (local file; needs a separate design).
- Database export covers **base tables only**; views/sequences/triggers/
  functions/extensions excluded and named in the artifact header.
- No `WITH (FORCE)`; no `CASCADE`; no automatic retry after an ambiguous outcome.

## Accepted findings (release disposition)
- MEDIUM: PRF02-FIND-001 (binary footprint ~2×), FIND-002 (startup methodology).
- LOW: PRF02-FIND-003 (no CSP/X-Frame on root HTML — pre-existing),
  FIND-004 (transient MySQL integration flake, not reproducible),
  FIND-005 (ignored release doc).
- NOT VERIFIED: PRF02-NR-001…005 (native non-macOS smoke, large-scale perf,
  reconnect soak, fresh XLSX adversarial fixtures, remote CI).

## Evidence index
- Requirements/architecture/contract: `docs/validation/prf02-t20/{requirements-traceability,architecture-compliance,api-contract-audit}.md`.
- Security: `docs/validation/prf02-t20/security-readiness.md`, `docs/validation/prf02-t19/security-results.md`.
- Performance: `docs/validation/prf02-t20/performance-readiness.md`.
- Regression/CI: `docs/validation/prf02-t20/regression-summary.md`.
- Engine matrix: `docs/validation/prf02-t20/cross-engine-capabilities.md`.
- Findings: `docs/validation/prf02-t20/remaining-findings.md`.
- T19 evidence set: `docs/validation/prf02-t19/`.

## Known release risks
1. Binary footprint/size increase vs M6 (documented).
2. Startup-time reference not freshly benchmarked with a dedicated harness.
3. `govulncheck`/`npm audit` should be re-run on the frozen RC dependency set.
4. Native runtime on Linux/Windows not smoke-tested locally.
5. MySQL destructive path verified by integration + unit, not by the browser E2E.

## Required release checklist (R01 preflight inputs)
- [ ] Re-run `gofmt`, `go vet`, `go test ./...`, `go test -race ./...`, integration (PG+MySQL), `CGO_ENABLED=0 build`.
- [ ] Re-run frontend `lint`, `typecheck`, `test`, `build`, `build:static`.
- [ ] Re-run `govulncheck ./...` and `npm audit --audit-level=critical` on the frozen tree; record results.
- [ ] Confirm working tree clean at the chosen freeze commit.
- [ ] Confirm `docs/api-contract.md`/OpenAPI match the frozen handlers.
- [ ] Confirm no BLOCKER/HIGH in `remaining-findings.md`.
- [ ] Decide disposition for PRF02-NR-001…005.

## Boundary
Release execution (R01–R10) is **not** started here. Do not tag, version-bump,
publish, upload, or deploy as part of PRF02-T20.
