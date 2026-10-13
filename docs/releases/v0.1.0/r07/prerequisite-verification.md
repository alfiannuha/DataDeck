# DataDeck v0.1.0 — R07 Prerequisite Verification

| # | Prerequisite | Evidence | Status |
|---|---|---|---|
| 1 | PRF02-T20 READY | `docs/validation/prf02-t20/final-readiness-report.md` | VERIFIED |
| 2–7 | R01–R06 READY | `docs/releases/v0.1.0/r0{1..6}/r0*-readiness-report.md` | VERIFIED |
| 8 | R05-approved source | `f92ce49` | VERIFIED |
| 9 | R06 build source == R05 | `provenance.json` source_commit `f92ce49…` | VERIFIED |
| 10 | CI green on approved source | CI 38326084373 / E2E 38326084548 (`f92ce49`); tip `ca4554d` CI 38333216225 success | VERIFIED |
| 11 | R06 artifact dir exists | `release/rc/v0.1.0/{binaries,packages,checksums,sbom,provenance}` | VERIFIED |
| 12 | Manifest exists | `provenance/provenance.json` | VERIFIED |
| 13 | Checksums exist | `checksums/SHA256SUMS`, `SBOM-SHA256SUMS` | VERIFIED |
| 14 | SBOM exists | `sbom/{frontend.cdx,datadeck-go.cdx,datadeck-go.spdx}.json` | VERIFIED |
| 15 | Provenance exists | `provenance/provenance.json` | VERIFIED |
| 16 | R06 findings register | `r06/findings-register.md` | VERIFIED |
| 17 | No unresolved BLOCKER/HIGH | R04–R06 registers | VERIFIED |

Release identity: **DataDeck 0.1.0 / channel `rc` / source `f92ce49`**. No tag exists.
