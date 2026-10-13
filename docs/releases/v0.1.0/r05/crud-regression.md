# DataDeck v0.1.0 — R05 CRUD Regression

Verified by unit/integration suites (`mutation_unit_test.go`, `table_rows_test.go`,
`insert.go`/`mutation.go` tests, T07–T11 integration) — re-run at `e81ca8e`.

## Insert
Valid row, required-field validation, `NULL`, `DEFAULT`, generated-column exclusion,
duplicate-PK rejection (`CONSTRAINT_VIOLATION`), BIGINT/DECIMAL string precision —
all PASS. No-PK tables allow Insert.

## Update
Single PK, composite PK, changed-row conflict (`ROW_CONFLICT`), deleted-row conflict
(`ROW_NOT_FOUND`), snapshot mismatch, generated-column protection, PK edit blocked —
PASS. Changed-fields-only; parameterized.

## Delete
Valid deletion with explicit confirmation showing identity; changed-row conflict;
missing row; no-PK blocked (`ROW_IDENTITY_REQUIRED`); multi-row safety
(`MUTATION_AFFECTED_MULTIPLE_ROWS` + rollback) — PASS.

## Duplicate
Opens the Add-Row form prefilled; generated/identity omitted; no immediate insert;
explicit confirmation — PASS (unit + component tests).

## Identity & concurrency (v0.1.0 policy)
- Edit/Delete require a **declared PRIMARY KEY** (composite supported); unique-only
  and no-PK are not Edit/Delete targets; no rowid/ctid/row-index identity.
- Original snapshot mandatory; optimistic concurrency enforced.
- Affected-rows: `0` → conflict/not-found; `1` → success; `>1` → rollback.
- No inline autosave / spreadsheet-style writes.

**No wrong-row mutation or silent overwrite observed.**
