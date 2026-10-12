# DataDeck v0.1.0 — R02 Version Inventory

Canonical source of truth: the repository **`VERSION`** file (`0.1.0`), mirrored
by the default in `backend/internal/version/version.go`. Every component is
aligned to it and validated by `scripts/check-version.sh`.

## After R02 (final)
| Location | Value | Notes |
|---|---|---|
| `VERSION` | `0.1.0` | Canonical |
| `backend/internal/version/version.go` `Version` | `0.1.0` | Default; overridable via `-ldflags -X` |
| `backend/internal/version/version.go` `Channel` | `development` default; `release` injected | New in R02 |
| `frontend/package.json` `version` | `0.1.0` | Was `0.0.0` |
| `frontend/package-lock.json` root `version` | `0.1.0` | Updated by `npm version --no-git-tag-version` |
| `frontend/src/lib/build-info.ts` fallback | `0.1.0` | Frontend build-time metadata |
| `docker/Dockerfile` `ARG VERSION` | `0.1.0` | Was `0.1.0-dev`; `ARG CHANNEL=release` added |
| `scripts/build-release.sh` | reads `VERSION`; injects `Version`/`Commit`/`BuildDate`/`Channel=release` | |
| CLI `--version` | `datadeck 0.1.0 (commit …, built …, channel …)` | |
| API `GET /api/v1/health` | `{status, version, channel}` | version/channel added in R02 |
| UI top bar | `v0.1.0` (`data-testid="app-version"`) | Minimal, nonintrusive |

## Before R02
`VERSION=0.1.0-dev`, `version.Version=0.1.0-dev`, `package.json=0.0.0`,
Dockerfile `ARG VERSION=0.1.0-dev`, no channel concept, no API/UI version surface,
no version validator.

No dependency was changed; `package-lock.json` changed only its root version.
