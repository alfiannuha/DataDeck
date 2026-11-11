# DataDeck v0.1.0 — R09 Functional / Database Smoke

Subject: **published** `datadeck_0.1.0_darwin_arm64` running on `127.0.0.1:8099`.
Full browser E2E suite (Playwright) run against the published binary; disposable
databases only.

## PostgreSQL (full E2E — 9/10, 1 cleanup flake)
| Scenario | Result |
|---|---|
| α/β isolation (browse/sort/filter, tab binding) | PASS |
| Table Data pagination + refresh | PASS |
| Table export (1000 rows despite page 100, menu target) | PASS |
| Table import (CSV, bound to menu target; read-only blocked) | PASS |
| Table Structure (α/β, read-only flags, constraint text) | PASS |
| Table context menu (capabilities, copy, refresh) | PASS |
| Database reconnect (db-scoped, α while Explorer on β) | PASS |
| Database export/restore (α-scoped; non-empty → 409) | PASS |
| Delete database (α only; protected + token substitution rejected) | PASS |
| `table-menu` cleanup | 1 timeout in full suite; **passes alone** → cleanup flake (R04-FIND-005 class) |

No α/β crossover observed → no BLOCKER.

## SQLite (API smoke, published binary)
Create profile → `CREATE TABLE` → `INSERT 'héllo'` → `SELECT` returned
`[[1,'héllo']]`; JSON envelope success. PASS.

## MySQL (BLOCKED — environment)
The local MySQL server was **not running** during R09 (`ECONNREFUSED` on 127.0.0.1:3306).
A MySQL profile query correctly returned `CONNECTION_ERROR` (no crash, sanitized).
MySQL runtime smoke = **BLOCKED (environment)**; MySQL feature coverage was provided
by the R05 integration suite (when the server was up). Not a product defect.

## Workspace / query / CRUD / import-export
Exercised via the published binary by the E2E suite above (query tabs, table data,
CRUD, import/export, structure, delete). All PASS except the cleanup flake.
