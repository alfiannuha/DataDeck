# Table Data

Double-click a table in the Schema Explorer to open a **Table Data** tab. Browsing
is entirely server-side and requires no SQL.

## Viewing records

- Rows load one page at a time (server-side pagination); only the current page is
  held in the browser.
- Columns and types come from the backend's canonical metadata.
- `NULL` values are shown distinctly from empty strings.
- **BIGINT / DECIMAL** values are transferred as strings to avoid precision loss.

## Toolbar

**Add Row · Filter · Sort · Refresh · Import · Export** (Add Row/Import are hidden
or disabled for read-only objects).

## Pagination

Navigate the current page window. Page size is bounded; large tables never fetch
the full dataset into memory.

## Sorting

Open **Sort** and choose a column and direction (`asc`/`desc`). Sorting is applied
by the database; navigation and filters are preserved. Sorting uses a deterministic
tie-breaker so paging is stable.

## Filtering

Open **Filter** and add one or more **column / operator / value** conditions:

`equals` · `not_equals` · `contains` · `starts_with` · `ends_with` ·
`greater_than` · `greater_or_equal` · `less_than` · `less_or_equal` ·
`is_null` · `is_not_null` · `in`.

- Values are typed and **bound as parameters** (never concatenated).
- Filters run **before** pagination and sorting, so paging reflects the filtered set.
- Changing filters resets to the first page.
- `LIKE` wildcards in user input are escaped literally.

## Refresh

**Refresh** re-fetches the current page for the tab's exact binding.

## Read-only tables and views

Views/materialized views/foreign tables are browsable but row Edit/Delete is
disabled, and import is blocked. Row **Insert** may be offered where the backend
reports it is supported.

## Not a spreadsheet

Table Data is a browse-and-edit tab with explicit actions. There is **no
spreadsheet-style autosave**; a database write happens only when you confirm a
dialog.

## Errors

Missing tables, permission errors, timeouts, and invalid filters surface the
backend code and message (e.g. `TABLE_NOT_FOUND`, `COLUMN_NOT_FOUND`,
`INVALID_FILTER`).
