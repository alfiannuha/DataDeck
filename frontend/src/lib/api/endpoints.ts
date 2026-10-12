import {
  apiFetch,
  apiFetchDownload,
  apiFetchForm,
  apiFetchMeta,
  type DownloadResult,
  type Paginated,
} from "@/lib/api-client";
import type {
  ConnectionCreateRequest,
  DatabaseInfo,
  ConnectionResponse,
  ConnectionTestRequest,
  DatabaseImportPreview,
  DatabaseRestoreResult,
  DatabaseDeletePreflight,
  DatabaseDeleteResult,
  DatabaseSchemaTree,
  DeleteConnectionResponse,
  TableExportFormat,
  FilterOperator,
  HealthResponse,
  QueryHistoryRecord,
  QueryRequest,
  QueryResult,
  SavedQueryRequest,
  SavedQueryResponse,
  ReconnectRequest,
  ReconnectResponse,
  RowMutationResult,
  TableDataPage,
  TableExportRequest,
  TableImportExecutionResult,
  TableImportFormat,
  TableImportSession,
  TableImportValidation,
  TableStructure,
  TestConnectionResponse,
  ImportValidationRequest,
  ImportMapping,
} from "@/types/api";

/**
 * Typed operations for every existing backend endpoint. This is the only place
 * that knows endpoint paths; components/hooks call these functions.
 */

export function getHealth(signal?: AbortSignal): Promise<HealthResponse> {
  return apiFetch<HealthResponse>("/health", { signal });
}

export function listConnections(
  signal?: AbortSignal,
): Promise<ConnectionResponse[]> {
  return apiFetch<ConnectionResponse[]>("/connections", { signal });
}

