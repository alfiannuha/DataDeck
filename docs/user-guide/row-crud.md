# Row CRUD

Safe single-row editing from a **Table Data** tab. Every mutation targets exactly
one row and is verified by the backend.

## Identity rules (v0.1.0)

- **Edit** and **Delete** require a **declared PRIMARY KEY**.
- **Composite primary keys** are supported (all PK columns form the identity).
- **Unique-only** identity is **not** sufficient for Edit/Delete.
- Tables **without a primary key** may support **Insert** and **Duplicate**, but
  Edit/Delete are disabled.
- No implicit identity (SQLite `rowid`, PostgreSQL `ctid`, or row position) is ever
  used.
- **Generated / identity** columns are protected; the primary key cannot be edited.

## Add Row

**Add Row** opens a form with the table's insertable columns. For each column you
choose a mode:

- **VALUE** — provide a value (parameter-bound).
- **NULL** — explicit SQL `NULL`.
- **DEFAULT** — let the database apply the column default.

Generated/identity columns are excluded or read-only. Cancel does not mutate.

## Edit Row

The row menu **Edit Row** opens a form with the current values. Only changed fields
are sent; generated/read-only and primary-key columns cannot be changed. The
backend verifies the update **affected exactly one row**.

## Delete Row

**Delete Row** asks for explicit confirmation showing the row's identity. The
backend deletes by the resolved identity and verifies **one** row was affected.

## Duplicate Row

**Duplicate Row** opens the **Add Row** form pre-filled from the existing row,
omitting identity/generated columns. It does **not** insert immediately — you
review/edit and confirm.

## Optimistic concurrency

Update and Delete carry an **original-value snapshot**. If the row changed (or was
deleted) since it was loaded, the backend reports a conflict and the UI prompts you
to reload rather than overwriting newer data.

## Affected-row safety

The backend enforces `0` / `1` / `>1` affected-rows:

- `0` → not found / conflict (`ROW_NOT_FOUND` / `ROW_CONFLICT`).
- `1` → success.
- `>1` → safety abort (`MUTATION_AFFECTED_MULTIPLE_ROWS`) with rollback.

There is no `LIMIT 1`-style workaround.

## Copy actions

The row menu also offers **Copy Row** and **Copy as JSON** (clipboard only, no
database write).

## Errors

`ROW_NOT_MUTABLE` (read-only view/no key), `ROW_IDENTITY_REQUIRED`,
`COLUMN_READ_ONLY`, `INVALID_COLUMN_VALUE`, `CONSTRAINT_VIOLATION`,
`ROW_CONFLICT`, `ROW_NOT_FOUND`, `MUTATION_AFFECTED_MULTIPLE_ROWS`.

## Not a spreadsheet

There is no inline autosave and no per-keystroke write. Each mutation is an
explicit, confirmed action.
