import type { LanguageSupport } from "@codemirror/language";
import { MySQL, PostgreSQL, SQLite, sql } from "@codemirror/lang-sql";

/** SQL dialects DataDeck can edit. */
export type SqlDialect = "postgres" | "mysql" | "sqlite";

/**
 * Map a dialect id to its CodeMirror language support. Adding a dialect is a
 * new case here — the editor itself does not change.
 */
export function languageForDialect(dialect: SqlDialect = "postgres"): LanguageSupport {
  switch (dialect) {
    case "mysql":
      return sql({ dialect: MySQL, upperCaseKeywords: true });
    case "sqlite":
      return sql({ dialect: SQLite, upperCaseKeywords: true });
    case "postgres":
    default:
      return sql({ dialect: PostgreSQL, upperCaseKeywords: true });
  }
}
