# DataDeck v0.1.0 — R07 SBOM Verification

| File | Format | Name/Version | Count |
|---|---|---|---|
| `sbom/frontend.cdx.json` | CycloneDX **1.5** | frontend / 0.1.0 | 514 components |
| `sbom/datadeck-go.cdx.json` | CycloneDX **1.7** | datadeck_0.1.0_linux_amd64 / sha256:44939859… | 29 components |
| `sbom/datadeck-go.spdx.json` | SPDX **2.3** | datadeck_0.1.0_linux_amd64 | 29 packages |

- All three parse as valid JSON of the declared format; the Go SBOMs reference the
  exact binary by its SHA-256 (traceable to the verified artifact).
- **Dependency consistency:** the Go SBOM contains all shipped direct deps —
  `go-chi/chi`, `jackc/pgx`, `go-sql-driver/mysql`, `modernc.org/sqlite`,
  `xuri/excelize`. The frontend SBOM is generated from the committed lockfile.
- SBOM checksums recorded in `checksums/SBOM-SHA256SUMS`.

## Limitations (disclosed)
- Go SBOM is `syft`-scanned from the compiled binary (module inventory), not a full
  source/license SBOM; license fields are partial.
- Frontend SBOM lists npm packages, not individual bundled chunks.
- No target/CVE/relationship enrichment beyond tool defaults.
