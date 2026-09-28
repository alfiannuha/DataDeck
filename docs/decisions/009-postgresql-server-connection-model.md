# ADR-009: PostgreSQL Server Connection Model (PRF-01)

## Status

Proposed — architecture approved for implementation planning; several parameters
are marked **Needs Validation** and must be confirmed during PRF01-T01+.
No production code is changed by this ADR.

## Context

Today a connection profile maps 1:1 to a *database* for every driver:

- `model.ConnectionProfile.DatabaseName` (storage column `database_name TEXT NOT NULL`).
- PostgreSQL `postgresDSN` embeds `cfg.Database` in the path, so a pool is bound
  to exactly one database for its lifetime.
- `ConnectionManager` keys pools by `connection_id` only.
- `POST /query/execute` takes `{connection_id, sql}`; `GET /connections/{id}/schemas`
  introspects the profile's single database.
- The frontend binds a query tab to `connectionId` only; schema queries are keyed
  by `connectionId`; the new-connection form requires `Database` for PostgreSQL.

This is correct but limiting: users must create one profile per PostgreSQL
database, duplicating host/credentials/SSL, and the schema explorer cannot show
sibling databases on the same server.

PRF-01 makes a PostgreSQL profile represent a **server/instance**, with database
selection after connect. PostgreSQL's hard constraint: **a session is attached to
one database for its lifetime**; there is no `USE database`. Switching databases
must therefore mean acquiring/creating a *different pool*, never mutating a
session.

Security guarantees from M1–M6 remain binding: AES-256-GCM credential storage,
no credential/DSN leakage to API or logs, sanitized errors, loopback default,
connection isolation, 30 s query timeout, 50 MB result cap, BIGINT-as-string,
and (M6) tag/version-driven release gates.

## Decision

We adopt a **server-level PostgreSQL profile with explicit, per-use database
binding**, implemented additively, without changing MySQL or SQLite semantics.

### 1. Meaning of a PostgreSQL Connection Profile

A PostgreSQL profile identifies a **server** and its credentials:

```
host, port, username, encrypted_password, ssl_mode (+ future SSH)
```

`database_name` remains in the model but becomes an **optional default
database** for PostgreSQL (see §2/§3). For an existing profile it keeps its
current meaning ("the database to use") and continues to act as that default.
The frontend label changes from "Database" to "Default database (optional)" for
PostgreSQL.

### 2. Optional / Default Database Behavior

- PostgreSQL: `database_name` **MAY be empty** on create/update.
- MySQL: `database_name` **MUST** remain non-empty (unchanged); the server/DB
  distinction does not apply.
- SQLite: `database_name` remains the **file path** (unchanged).

Storage keeps `database_name TEXT NOT NULL`; an omitted PostgreSQL database is
stored as the **empty string** (no destructive column change, no NULL ambiguity
in repositories). Validation moves from "required" to "optional, non-blank-or-
empty" for PostgreSQL only.

### 3. Bootstrap Database Strategy

For PostgreSQL, the effective database for an operation is resolved in this
order:

1. **Explicit request database** (`database` on execute/schemas/discovery call).
2. **Profile default** (`database_name`) when non-empty.
3. **Bootstrap candidates**, tried in order for *discovery only*:
   a. `postgres` (the conventional maintenance database),
   b. a database named after the login user (`$username`) when it exists,
   c. **otherwise fail** with `BOOTSTRAP_DATABASE_UNAVAILABLE`.

We do **not** silently pick an arbitrary database, and we do not treat
`template0`/`template1` as usable. Bootstrap is used to run the discovery query
and to enable first connection; it is **not** written back into the profile.

Failure handling: if `postgres` does not exist or CONNECT is denied, the next
candidate is tried; if none works, the request fails with an actionable message
("connect to a database explicitly") and the UI prompts the user to pick one.
Whether to persist the *discovered/selected* database back into
`database_name` is **Needs Validation** (proposed: do not mutate the profile;
selection is per tab).

### 4. Database Discovery Query and Filtering

Discovery runs **one catalog query on the bootstrap pool** (no per-database
connections):

```sql
SELECT datname
FROM pg_database
WHERE datallowconn
  AND NOT datistemplate
  AND has_database_privilege(oid, 'CONNECT')
ORDER BY datname;
```

Filtering rules:

