# DataDeck v0.1.0 — Import / Export Security

## Table import (`internal/tableimport`, handler)
- **CSV** (RFC 4180) and **restricted SQL** only; XLSX import is deferred.
- SQL import is **INSERT-only**: a dialect-aware tokenizer/parser rejects DROP,
  DELETE, UPDATE, ALTER, TRUNCATE, CREATE, cross-table scripts, non-literal values,
  and trailing unparsed content. Never regex/semicolon-split validation.
- Upload is **size-capped** (64 MiB) and spooled to a private temp file; the file is
  bound to the target and its **SHA-256** re-checked at execute.
- Preview/validation happens **before** any mutation; execute is disabled while errors
  exist.
- Execution is **one parameterized, all-or-nothing transaction**; a constraint
  failure rolls the whole import back.
- No arbitrary SQL execution path.

## Table export (`internal/export`)
- CSV/XLSX **formula-injection guard**; SQL uses dialect-quoted literals and the
  same value encoder (NULL, Unicode, BIGINT/DECIMAL as strings, JSON, binary).
- Streams with bounded memory; spooled to a temp file, removed after serving.
- Export is bound to the explicit table; no cross-database scope expansion.

## Database export / restore (`internal/dbscript`)
- Export: dialect SQL for **base tables only** (views/sequences/triggers/functions/
  extensions excluded and named in the header). No external tooling.
- Restore accepts **only DataDeck artifacts**: manifest binds format version +
  engine + database to the explicit target; statements validated against an
  allowlist (`CREATE TABLE`, `CREATE INDEX`, `ALTER TABLE … ADD CONSTRAINT`,
  `INSERT`, transaction control); `CREATE TABLE … AS SELECT` and cross-database /
  dangerous statements rejected.
- Restore requires an **empty** target (preflight) — no DROP/TRUNCATE/REPLACE.
- Applied in one transaction (PostgreSQL/SQLite); MySQL restore disabled.

## File upload hardening
| Test | Result |
|---|---|
| Malformed CSV / SQL / non-DataDeck artifact | typed 400 (`IMPORT_*`/`RESTORE_FORMAT_UNSUPPORTED`) |
| Oversized upload | `413 IMPORT_TOO_LARGE` |
| SQL injection / dangerous statements | rejected by allowlist/parser |
| Changed file after upload | `IMPORT_VALIDATION_ERROR` |
| Temp files | removed on success, failure, and TTL sweep |
| XLSX ZIP-bomb / symlink fixtures | **NOT RUN** (reader limits + size cap; LOW) |

Unit/integration evidence: `internal/tableimport/*_test.go`,
`internal/dbscript/dbscript_test.go`, `internal/api/handler/table_import_test.go`,
`database_script_test.go`, `database_delete_mysql_integration_test.go`.
