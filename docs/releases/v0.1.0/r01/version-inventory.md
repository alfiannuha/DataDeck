# DataDeck v0.1.0 — R01 Version Inventory

R01 does **not** modify any version. R02 owns version finalization.

| Location | Current value | Notes |
|---|---|---|
| `VERSION` (repo root) | `0.1.0-dev` | Release version source |
| `backend/internal/version/version.go` (`Version`) | `0.1.0-dev` | Injected at build via `-ldflags -X` |
| `docker/Dockerfile` `ARG VERSION` | `0.1.0-dev` | Passed to `-X …version.Version=${VERSION}` |
| `docker/Dockerfile` `ARG COMMIT` / `BUILD_DATE` | `unknown` | Set by build scripts |
| `frontend/package.json` `version` | `0.0.0` | Frontend package version (not surfaced in UI) |
| Build scripts (`scripts/build-release.sh`, `build-embedded.sh`) | derive from `VERSION` / ldflags | No hardcoded release version |
| CLI `--version` | `datadeck 0.1.0-dev (commit unknown, built unknown)` | From `version.Version` |
| API `/api/v1/health` | `{"status":"healthy"}` | No version field today |
| UI About/version display | none found | Top-bar only surfaces SW "new version available" |
| `docs/` version references | `0.1.0-dev` / "v0.1.0" | Update in R03 |

## R02 inputs (do not change in R01)
1. `VERSION` → `0.1.0`.
2. `backend/internal/version/version.go` default `Version` → `0.1.0` (or keep dev default + inject at build).
3. Confirm `docker/Dockerfile` default ARG → `0.1.0`.
4. Decide whether `frontend/package.json` version should track the app version (currently `0.0.0`).
5. Optional: expose a version field in `/health` or an About display.
6. Ensure `--version` prints the finalized version when built with the release flags.
