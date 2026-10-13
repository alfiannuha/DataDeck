# DataDeck v0.1.0 — R07 Artifact Verification & Release Integrity

## A. Executive Summary
- **Approved source:** `f92ce49` (CI green); verification tip `ca4554d` (CI green). **RC identity:** DataDeck `0.1.0`, channel `rc`.
- **Decision:** **R07 READY** — all 5 platform artifacts + 5 archives independently verified (checksums, format/arch, archive↔binary identity), native runtime + security PASS, SBOM + provenance validated, zero unresolved BLOCKER/HIGH.

## B. Prerequisites
R01–R06 READY; PRF02-T20 READY; CI green on the built source. `prerequisite-verification.md`.

## C. Artifact Inventory
5 binaries + 5 packages; all sizes/hashes recomputed; 10/10 match `SHA256SUMS` **and** provenance; no missing/duplicate/stale files. `artifact-inventory.md`.

## D. Platform Matrix
darwin arm64 (native PASS), darwin amd64, linux amd64, linux arm64, windows amd64 — all compile + binary-inspection PASS; non-mac runtime NOT VERIFIED.

## E. Integrity Verification
Checksums independent-verified; Mach-O/ELF/PE formats and architectures correct; archives intact, no traversal/absolute paths, exec bits preserved; archive contents byte-identical to standalone binaries. `artifact-inventory.md`.

## F. Metadata
`Version=0.1.0`, `channel=rc`, full commit `f92ce49…` embedded in every binary (CLI + health verified). No mismatch.

## G. Runtime Smoke
macOS arm64: CLI (no key) → `0.1.0/rc`; loopback `127.0.0.1`; health `0.1.0/rc`; UI + JS asset 200; clean shutdown. `runtime-smoke-matrix.md`.

## H. Security
Loopback-only; CSRF/Origin present in the binary (cross-origin → 403; same-origin → 400); ENCRYPTION_KEY fail-closed; no unexpected outbound; artifact secret scan clean; DNS-rebinding residual re-verified → MEDIUM retained. `security-verification.md`.

## I. SBOM
CycloneDX 1.5 (frontend, 514) + CycloneDX 1.7/SPDX 2.3 (Go, 29); key Go deps present; limitations disclosed. `sbom-verification.md`.

## J. Provenance
Source/toolchain/environment/artifact mapping verified; **unsigned** self-reported metadata. `provenance-verification.md`.

## K. Reproducibility
Go build deterministic (two rebuilds identical); **not** byte-identical to the R06 artifact (frontend export differs; no `SOURCE_DATE_EPOCH`) → toolchain-reproducible, bit-for-bit **NOT VERIFIED** (MEDIUM). `reproducibility-assessment.md`.

## L. Docker
**N/A / NOT RUN** — no daemon; binary-only RC set; Dockerfile re-inspected; carried to R08 if in publication scope. `docker-verification.md`.

## M. Documentation
Artifact names, platforms, CLI, ENCRYPTION_KEY, and limitations align with R03 docs; no fabricated download URLs or publication claims. `documentation-alignment.md`.

## N. Findings
No BLOCKER/HIGH. MEDIUM: R04-FIND-001/002, R07-F-001 (reproducibility), PRF02-FIND-001/002. LOW/NOT-RUN: R07-F-002/003, R04-FIND-003, R05-FIND-004/005. `findings-register.md`.

## O. R08 Handoff
Verified release manifest in `release-manifest.md` (10 artifacts, hashes, distribution eligibility, publication conditions). RC set is coherent (single source `f92ce49`).

## P. Final Decision
**R07 READY.** All mandatory conditions pass: 5 targets present, checksums independently verified, no version/channel/arch mismatch, archives safe and identity-matched, embedded frontend verified, native smoke PASS, loopback + CSRF + encryption verified, SBOM + provenance validated, docs aligned, no BLOCKER/HIGH, no tag, no publish.

## Q. Commit / Report
- **Files inspected:** AGENTS.md, PRD, ADR-010, `docs/api-contract.md`, PRF02-T20, R01–R06 artifacts, all files under `release/rc/v0.1.0/`.
- **Files changed:** `docs/releases/v0.1.0/r07/*` (this report + verification docs). No R06 artifact modified.
- **Commands:** `shasum -a 256` (independent recompute), `file`, `tar -tzvf`/`-xzf`, `unzip -l/-t`, native `--version`/startup/health/CSRF/key tests, `strings` secret scan, `python3` JSON SBOM/provenance parse, controlled `go build` rebuild compare.
- **Artifact paths:** `release/rc/v0.1.0/{binaries,packages,checksums,sbom,provenance}`.
- **SHA-256 values:** in `release-manifest.md` + `release/rc/v0.1.0/checksums/SHA256SUMS`.
- **Evidence paths:** `docs/releases/v0.1.0/r07/`.
- **Final report commit:** this R07 docs commit.

**Stop — do not begin R08.**
