# DataDeck v0.1.0 — R06 Toolchain Report

| Tool | Version | Notes |
|---|---|---|
| Go (binary) | `go1.26.5 darwin/arm64` | local launcher |
| Go (build toolchain) | **`go1.26.9`** (`go env GOVERSION`) | matches `go.mod` `toolchain go1.26.9` and CI pin |
| Node.js | `v24.14.0` | matches CI |
| npm | `11.9.0` | lockfile-driven (`npm ci`) |
| Git | 2.55.0 | |
| OS / arch | macOS 26.6.2 (arm64) | native build host |
| Docker | **not available** | Docker build NOT RUN (see docker-build-report.md) |
| SBOM | `syft` (installed ad hoc) + `npm sbom` | CycloneDX/SPDX |
| Checksums | `shasum -a 256` | macOS |

Build flags: `CGO_ENABLED=0`, `-trimpath`, `-ldflags "-s -w -X …Version=0.1.0
-X …Commit=<full sha> -X …BuildDate=<UTC> -X …Channel=rc"`.
