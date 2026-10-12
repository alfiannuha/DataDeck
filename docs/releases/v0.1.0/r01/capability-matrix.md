# DataDeck v0.1.0 — R01 Database Engine Capability Matrix

Legend: **SUPPORTED** (verified) · **UNSUP-POLICY** (unsupported by approved ADR-010
policy) · **BLOCKED** · **NOT VERIFIED**. No untested capability is marked SUPPORTED.
UI capability gating (`database-actions.ts`, `table-actions.ts`) matches this matrix.

| Capability | PostgreSQL | MySQL | SQLite |
|---|---|---|---|
| Connection | SUPPORTED | SUPPORTED | SUPPORTED |
| Schema discovery | SUPPORTED (multi-db) | SUPPORTED (single-db) | SUPPORTED (file) |
| Query execution | SUPPORTED | SUPPORTED | SUPPORTED |
| TableData browse | SUPPORTED | SUPPORTED | SUPPORTED |
| Filter / Sort | SUPPORTED | SUPPORTED | SUPPORTED |
| Structure | SUPPORTED | SUPPORTED | SUPPORTED |
| CRUD (insert/update/delete/duplicate) | SUPPORTED | SUPPORTED | SUPPORTED |
| Table import (CSV/SQL) | SUPPORTED | SUPPORTED | SUPPORTED |
| Table export (CSV/XLSX/SQL) | SUPPORTED | SUPPORTED | SUPPORTED |
| Reconnect | SUPPORTED | SUPPORTED (unit; live e2e NOT VERIFIED) | SUPPORTED (unit) |
| Database export | SUPPORTED | SUPPORTED (unit) | SUPPORTED (unit) |
| Database restore | SUPPORTED | **UNSUP-POLICY** | SUPPORTED |
| Delete database | SUPPORTED | SUPPORTED (integration) | **UNSUP-POLICY** |

## Policy notes
- MySQL restore disabled (DDL implicit commit; no atomic guarantee) — ADR-010 §16.
- SQLite delete disabled (local file) — ADR-010 §17.
- XLSX import deferred for all engines — ADR-010 §15.
- Database export covers base tables only.

## Gating alignment
- `Export SQL` enabled for all engines; `Import SQL (Restore)` disabled for MySQL.
- `Delete Database` disabled for SQLite (and for unknown engines).
- Verified in frontend tests and e2e (menu items + backend `501` responses).
