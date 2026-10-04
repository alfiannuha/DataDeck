# ADR-010: Database Explorer & Table Data Management (PRF-02)

## Status

Proposed — UX and architecture locked for implementation planning. Several
parameters are marked **Needs Validation**; no production code is changed by
this ADR. Implementation begins only in PRF02-T01+.

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
    execution_time_ms: number
  }
  meta: { page, page_size, total?, has_more }
```

```
GET  /api/v1/connections/{id}/table-data/row-count?database=&schema=&table=&filters=
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

\* XLSX is advertised only after the library is validated (see §15); otherwise
`xlsx` is omitted and the UI hides it. SQLite “database” actions are limited to
the file/store; there is no logical database to create/drop.

### 19. PostgreSQL Behavior

Server-level multi-database from PRF-01 is preserved. `database` selects the
pool `(connectionId, database)`; table data for a database requires an explicit
or profile-default database and otherwise returns `DATABASE_REQUIRED`. `schema`
is meaningful (`public` etc.). Views/matviews/foreign tables are browsable when
introspected, read-only. DROP DATABASE must run from a maintenance database.

### 20. MySQL Behavior

Single database per profile (unchanged). `schema` is `null`; the catalog is the
profile database. Capabilities already mark `schemas=false`. Multi-statement SQL
import shares MySQL's implicit-commit caveats (documented). DROP DATABASE is
supported with sufficient privileges and is capability-gated.

### 21. SQLite Behavior

Single file; `schema` is `null` (or `main`). Writes serialize through the
existing single-connection pool (`SetMaxOpenConns(1)`), so a long import blocks
other operations — documented and covered by timeouts. `DROP DATABASE` is
unsupported. Table data/filter/sort/export/import are supported against the
connected file.

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

### 28. Explicit Non-Goals

No production code in this task; no authentication; no SSH tunneling; no cloud
sync; no table row editing; no `CREATE DATABASE`; no `ALTER TABLE`/schema
designer; no `DROP TABLE`; no change to PRF-01 database/tab binding; no
replacement of selected technologies; no native CLI tool assumptions.

## Consequences

- DataDeck gains a real database-client surface while preserving explicit SQL.
- New endpoints, contract codes, capability fields, and one new frontend
  dependency (context menu) plus a possible Go XLSX dependency — each must be
  justified and reviewed.
- `closeTab`/initial-tab behavior changes are breaking for existing UI tests and
  must be migrated in the implementation tasks.
- Server-side filtering/sorting/pagination centralizes SQL generation in the
  backend, improving safety at the cost of new backend surface area.

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

## Risks

- **HIGH if mishandled:** destructive DROP DATABASE and SQL import — mitigated by
  explicit targets, typed confirmation, capability gating, and transactions.
- **MEDIUM:** untrusted XLSX parsing and ZIP bombs — mitigated by a vetted
  library and hard expansion caps.
- **MEDIUM:** OFFSET deep scans / count cost — mitigated by depth guard and
  opt-in counts; keyset path documented.
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