export function createConnection(
  body: ConnectionCreateRequest,
): Promise<ConnectionResponse> {
  return apiFetch<ConnectionResponse>("/connections", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function testConnection(
  body: ConnectionTestRequest,
): Promise<TestConnectionResponse> {
  return apiFetch<TestConnectionResponse>("/connections/test", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function deleteConnection(
  connectionId: string,
): Promise<DeleteConnectionResponse> {
  return apiFetch<DeleteConnectionResponse>(
    `/connections/${encodeURIComponent(connectionId)}`,
    { method: "DELETE" },
  );
}

/**
 * Reconnect one database of a connection profile (PRF-02/T16). Database-scoped:
 * the backend replaces only the (connection, database) pool. The saved profile
 * and its credentials are reused; nothing is deleted and no SQL is executed.
 */
export function reconnectConnection(
  connectionId: string,
  database?: string | null,
): Promise<ReconnectResponse> {
  const body: ReconnectRequest = { database: database ?? undefined };
  return apiFetch<ReconnectResponse>(
    `/connections/${encodeURIComponent(connectionId)}/reconnect`,
    { method: "POST", body: JSON.stringify(body) },
  );
}

/**
 * Schema tree for a connection, optionally scoped to one database on a
 * server-level PostgreSQL connection (PRF-01).
 */
export function getSchemas(
  connectionId: string,
  options?: { database?: string; signal?: AbortSignal },
): Promise<DatabaseSchemaTree[]> {
  const query = options?.database
    ? `?database=${encodeURIComponent(options.database)}`
    : "";
  return apiFetch<DatabaseSchemaTree[]>(
    `/connections/${encodeURIComponent(connectionId)}/schemas${query}`,
    { signal: options?.signal },
  );
}

/** Selectable databases for a server-level connection (PostgreSQL; PRF-01). */
export function listDatabases(
  connectionId: string,
  signal?: AbortSignal,
): Promise<DatabaseInfo[]> {
  return apiFetch<DatabaseInfo[]>(
    `/connections/${encodeURIComponent(connectionId)}/databases`,
    { signal },
  );
}

export function executeQuery(
  body: QueryRequest,
  signal?: AbortSignal,
): Promise<QueryResult> {
  return apiFetch<QueryResult>("/query/execute", {
    method: "POST",
    body: JSON.stringify(body),
    signal,
  });
}

export interface ListParams {
  connectionId?: string;
  page?: number;
  pageSize?: number;
}

function listQuery(params: ListParams): string {
  const query = new URLSearchParams();
  if (params.connectionId) query.set("connection_id", params.connectionId);
  if (params.page) query.set("page", String(params.page));
  if (params.pageSize) query.set("page_size", String(params.pageSize));
  const qs = query.toString();
  return qs ? `?${qs}` : "";
}

export function getQueryHistory(
  params: ListParams = {},
  signal?: AbortSignal,
): Promise<Paginated<QueryHistoryRecord>> {
  return apiFetchMeta<QueryHistoryRecord>(
    `/query/history${listQuery(params)}`,
    { signal },
  );
}

export function listSavedQueries(
  params: ListParams = {},
  signal?: AbortSignal,
): Promise<Paginated<SavedQueryResponse>> {
  return apiFetchMeta<SavedQueryResponse>(
    `/queries/saved${listQuery(params)}`,
    { signal },
  );
}

export function createSavedQuery(
  body: SavedQueryRequest,
): Promise<SavedQueryResponse> {
  return apiFetch<SavedQueryResponse>("/queries/saved", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function updateSavedQuery(
  id: string,
  body: SavedQueryRequest,
): Promise<SavedQueryResponse> {
  return apiFetch<SavedQueryResponse>(
    `/queries/saved/${encodeURIComponent(id)}`,
    { method: "PUT", body: JSON.stringify(body) },
  );
}

export function deleteSavedQuery(
  id: string,
): Promise<DeleteConnectionResponse> {
  return apiFetch<DeleteConnectionResponse>(
    `/queries/saved/${encodeURIComponent(id)}`,
    { method: "DELETE" },
  );
}

export interface TableFilterParams {
  column: string;
  operator: FilterOperator;
  value?: unknown;
  values?: unknown[];
}

export interface TableDataParams {
  database?: string | null;
  schema?: string | null;
  table: string;
  page: number;
  pageSize: number;
  /** Structured, single-column server-side sort (never raw SQL). */
  sort?: { column: string; direction: "asc" | "desc" } | null;
  /** Structured, ANDed server-side filters (never raw SQL/WHERE). */
  filters?: TableFilterParams[];
}

/**
 * Bounded table data page (PRF-02). Sends only structured context — never SQL.
 * The backend resolves identifiers against metadata and generates the SELECT.
 */
export function browseTableData(
  connectionId: string,
  params: TableDataParams,
  signal?: AbortSignal,
): Promise<TableDataPage> {
  const query = new URLSearchParams();
  if (params.database) query.set("database", params.database);
  if (params.schema) query.set("schema", params.schema);
  query.set("table", params.table);
  query.set("page", String(params.page));
  query.set("page_size", String(params.pageSize));
  if (params.sort) {
    query.set("sort_column", params.sort.column);
    query.set("sort_direction", params.sort.direction);
  }
  if (params.filters && params.filters.length > 0) {
    query.set("filters", JSON.stringify(params.filters));
  }
  return apiFetch<TableDataPage>(
    `/connections/${encodeURIComponent(connectionId)}/table-data?${query.toString()}`,
    { signal },
  );
}

export interface TableStructureParams {
  database?: string | null;
  schema?: string | null;
  table: string;
}

/**
 * Read-only table structure (PRF-02/T13). Bound to the explicit
 * connection/database/schema/table; the backend never reads row data here.
 */
export function getTableStructure(
  connectionId: string,
  params: TableStructureParams,
  signal?: AbortSignal,
): Promise<TableStructure> {
  const query = new URLSearchParams();
  if (params.database) query.set("database", params.database);
  if (params.schema) query.set("schema", params.schema);
  query.set("table", params.table);
  return apiFetch<TableStructure>(
    `/connections/${encodeURIComponent(connectionId)}/table-structure?${query.toString()}`,
    { signal },
  );
}

export interface TableExportParams {
  database?: string | null;
  schema?: string | null;
  table: string;
  format: TableExportFormat;
  /** Structured sort/filters captured from the originating Table Data tab. */
  sort?: { column: string; direction: "asc" | "desc" } | null;
  filters?: TableFilterParams[];
  insertableOnly?: boolean;
  includeHeaders?: boolean;
  transaction?: boolean;
}

/**
 * Export a whole table (or the captured filter set) as a binary attachment
 * (PRF-02/T14). The target is always the explicit connection/database/schema/
 * table the caller supplies; the backend never falls back to the active tab or
 * Explorer selection. Errors arrive as the standard JSON envelope.
 */
export function exportTableData(
  connectionId: string,
  params: TableExportParams,
  signal?: AbortSignal,
): Promise<DownloadResult> {
  const body: TableExportRequest = {
    format: params.format,
    table: params.table,
  };
  if (params.database) body.database = params.database;
  if (params.schema) body.schema = params.schema;
  if (params.sort) body.sort = params.sort;
  if (params.filters && params.filters.length > 0) body.filters = params.filters;
  if (params.insertableOnly !== undefined) {
    body.insertable_only = params.insertableOnly;
  }
  if (params.includeHeaders !== undefined) {
    body.include_headers = params.includeHeaders;
  }
  if (params.transaction !== undefined) body.transaction = params.transaction;
  return apiFetchDownload(
    `/connections/${encodeURIComponent(connectionId)}/table-data/export`,
    { method: "POST", body: JSON.stringify(body), signal },
  );
}

export interface ImportTargetBody {
  /** Explicit target that must echo the upload binding (validate/execute). */
  database?: string | null;
  schema?: string | null;
  table: string;
  /** Optional explicit source → target mapping; omission auto-maps by name. */
  mapping?: ImportMapping[];
}

/** Database-level SQL export mode (PRF-02/T17). */
export type DatabaseExportMode = "schema_data" | "schema" | "data";

/**
 * Export a whole database as a dialect SQL script (PRF-02/T17). Self-contained
 * (no external tooling). The target is the explicit connection + database; the
 * backend never falls back to the Explorer selection. Errors arrive as the
 * standard JSON envelope.
 */
export function exportDatabase(
  connectionId: string,
  params: { database?: string | null; mode: DatabaseExportMode },
  signal?: AbortSignal,
): Promise<DownloadResult> {
  return apiFetchDownload(
    `/connections/${encodeURIComponent(connectionId)}/database-export`,
    {
      method: "POST",
      body: JSON.stringify({
        database: params.database ?? "",
        mode: params.mode,
      }),
      signal,
    },
  );
}

/**
 * Upload a DataDeck SQL export artifact for a preflighted restore (PRF-02/T17).
 * Nothing is written: the response is an executable preview bound to `token`.
 */
export function uploadDatabaseImport(
  connectionId: string,
  params: { database?: string | null; file: File },
  signal?: AbortSignal,
): Promise<DatabaseImportPreview> {
  const form = new FormData();
  form.append("file", params.file, params.file.name);
  form.append("database", params.database ?? "");
  return apiFetchForm<DatabaseImportPreview>(
    `/connections/${encodeURIComponent(connectionId)}/database-import`,
    form,
    signal,
  );
}

/** Execute a preflighted database restore (PRF-02/T17). */
export function executeDatabaseImport(
  connectionId: string,
  token: string,
): Promise<DatabaseRestoreResult> {
  return apiFetch<DatabaseRestoreResult>(
    `/connections/${encodeURIComponent(connectionId)}/database-import/${encodeURIComponent(token)}/execute`,
    { method: "POST" },
  );
}

/**
 * Preflight a database deletion (PRF-02/T18). Validates the target and returns
 * a short-lived, single-use confirmation token bound to the exact database.
 * Never runs DROP.
 */
export function deleteDatabasePreflight(
  connectionId: string,
  database: string,
): Promise<DatabaseDeletePreflight> {
  return apiFetch<DatabaseDeletePreflight>(
    `/connections/${encodeURIComponent(connectionId)}/databases/${encodeURIComponent(database)}/delete-preflight`,
    { method: "POST" },
  );
}

/**
 * Execute a database deletion (PRF-02/T18). The typed name and acknowledgment
 * are intent signals only; the backend re-validates the token, target and
 * database state, and reports success only after verifying the database is gone.
 */
export function deleteDatabase(
  connectionId: string,
  database: string,
  confirmation: {
    confirmationToken: string;
    typedDatabaseName: string;
    acknowledged: boolean;
  },
): Promise<DatabaseDeleteResult> {
  return apiFetch<DatabaseDeleteResult>(
    `/connections/${encodeURIComponent(connectionId)}/databases/${encodeURIComponent(database)}/delete`,
    {
      method: "POST",
      body: JSON.stringify({
        confirmation_token: confirmation.confirmationToken,
        typed_database_name: confirmation.typedDatabaseName,
        acknowledged: confirmation.acknowledged,
      }),
    },
  );
}

/**
 * Upload a CSV or SQL INSERT script for PRF-02/T15 import. The multipart body
 * carries the file plus its target binding; the response is a frozen session
 * (import_id, preview, columns) used by validate and execute.
 */
export function uploadTableImport(
  connectionId: string,
  params: {
    database?: string | null;
    schema?: string | null;
    table: string;
    format: TableImportFormat;
    csvHeader: boolean;
    file: File;
  },
  signal?: AbortSignal,
): Promise<TableImportSession> {
  const form = new FormData();
  form.append("file", params.file, params.file.name);
  form.append("database", params.database ?? "");
  if (params.schema) form.append("schema", params.schema);
  form.append("table", params.table);
  form.append("format", params.format);
  form.append("csv_header", String(params.csvHeader));
  return apiFetchForm<TableImportSession>(
    `/connections/${encodeURIComponent(connectionId)}/table-import`,
    form,
    signal,
  );
}

/**
 * Re-introspects the target and converts rows against the resolved mapping
 * without writing anything (PRF-02/T15). The returned mapping/errors drive the
 * import review step. The target must exactly echo the upload binding.
 */
export function validateTableImport(
  connectionId: string,
  importId: string,
  body: ImportTargetBody,
): Promise<TableImportValidation> {
  const request: ImportValidationRequest = {
    database: body.database ?? undefined,
    schema: body.schema ?? undefined,
    table: body.table,
    mapping: body.mapping,
  };
  return apiFetch<TableImportValidation>(
    `/connections/${encodeURIComponent(connectionId)}/table-import/${encodeURIComponent(importId)}/validate`,
    { method: "POST", body: JSON.stringify(request) },
  );
}

/**
 * Streams the spooled upload into one all-or-nothing transaction of
 * parameterized INSERTs (PRF-02/T15). A failure anywhere rolls the whole import
 * back. The target must echo the upload binding.
 */
export function executeTableImport(
  connectionId: string,
  importId: string,
  body: ImportTargetBody,
): Promise<TableImportExecutionResult> {
  const request: ImportValidationRequest = {
    database: body.database ?? undefined,
    schema: body.schema ?? undefined,
    table: body.table,
    mapping: body.mapping,
  };
  return apiFetch<TableImportExecutionResult>(
    `/connections/${encodeURIComponent(connectionId)}/table-import/${encodeURIComponent(importId)}/execute`,
    { method: "POST", body: JSON.stringify(request) },
  );
}

export interface InsertRowBody {
  database?: string | null;
  schema?: string | null;
  table: string;
  values: Record<string, { mode: "value" | "null" | "default"; value?: unknown }>;
}

/**
 * Inserts one row (PRF-02/T08). Sends structured {mode,value} per column — never
 * SQL/expressions. The backend validates metadata, binds values, and returns the
 * canonical row where the engine supports it.
 */
export function insertRow(
  connectionId: string,
  body: InsertRowBody,
): Promise<RowMutationResult> {
  return apiFetch<RowMutationResult>(
    `/connections/${encodeURIComponent(connectionId)}/table-data/rows`,
    { method: "POST", body: JSON.stringify(body) },
  );
}

export interface UpdateRowBody {
  database?: string | null;
  schema?: string | null;
  table: string;
  identity: Record<string, unknown>;
  expected?: Record<string, unknown>;
  changes: Record<string, unknown>;
}

/**
 * Updates one row (PRF-02/T09). Identity, expected and changes are structured;
 * no SQL/WHERE. The backend enforces optimistic concurrency and single-row
 * affected-row safety.
 */
export function updateRow(
  connectionId: string,
  body: UpdateRowBody,
): Promise<RowMutationResult> {
  return apiFetch<RowMutationResult>(
    `/connections/${encodeURIComponent(connectionId)}/table-data/rows`,
    { method: "PATCH", body: JSON.stringify(body) },
  );
}

export interface DeleteRowBody {
  database?: string | null;
  schema?: string | null;
  table: string;
  identity: Record<string, unknown>;
  expected?: Record<string, unknown>;
}

/**
 * Deletes one row (PRF-02/T10). Identity/expected are structured; no SQL/WHERE.
 * The backend enforces optimistic concurrency and single-row affected-row safety.
 */
export function deleteRow(
  connectionId: string,
  body: DeleteRowBody,
): Promise<RowMutationResult> {
  return apiFetch<RowMutationResult>(
    `/connections/${encodeURIComponent(connectionId)}/table-data/rows`,
    { method: "DELETE", body: JSON.stringify(body) },
  );
}
