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
| `INTROSPECTION_ERROR` | 502 | Introspection | Target-side schema read failed (unreachable or restricted) |
| `INTROSPECTION_TIMEOUT` | 504 | Timeout | Schema introspection exceeded its deadline |
| `NOT_IMPLEMENTED` | 501 | Capability | Operation not implemented for this driver yet |
| `NOT_FOUND` | 404 | Resource | Unknown connection id / resource |
| `TABLE_NOT_FOUND` | 404 | Resource | PRF-02: Table Data target not found in metadata |
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
| `GET` | `/api/v1/connections/{id}/schemas` | Structural hierarchy (catalogs, tables, columns, relations); optional `database` query parameter (PRF-01) |
| `GET` | `/api/v1/connections/{id}/databases` | List selectable databases on a server-level connection (PRF-01; PostgreSQL only) |
| `GET` | `/api/v1/connections/{id}/table-data` | Bounded, paginated table/view rows (PRF-02; backend-generated SELECT) |
| `POST` | `/api/v1/query/execute` | Synchronous raw SQL execution |
| `GET` | `/api/v1/query/history` | Audit log of executed queries |
| `POST` | `/api/v1/queries/saved` | Save a query snippet (PRD) |
| `GET` | `/api/v1/queries/saved` | List saved queries (M3 extension; optional `connection_id` filter) |
| `GET` | `/api/v1/queries/saved/{id}` | Get a saved query (M3 extension) |
| `PUT` | `/api/v1/queries/saved/{id}` | Update a saved query (M3 extension) |
| `DELETE` | `/api/v1/queries/saved/{id}` | Delete a saved query (M3 extension) |

### Per-endpoint notes

- `GET /api/v1/health` — returns `success: true` with
  `data: { "status": "healthy" }` and `error: null`. Implemented in M1-T01.
- `GET /api/v1/connections` — must mask/omit secrets.
- `POST /api/v1/connections` — encrypts `password` and `ssh_password` at rest;
  returns masked profile. Creation status (`200` vs `201`) is **Needs Validation**.
- `POST /api/v1/connections/test` — uses the same parameter shape as creation but
  does not persist; ephemeral 5s ping.
- `DELETE /api/v1/connections/{id}` — closes/evicts the pool, then deletes the
  profile (history cascades; saved-query references are set null).
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
  (`asc`|`desc`) for **single-column server-side sorting**. The backend resolves the relation against introspection
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

