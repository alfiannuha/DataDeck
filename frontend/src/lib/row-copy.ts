import { formatCellValue } from "@/lib/result-grid/format-cell";
import type { TableColumnInfo } from "@/types/api";

/**
 * Row copy helpers (PRF-02/T11 §7-8). Both operate on the canonical API row
 * snapshot + column metadata — never on rendered DOM text — so BIGINT stays
 * exact, NULL keeps its representation, and column order follows metadata.
 */

/** Tab-separated row values for spreadsheet/text paste. NULL renders as `NULL`. */
export function rowToTSV(columns: TableColumnInfo[], row: unknown[]): string {
  return columns
    .map((_, index) => formatCellValue(row[index]))
    .join("\t");
}

/**
 * JSON object keyed by column name using canonical API values: BIGINT remains
 * a string, NULL is null, booleans stay booleans. Columns without a name are
 * skipped rather than inventing keys.
 */
export function rowToJSON(columns: TableColumnInfo[], row: unknown[]): string {
  const object: Record<string, unknown> = {};
  columns.forEach((column, index) => {
    if (!column.name) return;
    const value = row[index];
    object[column.name] = value === undefined ? null : value;
  });
  return JSON.stringify(object, null, 2);
}
