# DataDeck v0.1.0 — R06 SBOM Report

| Artifact | Format | Tool | Components |
|---|---|---|---|
| `sbom/frontend.cdx.json` | CycloneDX JSON | `npm sbom` | 514 |
| `sbom/datadeck-go.cdx.json` | CycloneDX JSON | `syft` | 29 |
| `sbom/datadeck-go.spdx.json` | SPDX JSON | `syft` | (same scan) |

- **Frontend**: full npm dependency inventory from the committed lockfile.
- **Go**: module/package inventory scanned from the built Linux binary
  (`syft scan file:…`) — includes the main module and dependencies.
- Checksums for SBOMs are in `checksums/SBOM-SHA256SUMS`.

## Limitations (documented, not claimed complete)
- `syft` scanned the compiled binary, not a full source SBOM with licenses.
- Frontend SBOM covers npm packages (not the bundled/transpiled runtime chunks
  individually).
- No CVE/target/relationship enrichment beyond what the tools provide.
