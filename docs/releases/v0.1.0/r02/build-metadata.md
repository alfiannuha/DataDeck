# DataDeck v0.1.0 — R02 Build Metadata

## Injection points
| Artifact | Mechanism | Metadata |
|---|---|---|
| Backend binary (dev) | `go build ./cmd/server` | defaults: `0.1.0`, `unknown`, `unknown`, `development` |
| Backend binary (release) | `scripts/build-release.sh` `-ldflags -X` | `Version`, `Commit` (`git --short`), `BuildDate` (RFC3339 UTC / `SOURCE_DATE_EPOCH`), `Channel=release` |
| Docker image | `docker/Dockerfile` ARG `VERSION`/`COMMIT`/`BUILD_DATE`/`CHANNEL` + `-ldflags` | same, plus OCI labels |
| Frontend (static) | `NEXT_PUBLIC_DATADECK_VERSION` / `_CHANNEL` / `_COMMIT` inlined at build | fallbacks `0.1.0` / `development` / `unknown` |

## Docker / OCI labels (final stage)
```text
org.opencontainers.image.title     = "DataDeck"
org.opencontainers.image.version   = ${VERSION}     (default 0.1.0)
org.opencontainers.image.revision  = ${COMMIT}      (default unknown)
org.opencontainers.image.created   = ${BUILD_DATE}  (default unknown)
org.opencontainers.image.source    = https://github.com/alfiannuha/DataDeck
```
No fake commit/timestamp is hardcoded. No image is pushed in R02.

## Frontend/backend consistency (static export)
- The embedded frontend is built from the same source revision as the backend.
- `frontend/src/lib/build-info.ts` `BUILD_INFO.version` must equal the backend
  `version.Version`; `scripts/check-version.sh` enforces the package/lock/build-info
  alignment and is intended to run in CI before build/release.
- `NEXT_PUBLIC_*` values are inlined by Next.js at build time (no runtime server
  env needed for the static export).

## Reproducibility
- `-trimpath` used for release builds.
- `SOURCE_DATE_EPOCH` normalizes `BuildDate` when provided (see `build-release.sh`).
- No arbitrary local clock value is embedded silently when `SOURCE_DATE_EPOCH` is set.
- `CGO_ENABLED=0` preserved; no external runtime introduced.

## Docker base alignment
`docker/Dockerfile` builder base raised `golang:1.26.6-alpine` → `golang:1.26.9-alpine`
to match `go.mod` `toolchain go1.26.9` and the CI pin, avoiding an in-image
toolchain download. (Docker build not executed in R02; validated in R06/R07.)
