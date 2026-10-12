# DataDeck — API Contract

> Derived from `PRD.md` §6 and §3. The standard envelope is fixed by the PRD;
> anything not specified there is marked **Needs Validation** or **Recommended**.
> Base path: `/api/v1`. Content type: `application/json`.

---

## 1. Standard Envelope

Every JSON response uses:

```json
{
  "success": true,
  "data": {},
  "error": null,
  "meta": {}
}
```

| Field | Type | Required | Meaning |
|---|---|---|---|
| `success` | boolean | yes | `true` iff the operation completed without a declared error |
| `data` | object \| array \| null | yes | Payload on success; `null` on failure |
| `error` | object \| null | yes | `null` on success; populated on failure |
| `meta` | object | no | Non-payload metadata (see §7) |

The PRD error example omits `meta`; treat `meta` as optional on error responses.
**Needs Validation:** whether `error.meta` must always be present.

---

## 2. Success Response Conventions

- HTTP status is `200 OK` (or `201 Created` for resource creation — see
  **Needs Validation** §10).
- `success: true`, `error: null`, `data` contains the payload.
- `data` for collection endpoints is an array; for single resources an object.
- Never include credentials or encrypted secret material in `data`. Password
  fields on connection profiles are masked.

Example — `GET /api/v1/connections`:

```json
{
  "success": true,
  "data": [
    {
      "id": "9f1c7d2e-4a6b-4e89-b88a-d5f356bf7312",
      "name": "Production",
      "driver": "postgres",
      "host": "db.internal",
      "port": 5432,
      "database_name": "app",
      "username": "readonly",
      "ssl_mode": "require",
      "ssh_enabled": false,
      "created_at": "2026-09-25T14:32:00Z",
      "updated_at": "2026-09-25T14:32:00Z"
    }
  ],
  "error": null,
  "meta": {}
}
```

Note the absence of `encrypted_password` / `ssh_encrypted_password` — secrets are
never returned.

---

## 3. Error Response Conventions

```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "SQL_SYNTAX_ERROR",
    "message": "syntax error at or near \"WHERRE\"",
    "position": 62
  }
}
```

| Field | Type | Required | Meaning |
|---|---|---|---|
| `code` | string | yes | Stable machine-readable error code (table below) |
| `message` | string | yes | Human-readable, sanitized message |
| `position` | int | no | Character offset for SQL errors when the driver provides it |

Rules:

- `message` must be safe for display: no stack traces, no secrets, no connection
  strings with passwords.
- Drivers' raw errors may be wrapped but not forwarded verbatim if they contain
  credentials.
- Error codes are stable contracts; the frontend may branch on them.

### Error Code Table (superset of PRD examples)

| Code | HTTP | Kind | Notes |
|---|---|---|---|
| `VALIDATION_ERROR` | 400 | Validation | Body/field errors; include field details (**Needs Validation** on shape) |
| `SQL_SYNTAX_ERROR` | 400 | SQL | PRD example; includes `position` when available |
| `SQL_ERROR` | 400/500 | SQL | Generic statement failure (**Needs Validation** exact status split) |
| `QUERY_TIMEOUT` | 504 | Timeout | Context deadline exceeded |
| `QUERY_CANCELED` | 499 | Canceled | Query context canceled before completion |
| `CONNECTION_ERROR` | 502 | Connection | Dial/ping failure to target DB |
| `CONNECTION_TIMEOUT` | 504 | Timeout | PRF-02/T16: a database reconnect exceeded its deadline |
| `INTROSPECTION_ERROR` | 502 | Introspection | Target-side schema read failed (unreachable or restricted) |
| `INTROSPECTION_TIMEOUT` | 504 | Timeout | Schema introspection exceeded its deadline |
| `NOT_IMPLEMENTED` | 501 | Capability | Operation not implemented for this driver yet |
| `NOT_FOUND` | 404 | Resource | Unknown connection id / resource |
| `TABLE_NOT_FOUND` | 404 | Resource | PRF-02: Table Data target not found in metadata |
| `COLUMN_NOT_FOUND` | 400 | Validation | PRF-02: filter/sort/projection column not in metadata |
| `INVALID_FILTER` | 400 | Validation | PRF-02: malformed or type-incompatible filter |
| `MUTATION_AFFECTED_MULTIPLE_ROWS` | 500 | Safety | PRF-02: mutation matched >1 row; rolled back |
| `ROW_NOT_MUTABLE` | 400 | Safety | PRF-02: view/matview/foreign table is read-only |
| `ROW_IDENTITY_REQUIRED` | 400 | Safety | PRF-02: no primary key; Update/Delete disabled |
| `ROW_IDENTITY_INVALID` | 400 | Validation | PRF-02: identity keys/values invalid |
| `ROW_NOT_FOUND` | 404 | Resource | PRF-02: identity no longer identifies a row |
| `ROW_CONFLICT` | 409 | Concurrency | PRF-02: expected values no longer match |
| `COLUMN_READ_ONLY` | 400 | Safety | PRF-02: PK/identity/generated column cannot be modified |
| `INVALID_COLUMN_VALUE` | 400 | Validation | PRF-02: value invalid for its column (type/NULL/unknown column) |
| `CONSTRAINT_VIOLATION` | 409 | Constraint | PRF-02: NOT NULL/UNIQUE/FK/CHECK rejection |
| `EXPORT_FORMAT_UNSUPPORTED` | 400 | Validation | PRF-02 table export: format not supported by the driver, or no exportable columns |
| `EXPORT_LIMIT_EXCEEDED` | 400 | Safety | PRF-02 table export: result would exceed the export row/byte safety limit |
| `IMPORT_FORMAT_UNSUPPORTED` | 400 | Validation | PRF-02 table import: format not supported (XLSX deferred), or SQL script not a plain INSERT-only batch |
| `IMPORT_VALIDATION_ERROR` | 400 | Validation | PRF-02 table import: parse/convert/mapping failure, or the file changed after upload |
| `IMPORT_TOO_LARGE` | 413 | Validation | PRF-02 table import: upload exceeds the import size cap |
| `RESTORE_FORMAT_UNSUPPORTED` | 400 | Validation | PRF-02/T17 restore: file is not a DataDeck SQL export artifact |
| `RESTORE_TARGET_MISMATCH` | 400 | Validation | PRF-02/T17 restore: artifact engine/database does not match the explicit target |
| `RESTORE_UNSUPPORTED_STATEMENT` | 400 | Safety | PRF-02/T17 restore: a statement is outside the allowlist |
| `RESTORE_CONFLICT` | 409 | Safety | PRF-02/T17 restore: the target database is not empty |
| `RESTORE_UNSUPPORTED` | 501 | Capability | PRF-02/T17 restore: engine cannot guarantee atomic restore (MySQL) |
| `RESTORE_ERROR` | 400 | Execution | PRF-02/T17 restore: a validated statement failed (sanitized driver message) |
| `DATABASE_DELETE_UNSUPPORTED` | 501 | Capability | PRF-02/T18: engine has no safe DROP DATABASE (SQLite) |
| `DATABASE_DELETE_PROTECTED` | 400 | Safety | PRF-02/T18: target is a protected system/internal database |
| `DATABASE_DELETE_PRECHECK_FAILED` | 502 | Safety | PRF-02/T18: no maintenance connection or the target cannot be reached |
| `DATABASE_IN_USE` | 409 | Safety | PRF-02/T18: the target has active operations |
| `DATABASE_DELETE_CONFIRMATION_INVALID` | 400 | Safety | PRF-02/T18: token/typed name/acknowledgment invalid |
| `DATABASE_DELETE_TOKEN_EXPIRED` | 400 | Safety | PRF-02/T18: confirmation token expired |
| `DATABASE_DELETE_CONFLICT` | 409 | Safety | PRF-02/T18: concurrent delete, or the database still exists after DROP |
| `DATABASE_DELETE_FAILED` | 400 | Execution | PRF-02/T18: DROP DATABASE was rejected (sanitized driver message) |
| `DATABASE_DELETE_OUTCOME_UNKNOWN` | 502 | Safety | PRF-02/T18: deletion could not be verified; verify state before retrying |
| `PAYLOAD_TOO_LARGE` | 413 | Validation | Request body exceeds configured limit (**Recommended**) |
| `INTERNAL_ERROR` | 500 | Internal | Sanitized catch-all |

