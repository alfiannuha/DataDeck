# ADR-010: Database Explorer & Table Data Management (PRF-02)

## Status

Proposed — UX and architecture locked for implementation planning. Several
parameters are marked **Needs Validation**; no production code is changed by
this ADR. Implementation begins only in PRF02-T01+.

**Amendment (PRF02-T00A):** PRF-02 scope is intentionally expanded to include
**safe single-row CRUD** (Insert, Update, Delete, Duplicate). Row editing is no
longer a non-goal. The amendment adds row identity, mutation API, concurrency,
transaction, capability, error, security, testing, and acceptance requirements
in §§29–38 and updates §§5, 10, 18, 22, 24, 26, 27, 28 and the open
questions/risks.

## Context

DataDeck today is **query-first**: the only data surface is a raw `SELECT`
executed through `POST /api/v1/query/execute` and shown in a single global
result grid. The schema explorer is read-only navigation plus a few SQL-text
actions; there is no table browser, no filter/sort model, no server pagination
for data, no context menus, and no server-side import/export.

PRF-01 (ADR-009) established the non-negotiable binding rule this ADR must
preserve: a tab explicitly carries `connectionId` + `database`, bootstrap
databases are discovery-only, and explorer selection never mutates a tab.

PRF-02 turns DataDeck into a **database-client workspace**: users browse table
data in dedicated tabs while explicit SQL query tabs remain first-class. This
requires a typed tab model, a structured (non-SQL-text) table-data API, typed
filters/sorting/pagination, context menus, and carefully bounded import/export.

**Scope expansion (PRF02-T00A):** PRF-02 also provides **safe single-row CRUD**.
Users may Insert, Update, Delete, and Duplicate individual rows with explicit
mutation dialogs. This is enabled only when DataDeck can identify a row through
backend-verified metadata (primary key / proven unique key); otherwise the table
remains read-only. Bulk mutations and spreadsheet-style auto-save remain
non-goals.

### Current UX (as implemented)

- `useWorkspaceStore` (`frontend/src/store/useWorkspaceStore.ts`) holds a single
  `QueryTab[]`, seeded with one `initialTab` (`:80,:92`); `closeTab` recreates a
  tab when the list would empty (`:167-169`). There is no empty state.
