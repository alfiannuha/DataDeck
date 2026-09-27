import type { SqlDialect } from "./dialect";
import type { DatabaseSchemaTree } from "@/types/api";

/** A single autocomplete candidate. */
export interface SqlCompletion {
  label: string;
  type: "keyword" | "table" | "column";
  detail?: string;
}

/** Keywords common to the supported SQL dialects. */
export const PG_KEYWORDS: string[] = [
  "SELECT", "FROM", "WHERE", "GROUP", "ORDER", "BY", "HAVING", "LIMIT",
  "OFFSET", "JOIN", "LEFT", "RIGHT", "INNER", "OUTER", "FULL", "CROSS",
  "ON", "USING", "UNION", "INTERSECT", "EXCEPT", "INSERT", "INTO", "VALUES",
  "UPDATE", "SET", "DELETE", "RETURNING", "WITH", "AS", "RECURSIVE",
  "DISTINCT", "ALL", "NULL", "NOT", "AND", "OR", "IN", "EXISTS", "BETWEEN",
  "LIKE", "ILIKE", "IS", "TRUE", "FALSE", "CASE", "WHEN", "THEN", "ELSE",
  "END", "CAST", "COALESCE", "COUNT", "SUM", "AVG", "MIN", "MAX", "NOW",
  "CURRENT_DATE", "CURRENT_TIMESTAMP", "CREATE", "ALTER", "DROP", "TABLE",
  "INDEX", "VIEW", "SCHEMA", "SEQUENCE", "PRIMARY", "KEY", "FOREIGN",
  "REFERENCES", "UNIQUE", "CHECK", "DEFAULT", "CONSTRAINT", "CASCADE",
  "RESTRICT", "BEGIN", "COMMIT", "ROLLBACK", "EXPLAIN", "ANALYZE", "VACUUM",
  "TRUNCATE", "GRANT", "REVOKE",
];

/** Dialect-specific keyword additions. */
const DIALECT_KEYWORDS: Record<SqlDialect, string[]> = {
  postgres: ["JSONB", "SERIAL", "CONFLICT", "DO", "NOTHING", "RENAME"],
  mysql: [
    "AUTO_INCREMENT", "UNSIGNED", "ENGINE", "INNODB", "SHOW", "DESCRIBE",
    "IFNULL", "REPLACE", "DUPLICATE", "MODIFY", "CHANGE",
  ],
  sqlite: [
    "AUTOINCREMENT", "PRAGMA", "ROWID", "WITHOUT", "ATTACH", "DETACH",
    "RAISE", "ABORT", "FAIL", "GLOB",
  ],
};

/**
 * Derive completion candidates from introspection data. Returns table and
 * column names, both qualified (`schema.table`, `table.column`) and bare, so
 * typing after a dot still resolves. Tolerates missing/partial metadata and
 * handles engines with different hierarchy depths (schemas vs database-level
 * tables).
 */
export function extractSchemaCompletions(
  databases: DatabaseSchemaTree[] | undefined,
): SqlCompletion[] {
  if (!databases) return [];
  const seen = new Set<string>();
  const out: SqlCompletion[] = [];
  const add = (label: string, type: SqlCompletion["type"], detail?: string) => {
    if (!label) return;
    const key = `${type}:${label}`;
    if (seen.has(key)) return;
    seen.add(key);
    out.push({ label, type, detail });
  };

  for (const database of databases) {
    // Engines without a schema level place tables directly on the database.
    for (const table of database.tables ?? []) {
      const tableName = table.name ?? "";
      add(tableName, "table", database.name);
      for (const column of table.columns ?? []) {
        const columnName = column.name ?? "";
        if (!columnName) continue;
        add(`${tableName}.${columnName}`, "column", tableName);
        add(columnName, "column", column.data_type ?? tableName);
      }
    }
    for (const schema of database.schemas ?? []) {
      const schemaName = schema.name ?? "";
      for (const table of schema.tables ?? []) {
        const tableName = table.name ?? "";
        add(tableName ? `${schemaName}.${tableName}` : schemaName, "table", schemaName);
        add(tableName, "table", schemaName);
        for (const column of table.columns ?? []) {
          const columnName = column.name ?? "";
          if (!columnName) continue;
          add(`${tableName}.${columnName}`, "column", tableName);
          add(columnName, "column", column.data_type ?? tableName);
        }
      }
    }
  }
  return out;
}

/** Keywords for a dialect (common set plus dialect-specific additions). */
export function keywordCompletions(
  dialect: SqlDialect = "postgres",
): SqlCompletion[] {
  const combined = new Set([...PG_KEYWORDS, ...(DIALECT_KEYWORDS[dialect] ?? [])]);
  return [...combined].map((label) => ({ label, type: "keyword" as const }));
}

/** Keywords plus connection-scoped schema completions for the dialect. */
export function buildCompletions(
  databases: DatabaseSchemaTree[] | undefined,
  dialect: SqlDialect = "postgres",
): SqlCompletion[] {
  return [...keywordCompletions(dialect), ...extractSchemaCompletions(databases)];
}

/** Case-insensitive prefix filter; an empty prefix yields nothing. */
export function filterCompletions(
  completions: SqlCompletion[],
  prefix: string,
): SqlCompletion[] {
  const needle = prefix.trim().toLowerCase();
  if (!needle) return [];
  return completions.filter((completion) =>
    completion.label.toLowerCase().startsWith(needle),
  );
}
