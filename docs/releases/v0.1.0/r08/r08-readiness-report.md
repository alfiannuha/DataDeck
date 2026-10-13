# DataDeck v0.1.0 — R08 Git Tag & Hosted Release Publication

## A. Executive Summary
- **Repository:** `alfiannuha/DataDeck` (public). **Version:** `0.1.0`. **Tag:** `v0.1.0` (NOT created).
- **Publication status:** **NOT READY** — two channel-policy blockers prevent publishing the current artifact set.

## B. Prerequisites
R01–R07 READY; CI green on `f92ce49`; no unresolved BLOCKER/HIGH at entry. `prerequisite-verification.md`.

## C. Approved Source
`f92ce49ee1297a6c53bec8a51a3621862c391b85` (CI 38326084373 / E2E 38326084548). Tag target not yet finalized (channel blocker).

## D. Artifact Set (current, R07-verified)
5 binaries + 5 packages, all checksums independently verified (10/10). **But they
report `channel: rc`** — see §G.

## E. SBOM & Provenance
CycloneDX 1.5/1.7 + SPDX 2.3; `provenance.json` (unsigned). Valid; tied to `f92ce49`.

## F. Release Notes
R03 `release-notes.md` prepared; will be reconciled against the final release-channel
capabilities before publication.

## G. Approval
**Not requested** — blocked upstream by the channel policy; approval would be
premature because the artifact set cannot be published unchanged.

## H. Git Tag
Not created (neither local nor remote).

## I. GitHub Release
Not created; none exists.

## J. Asset Upload
None.

## K. Findings
**BLOCKER (2):**
- **R08-B-001** RC-channel artifacts cannot be promoted to the official `release`.
- **R08-B-002** `release.yml` omits `Channel=release`, so a tag-triggered build would
  report `channel: development`.
MEDIUM/LOW carried from R04–R07. `publication-findings.md`.

## L. Release Impact
Publication is blocked. Required sequence:
1. Fix `release.yml` to inject `Channel=release` (in-scope release-mechanics fix).
2. Produce official **`channel=release`** artifacts (R06-rerun / Option B),
   packaging + checksums + SBOM + provenance.
3. Independently re-verify (R07-style).
4. Finalize the tag target, then return to R08 for the approval gate.

## M. Final Decision
**R08 NOT READY.**

Reason: the R07-verified artifacts are `channel: rc`, and the release CI build omits
`channel=release`. Per R08 §11/§12, RC binaries cannot be promoted to the official
release, and a tag-triggered build would ship the wrong channel metadata. Therefore
publication is stopped; no tag or release was created.

## N. Commit / Report
- **Files inspected:** AGENTS.md, PRD, ADR-010, R01–R07 artifacts, `VERSION`, `release/rc/v0.1.0/*`, `.github/workflows/release.yml`, git/gh repo+tag+release state.
- **Files changed:** `docs/releases/v0.1.0/r08/*` (prepublication docs).
- **Commands:** `git remote/tag/ls-remote`, `gh repo view/release list`, `--version` (channel), `release.yml` inspection.
- **Tag target:** not finalized (channel blocker).
- **Release URL:** none.
- **Asset hashes:** none published.
- **Evidence paths:** `docs/releases/v0.1.0/r08/`.
- **Final report commit:** this R08 docs commit.

**Stopped at the publication gate — no tag, no release, no upload.**