`SQL_SYNTAX_ERROR` and the envelope shape are fixed by PRD; the remaining codes
are **Recommended** conventions to keep the frontend handling uniform. Confirm
before treating as immutable.

---

## 4. HTTP Status Usage

| Status | Used for |
|---|---|
| `200 OK` | Successful reads and successful query execution |
| `201 Created` | Resource creation (e.g., new profile/snippet) — **Needs Validation** |
| `400 Bad Request` | Validation errors and SQL syntax errors (PRD) |
| `404 Not Found` | Unknown resource id — **Recommended** |
| `413 Payload Too Large` | Oversized request body — **Recommended** |
| `500 Internal Server Error` | Internal failures and some SQL errors (PRD) |
| `502 Bad Gateway` | Target DB connection failure — **Recommended** |
| `504 Gateway Timeout` | Query timeout — **Recommended** |

The PRD explicitly shows only `200`, `400`, and `500`. Additional statuses are
compatible extensions and are marked **Recommended**/**Needs Validation**.

---

## 5. Error Categories

### 5.1 Validation Errors

- Missing/invalid fields, bad types, unknown driver, out-of-range timeout.
- Return `400` with `VALIDATION_ERROR`.
- Must not echo back sensitive input.

### 5.2 Internal Errors

- Panics/unexpected failures caught by the panic-recovery middleware.
- Return `500` with `INTERNAL_ERROR` and a generic message; log the real cause
  server-side with sanitization (see `security.md`).

### 5.3 SQL Errors

- Statement rejected by the target DB.
- Return the database message and `position` when available.
- PRD example code: `SQL_SYNTAX_ERROR`.

### 5.4 Timeout Errors

- Query exceeded its context deadline (default 30s, PRD §11.3).
- Return `504` with `QUERY_TIMEOUT`. **Needs Validation:** PRD does not fix the
  HTTP status for timeouts.

### 5.5 Connection Errors

- Target DB unreachable, auth failed, or 5s health ping failed.
- Return `502` with `CONNECTION_ERROR`.
- Never include the password in the message.

### 5.6 Database Selection Errors (PRF-01)

- `DATABASE_REQUIRED` (`400`) — a PostgreSQL server-level profile is used without
  an explicit or default database. The request is rejected rather than silently
  choosing a database; use `GET /connections/{id}/databases` to pick one.
- `DATABASE_NOT_FOUND` (`400`) — the requested PostgreSQL database does not exist
  (SQLSTATE `3D000`).
- `DATABASE_CONNECT_DENIED` (`400`) — the credentials lack `CONNECT` on the
  requested database (SQLSTATE `42501`).
- `BOOTSTRAP_DATABASE_UNAVAILABLE` (`502`) — no bootstrap/default database could
  be reached for discovery.
- `DISCOVERY_TIMEOUT` (`504`) — database discovery exceeded its deadline.
- MySQL/SQLite are single-database: a `database` value that differs from the
  profile database is rejected with `400 VALIDATION_ERROR`.

---

## 6. Query Execution Contract

**POST** `/api/v1/query/execute`

Request:

```json
{
  "connection_id": "9f1c7d2e-4a6b-4e89-b88a-d5f356bf7312",
  "sql": "SELECT id, full_name FROM users LIMIT 50;",
  "timeout_seconds": 30
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `connection_id` | string | yes | Existing profile id |
| `sql` | string | yes | Raw SQL, executed as provided (single statement) |
| `database` | string | no | PRF-01: target database on a server-level PostgreSQL profile. Precedence: request `database` → profile default; if neither exists the request is rejected with `DATABASE_REQUIRED`. Never used to redirect an existing tab — the client sends the tab's bound database. Ignored for MySQL/SQLite unless it differs from the profile database (then `VALIDATION_ERROR`). |
| `timeout_seconds` | int | no | Defaults to 30 (PRD §11.3); server clamps to a 300s maximum |

Success data (`200`):

```json
{
  "columns": [{ "name": "id", "type": "INT8" }],
  "rows": [[101]],
  "rows_affected": 2,
  "execution_time_ms": 12,
  "truncated": false
}
```

| Field | Type | Notes |
|---|---|---|
| `columns` | array<{name,type}> | Driver-reported names/types |
| `rows` | array<array<any>> | Row-major, positional arrays aligned to `columns` |
| `rows_affected` | int | Affected/returned count |
| `execution_time_ms` | int | Measured backend duration |
| `truncated` | boolean | `true` when the 50 MB payload cap was hit |

Serialization rules (PRD §11.4):

- 64-bit integers (`BIGINT`) are serialized as **strings** to preserve precision
  beyond JS `Number.MAX_SAFE_INTEGER`.
- `NULL` → JSON `null`.
- `NUMERIC`/`DECIMAL`/`MONEY` → strings (precision-preserving).
- `UUID` → canonical string; `DATE` → `YYYY-MM-DD`; `TIMESTAMP(TZ)` →
  RFC 3339 string.
- `JSON`/`JSONB` → embedded JSON (not a string).
- `BYTEA` → base64 string (deliberately distinct from text columns).
- MySQL `BIT` → base64 string and `TINYINT(1)` → number (`0`/`1`); no boolean
  coercion is applied because it could mask legitimate numeric data.
- SQLite is dynamically typed, so `columns[].type` reflects the declared type and
  may be empty for expressions/weakly-typed columns. (Deferred ambiguity; no
  value inference is performed.)
- Non-row statements return `columns: []`, `rows: []`, and `rows_affected` from
  the command tag. Result accumulation stops at the 50 MB cap (measured as the
  JSON-encoded row payload) and sets `truncated: true`.

`GET /api/v1/query/history` returns the audit log newest-first; an optional
`connection_id` query parameter filters by connection. Each record carries an
optional `database_name` (PRF-01: the database the query executed against; null
for legacy rows and implicit-database engines). Unspecified paging is
**Needs Validation.**

---

## 6.1 Connection Profile Contract (implemented in M1-T06)

Request body for `POST /api/v1/connections` and `POST /api/v1/connections/test`:

```json
{
  "name": "Local PG",
  "driver": "postgres",
  "host": "127.0.0.1",
  "port": 5432,
  "database_name": "app",
  "username": "appuser",
  "password": "secret",
  "ssl_mode": "disable"
}
```

- `name` is required for create, ignored/optional for `/test`.
- `driver` is one of `postgres`, `mysql`, `sqlite`.
- Saved queries accept an optional `database_name` (PRF-01) recording the
  intended database context; it is returned on read and restored when opening a
  snippet. A server-level PostgreSQL snippet without `database_name` stays
  unbound and prompts for a database rather than inheriting the current one.
- `postgres`/`mysql` require `host` and `username`; `port` defaults to `5432`
  (PostgreSQL) or `3306` (MySQL).
- `database_name` is **required** for `mysql` and `sqlite`, and **optional** for
  `postgres`: a PostgreSQL profile represents a server/instance and the value is
  an initial/default database (PRF-01). When omitted, the database is selected
  after connecting; a query is not silently bound to any database.
- `sqlite` requires `database_name` as a **file path** and takes no
  host/port/username/password; `ssl_mode` is ignored.
- `ssl_mode` defaults to `disable` and must be one of
  `disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full`.
- `password` is never persisted in plaintext, never stored unencrypted, and
  never returned.

Response `data` for `GET /api/v1/connections`, `POST /api/v1/connections`, and
`POST /api/v1/connections/test`:

```json
{
  "id": "9f1c7d2e4a6b4e89b88ad5f356bf7312",
  "name": "Local PG",
  "driver": "postgres",
  "host": "127.0.0.1",
  "port": 5432,
  "database_name": "app",
  "username": "appuser",
  "ssl_mode": "disable",
  "created_at": "2026-09-25T14:32:00Z",
  "updated_at": "2026-09-25T14:32:00Z"
}
```

- `GET /api/v1/connections` returns an array (empty `[]` when none exist).
- `POST /api/v1/connections` returns the sanitized profile with `200 OK`
  (**Needs Validation:** the contract's `201 Created` remains a candidate).
- `POST /api/v1/connections/test` returns `data: { "status": "ok" }` on success
  and `502 CONNECTION_ERROR` on failure; it never persists a profile.
- `DELETE /api/v1/connections/{id}` closes any active pool, deletes the profile,
  and returns `data: { "id": "<id>" }`; unknown ids return `404 NOT_FOUND`.

---

## 6.2 Request Body & Content-Type Policy

All JSON endpoints enforce a consistent request policy:

- `Content-Type` **MUST** be `application/json` (parameters such as `charset`
  are accepted). Other types — including absent — are rejected with
  `400 VALIDATION_ERROR`. This blocks cross-origin "simple request" CSRF, which
  cannot set that content type without a preflight.
- Request bodies are capped at **1 MiB**; an oversized body yields
  `400 VALIDATION_ERROR` with an explicit size message (the envelope has no
  separate payload-too-large code).
- Exactly one JSON value is accepted; trailing values or garbage are rejected.
- Unknown fields are ignored (forward-compatible); wrong types are rejected.

## 7. `meta` and Pagination

Pagination is implemented for the two collection endpoints
(`GET /api/v1/query/history` and `GET /api/v1/queries/saved`) using a single,
consistent contract.

**Query parameters:** `page` (1-based, default `1`, maximum `1,000,000`) and
`page_size` (default `50`, clamped to a maximum of `200`). Invalid values are
rejected with `400 VALIDATION_ERROR`. Ordering is deterministic (history:
`executed_at DESC, id DESC`; saved queries: `updated_at DESC, id DESC`).

**`meta` shape:**

```json
{
  "meta": {
    "page": 1,
    "page_size": 50,
    "total": 128,
    "total_pages": 3
  }
}
```

`data` remains the array of items for the requested page. The same shape is used
by both endpoints.

---

## 8. Request ID Handling

The PRD does **not** mention request IDs. There is no requirement for a
correlation identifier.

**Recommended (not an immutable decision):** accept an optional client-provided
`X-Request-ID` header; generate a UUID when absent; echo it back in an
`X-Request-ID` response header and optionally in `meta.request_id`; include it in
server logs for correlation. Mark as **Needs Validation** before relying on it.

---

## 9. Endpoints Specified by the PRD

These are the only endpoints defined in PRD §6.1. Do not add endpoints not listed
here without an explicit requirement.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/v1/health` | Daemon health status |
| `GET` | `/api/v1/connections` | List profiles (passwords masked) |
| `POST` | `/api/v1/connections` | Persist a new connection profile |
| `POST` | `/api/v1/connections/test` | Verify connection parameters before saving |
| `DELETE` | `/api/v1/connections/{id}` | Terminate pool and remove the record |
| `POST` | `/api/v1/connections/{id}/reconnect` | Database-scoped reconnect: replace one `(connection, database)` pool with a fresh, health-checked one (PRF-02/T16) |
| `POST` | `/api/v1/connections/{id}/database-export` | Streamed dialect SQL dump of one database's base tables (PRF-02/T17) |
| `POST` | `/api/v1/connections/{id}/database-import` | Upload + preflight a database restore artifact (PRF-02/T17) |
| `POST` | `/api/v1/connections/{id}/database-import/{token}/execute` | Execute a preflighted database restore (PRF-02/T17) |
| `POST` | `/api/v1/connections/{id}/databases/{database}/delete-preflight` | Preflight a database deletion; returns a single-use confirmation token (PRF-02/T18) |
| `POST` | `/api/v1/connections/{id}/databases/{database}/delete` | Execute a database deletion after confirmation (PRF-02/T18) |
| `GET` | `/api/v1/connections/{id}/schemas` | Structural hierarchy (catalogs, tables, columns, relations); optional `database` query parameter (PRF-01) |
| `GET` | `/api/v1/connections/{id}/databases` | List selectable databases on a server-level connection (PRF-01; PostgreSQL only) |
| `GET` | `/api/v1/connections/{id}/table-data` | Bounded, paginated table/view rows (PRF-02; backend-generated SELECT) |
| `GET` | `/api/v1/connections/{id}/table-structure` | Read-only structure of one table/view (PRF-02/T13; columns, primary key, indexes, constraints, foreign keys) |
| `POST` | `/api/v1/connections/{id}/table-data/rows` | Single-row INSERT with VALUE/NULL/DEFAULT modes (PRF-02/T08) |
| `PATCH` | `/api/v1/connections/{id}/table-data/rows` | Single-row UPDATE by primary key with optimistic concurrency (PRF-02/T07) |
| `DELETE` | `/api/v1/connections/{id}/table-data/rows` | Single-row DELETE by primary key with optimistic concurrency (PRF-02/T07) |
| `POST` | `/api/v1/connections/{id}/table-data/export` | Streamed table/view export (CSV \| XLSX \| SQL) honoring filters/sort (PRF-02/T14) |
| `POST` | `/api/v1/query/execute` | Synchronous raw SQL execution |
| `GET` | `/api/v1/query/history` | Audit log of executed queries |
| `POST` | `/api/v1/queries/saved` | Save a query snippet (PRD) |
| `GET` | `/api/v1/queries/saved` | List saved queries (M3 extension; optional `connection_id` filter) |
| `GET` | `/api/v1/queries/saved/{id}` | Get a saved query (M3 extension) |
| `PUT` | `/api/v1/queries/saved/{id}` | Update a saved query (M3 extension) |
| `DELETE` | `/api/v1/queries/saved/{id}` | Delete a saved query (M3 extension) |

### Per-endpoint notes

- `GET /api/v1/health` — returns `success: true` with
  `data: { "status": "healthy", "version": "0.1.0", "channel": "development" }`
  and `error: null`. The `version`/`channel` fields (PRF02-R02) expose
  non-sensitive build metadata; `version` is the canonical application version
  and `channel` is `development` or `release`. Implemented in M1-T01; version
  metadata added in R02.
- `GET /api/v1/connections` — must mask/omit secrets.
- `POST /api/v1/connections` — encrypts `password` and `ssh_password` at rest;
  returns masked profile. Creation status (`200` vs `201`) is **Needs Validation**.
- `POST /api/v1/connections/test` — uses the same parameter shape as creation but
  does not persist; ephemeral 5s ping.
- `DELETE /api/v1/connections/{id}` — closes/evicts the pool, then deletes the
  profile (history cascades; saved-query references are set null).
- `POST /api/v1/connections/{id}/reconnect` — **PRF-02/T16**. Body:
  `{ database? }` (application/json; optional). Replaces the pool for the
  explicit target with a freshly opened, health-checked one. **Database-scoped:**
  on a server-level PostgreSQL profile only the `(connection, database)` pool is
  replaced; every other database pool on the same profile is left untouched, so
  reconnecting one database never disturbs another. `database` is required for a
  server-level PostgreSQL profile (`DATABASE_REQUIRED` otherwise); MySQL/SQLite
  ignore it unless it differs from the profile database (`VALIDATION_ERROR`).
  The saved profile and its **stored credentials are reused** — nothing is
  deleted and no credential re-entry is required. **No SQL is executed and no
  mutation is ever replayed**; a failed reconnect *fails closed* (the existing
  pool is kept). Success is returned only after the replacement connects and
  passes its health check. Response `data`: `{ status: "ok", database }`.
  Errors: `404 NOT_FOUND` (unknown profile), `400 DATABASE_REQUIRED` /
  `VALIDATION_ERROR`, `400 DATABASE_NOT_FOUND` / `DATABASE_CONNECT_DENIED`,
  `502 CONNECTION_ERROR`, `504 CONNECTION_TIMEOUT`, `499 QUERY_CANCELED`. The
  saved connection configuration is never modified.
- `POST /api/v1/connections/{id}/database-export` — **PRF-02/T17**. Body:
  `{ database, mode: "schema_data" | "schema" | "data" }`. Streams a
  dialect-correct SQL script for the **base tables** of the explicit database.
  **Self-contained:** no external `pg_dump`/`mysqldump`/`sqlite3` is invoked, so
  the single-binary, CGO-free distribution is preserved. Coverage is **base
  tables only** (columns/types/NOT NULL/DEFAULT/identity/generated, PRIMARY KEY,
  UNIQUE, CHECK, indexes, FOREIGN KEY) — views, sequences, triggers, functions
  and extensions are **excluded** and named in the artifact header. Ordering is
  dependency-free: tables are created without foreign keys, data is loaded, then
  indexes and foreign keys are added, so circular/self-referencing FKs need no
  graph sort. The artifact carries a `-- datadeck:export` manifest (format
  version, engine, database, mode, object counts). Response is
  `application/octet-stream` with `Content-Disposition: attachment` and
  `Cache-Control: no-store` (errors use the envelope). The target is the explicit
  connection + database; the Explorer/active tab is never used.
- `POST /api/v1/connections/{id}/database-import` — **PRF-02/T17** upload +
  preflight. Body: `multipart/form-data` with `file` (a DataDeck SQL export
  artifact) and `database`. Validates the manifest (format version, engine, and
  database must equal the explicit target) and every statement against the
  restore allowlist (`CREATE TABLE`, `CREATE [UNIQUE] INDEX`, `ALTER TABLE … ADD
  CONSTRAINT`, `INSERT`, transaction control), then **preflights** the target:
  it must be empty (no base tables). Nothing is written; the response is an
  executable preview bound to a token. **MySQL is rejected with
  `501 RESTORE_UNSUPPORTED`** (DDL implicitly commits, so no atomic restore).
  Errors: `400 RESTORE_FORMAT_UNSUPPORTED` / `RESTORE_TARGET_MISMATCH` /
  `RESTORE_UNSUPPORTED_STATEMENT`, `409 RESTORE_CONFLICT`, `404 NOT_FOUND`
  (unknown profile), `413 IMPORT_TOO_LARGE`.
- `POST /api/v1/connections/{id}/database-import/{token}/execute` — **PRF-02/T17**.
  Re-validates the spooled artifact and applies it to the explicit target in a
  **single all-or-nothing transaction** (PostgreSQL/SQLite). The SQL parser is
  restricted and dialect-aware (never a regex/semicolon split): the whole script
  is validated before any statement runs, `CREATE TABLE … AS SELECT` is rejected,
  and statements that could redirect the operation (CREATE/DROP DATABASE, USE,
  ATTACH, COPY, GRANT, …) are refused. Restore never replays a mutation and never
  redirects to another database. Response `data`:
  `{ status: "ok", database, statements }`. Errors: `400 RESTORE_ERROR`
  (sanitized), `404 NOT_FOUND` (unknown/expired token), `501 RESTORE_UNSUPPORTED`,
  `504 QUERY_TIMEOUT`.
- `POST /api/v1/connections/{id}/databases/{database}/delete-preflight` —
  **PRF-02/T18**. Validates the explicit target and returns a short-lived
  (5 min), single-use, cryptographically random token bound to the exact
  `connection + database + engine`. **Never runs DROP.** Supported engines:
  PostgreSQL, MySQL. **SQLite is rejected with `501 DATABASE_DELETE_UNSUPPORTED`**
  (a database is a local file; safe file deletion needs a separate design).
  Protected system databases are rejected: PostgreSQL `postgres`/`template0`/
  `template1`; MySQL `mysql`/`information_schema`/`performance_schema`/`sys`
  (`400 DATABASE_DELETE_PROTECTED`). The target is never the Explorer selection
  or the active tab.
- `POST /api/v1/connections/{id}/databases/{database}/delete` — **PRF-02/T18**.
  Body: `{ confirmation_token, typed_database_name, acknowledged }`. The typed
  name (case-sensitive exact match) and boolean are **intent signals only**; the
  backend re-validates the token binding, protection policy, database existence
  and active operations before running `DROP DATABASE <quoted>` (never
  `WITH (FORCE)`, never `CASCADE`). PostgreSQL runs DROP on a **maintenance
  connection** (never the target database). The target's managed pool is retired
  first; other databases' pools are untouched. Active operations block deletion
  (`409 DATABASE_IN_USE`) and are never force-terminated. **No automatic retry**
  after an ambiguous failure. Success is returned only after independently
  verifying the database is gone; if verification fails the outcome is reported
  as `502 DATABASE_DELETE_OUTCOME_UNKNOWN`. Response `data`:
  `{ status: "deleted", database }`. Errors: `400 DATABASE_DELETE_*`,
  `409 DATABASE_IN_USE`/`DATABASE_DELETE_CONFLICT`, `502 DATABASE_DELETE_PRECHECK_FAILED`/
  `DATABASE_DELETE_OUTCOME_UNKNOWN`, `504 QUERY_TIMEOUT`. Never deletes the
  connection profile, saved queries, or history.
- `GET /api/v1/connections/{id}/schemas` — implemented in M1-T07. Returns a
  database-neutral tree in `data` (array of databases → schemas → tables with
  columns, primary key, foreign keys, indexes). Activates/reuses the managed
  pool. Errors: unknown id → `404 NOT_FOUND`; unsupported driver →
  `400 VALIDATION_ERROR`; unreachable or permission-restricted catalog read →
  `502 INTROSPECTION_ERROR`; timeout → `504 INTROSPECTION_TIMEOUT`. Caching
  semantics remain **Needs Validation.** PRF-01: an optional `database` query
  parameter selects the database; omitted → profile default; a server-level
  profile without either yields `400 DATABASE_REQUIRED`.
- `GET /api/v1/connections/{id}/databases` — PRF-01. Returns lightweight
  metadata (`{name, bootstrap_candidate?}`) read from a single catalog query.
  Errors: unknown id → `404 NOT_FOUND`; MySQL/SQLite → `501 NOT_IMPLEMENTED`;
  no bootstrap database → `502 BOOTSTRAP_DATABASE_UNAVAILABLE`; timeout →
  `504 DISCOVERY_TIMEOUT`.
- `GET /api/v1/connections/{id}/table-data` — **PRF-02**. Query parameters:
  `database` (required for server-level PostgreSQL, else profile default),
  `schema` (required for PostgreSQL; ignored for MySQL/SQLite), `table`
  (required), `page` (default 1, max 1,000,000) and `page_size` (default 100,
  max 200, clamped), plus optional `sort_column` + `sort_direction`
  (`asc`|`desc`) for **single-column server-side sorting**, and optional
  `filters` (URL-encoded JSON array of `{column, operator, value|values}`;
  ANDed; max 20). Operators: `equals, not_equals, contains, starts_with,
  ends_with, greater_than, greater_or_equal, less_than, less_or_equal, is_null,
  is_not_null, in`. Every value is a bound parameter; `LIKE` metacharacters are
  escaped (`ESCAPE`); `is_null`/`is_not_null` take no value; `in` accepts 1–100
  values; BIGINT is accepted as an exact string. OR groups are **not**
  supported in v0.1.0. Unknown columns → `400 COLUMN_NOT_FOUND`; invalid
  operator/value/type → `400 INVALID_FILTER`. Filtering runs before sorting and
  pagination. The backend resolves the relation against introspection
  metadata and generates an explicit, dialect-quoted `SELECT … LIMIT n+1
  OFFSET m`; it never accepts SQL, a `WHERE` clause, or a raw sort expression.
  Response `data`: `{ database, schema?, table, object_type, columns[]
  (name/database_type/nullable/ordinal_position/primary_key), rows (array of
  arrays, same BIGINT-as-string/NULL/bytea/JSON rules as query execution),
  pagination{page,page_size,has_more}, truncated }`; `meta` mirrors
  `page/page_size/has_more`. There is no `total`/`COUNT(*)` by design (ADR-010
  §6); `has_more` comes from fetching one extra row. Errors: `400
  VALIDATION_ERROR` (missing/invalid args, missing database → `DATABASE_REQUIRED`),
  `404 NOT_FOUND` (unknown connection) / `TABLE_NOT_FOUND`, `400
  DATABASE_NOT_FOUND`/`DATABASE_CONNECT_DENIED`, `502 CONNECTION_ERROR`, `504
  QUERY_TIMEOUT`. Deep OFFSET can be slow on very large tables (documented
  limitation; keyset pagination is future work).
- `POST /api/v1/connections/{id}/table-data/rows` — **PRF-02/T08**. Body:
  `{ database, schema, table, values }` where each column value is
  `{ mode: "value" | "null" | "default", value? }`. Modes are distinct: `null`
  is a bound SQL NULL; `default` (and omitted columns) leave the value to the
  database. Generated/identity columns reject explicit `value`
  (`COLUMN_READ_ONLY`); `null` into NOT NULL → `INVALID_COLUMN_VALUE`; unknown
  columns/modes/values are rejected. Values are bound and identifiers
  metadata-validated; single-row only; no SQL/expressions from the client.
  Returns `{ affected_rows, row? }` (canonical row via RETURNING on
  PostgreSQL/SQLite; MySQL returns affected rows and the client refetches).
  Base tables are insertable even without a primary key; views/matviews/foreign
  tables → `ROW_NOT_MUTABLE`. Constraint failures → `409 CONSTRAINT_VIOLATION`.
- `PATCH|DELETE /api/v1/connections/{id}/table-data/rows` — **PRF-02/T07**.
  Body: `{ database, schema, table, identity, expected?, changes? }` (JSON;
  numbers via `json.Number` so BIGINT stays exact). `identity` must contain
  **exactly** the declared PRIMARY KEY columns (single or composite) — no
  unique-index/ctid/rowid fallback; no-PK tables → `400 ROW_IDENTITY_REQUIRED`,
  views/matviews/foreign tables → `400 ROW_NOT_MUTABLE`. `expected` enables
  NULL-safe optimistic concurrency: a mismatch after the row exists →
  `409 ROW_CONFLICT`; a vanished row → `404 ROW_NOT_FOUND`. PK/identity/generated
  columns are not updatable → `400 COLUMN_READ_ONLY`. Values are bound and
  identifiers metadata-validated. PRF-02/T09: the GUI Edit form submits **only
  changed fields** and requires a declared PRIMARY KEY; the `expected` snapshot
  is the normal GUI path (not optional), so stale edits return `409 ROW_CONFLICT`
  instead of silently overwriting. A transaction enforces affected-row safety
  (0 = conflict/not-found, >1 = `500 MUTATION_AFFECTED_MULTIPLE_ROWS` +
  rollback). Response `data`: `{ affected_rows, row? }` (canonical
  post-mutation row where readable). The Table Data page also advertises
  effective `row_capabilities` (insert/update/delete/duplicate), `row_identity`,
  and per-column `insertable`/`updatable` flags.
- `GET /api/v1/connections/{id}/table-structure` — **PRF-02/T13**. Query
  parameters: `database` (required for server-level PostgreSQL, else profile
  default), `schema` (required for PostgreSQL; ignored for MySQL/SQLite),
  `table` (required). Returns the read-only metadata of exactly one relation:
  it never reads a row of data and never mutates anything. Response `data`:
  `{ database, schema?, table, object_type, columns[]
  (name/database_type/normalized_type/nullable/default/primary_key/ordinal_position/
  insertable/updatable), primary_key{name?,columns[]}, indexes[]
  (name/unique/primary/method/partial/columns[]), constraints[]
  (name/type/columns[]/definition), foreign_keys[]
  (name/columns[]/referenced_schema/referenced_table/referenced_columns[]/
  on_update/on_delete) }`. `insertable`/`updatable` mirror T07: an
  identity/generated column is not insertable and a primary-key column is not
  updatable; non-base relations (views, foreign tables, matviews, partitions,
  `sqlite_sequence`) are read-only. Constraint `definition` text is returned as
  plain text and must be rendered as text only (never HTML) — see PRD §21.
  Errors: `400 VALIDATION_ERROR` (missing/invalid args, missing database →
  `DATABASE_REQUIRED`), `404 NOT_FOUND` (unknown connection) /
  `TABLE_NOT_FOUND`, `400 DATABASE_NOT_FOUND`/`DATABASE_CONNECT_DENIED`, `502
  CONNECTION_ERROR`, `504 INTROSPECTION_TIMEOUT`.
- `POST /api/v1/connections/{id}/table-data/export` — **PRF-02/T14**. Body:
  `{ database, schema, table, format: "csv" | "xlsx" | "sql", include_headers?
  (default true; CSV only), transaction? (default true; SQL only),
  insertable_only? (default false; SQL only), filters?, sort? }`. The target is
  **explicit and never inferred from explorer/global state**: `database` is
  required for server-level PostgreSQL (else profile default) and for
  MySQL/SQLite must equal the profile database (`DATABASE_REQUIRED` /
  `DATABASE_MISMATCH` otherwise). `filters`/`sort` use the exact table-data
  operators and rules from `GET table-data` and run in the database before the
  stream. Sinks: **CSV** streams directly (RFC 4180 quoting; formula-injection
  guard prefixes `'` to cells/headers starting `= + @ -` subject to the plain
  optional-number exception); **XLSX** writes a single typed sheet
  (formula-like leading characters stay literal text cells); **SQL** writes
  per-row `INSERT INTO <quoted> (<cols>) VALUES (…);` with a driver-matched
  transaction wrapper (`BEGIN`/`COMMIT`, MySQL `START TRANSACTION`;
  `transaction:false` omits it) and `insertable_only:true` restricts to
  insertable columns. The response is raw bytes with `Content-Disposition:
  attachment; filename="<sanitized>_<YYYYMMDD>.<ext>"` and `Cache-Control:
  no-store` (not the JSON envelope; errors still use the envelope). BIGINT
  serializes as string; `NULL` as empty/NULL; identifiers are dialect-quoted.
  Export runs under its own timeout (default 300s). Success is bounded: a
  result exceeding the export safety limit → `400 EXPORT_LIMIT_EXCEEDED`.
  Errors: `400 EXPORT_FORMAT_UNSUPPORTED` (format or no-exportable-columns),
  `404 TABLE_NOT_FOUND`, `400 VALIDATION_ERROR`/`INVALID_FILTER`/
  `COLUMN_NOT_FOUND`, `400 DATABASE_NOT_FOUND`/`DATABASE_CONNECT_DENIED`, `502
  CONNECTION_ERROR`, `504 QUERY_TIMEOUT`.
- `POST /api/v1/connections/{id}/table-import` — **PRF-02/T15** upload. Body:
  `multipart/form-data` with `file` plus the explicit target binding
  (`database` for server-level PostgreSQL / MySQL / SQLite, `schema` for
  PostgreSQL, `table`, `format: "csv" | "sql"`, `csv_header` bool for CSV). The
  target is **explicit and never inferred from explorer/global state**; it is
  fingerprinted (resolved database + schema/table + file SHA-256 + `format` +
  `csv_header`) and spooled server-side into a session; no target introspection
  happens at upload. Response `data` is a `TableImportSession`:
  `{ import_id, database?, schema?, table, format, filename?, size_bytes?,
  header_row, columns[], row_count, preview[][ImportCell], created_at }` where
  `ImportCell = { value?, is_null }` (explicit NULL is `is_null:true`). The
  preview is capped (default 50 rows). Uploads are size-capped →
  `413 IMPORT_TOO_LARGE`; XLSX/unknown formats and non-INSERT-only SQL →
  `400 IMPORT_FORMAT_UNSUPPORTED`; parse/convert failure →
  `400 IMPORT_VALIDATION_ERROR`. Sessions expire (default 30 min).
- `POST /api/v1/connections/{id}/table-import/{import_id}/validate` —
  **PRF-02/T15**. Body: `{ database?, schema?, table, mapping?[] }` — the target
  must **echo** the upload binding (`{source,target}` pairs override auto-mapping
  by name; omission auto-maps source columns to insertable target columns by
  name). Re-introspects the target and converts every row against the resolved
  mapping **without writing anything**; the server re-hashes the spooled file
  and rejects a changed file with `400 IMPORT_VALIDATION_ERROR`. Response `data`
  is a `TableImportValidation`: `{ valid, database?, schema?, table,
  object_type, row_count, columns[], mapping[ImportMappingResolved],
  errors[ImportValidationError{row,column?,message}] }`. Unknown/expired session
  → `404 NOT_FOUND`; read-only object → `400 ROW_NOT_MUTABLE`; target not found →
  `404 TABLE_NOT_FOUND`.
- `POST /api/v1/connections/{id}/table-import/{import_id}/execute` —
  **PRF-02/T15**. Same body as validate. Streams the spooled upload into **one
  all-or-nothing transaction** of parameterized `INSERT`s: any failure anywhere
  rolls the entire import back (no partial import). Response `data`:
  `{ inserted }`. Errors: `409 CONSTRAINT_VIOLATION`
  (NOT NULL/UNIQUE/FK/CHECK; the whole transaction is rolled back),
  `400 INVALID_COLUMN_VALUE`, `404 NOT_FOUND`/`TABLE_NOT_FOUND`, `413
  IMPORT_TOO_LARGE` (untrusted file size), `400 IMPORT_VALIDATION_ERROR` (changed
  file), `400 ROW_NOT_MUTABLE`, `400 DATABASE_NOT_FOUND`/
  `DATABASE_CONNECT_DENIED`, `502 CONNECTION_ERROR`, `504 QUERY_TIMEOUT`. Import
  runs under its own timeout (default 300s).
- `POST /api/v1/query/execute` — implemented in M1-T08. Errors: `400 SQL_SYNTAX_ERROR`
  (with `position`) / `SQL_ERROR`, `404 NOT_FOUND` (unknown connection),
  `502 CONNECTION_ERROR`, `504 QUERY_TIMEOUT`, `499 QUERY_CANCELED`; PRF-01 adds
  `400 DATABASE_REQUIRED` / `DATABASE_NOT_FOUND` / `DATABASE_CONNECT_DENIED` and
  `502 BOOTSTRAP_DATABASE_UNAVAILABLE`. Every attempt is recorded in history.
- `GET /api/v1/query/history` — implemented in M1-T08; newest-first with an
  optional `connection_id` filter. Paging remains **Needs Validation** (a
  default limit of 100 rows applies).
- `POST /api/v1/queries/saved` — snippet create (PRD). Body:
  `{ title, sql_text, connection_id?, tags? }`. The SQL is stored, never
  executed. Unknown `connection_id` → `404 NOT_FOUND`.
- `GET/PUT/DELETE /api/v1/queries/saved[/{id}]` — **M3 API extensions** required
  by the Saved Queries UI. List is newest-updated first with an optional
  `connection_id` filter; there is no pagination (see limitations). Deleting a
  connection sets saved-query `connection_id` to null (`ON DELETE SET NULL`).

### Endpoints that do NOT exist (do not invent)

- No SSE/streaming endpoint (despite the PRD architecture diagram mentioning SSE).
- No auth endpoints.
- No profile update/`PUT`/`PATCH`.
- No pagination/search endpoints.

---

## 10. Contract Open Questions (Needs Validation)

Resolved during M1 (implementation now defines these):

- Status codes for timeouts/SQL/connection errors (see §4 and the error table).
- `meta` is always present (`{}` when unused), including on errors.
- Binary/UUID/numeric/date/time serialization (see §6).
- Health endpoint body (`data: { "status": "healthy" }`).
- Creation returns `200 OK` (the `201` candidate was not adopted).
- `DELETE` returns `data: { "id": "<id>" }`.
- Required request fields are marked in OpenAPI and enforced in code:
  create requires `name` and `driver`; `database_name` is required for
  `mysql`/`sqlite` and optional for `postgres` (PRF-01); test requires `driver`
  (plus `database_name` for `mysql`/`sqlite`); execute requires `connection_id`
  and `sql`. `port`, `ssl_mode`, `password` and `timeout_seconds` are optional.
  `host`/`username` are required **at runtime for PostgreSQL/MySQL only**
  (SQLite targets a file), so they are not schema-`required`.

Still open:

1. Exact `VALIDATION_ERROR` payload shape (no field-level details).
2. Whether `PUT`/`PATCH` profile updates are in scope.
3. Whether request IDs are required (currently optional echo, never required).
4. Non-paginated ordering beyond the pagination contract (see §7).
5. SSE vs. synchronous-only result delivery.

---

## 11. OpenAPI Artifact

The OpenAPI document is generated from the implementation with the PRD-selected
`swaggo/swag` tooling and committed at `backend/docs/swagger.json` and
`backend/docs/swagger.yaml` (OpenAPI **3.1.0**).

Regenerate with:

```bash
cd backend && make swagger
# go run github.com/swaggo/swag/v2/cmd/swag@latest init \
#   -g cmd/server/main.go -o docs --parseInternal --outputTypes json,yaml --v3.1
```

**Decision (ADR-006):** OpenAPI **3.1.0** is formalized for this project —
see `docs/decisions/006-openapi-version.md`. PRD §3 ("OpenAPI 3.0") conflicts
with PRD §8.2 (`swaggo/swag` v1 → Swagger 2.0); ADR-006 resolves the conflict in
favor of `swaggo/swag/v2 --v3.1`. M2 code generation must use a 3.1-capable
tool.

The generated schema is contract-tested (`internal/api/contract_test.go`) to
expose exactly the implemented paths and to never contain `encrypted_password`,
SSH secrets, or encryption keys.

**Frontend types:** derive them from this document rather than hand-writing
DTOs — `cd frontend && npm run generate:api` writes
`src/types/generated/openapi.ts`, and `src/types/api.ts` re-exports DTO aliases
from it. Known limitation: the success envelope's `data` field is not typed in
the current document (a `swaggo/swag` generic limitation), so success payload
types are paired with the handwritten `ApiEnvelope<T>` wrapper.

