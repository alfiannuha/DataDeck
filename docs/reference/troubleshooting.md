# Troubleshooting

Common problems and verified resolutions for DataDeck v0.1.0.

## The app will not start

- **Missing/invalid `ENCRYPTION_KEY`** — startup aborts by design. Set a key of
  exactly 32 bytes or 64 hex characters:
  `export ENCRYPTION_KEY="$(openssl rand -hex 32)"`.
- **Port in use** — the daemon fails to bind. Set `PORT` to a free port.
- **Storage path not writable** — point `STORAGE_PATH` at a writable directory
  (e.g. `~/.datadeck/datadeck.db`). The parent is created `0700`.

## Browser cannot connect

Confirm the daemon is running and open the exact `HOST:PORT` it logs (default
`http://127.0.0.1:8080`). The server binds **loopback only** unless `ALLOW_REMOTE=1`
is set; a non-loopback `HOST` without that flag is refused at startup.

## Credential decryption failure

If profiles fail to open after an environment change, you likely changed
`ENCRYPTION_KEY`. Restore the original key, or delete and recreate the affected
profiles. There is **no key rotation** in v0.1.0.

## Connection failures

| Symptom | Likely cause |
|---|---|
| `CONNECTION_ERROR` | Host/port unreachable, wrong credentials, or network/firewall |
| `DATABASE_REQUIRED` | Server-level PostgreSQL profile used without selecting a database |
| `DATABASE_NOT_FOUND` | Target database does not exist |
| `DATABASE_CONNECT_DENIED` | Credentials lack `CONNECT` on the database |
| SQLite permission error | The process cannot read/write the database file (check ownership/path) |

## Query problems

- **`SQL_SYNTAX_ERROR`** — the message includes a position where available.
- **`QUERY_TIMEOUT`** — the query exceeded its timeout; narrow it or add indexes.
- **`QUERY_CANCELED`** — the query was cancelled.
- **Connection lost** — the pool is evicted and the next request reconnects.

## Table / data problems

- **`TABLE_NOT_FOUND` / `COLUMN_NOT_FOUND`** — the relation or column is not in the
  current metadata; **Refresh** the explorer.
- **`ROW_CONFLICT`** — the row changed since it was loaded; reload before retrying.
- **`ROW_NOT_FOUND`** — the row was deleted elsewhere.
- **`ROW_NOT_MUTABLE` / `ROW_IDENTITY_REQUIRED`** — the object is read-only or has no
  declared primary key; Edit/Delete is disabled.
- **`CONSTRAINT_VIOLATION`** — a NOT NULL/UNIQUE/FK/CHECK rule rejected the change.

## Import / export problems

- **`IMPORT_FORMAT_UNSUPPORTED`** — wrong format, or an SQL script that is not a
  plain INSERT-only batch for the target table.
- **`IMPORT_VALIDATION_ERROR`** — parse/convert/mapping failure, or the uploaded file
  changed after upload.
- **`IMPORT_TOO_LARGE`** — the upload exceeded the size cap.
- **`EXPORT_FORMAT_UNSUPPORTED` / `EXPORT_LIMIT_EXCEEDED`** — the format is not
  applicable (e.g. SQL on a view) or the export exceeds the safety limit.
- **Restore rejected** — `RESTORE_FORMAT_UNSUPPORTED` (not a DataDeck artifact),
  `RESTORE_TARGET_MISMATCH` (engine/database mismatch), `RESTORE_CONFLICT` (target
  not empty), `RESTORE_UNSUPPORTED` (MySQL).

## Delete Database blocked

- **`DATABASE_DELETE_PROTECTED`** — a protected system database.
- **`DATABASE_IN_USE`** — active operations use the database; wait and retry.
- **`DATABASE_DELETE_CONFIRMATION_INVALID` / `_TOKEN_EXPIRED`** — re-run the
  preflight and confirm again.
- **`DATABASE_DELETE_OUTCOME_UNKNOWN`** — the result could not be verified; check the
  server before retrying. DataDeck never retries automatically.
- **`DATABASE_DELETE_UNSUPPORTED`** — delete is disabled for this engine (SQLite).

## Reconnect

- **`CONNECTION_TIMEOUT` / `CONNECTION_ERROR`** — the replacement pool could not be
  established; the previous pool is kept (fail-closed). Retry after checking
  connectivity. Reconnect never replays a query or mutation.
