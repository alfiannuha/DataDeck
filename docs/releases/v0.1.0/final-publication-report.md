# DataDeck v0.1.0 — Final Publication Report

**Status: PUBLISHED AND VERIFIED**

## Final source & tag
| Field | Value |
|---|---|
| Final source commit | `6d99856f992bfe1d9425334821eae98125f7e32d` |
| Workflow fix commit | `6d99856` — injects `-X …version.Channel=release` and full `${GITHUB_SHA}` in `release.yml` |
| Git tag | `v0.1.0` (annotated) → `6d99856` (local = remote) |
| Version | `0.1.0` |
| Channel | `release` (backend CLI + `/health`; verified) |

## CI result
- CI run 38335302035 and E2E run 38335301900 on `6d99856` → **success**.
- Tag-triggered Release workflow run **38335647512** → **success** (5 builds + SBOM/packaging/checksums + Publish GitHub release).

## Artifact inventory (published)
| Artifact | Platform/arch |
|---|---|
| `datadeck_0.1.0_darwin_arm64.tar.gz` | macOS arm64 |
| `datadeck_0.1.0_darwin_amd64.tar.gz` | macOS amd64 |
| `datadeck_0.1.0_linux_amd64.tar.gz` | Linux amd64 |
| `datadeck_0.1.0_linux_arm64.tar.gz` | Linux arm64 |
| `datadeck_0.1.0_windows_amd64.zip` | Windows amd64 |
| `SHA256SUMS` | integrity |
| `datadeck-go.cdx.json`, `datadeck-go.spdx.json`, `frontend.cdx.json` | SBOM/SPDX |
| `THIRD_PARTY_NOTICES.txt` | licenses |
| `published-provenance.json` | provenance (unsigned, disclosed) |

Archives contain the binary (`datadeck`/`datadeck.exe`) + `VERSION` (safe entries; no
traversal/absolute paths/symlinks).

## Verification results
- **Version/channel**: `datadeck --version` (fresh download) → `datadeck 0.1.0 (commit 6d99856…, channel release)`. `/health` → `0.1.0`/`release`.
- **Architectures**: verified as Mach-O arm64/x86_64, ELF x86-64/aarch64, PE32+ x86-64.
- **SHA-256 (published)**: fresh download → **9/9 `SHA256SUMS` entries match**; provenance asset hashes → **0 mismatches (10/10)**.
- **SBOM**: CycloneDX 1.5/1.7 + SPDX 2.3 parse; key Go deps present.
- **Provenance**: channel `release`, source `6d99856`, unsigned.
- **Archive safety**: PASS.
- **Native smoke (macOS arm64)**: CLI, startup, loopback, `/health`, UI, clean shutdown — PASS. Other platforms: build/format/checksum verified; native runtime **NOT VERIFIED** (no runners) — carried to maintenance.
- **BLOCKER/HIGH**: none.

## Publication
| Field | Value |
|---|---|
| GitHub Release | https://github.com/alfiannuha/DataDeck/releases/tag/v0.1.0 |
| Title | DataDeck v0.1.0 |
| Draft / Prerelease | false / false (stable) |
| Published at | 2026-10-13T02:19:20Z |
| Release notes | Updated to the proper v0.1.0 notes (not auto-generated) |
| Published asset verification | 11 assets downloaded; 9/9 checksums; provenance 10/10 |

## Remaining findings (no BLOCKER/HIGH)
- R09-F-001 (LOW/MED): published frontend UI channel label `development` (backend `release`) — metadata only; fix `release.yml` `build:static` env (`NEXT_PUBLIC_DATADECK_CHANNEL/_COMMIT`) in a patch.
- R09-F-002 (LOW): `SHA256SUMS` uses `sbom/`/`licenses/` prefixes vs flat assets (verified by basename).
- R09-F-003 (LOW): archive binary named `datadeck`/`datadeck.exe` vs docs `datadeck_0.1.0_*`.
- R04-FIND-001/002, R07-F-001 (MEDIUM, accepted); non-mac native runtime NOT VERIFIED.

## Notes on this execution
The release had already been published in R08 from the same approved source `6d99856`
(the release workflow ran on the `v0.1.0` tag **after** the `Channel=release` fix).
Per the tag/release collision policy, this execution **did not create a duplicate tag or
release**; it independently re-verified the published artifacts and replaced the
auto-generated release body with the proper v0.1.0 notes. No artifact was rebuilt,
re-uploaded, or replaced; the tag was not moved.

## Final status
**PUBLISHED AND VERIFIED.**
