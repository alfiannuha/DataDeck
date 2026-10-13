# DataDeck v0.1.0 — R05 Backend Regression

Commit `e81ca8e`. Working dir `backend/`.

| Check | Command | Result |
|---|---|---|
| Modules | `go mod verify` | all modules verified |
| Formatting | `gofmt -l .` | clean |
| Vet | `go vet ./...` | clean |
| Unit | `go test -count=1 ./...` | 15/15 packages ok |
| Race | `go test -race -count=1 ./...` | 15/15 packages ok |
| Integration (PG+MySQL) | `DATADECK_TEST_PG_HOST=… DATADECK_TEST_MYSQL_HOST=… go test -tags=integration -p 1 -count=1 ./...` | 15/15 ok (PG 17.11, MySQL 8.0.46) |

## API contract
`internal/api/contract_test.go` (`TestSwaggerSpecMatchesImplementation`,
`TestEnvelopeConsistencyAcrossEndpoints`) PASS — routes ↔ committed `swagger.json` ↔
envelope consistent; `/health` now includes `version` + `channel` (R02). No
undocumented endpoint.

## R04 CSRF middleware (backend)
`middleware/csrf.go` + `csrf_test.go` PASS; wired in `router.go` after CORS. Unit
cases cover cross-origin reject (POST/DELETE/multipart), same-origin allow,
allowlisted dev origin, no-Origin allow, null-Origin reject, cross-origin GET allow.

## Notable regression suites re-run
- Connection identity/isolation: `internal/database` integration (pool key,
  reconnect isolation, outage recovery) PASS.
- TableData (browse/sort/filter/page): unit + integration PASS.
- CRUD identity/concurrency: `mutation_unit_test`, `table_rows_test` PASS.
- Import parser + DB restore: `tableimport`, `dbscript` PASS.
- Delete database: `database_delete_test`, `database_admin_test`, `database_delete_mysql_integration_test` PASS.
