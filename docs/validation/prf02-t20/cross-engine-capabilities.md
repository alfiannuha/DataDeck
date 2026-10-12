# PRF02-T20 — Cross-Engine Capability Matrix

Cell legend: **S+V** supported & verified · **S-U** supported, unit-verified only ·
**N/A** unsupported by approved policy · **NT** not tested · **FAIL** failing.
"Verified" means reproduced at HEAD with engine-specific execution.

| Feature | PostgreSQL | MySQL | SQLite |
|---|---|---|---|
| Browse / page | S+V (integration+e2e) | S+V (integration) | S+V (unit) |
| Sort | S+V | S+V | S+V |
| Filter | S+V | S+V | S+V |
| Table structure | S+V | S+V | S+V |
| Insert | S+V | S+V | S+V |
| Update | S+V | S+V | S+V |
| Delete row | S+V | S+V | S+V |
| Duplicate | S+V | S+V | S+V |
| Table export (CSV/XLSX/SQL) | S+V | S+V | S+V |
| Table import (CSV/SQL) | S+V | S+V | S+V |
| Database reconnect | S+V (e2e+integration) | S-U (shared manager; NT live) | S-U (unit) |
| Database export | S+V (e2e) | S+V (unit) | S+V (unit) |
| Database restore | S+V (e2e + unit round-trip) | **N/A** (disabled: DDL implicit commit) | S+V (unit round-trip) |
| Delete database | S+V (e2e) | S+V (**T20 integration test added**) | **N/A** (disabled: local file) |

## Engine-specific policy justifications
- **MySQL restore disabled** — atomicity cannot be guaranteed (ADR-010 §16).
- **SQLite delete disabled** — a database is a local file; safe deletion needs a
  separate approved design (ADR-010 §17).
- **XLSX import deferred** for all engines (ADR-010 §15).
- Export covers **base tables only**; views/sequences/triggers/functions/
  extensions are excluded and named in the artifact header (§16).

No cell is a generic checkmark: unit-only / not-tested cells are marked as such.
