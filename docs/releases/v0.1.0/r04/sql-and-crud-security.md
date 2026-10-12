# DataDeck v0.1.0 — SQL Injection & CRUD Security

## Structured SQL (no frontend SQL)
Table browse, sort, filter, CRUD, structure, and export build SQL from
**introspection metadata**:
- Identifiers are validated against metadata and **dialect-quoted** (`quoteIdentifier`).
- Values are **bound parameters**; filter operators and sort directions use allowlists.
- Pagination is bounded; database/schema/table are explicit.

Adversarial identifiers/values (`users"; DROP TABLE users; --`,
`name' OR '1'='1`, `public.users; DELETE FROM audit_log`, `%_\`, `NULL`,
`DEFAULT`) cannot escape the structured boundary — covered by
`mutation_unit_test.go`, `table_filter` tests, and `table_rows_test.go`. LIKE
wildcards in user input are escaped.

## Row identity (v0.1.0 policy)
- Edit/Delete require a **declared PRIMARY KEY**; composite PK supported.
- Unique-only identity is **insufficient**; no-PK tables are Insert/Duplicate only.
- No row-index/OFFSET, SQLite `rowid`, or PostgreSQL `ctid` identity.
- Original snapshot is mandatory; generated columns are protected; PK editing is disabled.

## Optimistic concurrency & affected-rows
`mutation.go`: `0` → `ROW_NOT_FOUND`/`ROW_CONFLICT`; `1` → success; `>1` →
`MUTATION_AFFECTED_MULTIPLE_ROWS` + rollback. No `LIMIT 1` workaround. Tested with
stale rows, composite PKs, NULL transitions, and BIGINT/DECIMAL (string-preserved).

## Raw SQL
The editor executes arbitrary SQL with the user's credentials — intentional and
documented. Database permissions remain the authorization boundary; DataDeck adds
a timeout, cancellation, and sanitized errors but does **not** attempt to sandbox
raw SQL (no frontend keyword filtering as a security control).

## Transactions
- Row mutations: per-statement transaction with affected-row verification.
- Table import: one all-or-nothing transaction.
- DB restore: one transaction on PostgreSQL/SQLite; **MySQL restore disabled**
  (DDL implicitly commits — no atomicity claim).

## Logging
Errors and logs are sanitized: no SQL parameter values, DSNs, passwords, or keys.
