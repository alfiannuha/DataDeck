import type { TableColumnInfo } from "@/types/api";

export type RowFieldMode = "value" | "null" | "default";

const INTEGER_TYPES = new Set([
  "smallint", "integer", "int", "int2", "int4", "int8", "bigint", "serial", "bigserial",
]);

/**
 * Conservative client-side validation for row-form fields (PRF-02/T11 §14).
 * Catches obvious mistakes before a request; the backend remains authoritative
 * and the full database type system is deliberately NOT duplicated here.
 *
 * Checks: NULL/DEFAULT legality against metadata, and value shape for integer,
 * UUID, JSON and boolean columns (empty input fails those checks naturally).
 * Temporal and decimal formats are left to the backend — engines accept more
 * shapes than a client-side regex should assume.
 */
export function validateRowField(
  column: TableColumnInfo,
  mode: RowFieldMode,
  value: string,
): string | null {
  const name = column.name ?? "column";

  if (mode === "null") {
    return column.nullable ? null : `${name}: NULL is not allowed.`;
  }

  if (mode === "default") {
    if (!column.nullable && !column.has_default) {
      return `${name}: a value is required — the column does not allow NULL and has no database default.`;
    }
    return null;
  }

  const base = (column.database_type ?? "").toLowerCase().split(/[( ]/)[0];
  if (INTEGER_TYPES.has(base)) {
    if (!/^-?\d+$/.test(value)) return `${name}: must be a whole number.`;
  } else if (base === "uuid") {
    if (
      !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value)
    ) {
      return `${name}: must be a UUID (8-4-4-4-12).`;
    }
  } else if (base === "json" || base === "jsonb") {
    try {
      JSON.parse(value);
    } catch {
      return `${name}: must be valid JSON.`;
    }
  } else if (base === "bool" || base === "boolean") {
    if (value !== "true" && value !== "false") {
      return `${name}: choose true or false.`;
    }
  }
  return null;
}
