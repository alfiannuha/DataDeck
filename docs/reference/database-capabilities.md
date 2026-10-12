# Database Capabilities

Capability matrix for DataDeck v0.1.0. "Supported" means implemented **and
verified**; "Limited" means implemented with documented constraints; a disabled
capability returns an explicit error rather than failing silently.

| Capability | PostgreSQL | MySQL | SQLite |
|---|---|---|---|
| Connect | Supported | Supported | Supported |
| Query execution | Supported | Supported | Supported |
| Database discovery | Supported (server-level, multi-database) | Single database per profile | Single file |
| Schema Explorer | Supported | Supported | Supported |
| Table Data browse | Supported | Supported | Supported |
| Filter / Sort / Pagination | Supported (server-side) | Supported | Supported |
| Table Structure | Supported | Supported | Supported |
| Insert row | Supported | Supported | Supported |
| Update row | Supported | Supported | Supported |
| Delete row | Supported | Supported | Supported |
| Duplicate row | Supported | Supported | Supported |
| Table export (CSV/XLSX/SQL) | Supported | Supported | Supported |
| Table import (CSV / restricted SQL) | Supported | Supported | Supported |
| Table import (XLSX) | Not available | Not available | Not available |
| Database export (base tables; SQL) | Supported | Supported | Supported |
| Database restore (Deck artifact) | Supported (atomic) | **Disabled** | Supported (atomic) |
| Reconnect | Supported | Supported | Supported |
| Delete database | Supported | Supported | **Disabled** |
| Multiple databases per profile | Supported | No | No |

## Notes

- **Row CRUD** requires a **declared primary key** for Update/Delete (composite PK
  supported). Tables without a PK allow Insert/Duplicate only.
- **Table import** is INSERT-only for one table and runs atomically.
- **Database export** covers **base tables only**; views/sequences/triggers/
  functions/extensions are excluded and named in the artifact header.
- **MySQL restore is disabled** because MySQL DDL implicitly commits and DataDeck
  cannot guarantee an atomic restore.
- **SQLite database deletion is disabled** (a database is a local file).
- **XLSX import** is deferred to a future release.

Cross-engine gaps above are deliberate, documented policy — not untested claims.
