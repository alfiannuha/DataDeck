# DataDeck v0.1.0 — R01 CI Preflight

## Workflows (inspected)
| Workflow | Triggers | Jobs (summary) |
|---|---|---|
| `.github/workflows/test.yml` | push/PR → `main` | backend (gofmt, vet, test, race, govulncheck, CGO-free build); frontend (npm ci, lint, typecheck, test, build, build:static, audit critical); integration (PG17 + MySQL8, `-p 1`) |
| `.github/workflows/e2e.yml` | push/PR → `main` | build embedded binary, start server, `npx playwright test` |
| `.github/workflows/release.yml` | **push tags `v*`** + workflow_dispatch | release build matrix, SBOM, packages, checksums, `gh release create` (publishing on tag) |

## CI status for the baseline
- `origin/main` = `624defc` (PRF02-T09). Baseline `351444f` is **11 commits ahead
  and unpushed**.
- `gh run list` shows the most recent CI runs are for **PRF02-T09** (2026-10-05);
  **no run targets `351444f` or any T10–T20 commit**.
- **CI status for the candidate baseline: NOT VERIFIED.**

## Equivalent local evidence (executed at HEAD)
| CI job | Local equivalent | Result |
|---|---|---|
| Backend format/vet | `gofmt -l .`, `go vet ./...` | PASS |
| Backend unit + race | `go test -count=1 ./...`, `go test -race -count=1 ./...` | PASS |
| Backend integration | `go test -tags=integration -p 1 ./...` (PG+MySQL) | PASS |
| CGO-free build | `CGO_ENABLED=0 go build ./...` | PASS |
| Frontend ci/lint/types/test/build | `npm ci`, `npm run lint/typecheck/test/build` | PASS (471 tests) |
| Frontend static export | `npm run build:static` | PASS |
| Browser E2E | `npx playwright test` (single binary) | PASS (10/10) |
| govulncheck / npm audit | (last run) | PASS / no critical |

## Release pipeline side effects
- `release.yml` **publishes on tag push** (`gh release create`). `scripts/build-release.sh`
  only cross-compiles with `-trimpath`/`-ldflags`; it does **not** tag/push.
  `scripts/release-dry-run.sh` writes archives/checksums/SBOM into the local
  `release/` dir only. No publishing script was executed in R01.

## Required action for R06
Push the baseline (or open a PR) so CI validates T10–T20 before release-candidate
construction; treat CI green on the frozen commit as a precondition for R06.
