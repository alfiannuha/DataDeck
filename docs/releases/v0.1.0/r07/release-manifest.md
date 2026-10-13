# DataDeck v0.1.0 — R07 Verified Release Manifest (for R08)

Application **DataDeck** · version **0.1.0** · channel **rc** · source
**`f92ce49ee1297a6c53bec8a51a3621862c391b85`** (short `f92ce49`).

All SHA-256 values below were **independently recomputed** from artifact bytes and
match `checksums/SHA256SUMS` and `provenance.json`.

| Artifact | Platform | Arch | Size | SHA-256 | Dist | Runtime verified |
|---|---|---|---|---|---|---|
| `datadeck_0.1.0_darwin_arm64` | darwin | arm64 | 24,217,202 | `dfe8411d8ce7247f7e10a437a0dca0a6dd8c31c6f80763dae792be8ef3cf3ecc` | yes | **PASS (native)** |
| `datadeck_0.1.0_darwin_amd64` | darwin | amd64 | 25,638,640 | `d0a5a38710687415974fc44306482c0d835db898ef79ab030cc030320e8ccf93` | yes | NOT VERIFIED |
| `datadeck_0.1.0_linux_amd64` | linux | amd64 | 25,133,218 | `44939859bdbe4e52a4a11591d7cec965a7f5c5b43540bee18bb8be2c8e11d374` | yes | NOT VERIFIED |
| `datadeck_0.1.0_linux_arm64` | linux | arm64 | 23,658,658 | `62513343992341fbb553eda16fb88bc0008b25281bb72677f4ea34a8dc51722a` | yes | NOT VERIFIED |
| `datadeck_0.1.0_windows_amd64.exe` | windows | amd64 | 25,721,344 | `5e2f1cfc238958482b795692820c2c8bc62ddd4d9bb3bff704780554140f91b9` | yes | NOT VERIFIED |
| `datadeck_0.1.0_darwin_arm64.tar.gz` | darwin | arm64 | 8,776,189 | `830276200f5ebe1c427a0ae0e89f1a8bc754f1352c31505845e6732c6885d868` | yes | — |
| `datadeck_0.1.0_darwin_amd64.tar.gz` | darwin | amd64 | 9,356,940 | `6b6cdda16456f37679cfb84dfec27021ca302f0917d500112d55b3d925e14abf` | yes | — |
| `datadeck_0.1.0_linux_amd64.tar.gz` | linux | amd64 | 9,221,700 | `b55713f861eded85d719fcfb1123c728163bb6d056e3a2db9b5d8a5fd91219af` | yes | — |
| `datadeck_0.1.0_linux_arm64.tar.gz` | linux | arm64 | 8,449,992 | `4f0a85723851750a7a6020cba1cb8df005cb7c5230473fa505bd0616c369bba2` | yes | — |
| `datadeck_0.1.0_windows_amd64.zip` | windows | amd64 | 9,401,016 | `c8e73bfc480bb74dbd63d07d3cde6da00f914582dc4e9e3d40eab0841b4dd9a2` | yes | — |

- SBOM: `sbom/frontend.cdx.json` (CycloneDX 1.5), `sbom/datadeck-go.cdx.json`
  (CycloneDX 1.7), `sbom/datadeck-go.spdx.json` (SPDX 2.3).
- Provenance: `provenance/provenance.json` (unsigned).
- Checksums: `checksums/SHA256SUMS`, `checksums/SBOM-SHA256SUMS`.

## Required publication conditions for R08
1. Tag `v0.1.0` at `f92ce49` (or the merged release commit) — not before R08.
2. Attach the 5 archives + binaries, `SHA256SUMS`, SBOMs, and provenance.
3. Do not fabricate download URLs; use the GitHub Release URLs.
4. Decide native runtime acceptance for non-mac targets (or document as accepted risk).
5. Decide whether to build/publish a Docker image.
