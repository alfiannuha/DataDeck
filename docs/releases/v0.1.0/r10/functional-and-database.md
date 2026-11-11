# DataDeck v0.1.0 — R10 Functional, Database Safety & Engine Matrix

## Functional completeness (evidence-linked)
Connection management (create/edit/test/open/reconnect/delete) — VERIFIED (unit + e2e).
Explorer (discovery, browsing, context menus, open table) — VERIFIED (e2e).
Query editor (execute, results, errors, history, saved) — VERIFIED (M1–M3 + e2e).
Table Data (pagination/sort/filter/refresh) — VERIFIED (unit + e2e).
Table Structure (columns/types/PK/FK/indexes/constraints) — VERIFIED (unit + e2e).
CRUD (insert/update/delete/duplicate, optimistic concurrency, PK-only) — VERIFIED
(unit + integration + e2e).
Import/export (CSV/XLSX/SQL export; CSV+INSERT-only SQL import; DB export/restore) —
VERIFIED (unit + integration + e2e), with documented engine restrictions.
Database management (reconnect, export, restore, delete) — VERIFIED.

## Database safety invariant
**No database operation executes against a different database than its originating
context.** Verified via α/β isolation across browse, sort, filter, CRUD, table
export/import, structure, reconnect, db-export/restore, delete (T19 + R09 e2e). No
crossover observed → invariant holds.

## Data integrity
Transaction boundaries (per-mutation; atomic import; transactional restore on
PG/SQLite), rollback on failure, PK/composite identity, optimistic concurrency,
affected-rows 0/1/>1 (rollback on >1), NULL semantics, BIGINT/DECIMAL string
precision, generated-column protection, import atomicity, destructive confirmation
tokens — all VERIFIED (T07–T18; R05/R09).

## Engine capability matrix (v0.1.0)
| Operation | PostgreSQL | MySQL | SQLite |
|---|---|---|---|
| Connect | VERIFIED | VERIFIED (R05) | VERIFIED |
| Discover | VERIFIED (multi-db) | VERIFIED (single-db) | VERIFIED |
| Query | VERIFIED | VERIFIED | VERIFIED |
| Browse / filter / sort | VERIFIED | VERIFIED | VERIFIED |
| CRUD | VERIFIED | VERIFIED | VERIFIED |
| Table import/export | VERIFIED | VERIFIED | VERIFIED |
| Database export | VERIFIED | VERIFIED | VERIFIED |
| Database restore | VERIFIED | **N/A (disabled)** | VERIFIED |
| Database delete | VERIFIED | VERIFIED (integration) | **N/A (disabled)** |
| XLSX import | N/A | N/A | N/A |

Documented limitations (ADR-010): MySQL restore disabled (DDL implicit commit);
SQLite delete disabled (local file); XLSX import deferred; database export is
base-tables-only (views/sequences/triggers/functions/extensions excluded).