- Selecting a connection only calls `setActiveConnection`
  (`frontend/src/components/sidebar/connection-list.tsx:118`); it does **not**
  open a tab (already correct for PRF-02 goal #1).
- `Workspace` always renders `TabsBar` + `EditorPanel` + `ResultsPanel`
  (`frontend/src/components/shared/workspace.tsx:9-18`).
- Schema explorer renders a tree; tables expand on **single click**, and hover
  action buttons insert SQL (`schema-tree.tsx:267-304`,
  `schema-explorer.tsx:80-92`). No right-click menus exist anywhere.
- `ResultGrid` (`frontend/src/components/grid/result-grid.tsx`) is TanStack
  Table `getCoreRowModel` + TanStack Virtual; no sort/filter/pagination models.
- Export is client-side over an already-fetched `QueryResult`
  (`frontend/src/lib/export/*`); no import path exists.
- Backend `Connector` (`backend/internal/database/driver.go:109-116`) offers
  `Introspect` and raw `Execute` only. `Capabilities`
  (`backend/internal/model/capabilities.go:6-25`) exists but is **not exposed
  over HTTP** (`Manager.Capabilities` has no route).
- Pagination `page/page_size` exists only for history/saved lists
  (`backend/internal/api/handler/pagination.go`), not for table data.
- `docs/api-contract.md:441-446` currently forbids inventing pagination/search
  endpoints; adding PRF-02 endpoints therefore requires this ADR plus a contract
  update.

## Decision

Adopt a **typed workspace-tab architecture** and a **structured, server-side
table-data API** that reuses the PRF-01 binding model and the existing
connector/capability system. Table browsing never sends arbitrary SQL from the
frontend; the backend validates identifiers against introspected metadata,
binds values as parameters, and composes dialect-quoted SQL.

### 1. Current UX

Summarized above. Key gaps to close: no table data surface, no empty state, no
server filtering/sorting/pagination for data, no context menus, no structured
data/database import-export, and capabilities invisible to the client.

### 2. Target UX

1. Selecting/opening a connection **never** opens a Query Editor.
2. With **zero tabs**, the workspace shows a useful empty state (connection
   summary + “Open a table” / “New query” affordances).
3. **Double-clicking a table** in the explorer opens a **Table Data tab**
   (`[ users ] [ orders ] [ Query 1 ]`).
4. A Table Data tab shows rows directly (no user-written SELECT) with
   server-side pagination, filtering, sorting, and refresh.
5. Query Editor remains a separate explicit **Query tab**.
6. Filters use a database-client UX (column / operator / value), inspired by
   tools such as TablePlus but with original implementation and assets.
7. Right-clicking a **table** opens management actions.
8. Right-clicking a **database** opens database actions.
9. A writable table exposes **Add Row**, **Edit Row**, **Duplicate Row**, and
   **Delete Row**; all mutations use explicit dialogs/drawers and go through
   backend-generated, parameterized SQL (no cell auto-save, §29–38).


### 3. Workspace / Tab Model

Replace `tabs: QueryTab[]` with a discriminated union:

```ts
interface TabBase {
  id: string;
  title: string;
  connectionId: string | null; // every database-bound tab carries this
  database: string | null;     // every database-bound tab carries this
}

interface QueryTab extends TabBase {
  kind: "query";
  sql: string;
  dirty: boolean;
  savedQueryId?: string | null;
  savedTitle?: string | null;
  savedTags?: string | null;
}

interface TableDataTab extends TabBase {
  kind: "table-data";
  schema: string | null; // null for MySQL/SQLite (no schema level)
  table: string;
  filters: TableFilter[];
  sort: TableSort[];      // v0.1.0: at most one entry
  page: number;           // 1-based
  pageSize: number;       // bounded (see §6)
}

interface TableStructureTab extends TabBase {
  kind: "table-structure";
  schema: string | null;
  table: string;
}

type WorkspaceTab = QueryTab | TableDataTab | TableStructureTab;
```

Rules:

- `kind` is a required discriminant. Existing transient tabs have no persisted
  form (tabs are memory-only per `docs/architecture.md` §13), so no data
  migration is required; newly created tabs set `kind: "query"`.
- Table-bound tabs additionally carry `schema` (or `null`) and `table`.
- Every database-bound tab pins `connectionId` + `database`; the PRF-01 rule
  stands: **explorer selection never mutates existing tab bindings.**
- New actions: `openTableData(connectionId, database, schema, table)`,
  `openTableStructure(...)`, `updateTableData(id, patch)` (filters/sort/page),
  and `openQueryForTable(...)` (generates SQL into a **new** Query tab, never
  auto-executes).
- Row mutation is **not** stored in the tab: the tab holds only the browse
  binding; a row's identity is derived per result from backend-verified metadata
  and sent explicitly with each mutation request (§30). The tab never caches a
  client-decided identity.
- `setTabConnection`/`setTabDatabase` (PRF01-FIX-01) keep their semantics and
  apply to all tab kinds; clearing a connection clears the database.
- `closeTab` allows the list to reach zero and sets `activeTabId: null`
  (the empty state, §4). `Workspace` branches on `activeTab.kind` to render
  `EditorPanel`/`ResultsPanel` or `TableDataPanel`/`TableStructurePanel`.

### 4. Empty-State Behavior

- `tabs` may be empty; `activeTabId` may be `null`.
- `Workspace` renders `WorkspaceEmptyState` when `tabs.length === 0`:
  shows the active connection (name/driver/database) or a “select a connection”
  prompt, plus buttons: **New Query** and (when a connection is active) a hint
  to double-click a table. No SQL is executed from the empty state.
- Removing the auto-created `initialTab` and the recreate-on-last-close behavior
  is an intentional, breaking UI change; tests that assumed a permanent tab are
  updated in the implementation tasks.

### 5. Table Data API

Add structured endpoints (contract update + ADR required; this ADR supplies
both). All responses use the standard envelope.

```
POST /api/v1/connections/{id}/table-data/query
  body: {
    database: string,            // required for server-level PG; profile default otherwise
    schema: string | null,       // required where the engine has schemas (PG), else null
    table: string,
    columns?: string[],          // optional projection, subset of introspected columns
    filters?: TableFilter[],
    sort?: TableSort[],          // v0.1.0: 0..1
    page?: number,               // default 1
    page_size?: number,          // default 100, max 200 (reuses pagination constants)
    include_total?: boolean      // default false
  }
  200 data: {
    columns: QueryColumn[],
    rows: unknown[][],
    page: number, page_size: number,
    has_more: boolean,
    total?: number, total_exact?: boolean,
    truncated: boolean,
    execution_time_ms: number,
    // Row-CRUD metadata (PRF02-T00A, §30): derived server-side, never trusted
    // from the client.
    identity: { kind: "primary_key" | "unique", columns: string[] } | null,
    row_mutable: { insert: boolean, update: boolean, delete: boolean }
  }
  meta: { page, page_size, total?, has_more }
```

```
GET  /api/v1/connections/{id}/table-data/row-count?database=&schema=&table=&filters=
POST /api/v1/connections/{id}/table-data/rows          (INSERT; §32)
PATCH /api/v1/connections/{id}/table-data/rows         (UPDATE; §33)
DELETE /api/v1/connections/{id}/table-data/rows        (DELETE with body identity; §34)
POST /api/v1/connections/{id}/table-data/export   (CSV | SQL | XLSX; streamed)
POST /api/v1/connections/{id}/table-data/import   (multipart: CSV | SQL | XLSX)
GET  /api/v1/connections/{id}/capabilities        (capability descriptor, §18)
POST /api/v1/connections/{id}/database/export     (database-level SQL export; streamed)
POST /api/v1/connections/{id}/database/import     (database-level SQL import)
POST /api/v1/connections/{id}/database/drop       (destructive, §17)
```

`table-data/query` is a read-only projection of one table/view. It never accepts
SQL text. It reuses the managed pool for `(connectionId, database)` — the same
identity PRF-01 uses — so table browsing and query tabs share database isolation.

Mutation endpoints (`rows`) are defined fully in §29–38. They never accept SQL
text, a raw `WHERE` clause, or a client-decided identity: as with `query`, the
backend re-validates `database`/`schema`/`table` against introspection and
composes parameterised SQL. `DELETE` carries its target in the JSON body (never
the URL) so the identity is explicit and auditable.

### 6. Pagination Model

- Request: `page` (1-based, default 1) + `page_size` (default 100, max 200).
  Reuse `backend/internal/api/handler/pagination.go` constants (default 50 is
  history-specific; table data defines its own default 100 with the same max
  200) — final constants **Needs Validation**.
- Implementation: `LIMIT page_size+1 OFFSET (page-1)*page_size`; `has_more`
  is derived from the extra row. Response page is capped at the requested size.
- `total`/`total_pages` are **opt-in** (`include_total`) because `COUNT(*)` can
  be expensive. When requested, run the same filtered `COUNT(*)` under the
  statement timeout; on timeout/plan limit, omit `total` and set
  `total_exact: false` rather than failing the page.
- **Tradeoff (explicit):** OFFSET pagination is O(offset) on large tables and
  can skip/duplicate rows under concurrent writes. v0.1.0 uses bounded OFFSET
  because it is consistent with the existing architecture and supports
  arbitrary page jumps. **Keyset (cursor) pagination** is the future path:
  deterministic ordering by primary key + `WHERE (pk) > (last_pk)`. The
  limitation is documented in the contract and UI (“page N of many”).
- Deep-page guard: reject `offset` beyond a configured ceiling
  (`maxOffset`, e.g. 1,000,000 rows) with `PAGE_OUT_OF_RANGE` to prevent
  pathological scans. Needed even though `maxPage` already bounds history.

### 7. Filter Model

```ts
type FilterOperator =
  | "equals" | "not_equals"
  | "contains" | "starts_with" | "ends_with"
  | "greater_than" | "greater_or_equal"
  | "less_than" | "less_or_equal"
  | "is_null" | "is_not_null"
  | "in";

interface TableFilter {
  column: string;
  operator: FilterOperator;
  value?: string | number | boolean | null; // omitted for is_null/is_not_null
  values?: Array<string | number | boolean>; // for in; bounded (e.g. max 100)
}
```

- A filter is valid only for a **single** column that exists in the
  introspected table metadata; unknown column → `COLUMN_NOT_FOUND` (400).
- Operator/type compatibility is enforced by a type category derived from the
  column's `data_type`:
  - text-like (char/text/uuid/enum/json-cast): `equals/not_equals/contains/
    starts_with/ends_with/in/is_null/is_not_null`
  - numeric (int/float/decimal/numeric/money): `equals/not_equals/
    greater_than/greater_or_equal/less_than/less_or_equal/in/is_null/is_not_null`
  - temporal (date/time/timestamp): comparison + equality + null operators only
  - boolean: `equals/not_equals/is_null/is_not_null`
  - binary/other: equality + null operators only
  - incompatible operator → `INVALID_FILTER` (400).
- All values are **bound parameters**, never concatenated. `in` expands to a
  bounded placeholder list. `contains/starts_with/ends_with` escape `%`/`_`
  and use an explicit `ESCAPE` clause.
- Multiple filters combine with `AND` in v0.1.0; OR groups are **Needs
  Validation** (deferred).

### 8. Sorting Model

```ts
interface TableSort { column: string; direction: "asc" | "desc"; }
```

- Server-side only (the grid never re-sorts fetched pages).
- v0.1.0 supports **at most one** sort column (multi-column sorting deferred and
  listed as Needs Validation).
- The sort column must exist in the table metadata. When no sort is provided,
  the backend orders by primary-key columns ascending if a PK exists; otherwise
  it uses a deterministic but unspecified order and the response notes it.
  Stable ordering matters for OFFSET paging.
- `NULLS FIRST/LAST` handling is engine-default in v0.1.0 (documented).

### 9. SQL Generation / Security

The backend composes:

```
SELECT <quoted projection> FROM <quoted schema.> <quoted table>
[WHERE <predicates>] [ORDER BY <quoted col> <ASC|DESC>]
LIMIT $n OFFSET $m
```

Guarantees:

- **Identifier validation:** schema/table/columns come from a lookup against
  `Introspect` output for the bound `(connectionId, database)`; anything not
  found is rejected. Identifiers are then quoted with the driver-specific
  quote character (backtick for MySQL, double quote for PG/SQLite) and any
  embedded quote character is doubled.
- **Value binding:** every literal is a bound parameter (`?`/`$n`/named as the
  driver requires). No untrusted value is ever concatenated.
- Projection is bounded to introspected columns; `SELECT *` is expanded
  server-side to the metadata column list (so column order/types are known and
  result encoding stays consistent with `POST /query/execute`).
- Same `context.WithTimeout` (default 30 s), same 50 MB JSON cap (`truncated`),
  same BIGINT/NUMERIC-as-string encoding as raw execution.
- The frontend never constructs SQL for browsing (only “New Query” scaffolding
  reuses the existing identifier-quoting helpers in `frontend/src/lib/sql/`).

### 10. Table Context Menu

Right-click a table (new Radix `ContextMenu` primitive; dependency justified in
implementation) shows, gated by capabilities:

| Action | Behavior |
|---|---|
| View Data | Opens a `TableDataTab` (double-click default) |
| New Query | Generates `SELECT …` with quoted identifiers into a **new** Query tab; never auto-executes |
| View Structure | Opens a `TableStructureTab` |
| Refresh | Re-fetches the current table’s schema/page |
| Export → CSV / XLSX / SQL | Calls the table export endpoint (streamed) |
| Import → CSV / XLSX / SQL | Opens import dialog (multipart upload) |
| Copy Table Name | Clipboard: bare name |
| Copy Qualified Name | Clipboard: `schema.table` (or `database.table` where applicable) |

“Drop Table” (mentioned in PRD §7.2) is intentionally **not** part of PRF-02
(non-goal); if added later it follows the §17 destructive pattern.

**Row context menu** (inside a Table Data tab, §31): Edit Row, Duplicate Row,
Copy Row, Copy as JSON, Delete Row. Edit/Delete/Duplicate are shown only when
the table is writable and a safe row identity was resolved by the backend
(§30); otherwise the menu items are disabled with an explanation. The table
toolbar additionally exposes **Add Row**, **Filter**, **Sort**, and **Refresh**.

### 11. Database Context Menu

Right-click a database (server-level PG, and the single-database case for
MySQL/SQLite where meaningful), gated by capabilities:

| Action | Behavior |
|---|---|
| New Query | New Query tab bound to that database; no execution |
| Refresh | Re-run discovery for the server |
| Reconnect | Evict the pool(s) for that connection/database and re-open lazily (reuses `CloseConnection`/`Close`) |
| Import SQL | Database-level SQL import (streamed) |
| Export SQL | Database-level SQL export (streamed) |
| Copy Database Name | Clipboard |
| Delete Database | Destructive flow (§17) |

Actions unsupported by an engine are hidden/disabled, not silently failed; the
backend also returns `NOT_IMPLEMENTED` as defense in depth.

### 12. Table Structure Design

`TableStructureTab` renders existing introspected metadata (`model.Table`:
columns/type/nullable/default/ordinal, primary key, foreign keys, indexes,
`Table.Type` for base/partitioned/view/matview/foreign). It is read-only in
PRF-02 (no schema designer — non-goal). Reuses `GET /connections/{id}/schemas`
(with `database`) rather than a new structure endpoint; a dedicated
`GET …/tables/{table}` endpoint is **Needs Validation** only if per-table
fetching proves necessary for very large schemas.

### 13. CSV Export / Import

- **Export:** streamed from the table-data engine (same filters/sort as the
  grid, or full table when no filters). RFC 4180 escaping; formula-injection
  guard (prefix `'` for cells starting `= + - @` and tab/CR leading) reused from
  `frontend/src/lib/export/values.ts` if the export runs client-side, and
  reimplemented identically server-side. Filename is generated and sanitized;
  no credentials/DSN in the name.
- **Import:** multipart upload, streamed parse (no full-file buffering),
  configurable chunk batches. Header row optional; explicit column mapping
  (source header → target column) validated against metadata.
  - Delimiter/quote/encoding (UTF-8 default, BOM tolerated), malformed rows →
    `IMPORT_VALIDATION_ERROR` with row/column.
  - **NULL semantics:** unquoted empty field → `NULL`; quoted empty `""` →
    empty string (documented; overridable per import).
  - **Type conversion:** per target column data type; failure aborts the batch.
  - **Transaction:** default whole-file transaction (all-or-nothing), aborted
    on first error with a sanitized report of up to N errors; partial-commit
    mode is a future opt-in (Needs Validation). Engine caveat: implicit commits
    around DDL do not apply to CSV data inserts.
  - Partial failure reports counts inserted/rolled back and the first failing
    row numbers.

### 14. SQL Export / Import

- **Table SQL export:** `INSERT INTO <quoted table> (<cols>) VALUES (…);` per
  row, correctly typed and NULL-explicit; optional transaction wrapper matching
  driver (`BEGIN/COMMIT`, MySQL `START TRANSACTION`). Identifiers quoted per
  dialect.
- **Database SQL export:** per-table DDL + data produced by this app (not by
  assuming native tools like `pg_dump`/`mysqldump`, which are **not** assumed
  available). DDL fidelity is best-effort and documented; unsupported objects
  are reported, not silently dropped.
- **SQL import:** executes the provided SQL statements sequentially in a
  transaction where the engine allows it. This is inherently privileged and
  untrusted; it is gated behind a capability, requires an explicit target
  `database` in the body, runs under the statement timeout, and reports the
  first failing statement/position. Multi-statement boundaries are parsed
  conservatively; statement splitting that mishandles dialect specifics is a
  known risk.
- Database-level import/export is designed **separately** from table-level
  (different endpoints, different limits/confirmations).

### 15. XLSX Export / Import

- **Export:** workbook with a single sheet, header row + typed cells; formula
  injection guarded by writing potentially dangerous leading characters as
  text. Streamed/bounded write.
- **Import:** read the first sheet (or a user-selected sheet), header row →
  mapping, blank cell → `NULL`; strict shared-string/number/date handling.
- **Library:** no XLSX library exists in either stack. A pure-Go, CGO-free
  writer/reader (e.g. `github.com/xuri/excelize/v2`) is the likely choice but is
  **Needs Validation** and must be justified per AGENTS.md §11 (footprint,
  license, advisories). Until approved, the `xlsx` capability is advertised as
  false and the UI hides XLSX actions. A minimal hand-rolled reader/writer is
  not acceptable for untrusted input (see §16 threat model).
- **ZIP-bomb / expansion limits** (XLSX is a ZIP): cap compressed and
  decompressed size, entry count, and compression ratio; reject on breach.

### 16. Database Import / Export

Designed separately from table import/export. Export produces a single SQL
stream (schema + optional data) or a multi-file archive in a later iteration;
v0.1.0 targets a streamed SQL text response. Import consumes SQL text/streams
against an explicit target database with the same transaction and timeout
guarantees. Native command-line tools are **not** assumed. Database-level
operations are capability-gated per engine.

### 17. Destructive Database-Management Safety (DROP DATABASE)

`DROP DATABASE` is HIGH RISK and must never be reachable by accident.

Frontend:

- Destructive styling (danger variant) and a dedicated confirmation dialog.
- Dialog displays **connection name, server host:port, and exact database
  name**.
- User must **type the exact database name** to enable the confirm action.
- No default-focused destructive button; Escape/backdrop cancels.

Backend (`POST /api/v1/connections/{id}/database/drop`):

- Requires an explicit body `{ database: string }`; **never** derives the
  target from explorer/global state.
- Re-validates the database exists/enumerated on the server.
- Runs the drop from a session **not attached to the target** when the engine
  forbids dropping the current database (PostgreSQL: use the bootstrap/
  maintenance database; MySQL: any database is fine but privileges required;
  SQLite: unsupported → `NOT_IMPLEMENTED`).
- Capability-gated (`drop_database`), sanitized errors, audit-friendly logging
  without credentials.
- After success: evict/close the pool for `(connectionId, database)`.
- Tabs bound to the deleted database transition to an explicit **unavailable**
  state (existing `DATABASE_NOT_FOUND`/`CONNECTION_ERROR` handling); they show a
  recoverable error and **never** fall back to executing against another
  database.
- Confirmation of destructive intent is server-validated (name match) in
  addition to the frontend typed confirmation.

### 18. Capability Matrix

Extend `model.Capabilities` with the PRF-02 surface and expose it via
`GET /connections/{id}/capabilities`:

```go
BrowseData       bool
Filtering        bool
Sorting          bool
Structure        bool
TableExport      bool
TableImport      bool
DatabaseExport   bool
DatabaseImport   bool
Reconnect        bool
DropDatabase     bool
RowInsert        bool
RowUpdate        bool
RowDelete        bool
RowDuplicate     bool
ExportFormats    []string // e.g. ["csv","sql","xlsx"]
ImportFormats    []string
```

| Capability | PostgreSQL | MySQL | SQLite |
|---|---|---|---|
| browse data | yes | yes | yes |
| filtering | yes | yes | yes |
| sorting | yes | yes | yes |
| structure | yes | yes | yes |
| table export | yes (CSV/SQL/XLSX*) | yes (CSV/SQL/XLSX*) | yes (CSV/SQL/XLSX*) |
| table import | yes (CSV/SQL/XLSX*) | yes (CSV/SQL/XLSX*) | yes (CSV/SQL/XLSX*) |
| database export | yes | yes | n/a (single file) |
| database import | yes | yes | n/a (single file) |
| reconnect | yes | yes | yes |
| delete database | yes | yes | no (`NOT_IMPLEMENTED`) |
| row insert | yes | yes | yes |
| row update | yes | yes | yes |
| row delete | yes | yes | yes |
| row duplicate (via insert) | yes | yes | yes |

Engine-level CRUD capabilities are necessary but **not sufficient**; the actual
table-level mutability is derived from introspection metadata (§30). Expected
object-level behavior:

| Object | PG | MySQL | SQLite |
|---|---|---|---|
| base table with PK | CRUD | CRUD | CRUD |
| composite PK | CRUD (identity = all PK columns) | CRUD | CRUD |
| table without PK/unique | read-only for Update/Delete; Insert may be allowed | read-only for Update/Delete; Insert may be allowed | read-only for Update/Delete; Insert may be allowed |
| generated / identity / autoincrement column | insertable only per metadata; never updatable; generated always read-only | same | same |
| view | read-only by default; CRUD only if engine reports updatable and identity exists | read-only by default; CRUD only if updatable + identity | read-only (SQLite views are not directly updatable without triggers) |
| materialized view | read-only | n/a | n/a |
| foreign table | insert/update/delete only if the foreign table exposes a safe identity and the FDW permits it; otherwise read-only | n/a | n/a |

\* XLSX is advertised only after the library is validated (see §15); otherwise
`xlsx` is omitted and the UI hides it. SQLite “database” actions are limited to
the file/store; there is no logical database to create/drop.

### 19. PostgreSQL Behavior

Server-level multi-database from PRF-01 is preserved. `database` selects the
pool `(connectionId, database)`; table data for a database requires an explicit
or profile-default database and otherwise returns `DATABASE_REQUIRED`. `schema`
is meaningful (`public` etc.). Views/matviews/foreign tables are browsable when
introspected, read-only. DROP DATABASE must run from a maintenance database.
Row CRUD uses `INSERT … RETURNING` / `UPDATE … WHERE pk=$n` / `DELETE … WHERE
pk=$n`; `RETURNING` is native and is used to return the mutated row (§32);
identity columns come from `table.PrimaryKey` or a validated unique index.

### 20. MySQL Behavior

Single database per profile (unchanged). `schema` is `null`; the catalog is the
profile database. Capabilities already mark `schemas=false`. Multi-statement SQL
import shares MySQL's implicit-commit caveats (documented). DROP DATABASE is
supported with sufficient privileges and is capability-gated. Row CRUD relies on
affected-row counts and a follow-up `SELECT` keyed by the identity (MySQL lacks
portable `RETURNING`); `LAST_INSERT_ID()` is used for single auto-increment
inserts.

### 21. SQLite Behavior

Single file; `schema` is `null` (or `main`). Writes serialize through the
existing single-connection pool (`SetMaxOpenConns(1)`), so a long import blocks
other operations — documented and covered by timeouts. `DROP DATABASE` is
unsupported. Table data/filter/sort/export/import are supported against the
connected file. Row CRUD is supported for tables with a PK/`rowid`; SQLite
3.35+ `RETURNING` is used when available, otherwise the identity is resolved via
`last_insert_rowid()` / a keyed `SELECT` after the write. Writes serialize
through the single connection.

### 22. Security Threat Model (untrusted file input + new endpoints)

| Threat | Mitigation |
|---|---|
| SQL injection via identifiers | Identifiers validated against introspection and dialect-quoted; never taken from request text |
| SQL injection via filter values | All values bound as parameters; `LIKE` wildcards escaped with `ESCAPE` |
| Malicious SQL import | Capability-gated, explicit target database, transaction + timeout, sanitized errors, size/statement caps |
| Malicious CSV | Streaming parser, row/column/size caps, strict type conversion, no formula evaluation, encoding validation |
| Malicious XLSX | Vetted library only; ZIP entry/size/ratio caps; reject macros/external links; first-sheet only |
| Path traversal | No filesystem paths from client input; exports use generated, sanitized filenames; imports are in-memory streams |
| Decompression/ZIP bomb | Compression-ratio and absolute decompressed-size caps before materializing |
| Formula injection | Leading `= + - @`/tab/CR guarded during CSV and XLSX export |
| Oversized files | Upload body cap + streaming parse; `PAYLOAD_TOO_LARGE` (413) |
| Malformed encodings | Validate UTF-8; BOM tolerated; reject otherwise with row context |
| Transaction abuse | Bounded statement count/timeout; rollback on failure; DDL caveats documented |
| Wrong-database execution | Every tab and table-data request carries explicit `connectionId`+`database`; bootstrap never used for data; no fallback after deletion |
| Credential leakage | Unchanged: credentials decrypted only in memory, never logged/returned; export filenames contain no secrets |
| PWA caching API | Unchanged: `/api/` bypasses the service worker cache; import/export responses use no-store |

CRUD-specific threats (PRF02-T00A):

| Threat | Mitigation |
|---|---|
| SQL injection via column/value | Identifiers validated against introspection and dialect-quoted; all mutation values bound as parameters; no raw `WHERE` accepted |
| Forged identity metadata | The client cannot declare an identity: the backend resolves identity columns from PK/unique metadata and rejects unknown/non-unique columns |
| Mass update | UPDATE/DELETE require a resolved unique identity and verify exactly one affected row (`MUTATION_AFFECTED_MULTIPLE_ROWS`) |
| Mass delete | Same as mass update; DML without a validated identity is rejected before execution |
| Wrong-database mutation | Mutation target is the request's explicit `connectionId`+`database`; explorer selection is never mutation authority |
| Stale row | Optimistic concurrency via affected-row verification and original-value predicates; `ROW_NOT_FOUND`/`ROW_CONFLICT` surfaced, never silent overwrite |
| Race / concurrent update | Optional original-value predicates and documented lost-update behavior; engine version columns when available |
| Generated-column mutation | Generated/read-only columns rejected with `COLUMN_READ_ONLY`; identity/auto columns follow metadata insertability |
| Oversized field values | Per-field and per-request size caps; `PAYLOAD_TOO_LARGE`/`INVALID_COLUMN_VALUE` |
| Sensitive values in logs/errors | Mutation values are never logged; errors are sanitized (no values, SQL text, or DSN) |
| DML in read-only objects | Tables/views without safe identity remain read-only; engine object type gates capability |

### 23. Performance Strategy

- The grid renders only the bounded current page; existing TanStack Virtual
  remains for rows within the page. No full-table fetch.
- Backend memory bounded: `LIMIT page_size+1`; export/import **stream** rather
  than buffer; the 50 MB JSON cap still applies to pages.
- `COUNT(*)` is opt-in and timeout-bounded.
- OFFSET depth guard prevents pathological deep scans; keyset is the documented
  future.
- Connection pools reuse PRF-01 identities; table browsing adds no new pool
  dimension beyond `(connectionId, database)`.
- Indexes/PK ordering used for default sort where available.
- A mutation never reloads the whole table: it targets one row by identity and,
  on success, invalidates/refetches only the current bounded page (or re-reads
  the mutated row by identity when `RETURNING` is unavailable).

### 24. Error Model

Reuse the envelope and existing codes; add documented codes (contract updated
before use):

| Code | HTTP | Meaning |
|---|---|---|
| `TABLE_NOT_FOUND` | 404 | Table/schema not found in metadata |
| `COLUMN_NOT_FOUND` | 400 | Filter/sort/projection column unknown |
| `INVALID_FILTER` | 400 | Operator/type/arity mismatch |
| `INVALID_SORT` | 400 | Unknown column or invalid direction |
| `PAGE_OUT_OF_RANGE` | 400 | Offset beyond configured ceiling |
| `UNSUPPORTED_FORMAT` | 400 | Export/import format not supported |
| `IMPORT_VALIDATION_ERROR` | 400 | Row/column parse/type failure (row context) |
| `IMPORT_TOO_LARGE` / `EXPORT_TOO_LARGE` | 413 | Byte/row caps exceeded |
| `PAYLOAD_TOO_LARGE` | 413 | Existing recommended code, now used for uploads |
| `DESTRUCTIVE_CONFIRMATION_MISMATCH` | 400 | Typed name mismatch on drop |
| `ROW_NOT_FOUND` | 404 | Target row no longer exists (0 affected rows) |
| `ROW_NOT_MUTABLE` | 400 | Object/table is read-only (view, no identity, engine) |
| `ROW_IDENTITY_REQUIRED` | 400 | Update/Delete without a resolved safe identity |
| `ROW_IDENTITY_INVALID` | 400 | Identity columns not the verified PK/unique key |
| `ROW_CONFLICT` | 409 | Row changed/deleted concurrently (original-value/affected-row check failed) |
| `COLUMN_READ_ONLY` | 400 | Attempt to write a generated/read-only/identity column |
| `INVALID_COLUMN_VALUE` | 400 | Type/length/constraint validation failure on a provided value |
| `MUTATION_AFFECTED_MULTIPLE_ROWS` | 500 | Safety failure: DML matched more than one row; rolled back |

`ROW_CONFLICT`, `ROW_NOT_FOUND`, and `MUTATION_AFFECTED_MULTIPLE_ROWS` map to
distinct user-facing guidance; `MUTATION_AFFECTED_MULTIPLE_ROWS` is a server-side
safety abort and is never reported as success.

Existing `DATABASE_*`, `CONNECTION_ERROR`, `QUERY_TIMEOUT`, `NOT_IMPLEMENTED`,
`INTROSPECTION_*`, `VALIDATION_ERROR`, `INTERNAL_ERROR` remain authoritative.
Errors are sanitized (no SQL text, no DSN, no credentials).

### 25. Migration / API Compatibility

- **No SQLite migration** is required by this ADR: no new persisted columns.
  (If import history/export job tracking is later persisted, it needs its own
  additive migration.)
- New endpoints and the capability descriptor are **additive**; existing
  endpoints and PRF-01 `database` semantics are unchanged. `docs/api-contract.md`
  and `backend/docs/swagger.*` must be updated in the implementation task before
  the code change.
- Frontend tab model change is transient UI state; no local persistence, so no
  upgrade migration. Existing Query tabs default to `kind: "query"`.
- MySQL/SQLite existing behavior is unchanged; new features are opt-in via
  capabilities.

### 26. Testing Strategy

- **Backend unit:** filter/sort → SQL builder with identifier-quoting and
  injection corpus; operator/type matrix; pagination math; capability gating.
- **Backend integration (`-tags=integration`, `-p 1`):** per driver (PG17,
  MySQL8, SQLite) browse/filter/sort/page, views, missing table/column, count
  timeout, export/import round-trips, DROP DATABASE happy/sad paths (PG/MySQL),
  SQLite `NOT_IMPLEMENTED`.
- **Frontend:** store tests for typed tabs, empty state, table-data tab
  transitions, filter/sort/page state; component tests for `TableDataPanel`,
  `TableStructurePanel`, context menus (keyboard + right-click), import dialog
  validation, destructive confirm (exact-name), capability-driven hiding.
- **E2E (Playwright):** double-click table → Table Data tab → page/filter/sort →
  correct database marker (extend the PRF-01 safety scenario so a table bound to
  alpha never reads beta); export download; no auto-execution on “New Query”.
- **Security tests:** formula-injection export, malicious CSV/XLSX fixtures,
  oversized uploads, SQL-import rollback, `%`/`_` escaping, DSN non-leakage.
- **CRUD backend unit:** identifier/identity validation, SQL generation and
  parameter binding, generated/read-only column rejection, affected-row
  verification (0/1/>1), injection corpus, NULL-vs-DEFAULT-vs-value encoding.
- **CRUD integration (per engine, `-tags=integration`, `-p 1`):** insert,
  update, delete, duplicate, composite PK, no-PK read-only, NULL/default,
  generated/identity columns, stale row (`ROW_CONFLICT`/`ROW_NOT_FOUND`),
  wrong-database attempt, rollback on validation failure, PG `RETURNING`,
  MySQL affected-rows + `LAST_INSERT_ID()`, SQLite `last_insert_rowid()`.
- **CRUD frontend:** Add Row, Edit Row, Delete confirmation (identity shown),
  Duplicate Row (opens insert form, no immediate insert), read-only state,
  mutation error surfaces, filters/sort/page/tab binding preserved after a
  mutation, no automatic cell writes.
- **CRUD E2E (Playwright, BLOCKER gate):** databases `alpha` and `beta` contain
  identically named tables with different marker values; open
  `TableDataTab → alpha.users`, change explorer selection to `beta`, then
  UPDATE/DELETE/INSERT from the alpha tab and assert the mutation affected
  **alpha only**. Any mutation against beta is a BLOCKER.
- **Regression:** full PRF-01 suite stays green (binding, discovery, migration,
  history/saved); browser release flow stays green.

### 27. Acceptance Criteria

1. Selecting a connection does not open a query editor.
2. Zero tabs renders an empty state.
3. Double-clicking a table opens a Table Data tab titled with the table name.
4. Table Data shows rows without user-written SQL.
5. Query Editor remains an explicit Query tab.
6. Table Data supports server-side pagination, filtering, sorting, refresh.
7. Filters use column/operator/value and are validated per data type.
8. Right-click table/database menus exist, capability-gated.
9. A typed `WorkspaceTab` union with the required fields is implemented.
10. Explorer selection never mutates existing tab bindings.
11. No frontend-generated arbitrary SQL for browsing; identifiers validated and
    values parameterized server-side.
12. Import/export implemented for CSV/SQL (XLSX only with an approved library)
    with the documented limits, NULL semantics, transactions, and formula guard.
13. DROP DATABASE requires typed confirmation, explicit target, correct session,
    and leaves bound tabs in a safe unavailable state with no fallback.
14. Capability matrix exposed and honest per engine.
15. PRF-01 guarantees and MySQL/SQLite behavior remain intact.
16. Safe row INSERT supported (metadata-driven columns, DEFAULT/NULL/value
    distinguishable, parameterized).
17. Safe row UPDATE supported (changed fields only, generated/read-only
    protected, PK policy documented, exactly one intended row).
18. Safe row DELETE supported with explicit confirmation showing row identity.
19. Duplicate Row supported through the reviewed INSERT flow (no immediate
    insert; identity/generated columns not copied unless insertable).
20. Update/Delete require a backend-verified safe row identity; composite PK
    supported; no-PK tables are not UPDATE/DELETE targets.
21. Generated/read-only columns cannot be mutated.
22. Mutation SQL is backend-generated and fully parameterized; no raw `WHERE`
    input is accepted anywhere.
23. Wrong-database mutation is impossible under tested PRF-01 binding guarantees.
24. Affected-row safety is verified (0 = not found, 1 = success, >1 = safety
    failure rolled back).
25. Concurrent/stale mutation behavior is documented and surfaces
    `ROW_CONFLICT`/`ROW_NOT_FOUND`.
26. CRUD errors are sanitized; mutation UI is explicit with no automatic
    cell-write behavior.
27. PG/MySQL/SQLite regression remains intact.

### 28. Explicit Non-Goals

No production code in this task; no authentication; no SSH tunneling; no cloud
sync; no bulk UPDATE/DELETE; no spreadsheet-style auto-save; no schema designer;
no `ALTER TABLE`; no `DROP TABLE`; no `CREATE DATABASE`; no change to PRF-01
database/tab binding; no replacement of selected technologies; no native CLI
tool assumptions.

## Table Row CRUD (PRF-02 Amendment)

### 29. Row CRUD Scope & UX Flows

Supported operations on a row: Insert, Update (Edit), Delete, Duplicate. All are
single-row; bulk mutations and cell-level auto-save are non-goals. A mutation is
allowed only when the table is writable **and** a safe identity is resolvable
(§30); otherwise the UI disables Update/Delete and explains why.

```
Add Row     → insert form → validate → Insert → refresh page
Edit Row    → form/drawer → validate → Save   → mutation → refresh row/page
Delete Row  → confirmation (row identity shown) → explicit mutation → refresh
Duplicate   → insert form (prefilled, identity/generated omitted) → Insert
```

The toolbar exposes `Add Row`, `Filter`, `Sort`, `Refresh`. The row context menu
exposes `Edit Row`, `Duplicate Row`, `Copy Row`, `Copy as JSON`, `Delete Row`.
No spreadsheet-style "write on every keystroke": a database write happens only
when the user confirms a dialog/drawer. A failed mutation must preserve the
tab's filters, sorting, page, and binding (§38).

### 30. Row Identity Model

Mutations need a **backend-verified** identity. The client may send identity
columns/values, but the backend independently validates them against introspected
metadata:

1. **Primary key** — preferred. Identity = all PK columns in order.
2. **Composite primary key** — supported; identity includes every PK column
   (e.g. `{order_id, product_id}`).
3. **Stable unique key** — only if a unique constraint/index is proven safe by
   metadata (single non-null unique column, or a unique index whose columns are
   all non-nullable) and the ADR explicitly approves it. Otherwise unavailable
   for v0.1.0 (**Needs Validation**).
4. **Otherwise mutation is unavailable** (read-only).

Hard rules:

- Never identify a row for UPDATE/DELETE using an arbitrary non-unique column.
- Never fall back to "all displayed values" as an implicit identity.
- `identity.columns` must equal the metadata-derived identity column set exactly;
  extra, missing, or renamed columns → `ROW_IDENTITY_INVALID`.
- The request `identity` is a hint; the backend derives the canonical identity
  set from metadata and rejects mismatches.

Resolution output is exposed to the UI in `table-data/query`
(`identity: {kind, columns} | null` and `row_mutable`), so the client cannot
invent one.

If no safe identity exists:

| Operation | Allowed? |
|---|---|
| Browse | yes |
| Filter | yes |
| Sort | yes |
| Export | yes |
| Insert | yes, if the target is insertable (no identity needed) |
| Duplicate | yes, as INSERT, if safe |
| Update | disabled |
| Delete | disabled |

### 31. TableDataTab CRUD UX

- Toolbar: `Add Row`, `Filter`, `Sort`, `Refresh`.
- Row context menu: `Edit Row`, `Duplicate Row`, `Copy Row`, `Copy as JSON`,
  `Delete Row`; disabled items carry an explanation (e.g. “No primary key”).
- Dialogs/drawers are explicit for v0.1.0 (no inline grid editing). Closing a
  dirty form asks for confirmation.
- View objects (view/matview/foreign) default to read-only unless engine
  metadata proves updatability and a safe identity exists.
- After any successful mutation: refetch only the bounded current page, keeping
  filter/sort/page state. No full-table reload.

### 32. Insert Row

Request (`POST .../table-data/rows`):

```json
{
  "database": "ccm", "schema": "public", "table": "users",
  "values": {
    "name": { "mode": "value", "value": "Alfian" },
    "nickname": { "mode": "null" },
    "created_at": { "mode": "default" }
  }
}
```

- Every provided value uses an explicit `mode`: `value` | `null` | `default`.
  Omitted keys are also treated as `default`. This preserves the
  DISTINCT/NULL/default distinction end-to-end; it is never collapsed.
- Column list is metadata-driven; unknown columns → `COLUMN_NOT_FOUND`.
- Generated/always-computed columns reject `value` (`COLUMN_READ_ONLY`);
  identity/autoincrement columns accept `default`/`null` per metadata but not an
  arbitrary value unless the engine allows explicit insert into them (documented
  per engine).
- Types are validated against column metadata (`INVALID_COLUMN_VALUE`).
- All values are bound parameters; DEFAULT is rendered as the dialect keyword,
  never as a client string.

Result behavior per engine (RETURNING is **not** assumed portable):

- **PostgreSQL:** `INSERT … RETURNING <identity/columns>` when available; return
  the created row.
- **MySQL:** execute insert, read `LAST_INSERT_ID()` for a single auto-increment
  key, then `SELECT` the row by identity. Composite/PK-less/default-identity
  inserts may return only the echoed submitted values plus affected rows.
- **SQLite:** `INSERT … RETURNING` on 3.35+, else `last_insert_rowid()` +
  keyed `SELECT`.
- Response: `{ row?: unknown[], affected_rows: 1 }`; the client refetches the
  page regardless.

### 33. Update Row

Request (`PATCH .../table-data/rows`):

```json
{
  "database": "ccm", "schema": "public", "table": "users",
  "identity": { "columns": { "id": 10 } },
  "changes": { "name": { "mode": "value", "value": "Alfian" } },
  "expected": { "name": { "mode": "value", "value": "Old" } }
}
```

- Only columns present in `changes` are updated (SQL sets only changed fields).
- Generated/read-only columns → `COLUMN_READ_ONLY`; identity/auto columns are
  read-only.
- **PK editing policy (v0.1.0):** primary-key columns are **read-only** in the
  Edit form. Changing a PK is a delete+insert semantically and is explicitly out
  of scope; a future safe cross-engine design may revisit this (Needs
  Validation).
- The UPDATE targets exactly the identity row. `expected` (original values) is
  optional but recommended for optimistic concurrency (§36); when supplied it is
  added to the predicate with bound parameters.
- Affected-row verification: `0` → `ROW_NOT_FOUND`/`ROW_CONFLICT` (stale),
  `1` → success, `>1` → `MUTATION_AFFECTED_MULTIPLE_ROWS` safety failure and
  rollback. A `>1` result is never reported as success.

### 34. Delete Row

Request (`DELETE .../table-data/rows`, identity in the body):

```json
{
  "database": "ccm", "schema": "public", "table": "users",
  "identity": { "columns": { "id": 10 } },
  "expected": { "name": { "mode": "value", "value": "Old" } }
}
```

- Requires explicit user confirmation that displays enough identity to
  understand what is deleted (identity column values, and for composite keys all
  of them).
- Backend DELETE uses the validated safe identity; no raw `WHERE`; never
  `DELETE … LIMIT 1` as a substitute for a real identity.
- Affected-row verification identical to Update (0/1/>1).
- Soft-delete is not assumed (it is a per-table behavior, not a generic feature).

### 35. Duplicate Row

- Duplicate is an INSERT derived from an existing row.
- It must **not** copy auto-increment identity values, generated, or computed
  columns unless the engine metadata explicitly marks them insertable.
- Clicking “Duplicate Row” opens a prefilled **Insert Row form**; it does not
  insert immediately. The user reviews/edits and confirms.
- The new row’s identity is engine-assigned; the original is untouched.

### 36. Concurrency / Stale Data

Bounded cross-engine v0.1.0 strategy:

- **Affected-row verification** is always performed for UPDATE/DELETE.
- **Original-value predicates (`expected`)** are supported for both; when
  provided, the DML predicate includes the original values, so a concurrent
  change yields 0 affected rows (`ROW_CONFLICT`).
- **Version/timestamp columns** (if present and known) may be added to the
  predicate as an engine-specific enhancement (Needs Validation).
- Detects: row deleted after browsing (`ROW_NOT_FOUND`), row changed after
  browsing (`ROW_CONFLICT`), identity changed externally (`ROW_CONFLICT` /
  `ROW_NOT_FOUND`).
- **Documented default:** without `expected`, a concurrent update is last-write-
  wins; this is explicitly surfaced in the UI and never silent. The UI can offer
  “compare original values” for safer writes.

### 37. Transactions

Each mutation:

```
BEGIN
  execute parameterised DML
  verify affected rows
  if verification fails: ROLLBACK  (when the engine allows)
COMMIT
```

- A single-row mutation is its own transaction. If affected-row safety
  validation fails, roll back where possible.
- `MUTATION_AFFECTED_MULTIPLE_ROWS` aborts and rolls back.
- MySQL caveat: DDL/implicit-commit statements do not participate in row DML,
  but row INSERT/UPDATE/DELETE are explicit; SQLite `BEGIN IMMEDIATE` is used
  where needed to avoid lock-upgrade surprises (Needs Validation).
- Mutations respect the existing statement timeout.

### 38. Mutation SQL Generation & Wrong-Database Safety

- SQL is generated **only** by the backend, reusing the §9 identifier
  validation/quoting and value-binding rules:
  `UPDATE <q schema>.<q table> SET <q col> = $n … WHERE <identity/expected>
  predicates`; `DELETE FROM … WHERE …`; `INSERT INTO … (cols) VALUES (…)`.
- No endpoint accepts a raw `WHERE` clause, raw SQL text, or a client table
  name outside the introspected `(connectionId, database)`.
- Wrong-database safety (PRF-01 preserved): the mutation target is the request’s
  explicit `connectionId`+`database`+`schema`+`table`. Explorer selection is
  **never** mutation authority and cannot redirect an existing `TableDataTab`.
  After a database is dropped, stale-tab mutations fail safely
  (`DATABASE_NOT_FOUND`) and must never fall back to another database.
- Errors are sanitized: no SQL text, no values, no DSN.

## Consequences

- DataDeck gains a real database-client surface while preserving explicit SQL.
- New endpoints, contract codes, capability fields, and one new frontend
  dependency (context menu) plus a possible Go XLSX dependency — each must be
  justified and reviewed.
- `closeTab`/initial-tab behavior changes are breaking for existing UI tests and
  must be migrated in the implementation tasks.
- Server-side filtering/sorting/pagination centralizes SQL generation in the
  backend, improving safety at the cost of new backend surface area.
- Row CRUD adds mutation surface: it is gated by backend-verified identity,
  affected-row verification, and per-engine RETURNING/affected-row differences;
  no-PK objects stay read-only, and wrong-database mutation is prevented by the
  PRF-01 binding.

## Alternatives Considered

- **Generate `SELECT` on the frontend and reuse `/query/execute`.** Rejected:
  reintroduces SQL-text browsing, weakens injection safety, and cannot express
  bounded paging/count semantics cleanly.
- **Client-side filter/sort/pagination only.** Rejected: cannot scale to
  millions of rows and would fetch unbounded data.
- **Keyset pagination in v0.1.0.** Deferred: more complex and incompatible with
  arbitrary page jumps; OFFSET is acceptable with an explicit depth guard.
- **Multi-column sort now.** Deferred; single-column keeps the API and UI
  minimal, multi-column is listed as Needs Validation.
- **Hand-rolled XLSX.** Rejected for untrusted input; a vetted pure-Go library
  is required (Needs Validation).
- **Right-click only (no hover actions).** Rejected: keep existing hover actions
  for discoverability, add context menus without removing them.
- **Client-generated UPDATE/DELETE SQL.** Rejected: would make the browser the
  authority for row identity and SQL safety; the backend must derive identity
  from metadata and bind all values.
- **Implicit identity from displayed values.** Rejected: unsafe, and can update
  or delete multiple rows.
- **Spreadsheet-style auto-save.** Rejected for v0.1.0: explicit dialogs avoid
  accidental writes and preserve auditability.
- **Universal `RETURNING`.** Rejected: not portable (MySQL); per-engine follow-up
  read by identity is used where needed.

## Constraints

- Preserve AGENTS.md rules: no silent architecture/contract changes, no new
  unjustified dependencies, no weakening tests, no credential exposure.
- Backend stays pure Go (`CGO_ENABLED=0`); no CGO-dependent XLSX library.
- Reuse `(connectionId, database)` pool identity and the existing timeout/result
  caps; do not weaken PRF-01.
- Endpoint additions require updating `docs/api-contract.md` and regenerating
  swagger before/with the implementation.

## Open Questions (Needs Validation)

1. XLSX library choice and license/footprint; whether XLSX ships in the first
   PRF-02 implementation wave.
2. Table-data `page_size` default (100 vs 50) and `maxOffset` ceiling.
3. Multi-column sorting and OR filter groups.
4. Whole-file vs batched transaction semantics for large imports, and the
   maximum import size.
5. `total` count strategy on very large tables (statement timeout vs
   approximate counts).
6. Dedicated per-table structure endpoint vs reusing `/schemas`.
7. DROP DATABASE privilege requirements per engine and whether MySQL needs a
   maintenance-database session.
8. Whether capability descriptor is a new endpoint or embedded in the
   connection response.
9. Multi-statement SQL import splitting rules per dialect.
10. Whether a single non-null stable unique key (absent a PK) is approved as a
    safe row identity for Update/Delete in v0.1.0.
11. Optimistic-concurrency scope: mandatory `expected` predicates vs opt-in, and
    whether version/timestamp columns are auto-detected.
12. PK-editing policy beyond v0.1.0 (insert+delete vs a safe cross-engine
    approach).
13. Whether Duplicate is allowed for tables without a safe identity (INSERT-only)
    and how prefilled defaults are chosen.
14. View updatability detection per engine (which metadata proves a view is
    writable).
15. Import default handling for CSV/XLSX and whether `default` mode is supported
    on import or only on manual Insert.

## Risks

- **HIGH if mishandled:** destructive DROP DATABASE, SQL import, and row
  mutations — mitigated by explicit targets, typed confirmation, capability
  gating, transactions, and affected-row safety checks.
- **HIGH if mishandled:** mass UPDATE/DELETE through a bad identity — mitigated
  by backend-derived identity, unique-key validation, and the >1-affected-row
  abort.
- **MEDIUM:** untrusted XLSX parsing and ZIP bombs — mitigated by a vetted
  library and hard expansion caps.
- **MEDIUM:** OFFSET deep scans / count cost — mitigated by depth guard and
  opt-in counts; keyset path documented.
- **MEDIUM:** engine divergence in RETURNING/affected-rows/generated-column
  semantics — mitigated by per-engine integration tests and documented behavior.
- **LOW:** new context-menu dependency; UI test churn from the empty-state
  change.

## Compatibility & Self-Review vs M6 / PRF-01

- **Wrong-database execution:** table-data requests and tabs carry explicit
  `connectionId` + `database`; explorer selection is not an input to SQL
  generation; default database resolution keeps PRF-01 precedence; deleted
  databases yield a safe unavailable state with no fallback.
- **SQL injection:** only introspected identifiers (quoted per dialect) reach the
  SQL text; all values are bound parameters; `LIKE` patterns escaped.
- **Destructive operations:** DROP DATABASE is never implicit, requires typed
  confirmation and an explicit target, and runs outside the target session where
  required.
- **Unbounded result loading:** page-bounded fetch, 50 MB cap, opt-in count,
  offset ceiling; exports/imports stream.
- **Untrusted file parsing:** strict limits, vetted XLSX library, formula
  injection guards, and transaction rollback.
- **PRF-01 preservation:** pool identity, discovery, `DATABASE_REQUIRED`/
  `DATABASE_NOT_FOUND`, history/saved-query attribution, migration 002, and
  MySQL/SQLite semantics are untouched.
- **Row CRUD safety:** identity is derived from PK/unique metadata and
  re-validated server-side; SQL is backend-generated with bound parameters; no
  raw `WHERE`; >1 affected rows aborts; generated/read-only columns are
  protected; stale/deleted rows surface `ROW_CONFLICT`/`ROW_NOT_FOUND`; explorer
  selection is never mutation authority and dropped-database tabs fail safely
  without fallback.