- `datistemplate = false` → exclude `template0`/`template1`.
- `datallowconn = true` → exclude databases refusing connections.
- `has_database_privilege(oid,'CONNECT')` → hide databases the user cannot
  connect to (avoids offering unusable entries).
- `postgres` is **included only if the user can connect**; it is not
  special-cased in the list.
- Capabilities: a new `MultipleDatabases bool` capability is `true` for
  PostgreSQL and `false` for MySQL/SQLite; discovery endpoints are
  capability-gated.

Returned shape (neutral): `[{ "name": "app", "is_bootstrap_candidate": true }]`
— `is_bootstrap_candidate` is advisory only.

### 5. Database-Specific Pool Identity

Pool identity becomes a composite key:

```
PoolKey = { connection_id, database }
```

- PostgreSQL: `database` is the effective database (§3).
- MySQL/SQLite: `database` is derived from the profile (`database_name`), so the
  key degenerates to today's behaviour.

`ConnectionManager` keeps an internal `map[string]*sql.DB` keyed by an opaque
`connectionID + "\x00" + database` string (NUL is not valid in either), with
`Get`, `Open`, `Close`, `CloseConnection` (all pools for a connection id), and
`CloseAll` preserved for existing callers.

### 6. Pool Lifecycle and Cleanup

- Pools are created lazily on first use of `(connection, database)`.
- Deleting a profile closes **all** pools for that connection id
  (`CloseConnection`); `CloseAll` on shutdown is unchanged.
- Bounded pools: a per-connection cap on cached database pools (proposed: **16**,
  LRU-evicted) to prevent a user browsing many databases from holding unbounded
  connections; exact cap **Needs Validation**.
- Idle eviction (TTL) is explicitly deferred (M5 SEC-LOW-2 remains open).
- Pool limits from `Options` (MaxOpenConns etc.) apply **per pool**, so a single
  profile can hold `cap × MaxOpenConns` connections; this must be documented and
  is part of the acceptance review.

### 7. Query Database Binding

`POST /api/v1/query/execute` gains an **optional** `database`:

```json
{ "connection_id": "...", "database": "app", "sql": "SELECT 1" }
```

- Effective database = request `database` → profile default → (PostgreSQL)
  `DATABASE_REQUIRED` error. MySQL/SQLite ignore the field (must be absent or
  equal to the profile database; mismatches are rejected as validation errors).
- A query tab always sends its **bound** database; changing the globally
  selected database never rewrites a tab.
- History records the effective database (§9).

### 8. Schema Introspection Database Binding

`GET /api/v1/connections/{id}/schemas` gains an optional `database` query
parameter; introspection returns the schemas **of that one database** only
(never a merged tree across databases). Precedence: request `database` → profile
default → `400 DATABASE_REQUIRED`. **Clarification (PRF01-T04):** bootstrap
resolution (§3) is used for *discovery only*; user operations (query,
introspection) never silently substitute the bootstrap database, because that
would make the target database ambiguous. Response model is unchanged
(`model.Database{schemas,tables}`), with `Database.Name` set to the database
name.

### 9. History Database Attribution

`query_history` gains a nullable `database_name` column (migration 002). Every
execution stores the effective database; existing rows remain `NULL`
("pre-PRF-01 attribution"). The history API returns `database_name` (optional)
and the history UI shows it alongside the connection name. This is **additive**
and backward compatible.

### 10. Saved-Query Database Attribution

`saved_queries` gains a nullable `database_name` column (migration 002). When a
saved query is opened, the tab binds to its remembered database if present;
otherwise the user is asked to choose (no silent selection). Unbound snippets
(no connection) remain valid with `database_name = NULL`.

### 11. Existing Profile Migration

