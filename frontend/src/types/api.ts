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
  | "EXPORT_FORMAT_UNSUPPORTED"
  | "EXPORT_LIMIT_EXCEEDED"
  | "IMPORT_FORMAT_UNSUPPORTED"
  | "IMPORT_VALIDATION_ERROR"
  | "IMPORT_TOO_LARGE"
  | "ROW_NOT_MUTABLE"
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
export type ReconnectRequest = Schemas["handler.ReconnectRequest"];
export type ReconnectResponse = Schemas["handler.ReconnectResponse"];
/** Canonical engine identifier derived from the connection profile schema. */
export type DatabaseEngine = NonNullable<ConnectionResponse["driver"]>;

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
export type TableExportRequest = Schemas["model.TableExportRequest"];
export type TableExportFormat = NonNullable<TableExportRequest["format"]>;
export type TableFilter = Schemas["model.TableFilter"];
export type FilterOperator = NonNullable<TableFilter["operator"]>;
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
export type TableStructure = Schemas["model.TableStructure"];
export type StructureColumn = Schemas["model.StructureColumn"];
export type StructureIndex = Schemas["model.StructureIndex"];
export type StructureConstraint = Schemas["model.StructureConstraint"];
export type StructureForeignKey = Schemas["model.StructureForeignKey"];

/* Table import (PRF-02/T15) */
export type TableImportSession = Schemas["model.TableImportSession"];
export type TableImportFormat = "csv" | "sql";
export type ImportCell = Schemas["model.ImportCell"];
export type ImportMapping = Schemas["model.ImportMapping"];
export type ImportMappingResolved = Schemas["model.ImportMappingResolved"];
export type TableImportValidation = Schemas["model.TableImportValidation"];
export type ImportValidationError = Schemas["model.ImportValidationError"];
export type TableImportExecutionResult = Schemas["model.TableImportExecutionResult"];
export type ImportValidationRequest = Schemas["handler.ImportValidationRequest"];
