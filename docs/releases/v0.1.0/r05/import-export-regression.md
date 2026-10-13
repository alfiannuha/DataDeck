# DataDeck v0.1.0 — R05 Import / Export Regression

## Table export
CSV, XLSX, SQL (base tables). Verified: whole-table scope (1000 rows despite a
100-row page — `table-export.spec.ts`), NULL/Unicode/special characters,
BIGINT/DECIMAL precision (strings), CSV/XLSX formula-injection guard, stable column
order, menu-target isolation, streamed/bounded memory, temp cleanup. PASS.

## Table import
CSV + restricted **INSERT-only** SQL. Verified: upload → preview → mapping/
validation → execute; unquoted empty field → NULL; quoted empty string preserved;
atomic all-or-nothing (invalid row/duplicate key rolls the whole import back);
file-change re-hash rejection; size cap; target binding; no arbitrary SQL execution.
PASS. XLSX import is **unavailable** (as documented).

## Database export
PG/MySQL/SQLite, base tables only (views/sequences/triggers/functions/extensions
excluded and named in the artifact header); dependency-free ordering; dialect SQL
escaping; BIGINT/DECIMAL exactness; no cross-database leakage. PASS.

## Database restore
PostgreSQL + SQLite only; artifact manifest bound to engine+database; statement
allowlist; **empty target required** (non-empty → 409); single transaction; verified
absence. MySQL restore **disabled** (`RESTORE_UNSUPPORTED`). Round-trip tested for
SQLite (unit) and PostgreSQL (E2E: export α → empty → restore → data intact). PASS.

## Limits confirmed
Table import 64 MiB upload cap; DB restore 256 MiB artifact cap; XLSX import off.
No incomplete export is labelled a full backup (documented as base-tables-only).
