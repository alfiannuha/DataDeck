# DataDeck v0.1.0 — R06 Packaging Report

Artifact root: `release/rc/v0.1.0/` (gitignored; not committed).

| Package | Size | Contents verified |
|---|---|---|
| `datadeck_0.1.0_darwin_arm64.tar.gz` | 8,776,189 | binary only |
| `datadeck_0.1.0_darwin_amd64.tar.gz` | 9,356,940 | binary only |
| `datadeck_0.1.0_linux_amd64.tar.gz` | 9,221,700 | binary only |
| `datadeck_0.1.0_linux_arm64.tar.gz` | 8,449,992 | binary only |
| `datadeck_0.1.0_windows_amd64.zip` | 9,401,016 | `.exe` only |

- Archive integrity: `tar -tzf` / `unzip -t` → OK for all five.
- Contents contain **only the binary** (no `.env`, no DB, no metadata, no node_modules,
  no git metadata, no docs). No LICENSE file exists in the repository, so none is
  packaged (documented).
- No license/notice packaging policy files were found to include; flagged for R07.
- Naming matches the R03 installation guide
  (`datadeck_<version>_<os>_<arch>[.exe]`).
