# Supported Platforms

DataDeck v0.1.0 is distributed as a single CGO-free Go binary with the static
frontend embedded. Prebuilt targets:

| OS | Architecture | Artifact |
|---|---|---|
| macOS | arm64 (Apple Silicon) | `datadeck_0.1.0_darwin_arm64` |
| macOS | amd64 (Intel) | `datadeck_0.1.0_darwin_amd64` |
| Linux | amd64 | `datadeck_0.1.0_linux_amd64` |
| Linux | arm64 | `datadeck_0.1.0_linux_arm64` |
| Windows | amd64 | `datadeck_0.1.0_windows_amd64.exe` |

## Verification status

The release pipeline cross-compiles all of the above with `CGO_ENABLED=0`. At the
time of writing, **native runtime validation was performed on macOS arm64 only**;
the other targets are **build-verified (compile-only)** and should be smoke-tested
per platform before you rely on them.

## Runtime requirements

- No Node.js, Go, or external database tools are required at runtime.
- The host must reach the target database over the network (PostgreSQL/MySQL) or
  the filesystem (SQLite).

## Docker

A Dockerfile is provided (`docker/Dockerfile`) for container builds. In a container
the process binds `0.0.0.0:8080` with `ALLOW_REMOTE=1` (documented packaging choice)
and `ENCRYPTION_KEY` must be supplied at runtime — never baked into the image.
Container publishing is not part of v0.1.0's binary release.
