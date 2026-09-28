# M6 Final Release Readiness Review

## Executive Summary

DataDeck is **M6 READY**. The release candidate builds, runs, installs, upgrades,
and packages correctly, with resolved/accepted M5 findings, real hosted CI PASS
on the merge commit, valid checksums/SBOM/notices, no credential or key leakage,
and measured performance within the PRD memory target.

All release-critical gates passed on the **actual hosted pipeline** (not just
locally): CI (Backend, PG+MySQL integration, Frontend) and Browser E2E are green
on `586debe`, and the tag-driven Release workflow dry-run is green. No BLOCKER
and no unresolved HIGH issue was found.

## Release Candidate

| Item | Value |
|---|---|
| Version | `0.1.0-dev` (central `VERSION` file) |
| Commit | `586debe` (reviewed HEAD); artifacts built at `0ed37c7` |
| Artifact source | Hosted Release workflow **dry run** [36357041687](https://github.com/alfiannuha/DataDeck/actions/runs/36357041687) (Publish skipped); reproduced locally with the same scripts (`scripts/release-dry-run.sh`) because hosted artifact download stalled in this sandbox |

## Validation Environment

macOS Darwin arm64; go1.26.6 (`CGO_ENABLED=0`); Node 24.14.0 / npm 11; Next.js
15.5.26 / React 19.3.0; PostgreSQL 17.11, MySQL 8.0.46, SQLite
(`modernc.org/sqlite`); Docker Desktop; GitHub Actions hosted runners.

## Commands Executed

```text
Hosted:  CI run 36367953995 -> Backend PASS, Backend Integration (PostgreSQL) PASS, Frontend PASS
         Browser E2E run 36367954017 -> Single-binary release flow PASS
         Release dry-run 36357041687 -> Validate/Builds/SBOM/packaging PASS, Publish skipped
Local:   gofmt -l (clean), go vet, go test ./..., go test -race ./..., CGO_ENABLED=0 go build
         govulncheck ./... -> No vulnerabilities found
         frontend lint / typecheck / 277 tests / build / build:static
         make check -> 0
         shasum -a 256 -c release/SHA256SUMS -> all OK
         secret scan of binaries/SBOM/notices -> 0 precise hits
```

## M5 Risk Closure

| M5 finding | Disposition |
|---|---|
| Outage mapping returned `INTERNAL_ERROR` | **FIXED** (M6-T00): connection loss → `CONNECTION_ERROR`; SQL errors never reclassified; verified again during RC outage tests |
| pgx/x-text advisories | **FIXED**: pgx v5.9.2, x/text v0.39.0; `govulncheck` reports 0 |
| Go toolchain stdlib advisories | **FIXED**: `toolchain go1.26.6` pinned; CI installs Go 1.26.6 |
| Next/PostCSS advisory | **ACCEPTED** (build-time only; no 15.x fix) — documented in `docs/release/licenses.md` |
| 1000-table rendering (1001 DOM nodes) | **FIXED**: incremental `TableList`; 1000 tables → 109 DOM nodes + "Show more" |
| Idle RSS margin | Evaluated each milestone; final RC idle RSS 20.98 MB (< 25 MB) |

## Static Export

`DATADECK_STATIC_EXPORT=true` emits `frontend/out` (route-specific HTML, no SPA
rewrite needed); `NEXT_PUBLIC_API_URL` empty → same-origin `/api/v1`. No server
actions/routes/middleware/image optimizer. Documented in
`docs/release/static-export.md`.

## Single Binary

- Frontend + API embedded via `embed.FS`; served by the Go binary; `/api*` never
  served statically.
- **No Node runtime required** (validated with `env -i PATH=/usr/bin:/bin`).
- First run, existing store, graceful shutdown (`shutdown_complete`), and PWA all
  verified on the packaged artifact.

| Metric | Value |
|---|---|
| Executable size | 15.8 MB (binary in archive); 23.8 MB unstripped dev build |
| Startup → health | 126 ms (warm) |
| Idle RSS | **20.98 MB** |

## Embedded Asset Security

Embedded payload is the 29-file static export (`index.html`, `404.html`,
`_next/**`, `icons/**`, `manifest.json`, `sw.js`); no source maps. Binary scan: no
`.env` file, no `node_modules`, no `frontend/src`, no `go.sum`, no
`ENCRYPTION_KEY=<value>`, and 0 precise secret-pattern hits. Regex `.env` matches
are minified `process.env` substrings.

## Platform Matrix

| OS | Arch | Build | Runtime | Artifact (size) | Archive SHA-256 |
|---|---|---|---|---|---|
| darwin | arm64 | PASS | **PASS** (native) | `datadeck_0.1.0-dev_darwin_arm64` | `a25fed66…` |
| darwin | amd64 | PASS | **PASS** (Rosetta) | `datadeck_0.1.0-dev_darwin_amd64` | `2b476bca…` |
| linux | amd64 | PASS | **PASS** (RC artifact in Alpine container) | `datadeck_0.1.0-dev_linux_amd64` | `4349de22…` |
| linux | arm64 | PASS | NOT VERIFIED | `datadeck_0.1.0-dev_linux_arm64` | `86439caa…` |
| windows | amd64 | PASS | NOT VERIFIED | `datadeck_0.1.0-dev_windows_amd64.exe` | `e1975d47…` |

`shasum -a 256 -c SHA256SUMS` → all OK (archives + notices + frontend SBOM).

## Docker

Image builds from `docker/Dockerfile` (multi-stage; Alpine runtime): single
binary, **non-root** (uid 10001), explicit container network exposure
(`HOST=0.0.0.0` + `ALLOW_REMOTE=1`, warning logged), unauthenticated healthcheck,
named-volume persistence across restart, external `ENCRYPTION_KEY` (not baked
into `Config.Env`). **Image size 25,692,645 bytes (~24.5 MB)**.

## Supply Chain

| Check | Result |
|---|---|
| govulncheck | **No vulnerabilities found** (0 reachable) |
| npm audit | 1 moderate (`next`, build path) + 1 high (`postcss`, build-time transitive) + 0 critical |
| SBOM | Frontend CycloneDX 1.5 (493 components) in the artifact; Go CycloneDX 1.7 + SPDX 2.3 produced by the hosted pipeline job (`syft`) |
| Licenses | `THIRD_PARTY_NOTICES.txt` (Go shipped modules + npm summary); MPL-2.0 (mysql driver, lightningcss, axe-core) and optional LGPL sharp flagged for human review |
| Checksums | SHA-256 for all archives + SBOM + notices; verified |
| Artifact secret scan | 0 precise hits (example key, test credentials, private keys, tokens) |

## SBOM & Licenses

See `docs/release/security.md` and `docs/release/licenses.md`. No applicable
critical release-runtime vulnerability. Notice obligations for MPL-2.0 and the
optional LGPL sharp chain require human/legal review (documented).

## Hosted CI

**REMOTE CI: PASS** on the reviewed HEAD `586debe`:

- CI https://github.com/alfiannuha/DataDeck/actions/runs/36367953995 —
  **Backend** (gofmt, vet, test, race, govulncheck, CGO-free build) ✓,
  **Backend Integration (PostgreSQL + MySQL)** ✓, **Frontend** (npm ci, lint,
  typecheck, tests, build, static export, npm audit) ✓. SQLite is exercised by
  the backend unit + integration-tagged suites and browser E2E.
- Browser E2E https://github.com/alfiannuha/DataDeck/actions/runs/36367954017 —
  **Single-binary release flow** ✓ (2 tests).
- Release **dry run** https://github.com/alfiannuha/DataDeck/actions/runs/36357041687 —
  Validate ✓, 5 platform builds ✓, SBOM/licenses/packaging ✓, Publish skipped ✓.

## Release Workflow

Tag (`v*`) or manual dry-run → Validate (fail-closed quality/security gates,
version↔tag consistency) → matrix build (frontend export → embed → CGO-free
binary) → SBOM/licenses/packaging/checksums → publish (tags only). Jobs use
`needs` and **no `continue-on-error`**, so a mandatory failure stops publication.

## Fresh Install

Packaged archive install (no source checkout, no Node): startup, health,
frontend, PWA, first-run store (`0700` dir, `0600` files, migration applied),
restart persistence, PostgreSQL/MySQL/SQLite connections and queries. See
`docs/reviews/m6-fresh-install.md`.

## Upgrade & Migration

Pre-M6 store → RC: startup, schema re-validated (no new migration), profiles +
IDs preserved, encrypted credentials usable, history and saved queries
preserved, SQLite target config/data intact, all three engines work. Service
worker update lifecycle works with dirty SQL (no forced reload, no loss).
Rollback is safe **only while migrations are unchanged** (forward-only); broken
store fails explicitly. See `docs/reviews/m6-upgrade-e2e.md`.

## PWA / Service Worker

Manifest `standalone` (192/512 icons), SW active, offline shell + banner,
Cache Storage holds only the shell (no `/api`), and the update prompt is
user-controlled with dirty-SQL protection.

## PostgreSQL Regression

**PASS** — connection/test, introspection, query, history, exact BIGINT.

## MySQL Regression

**PASS** — connection/test, introspection, query, history, unsigned BIGINT.

## SQLite Regression

**PASS** — connection, introspection, DDL/query, exact BIGINT/NULL; lock/busy
behavior bounded.

## Product Regression

**PASS** — connection UI, schema explorer, editor (CodeMirror), query tabs,
virtualized result grid, history, saved queries, CSV/JSON export, schema actions,
PWA/offline/update. RC browser E2E 2/2 passed.

## Security Regression

**PASS** — credential isolation, AES-256-GCM key handling, native loopback
default, container exposure explicit, request limits, sanitized errors, PWA cache
safety, export formula protection, artifact secret scan clean.

## Adversarial Smoke

**PASS** — wrong credentials (`502`, no leak), query timeout (`504`), backend
restart (graceful), PG/MySQL outage (`CONNECTION_ERROR`, health 200, recovery),
>50 MB truncation (52,167 rows, valid JSON), malformed/oversized requests
(`400`).

## Performance

| Metric | Value |
|---|---|
| Startup | 126 ms (warm) |
| Idle backend RSS | 20.98 MB |
| 100k query (SQLite API) | 142 ms |
| 100k DOM nodes | 22 rows / 42 cells (~340 ms) |
| 1000-table explorer | 102 ms / 255 KB introspection; 109 DOM nodes + incremental |
| CSV export | ~131–411 ms (same bundle; M5-T07) |
| JSON export | ~35–127 ms |
| Executable size | 15.8 MB |
| Docker image size | 25,692,645 bytes |

## PRD Memory Target

Backend idle RSS measured on the release artifact: **20.98 MB < 25 MB → PASS**.
Executable size is reported separately and not conflated with RAM.

## Release Documentation

- README: install, configuration, encryption key, data location, backup,
  upgrade/rollback, Docker, known limitations, enforced limits.
- `.env.example`: placeholders only, with key/warning guidance.
- `docs/release/`: `static-export.md`, `docker.md`, `platform-matrix.md`,
  `security.md`, `licenses.md`.
- `docs/reviews/`: M6 baseline through RC validation and this review.

## Findings

### BLOCKER
None.

### HIGH
None.

### MEDIUM
- **M1 — Next/PostCSS advisories** (build-time only; no 15.x fix). Accepted;
  resolve with a planned Next 16 upgrade.
- **M2 — linux/arm64 and windows/amd64 runtime NOT VERIFIED.** Builds pass and
  checksums are valid, but no runtime execution was possible on those platforms
  here.
- **M3 — Go SBOM not reproducible locally.** The hosted pipeline generates the
  Go CycloneDX/SPDX SBOMs via `syft`; `syft` was unavailable locally, so the
  local artifact set contains only the frontend SBOM.

### LOW
- `syft` is installed `@latest` in the release workflow; CI actions use floating
  major tags (SHA/digest pinning recommended).
- `.env` regex matches in the secret scan are `process.env` substrings
  (false positives; classification documented).
- Docker image validated single-container on Docker Desktop only.
- Version remains the dev-style `0.1.0-dev`; a real `vX.Y.Z` tag is still to be
  created when publishing.
- Idle RSS is within target but the margin varies (≈21 MB vs 25 MB) across runs.
- GitHub annotation: `ubuntu-latest` → Ubuntu 26 migration.

## Blockers

None.

## Known Limitations

No authentication (loopback default; container binds wider inside its
namespace); forward-only migrations; cross-compiled Linux ARM64/Windows binaries
not runtime-verified; build-time npm advisories pending Next upgrade; deep
pagination uses `OFFSET`; MPL-2.0/LGPL license items awaiting human review.

## Post-Release Recommendations

Pin CI actions to SHAs and `syft` to a version; add linux-arm64 and Windows
runtime CI; generate the Go SBOM in every build; resolve the Next upgrade;
consider signed checksums/SBOM attestations; automate the install/upgrade E2E in
the release workflow.

## Exit Checklist

- [x] M5 findings resolved/accepted with evidence
- [x] single binary works
- [x] no Node runtime required
- [x] fresh install works
- [x] upgrade works
- [x] existing encrypted credentials survive
- [x] supported platform builds succeed
- [x] native runtime validation passes
- [x] checksums valid
- [x] Docker works
- [x] persistent storage works
- [x] no baked secrets
- [x] SBOM available
- [x] license/notices available
- [x] vulnerability audit complete
- [x] no applicable critical runtime vulnerability
- [x] REMOTE CI PASS
- [x] browser E2E PASS in hosted/release validation
- [x] release workflow dry-run PASS
- [x] PostgreSQL PASS
- [x] MySQL PASS
- [x] SQLite PASS
- [x] product regression PASS
- [x] security regression PASS
- [x] critical adversarial smoke PASS
- [x] performance re-measured
- [x] PRD <25 MB backend RAM explicitly evaluated (PASS)
- [x] large-schema rendering bounded
- [x] release documentation complete
- [x] no BLOCKER
- [x] no unresolved HIGH issue making release unsafe

## Final Status

**M6 READY**
