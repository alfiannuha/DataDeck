/**
 * Result-cell value rendering.
 *
 * Values arrive from the backend already JSON-safe (BIGINT as string, BYTEA as
 * base64, JSON embedded). We only format for display; we never coerce BIGINT
 * strings back to numbers.
 */

// Cache compact JSON per value object so repeated renders (scroll/virtualize)
// do not re-stringify the same JSON cell.
const jsonCache = new WeakMap<object, string>();

export function isNullValue(value: unknown): boolean {
  return value === null || value === undefined;
}

/** Display text for a cell. NULL renders as an explicit marker. */
export function formatCellValue(value: unknown): string {
  if (isNullValue(value)) return "NULL";
  if (typeof value === "boolean") return value ? "true" : "false";
  if (typeof value === "string") return value;
  if (typeof value === "number") return String(value);
  if (typeof value === "object") {
    const cached = jsonCache.get(value as object);
    if (cached !== undefined) return cached;
    const text = JSON.stringify(value);
    jsonCache.set(value as object, text);
    return text;
  }
  return String(value);
}

/** Raw value for clipboard: preserves BIGINT precision and JSON structure. */
export function rawCellValue(value: unknown): string {
  if (isNullValue(value)) return "";
  if (typeof value === "string") return value;
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}
