# DataDeck v0.1.0 — R07 Documentation Alignment

Verified RC artifacts against R03 docs.

| Item | Doc | Artifact | Aligned? |
|---|---|---|---|
| Version | README/release notes `0.1.0` | CLI/health `0.1.0` | yes |
| Channel | (RC) | CLI/health `rc` | yes |
| Artifact names | installation guide `datadeck_<ver>_<os>_<arch>[.exe]` | 5 names | yes |
| Platform matrix | supported-platforms (mac arm64/amd64, linux amd64/arm64, win amd64) | 5 targets | yes |
| CLI `--version` | installation/quick-start | exact usage verified | yes |
| ENCRYPTION_KEY | configuration/security guide | fail-closed verified | yes |
| Known limitations | limitations.md (XLSX import off; MySQL restore disabled; SQLite delete disabled; base-tables-only export) | matches capability matrix | yes |
| Download URLs | (none) | not fabricated | yes (release notes state not yet published) |

- No premature publication claim in docs; no invented download links/checksums.
- Docker is documented as a packaging option; RC set is binary-only.
