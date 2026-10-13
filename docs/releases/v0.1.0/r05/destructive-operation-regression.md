# DataDeck v0.1.0 — R05 Destructive-Operation (Delete Database) & Reconnect Regression

## Delete Database (disposable DBs only)
| Scenario | Result |
|---|---|
| Preflight (no DROP) → token | PASS |
| Protected DB (`postgres`/`template0`/`template1`; `mysql`/`information_schema`/`performance_schema`/`sys`) | rejected (`DATABASE_DELETE_PROTECTED`) |
| Typed name (case-sensitive) + acknowledgment required | PASS |
| Token TTL / single-use / replay / cross-target / connection-generation mismatch | rejected |
| Active operation → `DATABASE_IN_USE`; sessions never force-terminated | PASS |
| PostgreSQL DROP on maintenance connection (never target) | PASS |
| No `WITH (FORCE)` / no `CASCADE` / no auto-retry | PASS |
| Independent absence verification; unknown outcome → `DATABASE_DELETE_OUTCOME_UNKNOWN` | PASS |
| SQLite delete disabled (501) | PASS |
| Unrelated β database untouched | PASS (E2E) |
| MySQL live delete | PASS (integration) |

Evidence: `database_delete_test.go`, `database_admin_test.go`,
`database_delete_mysql_integration_test.go`, `delete-database.spec.ts`.

## Reconnect
| Scenario | Result |
|---|---|
| Correct (db-scoped) target while Explorer on another DB | PASS |
| Failed reconnect fails closed (old pool kept) | PASS |
| In-flight operation not cancelled; no SQL/mutation replay | PASS (unit) |
| Concurrent reconnect serialized (no pool stacking) | PASS |
| Pool lifecycle preserved; unrelated DBs untouched | PASS (PG integration) |
| Active QueryTab/TableDataTab/StructureTab binding + unsaved SQL preserved | PASS (E2E: tab count + bindings) |
| Requires a maintenance connection for PostgreSQL; fails closed otherwise | PASS |

No destructive bypass or wrong-target deletion observed.
