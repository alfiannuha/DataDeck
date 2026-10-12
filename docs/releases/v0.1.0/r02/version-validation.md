# DataDeck v0.1.0 — R02 Version Validation & Tests

## Validator
`scripts/check-version.sh` — parses structured metadata (JSON via `python3` for
the packages; anchored `sed` for the Go var, build-info fallback and Docker ARG).
Run result at HEAD:
```text
canonical version (VERSION): 0.1.0
ok:   backend/internal/version.Version = 0.1.0
ok:   frontend/package.json version = 0.1.0
ok:   frontend/package-lock.json version = 0.1.0
ok:   frontend/src/lib/build-info.ts fallback = 0.1.0
ok:   docker/Dockerfile ARG VERSION = 0.1.0
VERSION CONSISTENCY: OK (0.1.0)
```

## Backend tests
- `go test -count=1 ./...` → all 15 packages PASS (includes `internal/version`,
  `internal/api` router/health/contract).
- `router_test.go` `TestHealth` asserts `data.status=="healthy"`, non-empty
  `data.version`, and `data.channel=="development"` for an uninjected build.
- `version_test.go` asserts `String()` contains the version/commit/built markers.
- `go vet ./...`, `gofmt -l .` → clean.

## Frontend tests
- `npm run test` → **51 files / 472 tests PASS** (added 1: top bar renders the
  canonical version `v0.1.0` with channel in the title).
- `npm run lint` → 0 problems; `npm run typecheck` → clean.

## Integration / CLI / API smoke
| Check | Command | Result |
|---|---|---|
| CLI version (no server, no ENCRYPTION_KEY) | `/tmp/datadeck-r02 --version` | `datadeck 0.1.0 (commit unknown, built unknown, channel development)` |
| Release injection | host build with `-X …Channel=release` | `datadeck 0.1.0 (commit d0e6461, built …, channel release)` |
| API version | `curl /api/v1/health` | `{"status":"healthy","version":"0.1.0","channel":"development"}` |
| Static export | `npm run build:static` (with `NEXT_PUBLIC_DATADECK_*` for rc/release) | PASS |
| Single binary | `CGO_ENABLED=0 go build` + run | PASS; health served on loopback |
| Cross-compile w/ injection | darwin/amd64, linux/amd64, linux/arm64, windows/amd64 | compile PASS (no native runtime here) |

## Safety
- `--version` exits before starting the server and before any storage/key access
  (handled first in `cmd/server/main.go`).
- Metadata contains no secrets; version endpoint returns only version/channel.
