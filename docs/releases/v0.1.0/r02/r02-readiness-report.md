# DataDeck v0.1.0 — R02 Readiness Report

## 1. Files inspected
`AGENTS.md`; R01 artifacts (`docs/releases/v0.1.0/r01/*`); `VERSION`;
`backend/internal/version/version.go`; `backend/cmd/server/main.go`;
`backend/internal/api/health.go`, `router_test.go`, `contract_test.go`;
`docker/Dockerfile`; `scripts/{build-release,build-embedded,check-version}.sh`;
`frontend/package.json`, `package-lock.json`, `src/lib/build-info.ts`,
`src/components/shared/top-bar.tsx`; CI workflows; `docs/api-contract.md`.

## 2. R01 prerequisite evidence
R01 READY (`docs/releases/v0.1.0/r01/r01-readiness-report.md`); freeze record
`release-freeze.md`; baseline `351444f`; freeze commit `d0e6461`.

## 3. Frozen baseline commit
`351444f` (source); R01 documentation commit `d0e6461`.

## 4. Current commit
`d0e6461` + this R02 change set (hash reported in §29 after commit).

## 5. Version source of truth
Repository `VERSION` file (`0.1.0`), mirrored by `internal/version.Version` and
enforced by `scripts/check-version.sh`.

## 6. Version inventory before changes
`VERSION=0.1.0-dev`; `version.Version=0.1.0-dev`; `package.json=0.0.0`;
Dockerfile `ARG VERSION=0.1.0-dev`; no channel; no API/UI version; no validator.

## 7. Version inventory after changes
All `0.1.0` (see `version-inventory.md`); channel concept added; API + UI version
surfaces added; validator added.

## 8. Frontend package/lockfile changes
`package.json` and `package-lock.json` root version → `0.1.0` via
`npm version 0.1.0 --no-git-tag-version`. No dependency versions changed.

## 9. Backend buildinfo changes
`Version` default → `0.1.0`; new `Channel` (default `development`); `String()`
includes channel. `Commit`/`BuildDate` unchanged mechanism.

## 10. CLI version behavior
`datadeck --version` → `datadeck 0.1.0 (commit …, built …, channel …)`. Runs
before server start and before `ENCRYPTION_KEY`/storage access; exits 0.
Release-injected build reports `channel release`.

## 11. API version behavior
`GET /api/v1/health` now returns `{status, version, channel}` (non-sensitive).
OpenAPI + `docs/api-contract.md` updated; contract tests pass.

## 12. UI version behavior
Top bar shows `v0.1.0` (`data-testid="app-version"`, channel/commit in `title`).
Minimal, nonintrusive; no new navigation/settings module.

## 13. Docker metadata
`ARG VERSION=0.1.0`, `ARG CHANNEL=release`; `-X …Channel`; OCI labels added
(title/version/revision/created/source). Builder base aligned to
`golang:1.26.9-alpine`. Not pushed.

## 14. CI/build propagation
Canonical version flows from `VERSION` into backend ldflags, Docker args, artifact
naming (`datadeck_${VERSION}_${os}_${arch}`) and frontend build env. Validator
provided for CI. No CI version constant duplicated.

## 15. Artifact naming convention
`datadeck_<version>_<goos>_<goarch>[.exe]` (unchanged; e.g.
`datadeck_0.1.0_darwin_arm64`). No artifacts created in R02.

## 16. Build channel semantics
development (default) · rc (R06) · release (scripts/Docker). Version stays `0.1.0`.

## 17. Commit/timestamp provenance
Commit from `git rev-parse --short HEAD` (`unknown` outside a checkout);
timestamp RFC3339 UTC, `SOURCE_DATE_EPOCH`-aware. No fabricated values.

## 18. Reproducibility decisions
`-trimpath`; `SOURCE_DATE_EPOCH` honored; `CGO_ENABLED=0`; deterministic frontend
`NEXT_PUBLIC_*` fallbacks.

## 19. Version validation script
`scripts/check-version.sh` — validates VERSION vs version.go vs package.json vs
lockfile vs build-info vs Dockerfile ARG; exits nonzero on mismatch (PASS).

## 20. Security checks
No secret/DSN/key/path in metadata; health returns only version/channel; secret
scan clean (R01). `.env*`/data ignored.

## 21. Static export result
`npm run build:static` PASS; embedded into the binary.

## 22. Single-binary build result
`CGO_ENABLED=0 go build ./cmd/server` PASS; runs, serves loopback health with
version/channel; `--version` works without starting the server.

## 23. Cross-platform checks
Compile PASS with version injection: darwin/amd64, linux/amd64, linux/arm64,
windows/amd64 (no native runtime here).

## 24. Regression results
Backend `go test -count=1 ./...` PASS (15 pkgs); frontend 51 files / 472 tests
PASS; lint/typecheck clean; contract tests PASS. No runtime DB behavior changed.

## 25. Commands and logs
`npm version 0.1.0 --no-git-tag-version`; `swag init` (regen); `npm run generate:api`;
`./scripts/check-version.sh`; `gofmt -l .`; `go vet ./...`; `go test -count=1 ./...`;
`npm run lint/typecheck/test/build:static`; single-binary build + `--version` +
`/health`; cross-compiles. All PASS.

## 26. Findings / limitations
- Docker build not executed (no docker run in R02); base tag
  `golang:1.26.9-alpine` to be confirmed in R06.
- `frontend/package.json` now tracks the app version (was `0.0.0`); noted as a
  deliberate alignment for R02.
- No native runtime validation for cross-compiled targets.

## 27. Release baseline reconciliation
Changed files vs R01 (`d0e6461`): `VERSION`; `backend/internal/version/version.go`;
`backend/internal/api/health.go`, `router_test.go`; `backend/docs/swagger.{json,yaml}`;
`docker/Dockerfile`; `scripts/build-release.sh`; `scripts/check-version.sh` (new);
`frontend/package.json`, `package-lock.json`; `frontend/src/lib/build-info.ts` (new);
`frontend/src/components/shared/top-bar.tsx`, `top-bar.test.tsx`;
`frontend/src/types/generated/openapi.ts`; `docs/api-contract.md`; R02 docs.
**All changes are version/build-metadata/documentation.** No feature, architecture,
API-shape (beyond health metadata), or database-behavior change; no migration.

## 28. Files changed
See §27 (version/build metadata + docs + one frontend test).

## 29. Final commit hash
Recorded in git (this R02 commit).

## 30. **R02 READY**

Justification: canonical version is `0.1.0` across backend, frontend, CLI, API,
Docker and build scripts; validator passes; no version mismatch; no secrets
exposed; static export and single-binary builds pass; `CGO_ENABLED=0` preserved;
required tests pass; baseline change reconciled and limited to metadata; no tag
or publishing performed. R01 conditions (CI on baseline; stray untracked file)
remain tracked and are unaffected by R02.
