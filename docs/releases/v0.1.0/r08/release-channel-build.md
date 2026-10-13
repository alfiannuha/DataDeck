# DataDeck v0.1.0 — R08 Release-Channel Build (unblocks R08-B-001/B-002)

Produced the official **`channel=release`** artifact set from the release source
`6d99856f992bfe1d9425334821eae98125f7e32d` (adds the `release.yml` channel fix).
Committed in `6d99856`.

## Build
- Frontend static export with `NEXT_PUBLIC_DATADECK_CHANNEL=release`,
  `_VERSION=0.1.0`, `_COMMIT=6d99856…`; embedded into the CGO-free Go binary.
- `CGO_ENABLED=0 -trimpath -ldflags "-s -w -X …Version=0.1.0 -X …Commit=6d99856…
  -X …BuildDate=2026-10-13T02:06:39Z -X …Channel=release"`.
- Artifacts under `release/v0.1.0/` (gitignored; not committed).

## Verified channel
```text
$ datadeck_0.1.0_darwin_arm64 --version
datadeck 0.1.0 (commit 6d99856f992bfe1d9425334821eae98125f7e32d, built 2026-10-13T02:06:39Z, channel release)
```
Health (`/api/v1/health`) → `{"status":"healthy","version":"0.1.0","channel":"release"}`.

## Checksums (`release/v0.1.0/checksums/SHA256SUMS`, 10/10 verified)
```text
083d57476ad73a176d1ac064acde4bb957d6c9ca9863a1d2cd76ce4ce567bab5  binaries/datadeck_0.1.0_darwin_amd64
72a2932b27909c2827181e440a5f361d4a2371162f9c101dd268094a8259c9df  binaries/datadeck_0.1.0_darwin_arm64
3ae1c45cfcf074f984a1fb96000d1a722375e460082ae08b36b8f8991b03e692  binaries/datadeck_0.1.0_linux_amd64
6123239c204380a870d55dfeec58110a907650fddfc4ecfd415c4bf3b8c6f150  binaries/datadeck_0.1.0_linux_arm64
344389d7f3f2b9a9a18079bd2151ffb6165c07236328cc50aca20f0c6943c542  binaries/datadeck_0.1.0_windows_amd64.exe
c9ffae2d0923f1d84761b19d860a4ff75f87b96e52628c98b81f9c6f64ca3daf  packages/datadeck_0.1.0_darwin_amd64.tar.gz
581df0848ff416313298d1771ad98c153f34510dee1c25481749310c1972f2c9  packages/datadeck_0.1.0_darwin_arm64.tar.gz
c69474762d3d2962172d261a005b5e4de106a021817535db492b52d3eaad53bc  packages/datadeck_0.1.0_linux_amd64.tar.gz
566814515e90bb6c4af43edf7cc1809146c62be219a0e45f0dd8033311e0e68a  packages/datadeck_0.1.0_linux_arm64.tar.gz
ab294ac58391255af6f24fc9fb3dacf4ba3f4cf5dc3cd6e91d779a7381d39962  packages/datadeck_0.1.0_windows_amd64.zip
```

## Supply chain
- SBOM: `sbom/frontend.cdx.json` (CycloneDX), `sbom/datadeck-go.cdx.json`
  (CycloneDX), `sbom/datadeck-go.spdx.json` (SPDX).
- Provenance: `provenance/provenance.json` (channel `release`, source `6d99856…`,
  5 artifacts, unsigned).
- Checksums for SBOM/provenance in `checksums/SBOM-SHA256SUMS`.

## Verification (native macOS arm64, SHA `72a2932b…`)
CLI `channel release` (no key, exit 0); loopback `127.0.0.1`; health `0.1.0/release`;
UI 200; cross-origin POST 403 (CSRF); clean shutdown.

## R08-B-001 / R08-B-002
- B-001 **RESOLVED**: official artifacts now report `channel: release`.
- B-002 **RESOLVED**: `release.yml` now injects `Channel=release` and full `${GITHUB_SHA}`.

## Note on artifact provenance vs the CI publisher
`release.yml` triggers on `v*` tags and publishes via `gh release create`. On
approval, the tag push will run that workflow, which rebuilds the artifacts in CI
(with `channel=release`) and publishes them. The local `release/v0.1.0/` set above
is the verified reference recipe (same flags/source); the published bytes will be
the CI build for the same commit and should be re-verified in R09.
