/**
 * Shared helpers for inline row editing (PRF-05). Pure functions used by the
 * virtualized grid and (optionally) dialogs. The backend remains authoritative
 * for validation; this only shapes drafts and diffs.
 */
import type { TableColumnInfo } from "@/types/api";

export type RowEditMode = "value" | "null";

export interface DraftField {
  mode: RowEditMode;
  value: string;
}

export type RowDraft = Record<string, DraftField>;

export type ColumnCategory =
  | "boolean"
  | "integer"
  | "decimal"
  | "date"
  | "timestamp"
  | "json"
  | "uuid"
  | "binary"
  | "text";

/** Coarse column category from the database type (drives the input control). */
export function columnCategory(dataType: string | undefined): ColumnCategory {
  const base = (dataType ?? "").toLowerCase().split(/[ (]/)[0];
  switch (base) {
    case "bool":
    case "boolean":
      return "boolean";
    case "smallint":
    case "integer":
    case "int":
    case "int2":
    case "int4":
    case "int8":
    case "bigint":
    case "serial":
    case "bigserial":
    case "mediumint":
    case "year":
      return "integer";
    case "numeric":
    case "decimal":
    case "real":
    case "double":
    case "float":
    case "float4":
    case "float8":
    case "money":
      return "decimal";
    case "date":
      return "date";
    case "timestamp":
    case "timestamptz":
    case "datetime":
      return "timestamp";
    case "json":
    case "jsonb":
      return "json";
    case "uuid":
    case "uniqueidentifier":
      return "uuid";
    case "bytea":
    case "blob":
    case "binary":
    case "varbinary":
      return "binary";
    default:
      return "text";
  }
}

export function updatableColumns(columns: TableColumnInfo[]): TableColumnInfo[] {
  return columns.filter((column) => column.updatable === true);
}

export function primaryKeyColumns(columns: TableColumnInfo[]): string[] {
  return columns
    .filter((column) => column.primary_key)
    .map((column) => column.name ?? "")
    .filter(Boolean);
}

/** Map a positional row to a column-name record. */
export function rowToRecord(
  columns: TableColumnInfo[],
  row: unknown[],
): Record<string, unknown> {
  return Object.fromEntries(
    columns.map((column, index) => [column.name ?? "", row[index]]),
  );
}

function isTruthyBoolean(value: unknown): boolean {
  return value === true || value === "true" || value === "t" || value === 1;
}

/** Build an editable draft from an original record. NULL stays NULL (distinct
 * from an empty string). */
export function buildRowDraft(
  columns: TableColumnInfo[],
  record: Record<string, unknown>,
): RowDraft {
  const draft: RowDraft = {};
  for (const column of updatableColumns(columns)) {
    const name = column.name ?? "";
    const value = record[name];
    if (value === null || value === undefined) {
      draft[name] = { mode: column.nullable ? "null" : "value", value: "" };
      continue;
    }
    const category = columnCategory(column.database_type);
    if (category === "boolean") {
      draft[name] = { mode: "value", value: isTruthyBoolean(value) ? "true" : "false" };
    } else {
      draft[name] = {
        mode: "value",
        value: typeof value === "object" ? JSON.stringify(value) : String(value),
      };
    }
  }
  return draft;
}

function comparableOriginal(
  column: TableColumnInfo,
  value: unknown,
): string | null {
  if (value === null || value === undefined) return null;
  if (columnCategory(column.database_type) === "boolean") {
    return isTruthyBoolean(value) ? "true" : "false";
  }
  return typeof value === "object" ? JSON.stringify(value) : String(value);
}

/**
 * Diff a draft against the original record, returning only changed columns with
 * typed values (NULL → null, boolean → true/false, others → string). Untouched
 * columns are never sent, so unrelated fields are not overwritten.
 */
export function draftChanges(
  columns: TableColumnInfo[],
  draft: RowDraft,
  original: Record<string, unknown>,
): Record<string, unknown> {
  const changes: Record<string, unknown> = {};
  for (const column of updatableColumns(columns)) {
    const name = column.name ?? "";
    const field = draft[name];
    if (!field) continue;
    const before = original[name];
    if (field.mode === "null") {
      if (before !== null && before !== undefined) changes[name] = null;
      continue;
    }
    if (comparableOriginal(column, before) === field.value) continue;
    changes[name] =
      columnCategory(column.database_type) === "boolean"
        ? field.value === "true"
        : field.value;
  }
  return changes;
}
