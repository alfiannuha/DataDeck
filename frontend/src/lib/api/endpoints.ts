import { apiFetch, apiFetchMeta, type Paginated } from "@/lib/api-client";
import type {
  ConnectionCreateRequest,
  DatabaseInfo,
  ConnectionResponse,
  ConnectionTestRequest,
  DatabaseSchemaTree,
  DeleteConnectionResponse,
  HealthResponse,
  QueryHistoryRecord,
  QueryRequest,
  QueryResult,
  SavedQueryRequest,
  SavedQueryResponse,
  TestConnectionResponse,
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
