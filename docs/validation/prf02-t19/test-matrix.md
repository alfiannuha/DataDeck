# PRF02-T19 — Validation Environment & Traceability Matrix

## Environment

| Item | Value |
|---|---|
| OS | macOS 26.6.2 (build 25G83), arm64 |
| Go | local `go1.26.5`; `go.mod` `toolchain go1.26.9` (auto-selected) |
| Node.js | v24.14.0 |
| PostgreSQL | 17.11 (Homebrew, aarch64) |
| MySQL | 8.0.46 |
| SQLite | `modernc.org/sqlite` v1.38.2 (pure Go, CGO-free) |
| Browser E2E | Playwright 1.63.0 / Chromium |
| Commit | `7958007` (PRF02-T18) |
| Build mode | `CGO_ENABLED=0`, embedded frontend (`build:static`) |
| Store | ephemeral `STORAGE_PATH` under `$TMPDIR`, ephemeral `ENCRYPTION_KEY` |

Reproduction (backend): `cd backend && go test -count=1 ./...`, `go test -race -count=1 ./...`,
`DATADECK_TEST_PG_HOST=127.0.0.1 DATADECK_TEST_PG_USER=alfiannuha DATADECK_TEST_MYSQL_HOST=127.0.0.1 go test -tags=integration -p 1 ./...`.
Frontend: `npm run lint && npm run typecheck && npm run test && npm run build && npm run build:static`.
E2E: build the single binary, `HOST=127.0.0.1 PORT=8099`, then `E2E_BASE_URL=… E2E_PG_HOST=127.0.0.1 npx playwright test`.

### Environment limitations
- Remote CI (GitHub Actions) is **not** available in this environment → CI jobs are **NOT RUN** (not PASS).
- Only **native** runtime is macOS arm64. Other targets are **cross-compiled** (compile-only); no native runtime smoke was performed there.
- Live MySQL deletion E2E was **NOT RUN**; MySQL delete is covered by unit + capability tests.
- No 1,000,000-row dataset benchmark was run; table-data paging is bounded by design and benchmarked to 100k.

## Status legend
PASS · FAIL · BLOCKED · NOT RUN · NOT APPLICABLE (policy/capability reason).

## Requirements traceability (T01–T18 → evidence)

| Req | Feature | Task | Primary evidence | Engine | Status |
|---|---|---|---|---|---|
| RD-01 | Typed workspace tabs keep explicit binding | T01 | `useWorkspaceStore` tests; e2e table-data/menu α/β | all | PASS |
| RD-02 | Empty workspace / no auto QueryTab | T02 | workspace tests; e2e release-flow | n/a | PASS |
| RD-03 | Table Data browse (no raw SQL) | T03/T04 | handler `table_data*` tests; e2e table-data.spec | PG/MySQL/SQLite | PASS |
| RD-04 | Server-side pagination | T03 | `table_data_unit_test`, integration | all | PASS |
| RD-05 | Sorting | T05 | `table_data_unit_test` sort cases; integration | all | PASS |
| RD-06 | Filtering (all operators, LIKE escape) | T06 | `table_filter` + unit tests | all | PASS |
| RD-07 | Row identity / PK-only / composite | T07 | `table_rows_test`, identity tests, integration | all | PASS |
| RD-08 | Insert (VALUE/NULL/DEFAULT) | T08 | insert tests; e2e table-data CRUD | all | PASS |
| RD-09 | Update (optimistic concurrency) | T09 | update tests; integration | all | PASS |
| RD-10 | Delete + Duplicate | T10 | delete/duplicate tests | all | PASS |
| RD-11 | CRUD UX | T11 | table-data-view tests | all | PASS |
| RD-12 | Table context menu | T12 | schema-explorer tests; e2e table-menu.spec | PG | PASS |
| RD-13 | Table structure endpoint (ADR amendment) | T13 | `table_structure*` tests; e2e table-structure.spec; contract test | all | PASS |
| RD-14 | Table export CSV/XLSX/SQL | T14 | export tests; integration; e2e table-export.spec | all | PASS |
| RD-15 | Table import CSV/SQL (atomic, INSERT-only) | T15 | tableimport tests; integration; e2e table-import.spec | all | PASS |
| RD-16 | DB context menu + reconnect (db-scoped) | T16 | manager/handler reconnect tests; integration; e2e database-menu.spec | PG/MySQL (SQLite unit) | PASS |
| RD-17 | DB export/restore (base tables) | T17 | dbscript tests; SQLite round-trip; e2e database-script.spec | PG/SQLite restore; MySQL export-only | PASS (MySQL restore N/A by policy) |
| RD-18 | Delete Database (two-phase) | T18 | admin/delete tests; contract; e2e delete-database.spec | PG e2e; MySQL unit; SQLite disabled | PASS (MySQL live E2E NOT RUN) |

## Engine matrix (feature × engine)

| Feature | PostgreSQL | MySQL | SQLite |
|---|---|---|---|
| Browse/filter/sort/page | PASS | PASS (integration) | PASS (unit) |
| Row CRUD | PASS | PASS (integration) | PASS (unit) |
| Table structure | PASS | PASS (integration) | PASS (unit) |
| Table export/import | PASS | PASS (integration) | PASS (unit) |
| DB reconnect | PASS (e2e) | NOT RUN (unit) | PASS (unit) |
| DB export | PASS (e2e) | PASS (unit) | PASS (unit) |
| DB restore | PASS (e2e + unit round-trip) | NOT APPLICABLE (disabled) | PASS (unit round-trip) |
| DB delete | PASS (e2e) | NOT RUN (unit) | NOT APPLICABLE (disabled) |

Cross-engine gaps are capability/policy-bound and documented in ADR-010 (§13/§14/§15/§16/§17).
