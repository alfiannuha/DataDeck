# DataDeck v0.1.0 — R06 Build Matrix

Source `f92ce49`; `CGO_ENABLED=0 -trimpath`; version `0.1.0`; channel `rc`.

| Target | Artifact | Size | Format / arch (verified) | Build | Runtime smoke |
|---|---|---|---|---|---|
| darwin/arm64 | `datadeck_0.1.0_darwin_arm64` | 24,217,202 | Mach-O 64-bit arm64 | PASS | **PASS (native)** |
| darwin/amd64 | `datadeck_0.1.0_darwin_amd64` | 25,638,640 | Mach-O 64-bit x86_64 | PASS | NOT VERIFIED |
| linux/amd64 | `datadeck_0.1.0_linux_amd64` | 25,133,218 | ELF 64-bit LSB x86-64 static | PASS | NOT VERIFIED |
| linux/arm64 | `datadeck_0.1.0_linux_arm64` | 23,658,658 | ELF 64-bit LSB ARM aarch64 static | PASS | NOT VERIFIED |
| windows/amd64 | `datadeck_0.1.0_windows_amd64.exe` | 25,721,344 | PE32+ console x86-64 | PASS | NOT VERIFIED |

- All five artifacts are built from the **same** approved source revision.
- `file(1)` confirms each artifact's format/architecture matches its target (no
  host-build leakage).
- Native CLI (`darwin/arm64`): `datadeck 0.1.0 (commit f92ce49…, built …,
  channel rc)`, exit 0.
- Runtime smoke on non-mac targets: **NOT VERIFIED** (no native runners) — carried
  to R07.
