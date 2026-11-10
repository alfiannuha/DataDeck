# DataDeck v0.1.0 — R08 Publication Record

## Approval
Explicit publication approval was granted by the release owner (interactive gate):
"tag + let CI publish" — create `v0.1.0` at `6d99856` and publish the verified
`channel=release` artifact set to `alfiannuha/DataDeck`.

## Git tag
| Item | Value |
|---|---|
| Tag | `v0.1.0` (annotated; tag object `6133c66`) |
| Target commit | `6d99856f992bfe1d9425334821eae98125f7e32d` |
| Local resolves to | `6d99856` |
| Remote `refs/tags/v0.1.0^{}` | `6d99856` |
| Force-push | none |

## GitHub Release
| Item | Value |
|---|---|
| URL | https://github.com/alfiannuha/DataDeck/releases/tag/v0.1.0 |
| Title | DataDeck v0.1.0 |
| Draft / Prerelease | false / false |
| Published at | 2026-10-13T02:19:20Z |
| Workflow run | Release `38335647512` (success — 5 builds + SBOM/packaging + publish) |

## Assets (11)
Packages: `datadeck_0.1.0_{darwin_arm64,darwin_amd64,linux_amd64,linux_arm64}.tar.gz`,
`datadeck_0.1.0_windows_amd64.zip`.
Supply chain: `SHA256SUMS`, `datadeck-go.cdx.json`, `datadeck-go.spdx.json`,
`frontend.cdx.json`, `THIRD_PARTY_NOTICES.txt`, `published-provenance.json`.

## Remote checksum verification
Downloaded all assets and recomputed SHA-256: **9/9 entries in `SHA256SUMS` match**
(the manifest uses `sbom/`/`licenses/` path prefixes; assets are flat — verified by
basename). `published-provenance.json` records the SHA-256 of every published asset.

## Published binary verification
```
$ tar -xzf datadeck_0.1.0_darwin_arm64.tar.gz && ./datadeck --version
datadeck 0.1.0 (commit 6d99856f992bfe1d9425334821eae98125f7e32d, built 2026-10-13T02:15:03Z, channel release)
```
Archive also contains `VERSION` (`0.1.0`).

## Post-publication completion
The release workflow published SBOMs/checksums/notices but **not provenance**
(R08-F-001). An accurate `published-provenance.json` (hashes of the actual published
assets, source `6d99856`, release URL, run id, unsigned) was generated and uploaded,
completing the supply-chain set.
