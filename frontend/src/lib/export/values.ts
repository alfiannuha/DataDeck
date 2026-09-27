/**
 * Export value normalization.
 *
 * Values arrive from the backend already JSON-safe: BIGINT/DECIMAL are strings,
 * binary is base64, JSON is embedded. Export NEVER re-runs SQL and NEVER reads
 * values back from rendered DOM cells — it consumes the `QueryResult` directly.
 */

/** One cell as a CSV field string. */
export function csvCellValue(value: unknown): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "boolean") return value ? "true" : "false";
  if (typeof value === "number") return String(value);
  if (typeof value === "string") return value; // BIGINT/decimal/base64/timestamp stay exact
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

/** Quote/escape a CSV field per RFC 4180 (commas, quotes, newlines). */
export function csvEscape(cell: string): string {
  if (/[",\n\r]/.test(cell)) {
    return `"${cell.replace(/"/g, '""')}"`;
  }
  return cell;
}

/**
 * Column names, disambiguated so exports never silently overwrite a duplicate
 * column (e.g. `id, id` → `id, id_2`).
 */
export function disambiguateColumns(
  columns: { name?: string }[] | undefined,
): string[] {
  const seen = new Map<string, number>();
  return (columns ?? []).map((column, index) => {
    const base = (column.name ?? "").trim() || `column_${index + 1}`;
    const count = (seen.get(base) ?? 0) + 1;
    seen.set(base, count);
    return count === 1 ? base : `${base}_${count}`;
  });
}

/** JSON export representation of a cell: the value is already JSON-safe. */
export function jsonCellValue(value: unknown): unknown {
  return value === undefined ? null : value;
}

/** Leading characters spreadsheets interpret as a formula. */
const FORMULA_MARKERS = new Set(["=", "+", "@"]);

/**
 * Neutralize CSV formula injection for a cell.
 *
 * Policy (explicit): a text cell whose first character is `=`, `+`, `@`, or `-`
 * is prefixed with a single quote (`'`) so spreadsheets treat it as text. A
 * leading `-` followed by a digit is treated as a legitimate negative number or
 * BIGINT and is NOT prefixed, so numeric data is never corrupted. The prefix is
 * an intentional, documented transformation of formula-sensitive text.
 */
export function guardFormulaExport(cell: string): string {
  if (cell.length === 0) return cell;
  const first = cell[0];
  if (FORMULA_MARKERS.has(first)) {
    return `'${cell}`;
  }
  if (first === "-" && !/^-?\d/.test(cell)) {
    return `'${cell}`;
  }
  return cell;
}

