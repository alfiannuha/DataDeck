# DataDeck v0.1.0 — R10 Platform & Published-Artifact Integrity

## Platform readiness
| Platform | Build | Binary inspected | Download | SHA-256 | Native runtime |
|---|---|---|---|---|---|
| macOS arm64 | PASS | PASS (Mach-O arm64) | PASS | PASS | **PASS** |
| macOS amd64 | PASS | PASS (Mach-O x86_64) | PASS | PASS | NOT VERIFIED |
| Linux amd64 | PASS | PASS (ELF x86-64) | PASS | PASS | NOT VERIFIED |
| Linux arm64 | PASS | PASS (ELF aarch64) | PASS | PASS | NOT VERIFIED |
| Windows amd64 | PASS | PASS (PE32+) | PASS | PASS | NOT VERIFIED |

Only macOS arm64 was executed natively; others are download+hash+format verified.
Cross-compilation is not claimed as native runtime PASS.

## Published artifact integrity (R06→R09 chain)
| Identity | Value |
|---|---|
| Tag | `v0.1.0` → `6d99856f992bfe1d9425334821eae98125f7e32d` (local = remote) |
| Release | https://github.com/alfiannuha/DataDeck/releases/tag/v0.1.0 |
| Version / channel | `0.1.0` / `release` (backend CLI + health) |
| Assets | 11 (5 packages, SHA256SUMS, 3 SBOM/SPDX, THIRD_PARTY_NOTICES, published-provenance) |
| Checksums | 9/9 `SHA256SUMS` verified on fresh download; provenance hashes 10/10 |
| SBOM | CycloneDX 1.5/1.7 + SPDX 2.3 |
| Provenance | unsigned (disclosed) |
| Published CLI | `datadeck 0.1.0 (commit 6d99856…, channel release)` |

R06 manifest ↔ R07 verified ↔ R08 publication ↔ R09 download: consistent source
`6d99856`, channel `release`, no unexplained mismatch. No published asset was
modified/replaced; the tag was not moved.

## Findings carried into closure (non-blocking)
- R09-F-001: published **frontend** UI channel label `development` (backend `release`).
- R09-F-002: `SHA256SUMS` uses `sbom/`/`licenses/` prefixes vs flat assets.
- R09-F-003: archive binary named `datadeck`/`datadeck.exe` vs docs `datadeck_0.1.0_*`.
