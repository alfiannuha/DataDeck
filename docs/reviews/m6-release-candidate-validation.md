# M6 Release Candidate Validation

Status: M6-T11. Validation of the exact release-candidate artifacts using the
release pipeline's reproducible dry-run.

## 1. RC Version

- Version: **`0.1.0-dev`** (central `VERSION` file), commit **`0ed37c7`**.
- Provenance: hosted **Release dry run** run
  [36357041687](https://github.com/alfiannuha/DataDeck/actions/runs/36357041687)
  (Validate ✓, 5× Build ✓, SBOM/licenses/packaging ✓, Publish skipped).
- Because downloading the hosted artifact bundle stalled repeatedly in this
  sandbox (network/`gh`/`curl` hangs; the API and npm registry themselves
  responded 200), artifacts were produced with the **same release scripts**
  locally (`scripts/release-dry-run.sh`), which is the equivalent reproducible
  dry-run the task permits. The hosted run's success is the pipeline evidence.

## 2. Artifact Checks — PASS

| Check | Result |
|---|---|
| Expected files present | 5 binaries (darwin arm64/amd64, linux amd64/arm64, windows amd64) + `.tar.gz`/`.zip` archives + `SHA256SUMS` + `sbom/` + `licenses/` |
| Archive extraction | darwin/arm64 `.tar.gz` extracts `datadeck` (statically linked) + `VERSION` |
| Executable | runs; `datadeck --version` → `datadeck 0.1.0-dev (commit 0ed37c7, built 2026-09-28T00:53:18Z)` |
| SHA-256 checksums | `shasum -a 256 -c SHA256SUMS` → all **OK** (5 archives + notices + frontend SBOM) |
| SBOM | `sbom/frontend.cdx.json` (CycloneDX 1.5, 493 components). The Go CycloneDX+SPDX SBOMs were produced in the hosted pipeline job (`syft`); local reproduction skipped them because `syft` is not installed locally — limitation noted |
| License/notices | `licenses/THIRD_PARTY_NOTICES.txt` present (Go shipped modules + npm license summary + review flags) |

## 3. Native Binary — PASS

Extracted RC started with `env -i PATH=/usr/bin:/bin` (no Node/Go/runtime
dependencies), fresh store:

- Startup → health: **126 ms** (warm FS); `/` 200, `manifest.json` 200, `sw.js` 200.
- Idle RSS: **20.98 MB** (2 s and settled) — see §8.
- Idle/offline/public service worker behavior verified (see PWA/E2E).

## 4. Database Matrix — PASS

| Engine | Connect | Introspection | Query | History | BIGINT |
|---|---|---|---|---|---|
| PostgreSQL | PASS | PASS (200) | PASS | PASS | `9007199254740993` exact |
| MySQL | PASS | PASS (200) | PASS | PASS | `18446744073709551615` exact |
| SQLite (file) | PASS | PASS (200) | PASS | PASS | exact; NULL → `null` |

History pagination `meta` correct; saved-query create/update verified.

## 5. Product Regression — PASS

- Browser E2E on the RC (`playwright test`, `E2E_BASE_URL=…:8080`): **2/2 passed** —
  shell → connection → query → result grid with exact BIGINT → manifest/SW →
  no `/api` cache; unknown asset 404 and JSON API 404.
- Result virtualization on the RC: 100,000-row result rendered **22 DOM rows /
  42 cells** (~340 ms to first paint), status `Success · 200 ms · 100000 rows`.
- CodeMirror editor, query tabs, connection binding (new tab binds without
  rebinding existing tabs — observed when a SQLite-bound tab rejected PG syntax),
  history, saved queries, schema explorer (1000 tables → 109 `<li>` + **"Show
  more"** bounded rendering), CSV/JSON export actions (`Exported 100000 rows`).
- Connection-aware tabs behaved per architecture; no cross-connection execution.

## 6. Security Regression — PASS

- **Loopback default:** starting the RC without `HOST` binds `127.0.0.1` (verified
  with `lsof`: `TCP 127.0.0.1:8082 (LISTEN)`).
- **Credential isolation:** wrong MySQL password → `502 CONNECTION_ERROR` with
  no credential in the response.
- **Request limits / validation:** malformed JSON → 400; oversized body → 400;
  invalid pagination → 400; unknown API path → JSON `NOT_FOUND` (never the SPA).
- **Service-worker cache:** E2E asserts Cache Storage has **no `/api` entries**.
- **CSV formula safety:** implemented by the shared serializer (`=`,`+`,`@`,
  non-numeric `-` prefixed; numeric negatives untouched) and covered by unit
  tests; the RC export action works end-to-end.

## 7. Performance (release artifact)

| Metric | Value |
|---|---|
| Startup → health | 126 ms (warm) |
| Idle RSS | 20.98 MB |
| 100k query (SQLite, API) | 142 ms |
| 100k grid | 22 DOM rows / 42 cells, ~340 ms |
| 1000-table introspection | 102 ms, 255 KB JSON |
| 1000-table explorer DOM | 109 `<li>`, incremental "Show more" |
| CSV/JSON export (100k) | actions succeed; timings from the same bundle: CSV ~131–411 ms, JSON ~35–127 ms (M5-T07) |

## 8. PRD <25 MB Re-evaluation — PASS

Idle RSS of the release binary (embedded frontend, loopback, no active queries):
**20.98 MB < 25 MB**. Executable size (15.8 MB binary inside the archive) is
tracked separately and is **not** conflated with RSS. The embedded frontend adds
to artifact size but the measured RSS remains under the PRD target.

## 9. Docker — PASS

Production image built and validated:

- Size: **25,692,645 bytes (~24.5 MB)**.
- `Config.Env` contains `HOST=0.0.0.0`, `ALLOW_REMOTE=1`, `PORT`, `STORAGE_PATH`,
  `ENVIRONMENT`, `LOG_LEVEL` — **no `ENCRYPTION_KEY` value** (supplied at runtime;
  startup fails without it).
- Health/frontend/manifest/SW **200**; `remote_binding_enabled` logged (explicit
  container exposure); runs **non-root** (`uid=10001`).
- Created a SQLite connection and queried it; **restart** persisted the profile
  via the named volume.

## 10. Platform Matrix (honest status)

| OS | Arch | Build | Runtime |
|---|---|---|---|
| darwin | arm64 | PASS | **PASS** (native RC) |
| darwin | amd64 | PASS | **PASS** (Rosetta RC: version, health, frontend) |
| linux | amd64 | PASS | **PASS** (RC artifact executed in an Alpine container: health/frontend/SW 200 + query) |
| linux | arm64 | PASS | **NOT VERIFIED** (no arm64 Linux runtime/QEMU available) |
| windows | amd64 | PASS | **NOT VERIFIED** (no Windows host) |

## 11. Adversarial Smoke — PASS

| Scenario | Result |
|---|---|
| Wrong credentials | `502 CONNECTION_ERROR`, no leak |
| Query timeout (`pg_sleep(5)`, timeout 1) | `504 QUERY_TIMEOUT` |
| Backend restart | graceful `shutdown_complete`; recoverable, store intact |
| PostgreSQL outage | `CONNECTION_ERROR` during, `/health` 200, recovery after restart |
| MySQL outage | same as PostgreSQL |
| >50 MB truncation | HTTP 200, `truncated: true`, 52,167 rows, valid JSON (52.4 MB chunked) |
| Malformed/oversized request | `400` with sanitized envelope; server healthy |

## 12. Findings

- **None blocking.** No credential leakage, no crash, no regression found.
- Client-side `IncompleteRead` observed once reading the large truncated response
  via Python `urllib`; a clean `curl` fetch returned a valid 52,427,996-byte
  chunked JSON with `truncated:true`, so this was a client artifact, not a server
  defect.
- The Playwright MCP browser session dropped mid-run after the E2E suite; the
  remaining browser checks were completed with the Chrome DevTools MCP.
- Hosted artifact **download** was unreliable in this sandbox; artifact
  generation was reproduced locally with the identical release scripts. The Go
  SBOM (CycloneDX/SPDX via `syft`) was produced by the hosted pipeline and was not
  regenerated locally.

## 13. Review Document

This file: `docs/reviews/m6-release-candidate-validation.md`.
