# DataDeck v0.1.0 — R09 Supply Chain & Documentation Verification

## SBOM (downloaded from the release)
| Asset | Format | Version | Contents |
|---|---|---|---|
| `frontend.cdx.json` | CycloneDX 1.5 | 0.1.0 | 515 components |
| `datadeck-go.cdx.json` | CycloneDX 1.7 | datadeck_0.1.0_linux_amd64 | 29 components |
| `datadeck-go.spdx.json` | SPDX 2.3 | datadeck_0.1.0_linux_amd64 | 29 packages |
All parse; checksums match `SHA256SUMS`.

## Provenance (`published-provenance.json`)
- version `0.1.0`, channel `release`, source `6d99856…`, 10 asset entries.
- Every recorded SHA-256 matches the downloaded asset bytes (**10/10**).
- **Unsigned** self-reported metadata (no Sigstore/in-toto) — not claimed as
  cryptographically authenticated.

## Artifact consistency
R07 manifest ↔ R08 publication ↔ actual release assets ↔ downloaded bytes ↔ published
checksums: consistent (source `6d99856`, channel `release`). Package SHA-256 verified.

## Documentation vs published artifacts
| Item | Doc | Published artifact | Aligned? |
|---|---|---|---|
| Version `0.1.0` | README/notes | CLI + `/health` | yes |
| Channel | release (contract) | backend `release`; **UI title `development`** | **NO** → R09-F-001 |
| Package names | `datadeck_0.1.0_<os>_<arch>` | archives match; **binary inside is `datadeck`** | partial → R09-F-003 |
| Platform matrix | 5 targets | 5 packages | yes |
| Checksums | `SHA256SUMS` | present | yes (path-prefix quirk R09-F-002) |
| ENCRYPTION_KEY | fail-closed | verified | yes |
| Limits (XLSX import off, MySQL restore off, SQLite delete off) | capabilities | matches | yes |

## Findings summary
- **R09-F-001** (LOW/MEDIUM): published frontend build-info channel/commit stale
  (`development`/`unknown`) vs backend `release`. Fix `release.yml` to pass
  `NEXT_PUBLIC_DATADECK_VERSION/_CHANNEL/_COMMIT` to `build:static`.
- **R09-F-002** (LOW): `SHA256SUMS` path prefixes vs flat assets (carried R08-F-002).
- **R09-F-003** (LOW): archive binary named `datadeck` vs docs `datadeck_0.1.0_*`
  (carried R08-F-003).
