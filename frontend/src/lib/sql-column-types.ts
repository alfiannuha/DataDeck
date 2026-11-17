/**
 * Database-aware column type catalogue for the Add/Alter Column forms (PRF-05/L).
 * Types are grouped for a <select><optgroup>. The engine is derived from the
 * connection driver; unknown drivers fall back to PostgreSQL's list.
 */
export interface ColumnTypeGroup {
  label: string;
  types: string[];
}

const POSTGRES: ColumnTypeGroup[] = [
  { label: "Numeric", types: ["smallint", "integer", "bigint", "decimal", "numeric", "real", "double precision"] },
  { label: "String", types: ["varchar", "char", "text"] },
  { label: "Date & Time", types: ["date", "time", "timestamp", "timestamptz"] },
  { label: "Boolean", types: ["boolean"] },
  { label: "JSON", types: ["json", "jsonb"] },
  { label: "Binary", types: ["bytea"] },
  { label: "UUID", types: ["uuid"] },
];

const MYSQL: ColumnTypeGroup[] = [
  { label: "Numeric", types: ["tinyint", "smallint", "mediumint", "int", "bigint", "decimal", "float", "double"] },
  { label: "String", types: ["char", "varchar", "text", "mediumtext", "longtext"] },
  { label: "Date & Time", types: ["date", "datetime", "timestamp", "time", "year"] },
  { label: "Boolean", types: ["boolean"] },
  { label: "JSON", types: ["json"] },
  { label: "Binary", types: ["binary", "varbinary", "blob"] },
  { label: "Enumerations", types: ["enum", "set"] },
];

const SQLITE: ColumnTypeGroup[] = [
  { label: "Core", types: ["integer", "real", "text", "blob", "numeric"] },
];

export function columnTypeGroups(driver: string | undefined): ColumnTypeGroup[] {
  switch ((driver ?? "").toLowerCase()) {
    case "mysql":
      return MYSQL;
    case "sqlite":
      return SQLITE;
    default:
      return POSTGRES;
  }
}

const LENGTH_TYPES = new Set(["varchar", "char", "nvarchar", "nchar", "varbinary", "binary", "bit"]);
const PRECISION_TYPES = new Set(["numeric", "decimal", "number", "float", "double", "double precision"]);

/** Whether the base type takes a single length, e.g. varchar(255). */
export function typeTakesLength(base: string): boolean {
  return LENGTH_TYPES.has(base.trim().toLowerCase());
}

/** Whether the base type takes precision (and optional scale). */
export function typeTakesPrecision(base: string): boolean {
  return PRECISION_TYPES.has(base.trim().toLowerCase());
}

/** Compose `base(length)` or `base(precision[, scale])` from the form fields. */
export function composeColumnType(
  base: string,
  length: string,
  precision: string,
  scale: string,
): string {
  const name = base.trim();
  if (!name) return "";
  if (typeTakesLength(name) && length.trim()) {
    return `${name}(${length.trim()})`;
  }
  if (typeTakesPrecision(name) && precision.trim()) {
    const scalePart = scale.trim() ? `,${scale.trim()}` : "";
    return `${name}(${precision.trim()}${scalePart})`;
  }
  return name;
}

/** Strip a trailing `(…)` to recover the base type name. */
export function baseTypeName(dataType: string | undefined): string {
  return (dataType ?? "").replace(/\(.*\)\s*$/, "").trim().toLowerCase();
}
