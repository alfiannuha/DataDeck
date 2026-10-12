# DataDeck v0.1.0 — Destructive Operation Security (Delete Database)

## Capability
PostgreSQL and MySQL only. **SQLite disabled** (`501 DATABASE_DELETE_UNSUPPORTED`)
— a database is a local file; safe deletion is out of scope.

## Two-phase, fail-closed workflow
1. **Preflight** validates the target, engine support, protected policy, active
   operations, and existence, then returns a **short-lived (5 min), single-use,
   cryptographically random** token bound to `connection + database + engine`. No
   DROP runs.
2. **Execute** consumes the token once, re-resolves the profile, re-checks the token
   binding, requires a **case-sensitive typed exact name** + acknowledgment,
   re-checks protection/active operations/existence, retires the target pool, runs
   `DROP DATABASE <quoted>`, and **independently verifies** absence.

## Safeguards verified (unit + PG E2E + MySQL integration)
| Control | Evidence |
|---|---|
| Unknown/wrong/cross-target/reused/expired token rejected | `database_delete_test.go` |
| Wrong typed name / missing acknowledgment rejected | unit |
| Protected DBs rejected (`postgres`/`template0`/`template1`; `mysql`/`information_schema`/`performance_schema`/`sys`) | unit + e2e |
| SQLite rejected (501) | unit |
| Active operations block (`DATABASE_IN_USE`); sessions never force-terminated | `deleteBlockingCondition` |
| PostgreSQL DROP on a maintenance connection (never the target) | `deleteExecutionConfig` |
| No `WITH (FORCE)`, no `CASCADE` | source inspection |
| No automatic retry; ambiguous result → `DATABASE_DELETE_OUTCOME_UNKNOWN` | handler |
| Verified deletion; `409` if still present | `DatabaseExists` |
| Profile/saved queries/history preserved; tabs not rebound | design + tests |
| Audit line credential/token-free | `logger.Info` fields |

## Token binding / TTL / single-use
- Random 16-byte hex; TTL 5 min; consumed on execute (replay → invalid).
- Bound to connection id + database + engine; a token for `alpha` cannot delete `beta`
  (E2E).

## Race / lifecycle
Deletions are serialized per target (second concurrent delete → `409
DATABASE_DELETE_CONFLICT`); the target pool is retired before DROP; other databases'
pools are untouched.

## Residual
External (non-DataDeck) sessions still block DROP (no FORCE) — surfaced as a clear
error, by design.
