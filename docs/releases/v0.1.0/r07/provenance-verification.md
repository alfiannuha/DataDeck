# DataDeck v0.1.0 — R07 Provenance Verification

`provenance/provenance.json` (checksum in `checksums/SBOM-SHA256SUMS`).

| Field | Recorded | Matches actual? |
|---|---|---|
| application | DataDeck | yes |
| version | 0.1.0 | yes (binaries + health) |
| channel | rc | yes (CLI + health) |
| source_commit | `f92ce49ee1297a6c53bec8a51a3621862c391b85` | yes (embedded in binaries; == R05) |
| build_environment | macOS 26.6.2 arm64 | yes |
| go_version | go1.26.9 | yes |
| node_version | v24.14.0 | yes |
| build_timestamp | 2026-10-13T00:17:33Z | yes (embedded) |
| artifacts | 5 entries with sha256/size/build/smoke | yes (independently recomputed) |

- Every distributed artifact has a provenance entry, and every entry references a real
  artifact.
- **Trust level:** unsigned, self-reported metadata from the trusted local build
  process (no Sigstore/in-toto attestation). Not described as cryptographically
  trusted. CI identity: CI run 38326084373 (green on the built source `f92ce49`).
