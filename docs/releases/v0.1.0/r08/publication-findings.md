# DataDeck v0.1.0 — R08 Publication Findings

## BLOCKER / HIGH
None. The two pre-publication blockers were resolved before tagging:
- R08-B-001 (RC-channel promotion) → resolved via the release-channel build.
- R08-B-002 (`release.yml` missing `Channel=release`) → fixed in `6d99856`.

## MEDIUM
| ID | Finding | Evidence | Disposition |
|---|---|---|---|
| R08-F-001 | The release workflow does **not** generate/upload a provenance manifest; only SBOMs/checksums/notices were published initially. | `gh release view` asset list before completion (no provenance) | **Completed**: uploaded `published-provenance.json` (accurate published-asset hashes). Recommend `release.yml` emit provenance in a future task. |
| R08-F-002 | `SHA256SUMS` uses `sbom/…` and `licenses/…` path prefixes, but release assets are flat, so those 4 entries can't be verified by `shasum -c` without recreating the directory layout. | Downloaded `SHA256SUMS` vs asset names | Documented; recommend flattening checksum paths (future task). Package checksums (5/5) verify directly. |
| R04-FIND-001 | DNS rebinding residual | R04/R05/R07 | Accepted (MEDIUM) |
| R04-FIND-002 | npm build-time advisories | R04/R05 | Accepted (MEDIUM) |
| R07-F-001 | Bit-for-bit reproducibility not achieved | R07/R08 | Accepted (MEDIUM) |

## LOW
| ID | Finding | Disposition |
|---|---|---|
| R08-F-003 | Published archives contain the binary as `datadeck`/`datadeck.exe` (+ `VERSION`), while R03 docs name `datadeck_0.1.0_<os>_<arch>`. | Documented; recommend aligning docs or the packaging script (future task). |
| R07-F-002 | Non-mac/Windows native runtime NOT VERIFIED (no runners) | Carried to R09 |
| R07-F-003 / R08-F-004 | No repository LICENSE; `THIRD_PARTY_NOTICES.txt` is published instead | Documented |
| R04-FIND-003 | Static HTML lacks CSP/X-Frame | Accepted |
| R05-FIND-004/005 | Failure injection / large-scale perf | NOT RUN |

No public asset was replaced; no checksum regenerated to conceal a mismatch; no tag
moved.