- **No data rewrite.** Profiles with `database_name='CCM'` keep working: the
  value is treated as the connection's default database and as the tab's
  initial binding (exactly today's behaviour).
- New/edited PostgreSQL profiles may clear the field, becoming server-level.
- A migration is required only to add the two **nullable** attribution columns
  (§9/§10); it does not touch existing rows.
- We do **not** split existing profiles into multiple profiles, and we do not
  infer "server vs database" profiles — the presence of a default database is
  the only distinction.

### 12. API Compatibility

All changes are additive:

| Endpoint | Change |
|---|---|
| `POST/PUT /connections` | `database_name` optional for `postgres`; still required for `mysql`/`sqlite` |
| `GET /connections/{id}/schemas` | new optional `database` query param |
| `POST /query/execute` | new optional `database` field |
| `GET /connections/{id}/databases` | **new** (capability-gated) discovery endpoint |
| `GET /query/history` | `database_name` added to records (optional) |
| saved queries | `database_name` added to request/response (optional) |

New error codes (documented before use): `DATABASE_REQUIRED`,
`DATABASE_NOT_FOUND`, `DATABASE_CONNECT_DENIED`,
`BOOTSTRAP_DATABASE_UNAVAILABLE`. Existing envelopes/status conventions
unchanged. An older client that never sends `database` gets today's exact
behaviour (profile default).

### 13. MySQL Behavior

**Unchanged.** MySQL requires `database_name`; there is no server-level mode,
no discovery endpoint (`MultipleDatabases=false`), and no `USE`-style switching.
The `database` field on execute/schemas is rejected for MySQL unless equal to
the profile database.

### 14. SQLite Behavior

**Unchanged.** The file path is the database; no discovery or switching;
`database` field rejected as for MySQL.

### 15. Bootstrap/Default Database Unavailable

- Profile creation does **not** require connectivity (unchanged).
- On use, if no explicit/default/bootstrappable database resolves, the API
  returns `BOOTSTRAP_DATABASE_UNAVAILABLE` (HTTP 502) with an actionable message;
  `/health` is unaffected and the daemon stays up.
- The UI shows the discovery failure and requires the user to enter/select a
  database; it must not silently fall back to the profile default after failure.

### 16. Missing CONNECT Permission

- Discovery hides databases without `CONNECT` (§4).
- If a client explicitly requests a database it cannot connect to, the API
  returns `DATABASE_CONNECT_DENIED` (HTTP 400) — a target-authorization
  condition, sanitized (no DSN, no credential). The pool is not cached.
- Permission changes after discovery surface as connect errors on use and are
  not retried automatically beyond the normal pool open.

### Target Model

```
Connection Profile (postgres)
└── Server (host, port, user, secret, ssl)
    ├── pool(profile, "app")      ← tab A (database=app)
    ├── pool(profile, "reporting")← tab B (database=reporting)
    └── pool(profile, "postgres") ← discovery/bootstrap pool
```

A query tab carries `{ connectionId, database, sql, … }`.

### Frontend State Design

- `QueryTab` gains `database: string | null`; the tab binds `connectionId` **and**
  `database` at creation / first run, never implicitly.
- `useConnectionStore` keeps `activeConnectionId` and adds
  `activeDatabaseByConnection: Record<connectionId,string>` (the explorer's
  selected database; **UI selection only** — it never rewrites tabs or profiles).
- Schema explorer: a database selector appears when `MultipleDatabases` is true;
  schema queries are keyed by `(connectionId, database)`.
- Stale-response protection: the existing request-id + active-tab guard is
  extended so a response is applied only if the tab is still active **and** its
  `database` still matches the request.
- Opening history/saved entries binds the tab's database from the recorded
  attribution (or prompts when absent).
- New-connection dialog: "Database" becomes optional for PostgreSQL only.

## Compatibility Matrix

| Client / store | Backend | Result |
|---|---|---|
| Old store (no `database_name` on history/saved) + new backend | migration 002 adds NULL columns | Works; attribution absent |
| Old profile with `database_name='CCM'` | new backend | Works unchanged (default DB) |
| New profile without database | new backend | Server-level; discovery + selection required before query |
| Old frontend (no `database` field) | new backend | Works: profile default used (or `DATABASE_REQUIRED` if empty) |
| New frontend | old backend | `database`/discovery unsupported → ignored/404; degrade to single-DB behaviour (documented, not supported) |
| MySQL / SQLite profiles | new backend | Unchanged |

## Security Implications

- Credentials are **not** duplicated per database; the same encrypted secret is
  decrypted in memory per pool open and never cached as plaintext, logged, or
  returned.
- Discovery is a read-only catalog query; no per-database connect storm.
- Errors are sanitized: `DATABASE_*` codes carry no DSN/userinfo; existing
  `SQLError`/`CONNECTION_ERROR` mapping is preserved, and connection-loss
  mapping (M6-T00) remains.
- Pool caps bound resource use across databases (DoS guard).
- Loopback default, request limits, result cap, BIGINT handling, PWA cache
  safety and export protections are untouched.
- A profile's credential now grants access to *multiple* databases' pools; this
  matches the credential's real PostgreSQL privileges and is surfaced in docs.

## Failure Modes

| Failure | Behavior |
|---|---|
| No database supplied and `postgres` unavailable | `BOOTSTRAP_DATABASE_UNAVAILABLE` (502), actionable message, daemon healthy |
| Database renamed/dropped while a pool exists | next use fails as `DATABASE_NOT_FOUND`/connection error; pool evicted |
| Discovery returns zero connectable databases | empty list + prompt; no fallback connect |
| CONNECT denied on explicit database | `DATABASE_CONNECT_DENIED` (400), not cached |
| Pool cap reached | LRU eviction (closes oldest pool) — Needs Validation on cap |
| MySQL/SQLite given `database` | validation error (mismatch rejected) |
| Permission revoked mid-session | connect/query error surfaced sanitized; no silent retry loop |

## Consequences

- Positive: one PostgreSQL profile per server; schema explorer can span sibling
  databases; tabs remain deterministically bound; no `USE`/session mutation;
  fully backward compatible.
- Negative: new API surface (discovery), composite pool management, added
  migration (two nullable columns), more complex frontend state; per-profile
  connection count can grow (bounded by cap).
- Neutral: MySQL/SQLite semantics deliberately unchanged (PRF-01 is
  PostgreSQL-only).

## Alternatives Considered

1. **Multiple profiles per database** (status quo). Rejected: credential and
   config duplication; poor UX.
2. **`USE`/session switching.** Rejected: not supported by PostgreSQL.
3. **One pool per profile, reconnecting on switch.** Rejected: throws away
   pooling per database and risks contention; composite pools are cleaner.
4. **Auto-persist selected database into `database_name`.** Rejected for now:
   mutates profiles implicitly; selection is per tab/UI. Revisit if UX demands.
5. **Discover by connecting to each database.** Rejected: connect storms and
   permission noise; a single catalog query suffices.
6. **New `kind: server|database` profile column.** Rejected: churn; the empty
   default database is a sufficient, backward-compatible signal.

## Constraints

- Do not implement `USE`/session mutation.
- Do not change MySQL/SQLite semantics.
- Do not break existing profiles or the pre-PRF-01 store (additive migration only).
- Preserve AES-256-GCM, no-secret-return, sanitized errors, loopback default.
- No SSH tunneling or authentication in PRF-01.
- Implementation lands in PRF01-T01+; **this ADR changes no production code**.

## Acceptance Criteria (for the implementation tasks)

1. A PostgreSQL profile can be created without a database and used after
   selecting a database discovered from the server.
2. Existing profiles (`database_name='CCM'`) behave exactly as before.
3. A query tab is bound to `(connectionId, database)`; switching the UI-selected
   database never changes an existing tab's target.
4. Discovery lists only connectable, non-template databases via one catalog
   query.
5. Deleting a profile closes all its pools; pool count per profile is bounded.
6. History and saved queries record the database; migration 002 is additive and
   reversible-by-backup.
7. MySQL and SQLite behaviour is unchanged and covered by regression tests.
8. No credential/DSN leakage; loopback default and existing limits hold.
9. Backward-compatible API: old clients (no `database`) keep working.
10. All M1–M6 quality gates and the hosted CI/release pipeline remain green.

## Open Questions (Needs Validation)

1. Pool cache cap per connection (proposed 16) and eviction policy.
2. Whether to persist the first successfully used database as the profile
   default (proposed: no).
3. Discovery visibility of `postgres` when the user has CONNECT (proposed: listed).
4. Exact HTTP status mapping for `DATABASE_CONNECT_DENIED` (proposed 400) vs
   `BOOTSTRAP_DATABASE_UNAVAILABLE` (proposed 502).
5. Whether `database` must be rejected (strict) or ignored for MySQL/SQLite.
6. UI treatment when discovery returns zero databases.

## Risks

- Unbounded pools if the cap is not enforced (mitigated by §6 cap).
- Frontend trust in the UI-selected database could leak into tab binding if the
  stale-response guard is not extended to include `database`.
- Migration 002 is forward-only (consistent with M6-T10 rollback guidance).
- Old-frontend/new-backend and new-frontend/old-backend combinations are not a
  supported matrix; only old-client/new-backend is guaranteed.
