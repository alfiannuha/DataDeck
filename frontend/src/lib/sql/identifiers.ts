/**
 * Safe SQL identifier quoting for generated statements.
 *
 * The quote character is driver-specific: `"` for PostgreSQL/SQLite, `` ` ``
 * for MySQL. An embedded quote character is escaped by doubling it. Never build
 * SQL by raw concatenation of untrusted names.
 */
export function quoteIdentifier(identifier: string, quote = '"'): string {
  const escaped = identifier.split(quote).join(quote + quote);
  return `${quote}${escaped}${quote}`;
}

export function qualifyTable(
  schema: string,
  table: string,
  quote = '"',
): string {
  return schema
    ? `${quoteIdentifier(schema, quote)}.${quoteIdentifier(table, quote)}`
    : quoteIdentifier(table, quote);
}

/** Generate a `SELECT * … LIMIT n` statement for the given table. */
export function buildSelectTop100(
  schema: string,
  table: string,
  limit = 100,
  quote = '"',
): string {
  const safeLimit = Number.isFinite(limit) ? Math.max(1, Math.trunc(limit)) : 100;
  return `SELECT *\nFROM ${qualifyTable(schema, table, quote)}\nLIMIT ${safeLimit};`;
}

/** Escape a value for use as a single-quoted SQL string literal. */
export function quoteStringLiteral(value: string): string {
  return `'${value.split("'").join("''")}'`;
}

/** The identifier quote character for a driver (`"` by default). */
export function identifierQuoteFor(driver: string | null | undefined): string {
  return driver === "mysql" ? "`" : '"';
}

/** Generate `SELECT COUNT(*) FROM <namespace>.<table>` (namespace may be ""). */
export function buildCountRows(
  namespace: string,
  table: string,
  quote = '"',
): string {
  return `SELECT COUNT(*)\nFROM ${qualifyTable(namespace, table, quote)};`;
}

/** A statement whose result carries an engine-native DDL text column. */
export interface DdlQuery {
  sql: string;
  /** Position of the DDL text inside the first result row. */
  ddlColumnIndex: number;
}

/**
 * Whether accurate, engine-native DDL can be retrieved for a driver.
 *
 * PostgreSQL is intentionally unsupported: there is no native "SHOW CREATE"
 * statement and reconstructing DDL from catalog metadata would be incomplete,
 * so it is deferred rather than faked.
 */
export function supportsCopyDdl(driver: string | null | undefined): boolean {
  return driver === "mysql" || driver === "sqlite";
}

/**
 * Engine-native DDL retrieval statement.
 *
 * MySQL: `SHOW CREATE TABLE`; SQLite: the table's `sqlite_master.sql`. Returns
 * `null` for drivers without an accurate mechanism (PostgreSQL).
 */
export function buildCopyDdlQuery(
  driver: string | null | undefined,
  namespace: string,
  table: string,
): DdlQuery | null {
  if (driver === "mysql") {
    return {
      sql: `SHOW CREATE TABLE ${qualifyTable(namespace, table, "`")};`,
      ddlColumnIndex: 1,
    };
  }
  if (driver === "sqlite") {
    return {
      sql: `SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ${quoteStringLiteral(table)};`,
      ddlColumnIndex: 0,
    };
  }
  return null;
}

/** Extract the DDL text from a retrieval result row, or null when absent. */
export function extractDdl(
  rows: unknown[][] | undefined,
  ddlColumnIndex: number,
): string | null {
  const row = rows?.[0];
  if (!row) return null;
  const value = row[ddlColumnIndex];
  if (typeof value !== "string") return null;
  return value.trim() === "" ? null : value;
}
