# Table Import / Export

Table-level import and export operate on **one relation** at a time, bound to the
explicit connection/database/schema/table the action was invoked on.

## Export

Open **Export** from the table context menu or the Table Data toolbar. Targets:

| Format | Notes |
|---|---|
| **CSV** | Streams inline. Formula-injection guard prefixes `'` to cells/headers starting `= + @ -` (plain optional numbers are left intact). |
| **XLSX** | Single typed sheet; dangerous leading characters stay literal text. |
| **SQL** | One `INSERT` per row, dialect-quoted; optional transaction wrapper; base tables only. |

- Exports the **whole table** (or the current filter set), not just the visible page.
- Values are exact: `NULL`, empty string, Unicode, BIGINT/DECIMAL (as strings),
  boolean, temporal, JSON, binary.
- Streamed with bounded memory; a partial/failed export returns a JSON error, not a
  truncated file.
- The download name is `datadeck_<db>_<schema>_<table>_<date>.<ext>` (sanitized).

## Import

Open **Import** from the table context menu. Supported inputs:

- **CSV** (RFC 4180; optional header row). Unquoted empty fields become `NULL`; a
  quoted empty string stays an empty string.
- **Restricted SQL** — a plain batch of `INSERT` statements for the bound table.
  DDL/DML of any other kind and cross-table scripts are **rejected**.

Flow: **upload → preview → mapping/validation → execute**.

- The file is bound to the target (resolved database + schema/table + SHA-256). If
  the file changes after upload, it is rejected.
- The preview shows parsed rows and validation errors; the execute step is disabled
  while there are errors.
- Execution is **one all-or-nothing transaction** of parameterized INSERTs. A
  constraint failure rolls the entire import back — no partial import.
- Generated columns are excluded; values are converted against the target types.

> Table Import is **not** a general SQL script runner. It only replays value-level
> `INSERT`s into the bound table. Use
> [database import](database-management.md#import-sql-restore) for database-level
> scripts (PostgreSQL/SQLite only).

## Limits

- Upload size is capped (64 MiB for table import).
- The preview is bounded (a limited number of rows).
- XLSX **import** is not available in v0.1.0 (export only).

## Errors

`IMPORT_FORMAT_UNSUPPORTED`, `IMPORT_VALIDATION_ERROR`, `IMPORT_TOO_LARGE`,
`CONSTRAINT_VIOLATION` (whole import rolled back), `EXPORT_FORMAT_UNSUPPORTED`,
`EXPORT_LIMIT_EXCEEDED`.
