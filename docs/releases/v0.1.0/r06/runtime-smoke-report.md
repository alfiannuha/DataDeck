# DataDeck v0.1.0 — R06 Runtime Smoke Report

Native runner: macOS 26.6.2 arm64. Binary: `binaries/datadeck_0.1.0_darwin_arm64`.

| Check | Result |
|---|---|
| `--version` (no ENCRYPTION_KEY, no server) | `datadeck 0.1.0 (commit f92ce49ee1297a6c53bec8a51a3621862c391b85, built 2026-10-13T00:17:33Z, channel rc)`; exit 0 |
| Startup | succeeds; listener `127.0.0.1:8096` (loopback only) |
| ENCRYPTION_KEY handling | required; supplied key accepted |
| `GET /api/v1/health` | `{"status":"healthy","version":"0.1.0","channel":"rc"}` |
| Frontend `/` | `200 text/html` |
| PWA `manifest.json` | `200` |
| Shutdown (SIGTERM) | clean exit |

## Cross-platform runtime
Only macOS arm64 was executed natively. macOS amd64, Linux amd64/arm64, and Windows
amd64 are **compile-verified only** → runtime **NOT VERIFIED** (no native runners).
Carry to R07. Cross-compilation success is **not** claimed as native runtime PASS.
