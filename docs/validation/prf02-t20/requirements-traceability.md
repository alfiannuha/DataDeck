# PRF02-T20 — Requirements Traceability

Source of requirements: DataDeck PRD (PRF-02), ADR-010 + amendments. Status at
HEAD `e3f6c20` (code `7958007`). Verified = reproduced at HEAD; Reported = task
report only; N/A = approved policy.

| Req | Source | Task | Implementation | Test | Engine | Evidence | Status |
|---|---|---|---|---|---|---|---|
| RD-01 | ADR-010 §2/§3 | T01 | `useWorkspaceStore.ts` | store tests | all | unit + e2e | PASS |
| RD-02 | ADR-010 §2 | T02 | `workspace-empty-state.tsx` | workspace tests | n/a | unit | PASS |
| RD-03 | ADR-010 §4-6 | T03/T04 | `handler/table_data.go` | `table_data_unit_test`, integration | all | unit+integration+e2e | PASS |
| RD-04 | ADR-010 §6 | T03 | `table_data.go` pagination | unit | all | unit | PASS |
| RD-05 | ADR-010 §8 | T05 | `table_data.go` sort | unit | all | unit | PASS |
| RD-06 | ADR-010 §7 | T06 | `table_filter.go` | filter tests | all | unit | PASS |
| RD-07 | ADR-010 §30 | T07 | `table_data.go` rowCapabilities/rowIdentity | `table_rows_test` | all | unit+integration | PASS |
| RD-08 | ADR-010 §32 | T08 | `insert.go` | insert tests | all | unit+integration | PASS |
| RD-09 | ADR-010 §33 | T09 | `mutation.go` | update tests | all | unit+integration | PASS |
| RD-10 | ADR-010 §34/§35 | T10 | `mutation.go` | delete/duplicate tests | all | unit | PASS |
| RD-11 | ADR-010 §31 | T11 | CRUD dialogs | component tests | all | unit | PASS |
| RD-12 | ADR-010 §10 | T12 | `schema-tree.tsx` | menu tests | PG | unit+e2e | PASS |
| RD-13 | ADR-010 §12 (amended) | T13 | `handler/table_structure.go` | `table_structure*` | all | unit+integration+e2e | PASS |
| RD-14 | ADR-010 §13/§14/§15 | T14 | `export/*` | export tests | all | unit+integration+e2e | PASS |
| RD-15 | ADR-010 §13/§14 | T15 | `tableimport/*` | tableimport tests | all | unit+integration+e2e | PASS |
| RD-16 | ADR-010 §11/§11a | T16 | `Manager.Reconnect` | reconnect tests | PG/MySQL/SQLite | unit+integration+e2e | PASS |
| RD-17 | ADR-010 §16 | T17 | `dbscript/*` | dbscript tests | PG/SQLite restore; MySQL export | unit+integration+e2e | PASS (MySQL restore N/A) |
| RD-18 | ADR-010 §17 | T18 | `database_admin.go`, `database_delete.go` | delete tests | PG/MySQL; SQLite disabled | unit+integration+e2e | PASS |

## Deferred by approved policy (NOT APPLICABLE)
- XLSX **import** (T15 ships CSV + INSERT-only SQL) — ADR-010 §15.
- MySQL **restore** — ADR-010 §16 (`RESTORE_UNSUPPORTED`, DDL implicit commit).
- SQLite **delete database** — ADR-010 §17 (`DATABASE_DELETE_UNSUPPORTED`).
- Export of views/sequences/triggers/functions/extensions — base-tables-only, named in the artifact header.

## Verified at HEAD in T20
α/β isolation (e2e 10/10), RowIdentity/optimistic concurrency (unit), table import atomic rollback (unit), table export integrity (e2e 1000 rows), reconnect pool lifecycle (unit+integration), delete confirmation/protection (unit + PG e2e + **new MySQL integration**), production build, contract/OpenAPI.
