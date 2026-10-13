# DataDeck v0.1.0 — R08 Git Tag & Hosted Release Publication

## A. Executive Summary
- **Repository:** `alfiannuha/DataDeck` (public). **Version:** `0.1.0`. **Tag:** `v0.1.0` (NOT created).
- **Publication status:** **R08 AWAITING APPROVAL.** Prepublication checks pass on a
  corrected **`channel=release`** artifact set; explicit publication authorization is
  required before any tag/release.

## B. Prerequisites
R01–R07 READY; CI green on `f92ce49`; the release-workflow channel fix was added in
`6d99856`. `prerequisite-verification.md`.

## C. Approved Source & Tag Target
```text
source / tag target : 6d99856f992bfe1d9425334821eae98125f7e32d
branch              : main (pushed; CI re-runs on push)
```
`6d99856` = R05/R06/R07 code (`f92ce49`) + the CI channel fix (no application-code
change). Tag target must be finalized and approved (see §G/§M).

## D. Artifact Set (release channel)
5 binaries + 5 packages under `release/v0.1.0/`, reporting `channel=release`, from
`6d99856`. 10/10 checksums verified. `release-channel-build.md`.

## E. SBOM & Provenance
CycloneDX 1.5 + CycloneDX 1.7 + SPDX 2.3; `provenance.json` (channel `release`,
source `6d99856`, 5 artifacts, unsigned).

## F. Release Notes
R03 `release-notes.md` prepared; to be published as the release body after approval.

## G. Approval
**AWAITING explicit publication approval.** See §M and the approval request.

## H. Git Tag / I. GitHub Release / J. Asset Upload
Not created / not created / none — pending approval.

## K. Findings
BLOCKER R08-B-001/B-002 now **RESOLVED** (release-channel set + workflow fix).
Carried MEDIUM/LOW: R04-FIND-001/002, R07-F-001/002/003, PRF02-FIND-001/002,
R04-FIND-003, R05-FIND-004/005.

## L. Release Impact
On approval, publication will create tag `v0.1.0` at `6d99856` and trigger the
release workflow (which rebuilds `channel=release` artifacts in CI and publishes via
`gh release create`). R09 then verifies the published assets.

## M. Publication Approval Package
```text
Repository        : alfiannuha/DataDeck (public)
Tag               : v0.1.0
Title             : DataDeck v0.1.0
Target commit     : 6d99856f992bfe1d9425334821eae98125f7e32d
Version / channel : 0.1.0 / release
Artifacts         : 5 binaries + 5 packages (see release-channel-build.md)
Checksums         : release/v0.1.0/checksums/SHA256SUMS (10/10 verified)
SBOM / provenance : release/v0.1.0/{sbom,provenance}
CI evidence       : CI 38326084373 / E2E 38326084548 (code f92ce49); push of 6d99856 triggered fresh CI
Open risks        : R04-FIND-001 (DNS rebinding residual, MEDIUM), R04-FIND-002
                    (build-time npm advisories, MEDIUM), R07-F-001 (reproducibility,
                    MEDIUM); non-mac native runtime NOT VERIFIED
Side effects      : tag push triggers release.yml (build + gh release create); no
                    Docker push; no package-registry publish
Method            : push tag v0.1.0 at 6d99856 (workflow publishes), or pre-create
                    the GitHub Release with the verified assets
```

## N. Final Decision
**R08 AWAITING APPROVAL.** Blockers resolved; prepublication checks pass; no tag or
release created. Explicit approval required to publish.

## O. Commit / Report
- **Files changed:** `.github/workflows/release.yml`, `docs/releases/v0.1.0/r08/*`.
- **Commands:** workflow edit; release-channel build; packaging; `shasum`; `npm sbom`/`syft`; native smoke; `gh` repo/tag/release checks.
- **Tag target:** `6d99856` (pending approval).
- **Release URL:** none. **Asset hashes:** `release/v0.1.0/checksums/SHA256SUMS`.
- **Evidence paths:** `docs/releases/v0.1.0/r08/`.
- **Final report commit:** this R08 docs commit.

**Stopped at the approval gate — awaiting explicit publication authorization.**
