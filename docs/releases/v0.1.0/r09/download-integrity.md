# DataDeck v0.1.0 — R09 Download & Checksum Integrity

Downloaded fresh from the GitHub Release via `gh release download v0.1.0` into a
clean temporary directory (not local build output).

## Result
- 11 assets downloaded successfully (no HTTP error, no HTML error page, non-zero size).
- **SHA-256 independently recomputed and compared to published `SHA256SUMS`:
  9/9 entries match** (the 5 packages + 3 SBOMs + third-party notices).
  - Note: `SHA256SUMS` uses `sbom/…` and `licenses/…` path prefixes while the release
    assets are flat; verification is by basename. Documented finding R09-F-002.
- `published-provenance.json` SHA-256 of every published asset matches the downloaded
  bytes (10/10).

### Package SHA-256 (from published `SHA256SUMS`, verified)
```text
6c243d5d…f47a  datadeck_0.1.0_darwin_amd64.tar.gz
167808a5…f8a  datadeck_0.1.0_darwin_arm64.tar.gz
8666af6b…ea51  datadeck_0.1.0_linux_amd64.tar.gz
0fa6ebd1…8392  datadeck_0.1.0_linux_arm64.tar.gz
a35113b6…0022  datadeck_0.1.0_windows_amd64.zip
```

## Archive safety
Inspected entries before extraction: single binary (`datadeck` / `datadeck.exe`) plus
`VERSION`; **no path traversal, no absolute paths, no symlinks/devices, no duplicate
entries**, executable bit preserved. Extracted into isolated temp dirs only.

## Archive → binary identity
Each extracted binary reports the correct version/channel/commit (see
`platform-runtime-matrix.md`). Archive filenames are versioned; the binary inside is
named `datadeck`/`datadeck.exe` (documented finding R09-F-003).
