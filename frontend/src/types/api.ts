/**
 * Handwritten API contract layer.
 *
 * DTOs are derived from the generated OpenAPI types
 * (`src/types/generated/openapi.ts`, produced from backend/docs/swagger.json)
 * so they cannot drift from the backend contract. Only the generic envelope,
 * the error model and the error-code union are hand-maintained, because the
 * current OpenAPI document does not type the success envelope's `data` field.
 */

import type { components } from "@/types/generated/openapi";

type Schemas = components["schemas"];

/** Error object carried by the standard envelope. */
export interface ApiError {
  code: string;
  message: string;
  /** Present for SQL errors when the database reports a position. */
  position?: number;
}

/** Standard response envelope (docs/api-contract.md). */
export interface ApiEnvelope<T> {
  success: boolean;
  data: T;
  error: ApiError | null;
  meta: Record<string, unknown>;
}

/** Stable error codes defined by docs/api-contract.md. */
export type ErrorCode =
  | "VALIDATION_ERROR"
  | "SQL_SYNTAX_ERROR"
  | "SQL_ERROR"
  | "QUERY_TIMEOUT"
  | "QUERY_CANCELED"
  | "CONNECTION_ERROR"
  | "INTROSPECTION_ERROR"
  | "INTROSPECTION_TIMEOUT"
  | "NOT_FOUND"
  | "NOT_IMPLEMENTED"
  | "TABLE_NOT_FOUND"
  | "DATABASE_REQUIRED"
  | "DATABASE_NOT_FOUND"
  | "DATABASE_CONNECT_DENIED"
  | "BOOTSTRAP_DATABASE_UNAVAILABLE"
  | "DISCOVERY_TIMEOUT"
  | "PAYLOAD_TOO_LARGE"
  | "INTERNAL_ERROR";

/* --------------------------------------------------------------------------
 * Derived DTOs (see backend/docs/swagger.json)
 * ------------------------------------------------------------------------ */

export type HealthResponse = Schemas["api.HealthResponse"];

export type ConnectionResponse = Schemas["handler.ConnectionResponse"];
export type ConnectionCreateRequest = Schemas["handler.ConnectionRequest"];
export type ConnectionTestRequest = Schemas["handler.ConnectionTestRequest"];
export type TestConnectionResponse = Schemas["handler.TestConnectionResponse"];
export type DeleteConnectionResponse = Schemas["handler.DeleteConnectionResponse"];

export type QueryRequest = Schemas["handler.QueryRequest"];
export type QueryResult = Schemas["model.QueryResult"];
export type QueryColumn = Schemas["model.QueryColumn"];
export type QueryHistoryRecord = Schemas["handler.HistoryRecord"];

export type SavedQueryResponse = Schemas["handler.SavedQueryResponse"];
export type SavedQueryRequest = Schemas["handler.SavedQueryRequest"];

export type DatabaseSchemaTree = Schemas["model.Database"];
export type DatabaseInfo = Schemas["model.DatabaseInfo"];
export type TableDataPage = Schemas["model.TableDataPage"];
export type TableColumnInfo = Schemas["model.TableColumnInfo"];
export type RowCapabilities = Schemas["model.RowCapabilities"];
export type RowIdentityInfo = Schemas["model.RowIdentityInfo"];
export type RowMutationResult = Schemas["model.RowMutationResult"];
export type InsertValue = Schemas["model.InsertValue"];
export type IntrospectedSchema = Schemas["model.Schema"];
export type IntrospectedTable = Schemas["model.Table"];
export type IntrospectedColumn = Schemas["model.Column"];
export type IntrospectedPrimaryKey = Schemas["model.PrimaryKey"];
export type IntrospectedForeignKey = Schemas["model.ForeignKey"];
export type IntrospectedIndex = Schemas["model.Index"];
