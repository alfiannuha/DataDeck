# DataDeck v0.1.0 — R08 Git Tag & Hosted Release Publication

## A. Executive Summary
- **Repository:** `alfiannuha/DataDeck` (public). **Version:** `0.1.0`. **Tag:** `v0.1.0`.
- **Release URL:** https://github.com/alfiannuha/DataDeck/releases/tag/v0.1.0
- **Publication status:** **R08 READY** — the official release is published and post-publication verification passed.

## B. Prerequisites
R01–R07 READY; CI green on the approved source; no unresolved BLOCKER/HIGH at publication. `prerequisite-verification.md`.

## C. Approved Source
- **Tag target:** `6d99856f992bfe1d9425334821eae98125f7e32d` (R05/R06/R07 code `f92ce49` + CI channel fix).
- CI green: CI 38335302035 / E2E 38335301900.

## D. Artifact Set (channel `release`)
5 packages under the release (CI-built `channel=release`): darwin arm64/amd64, linux amd64/arm64, windows amd64. `published-provenance.json` records their SHA-256.

## E. SBOM & Provenance
CycloneDX 1.5 + 1.7 + SPDX 2.3; `published-provenance.json` (unsigned) — all published.

## F. Release Notes
R03 `release-notes.md` (prepared; published as guidance). No fabricated links; not marked published in the doc itself.

## G. Approval
Explicit approval recorded before tagging (`publication-record.md`): "tag + let CI publish".

## H. Git Tag
Annotated `v0.1.0` → `6d99856`; pushed; remote resolves to `6d99856`; not force-pushed.

## I. GitHub Release
`DataDeck v0.1.0`, tag `v0.1.0`, **published** (not draft / not prerelease) at 2026-10-13T02:19:20Z via Release workflow run 38335647512 (success).

## J. Asset Upload / checksums
11 assets (5 packages, SHA256SUMS, 3 SBOMs, THIRD_PARTY_NOTICES.txt, published-provenance.json). Downloaded all and recomputed SHA-256: **9/9 SHA256SUMS entries match**; published binary reports `0.1.0 … channel release`. `publication-record.md`.

## K. Findings
No BLOCKER/HIGH. MEDIUM: R08-F-001 (workflow didn't publish provenance — completed manually), R08-F-002 (SHA256SUMS path prefixes), R04-FIND-001/002, R07-F-001. LOW: R08-F-003 (archive binary named `datadeck` vs docs), R07-F-002 (non-mac native runtime), R07-F-003/R08-F-004, R04-FIND-003, R05-FIND-004/005. `publication-findings.md`.

## L. Release Impact
Eligible for R09 (published-artifact smoke test). Remaining post-publication tests: native runtime on non-mac platforms, published-artifact smoke on supported runners.

## M. Final Decision
**R08 READY.** The official `v0.1.0` release is published at the approved commit with `channel=release` artifacts; tag target and remote assets verified; checksums and SBOM/provenance published; no tag moved, no asset replaced, no checksum regenerated.

## N. Commit / Report
- **Files changed:** `.github/workflows/release.yml`, `docs/releases/v0.1.0/r08/*`.
- **Commands:** `git tag -a`/`push`; `gh run watch`; `gh release view`; `gh release download`; `shasum -a 256 -c`; `gh release upload`; `--version`.
- **Tag target:** `6d99856` → release `v0.1.0`.
- **Release URL:** https://github.com/alfiannuha/DataDeck/releases/tag/v0.1.0
- **Evidence paths:** `docs/releases/v0.1.0/r08/`.
- **Final report commit:** this R08 docs commit.

**Stop — do not begin R09 automatically.**
