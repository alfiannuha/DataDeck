# DataDeck v0.1.0 — R06 Release Candidate Readiness

## A. Executive Summary
- **Final source commit:** `f92ce49` (CI + E2E green). **Build channel:** `rc`.
- **Decision:** **R06 READY** — five platform RC binaries built from the approved
  source, packaged, checksummed, with SBOM + provenance; native smoke PASS; no
  unresolved BLOCKER/HIGH. Docker build and non-mac native runtime are documented
  gaps carried to R07.

## B. Prerequisites
R01–R05 READY; PRF02-T20 READY; CI green on `f92ce49` (run 38326084373 / E2E
38326084548). R01-FIND-006 resolved; R01-FIND-007 dispositioned (kept outside
release). R04 CSRF present.

## C. Source Integrity
`f92ce49` on `main`; tracked tree clean; scope freeze maintained (metadata/docs/
security/test-fix only). No stale frontend/embedded assets (built fresh from the
same revision).

## D. Toolchain
Go `go1.26.9` (build), Node `v24.14.0`, npm `11.9.0`, macOS 26.6.2 arm64, syft +
`npm sbom`, `shasum`. **Docker not available.**

## E. Build Matrix
5/5 targets built (`CGO_ENABLED=0`, `-trimpath`, rc metadata): darwin arm64/amd64,
linux amd64/arm64, windows amd64 — formats/arch verified; native CLI PASS; other
runtime NOT VERIFIED.

## F. Packaging
5 archives (4×`.tar.gz`, 1×`.zip`), binary-only, integrity verified.

## G. Build Metadata
Version `0.1.0`, channel `rc`, full commit `f92ce49…`, UTC timestamp, per binary.

## H. SBOM
CycloneDX (frontend 514 components; Go binary 29) + SPDX (Go). Limitations documented.

## I. Provenance
`provenance.json` with toolchain, environment, timestamp, and per-artifact
sha256/size/smoke. Checksummed.

## J. Runtime Smoke
`--version` (0.1.0/rc, no key), startup, loopback listener, health (`0.1.0`/`rc`),
UI 200, manifest 200, clean shutdown — all PASS on macOS arm64.

## K. Docker
**NOT RUN** (no daemon). Dockerfile reviewed; non-root, no baked key, port 8080.
Networking security documented; not a required binary-release deliverable.

## L. Reproducibility
Toolchain-reproducible; **bit-for-bit NOT VERIFIED** (build timestamp embedded;
`SOURCE_DATE_EPOCH` not set; no rebuild compare).

## M. Security
Artifact secret scan clean (no credentials/keys; 4 triaged false positives).
Archive integrity OK; no path traversal. Dependency posture unchanged.

## N. Findings
No BLOCKER/HIGH. Carried MEDIUM (R04-FIND-001/002, PRF02-FIND-001/002) and LOW/
NOT-RUN (R06-NR-001…004, R05-FIND-004/005).

## O. Release Impact
Eligible for R07 (artifact verification). Remaining gaps: native non-mac runtime,
Docker image, bit-for-bit reproducibility, license/notice packaging.

## P. Final Decision
**R06 READY.**

## Q. Commit / Report
- **Files inspected:** AGENTS.md, PRD, ADR-010, `docs/api-contract.md`, R01–R05
  artifacts, `VERSION`, `go.mod`/`go.sum`, `frontend/package.json`/lock, build
  scripts, `docker/Dockerfile`, `.github/workflows/release.yml`.
- **Files changed:** `docs/releases/v0.1.0/r06/*` (13 artifacts). RC build outputs
  under `release/rc/v0.1.0/` (gitignored; not committed).
- **Commands:** `npm ci`; `npm run build:static` (rc env); 5× `go build` with rc
  ldflags; `tar`/`zip`; `shasum`; `syft`; `npm sbom`; native smoke; secret scan.
- **Artifact paths:** `release/rc/v0.1.0/{binaries,packages,checksums,sbom,provenance}`.
- **Checksums:** `release/rc/v0.1.0/checksums/SHA256SUMS` (10/10 verified).
- **Evidence paths:** `docs/releases/v0.1.0/r06/`.
- **Final report commit:** this R06 docs commit (source built = `f92ce49`).

**Stop — do not begin R07.**
