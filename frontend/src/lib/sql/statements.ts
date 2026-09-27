/**
 * SQL statement utilities.
 *
 * Statement boundaries are found with a small, comment- and string-aware
 * scanner (single quotes, double quotes, dollar-quoted strings, line comments
 * and nested block comments). It deliberately does NOT split on every semicolon,
 * which would break literals and comments.
 */

export interface SqlStatement {
  /** Trimmed statement text. */
  text: string;
  /** Offset of the first non-whitespace character. */
  from: number;
  /** Offset just past the last non-whitespace character. */
  to: number;
}

/** Offset just past a quoted string starting at `start` (quote char at start). */
function skipQuoted(sql: string, start: number): number {
  const quote = sql[start];
  let i = start + 1;
  while (i < sql.length) {
    if (sql[i] === quote) {
      if (sql[i + 1] === quote) {
        i += 2;
        continue;
      }
      return i + 1;
    }
    i++;
  }
  return sql.length;
}

/**
 * Offset just past a dollar-quoted string starting at `start`, or `start` when
 * `start` is not a dollar-quote opener. Handles `$$…$$` and `$tag$…$tag$`.
 */
function skipDollarQuoted(sql: string, start: number): number {
  const match = /^\$([A-Za-z_][A-Za-z0-9_]*)?\$/.exec(sql.slice(start));
  if (!match) return start;
  const tag = match[0];
  const end = sql.indexOf(tag, start + tag.length);
  return end === -1 ? sql.length : end + tag.length;
}

function skipLineComment(sql: string, start: number): number {
  const newline = sql.indexOf("\n", start);
  return newline === -1 ? sql.length : newline + 1;
}

/** Nested block comments (PostgreSQL allows nesting). */
function skipBlockComment(sql: string, start: number): number {
  let depth = 1;
  let i = start + 2;
  while (i < sql.length) {
    if (sql[i] === "/" && sql[i + 1] === "*") {
      depth++;
      i += 2;
    } else if (sql[i] === "*" && sql[i + 1] === "/") {
      depth--;
      i += 2;
      if (depth === 0) return i;
    } else {
      i++;
    }
  }
  return sql.length;
}

/** Split a script into non-empty statements with their source offsets. */
export function splitSqlStatements(sql: string): SqlStatement[] {
  const statements: SqlStatement[] = [];
  const push = (from: number, toExclusive: number) => {
    const raw = sql.slice(from, toExclusive);
    const leading = raw.length - raw.trimStart().length;
    const trailing = raw.length - raw.trimEnd().length;
    const text = raw.trim();
    if (text.length > 0) {
      statements.push({ text, from: from + leading, to: toExclusive - trailing });
    }
  };

  let start = 0;
  let i = 0;
  while (i < sql.length) {
    const ch = sql[i];
    const next = sql[i + 1];
    if (ch === "'" || ch === '"') {
      i = skipQuoted(sql, i);
      continue;
    }
    if (ch === "$") {
      const end = skipDollarQuoted(sql, i);
      i = end > i ? end : i + 1;
      continue;
    }
    if (ch === "-" && next === "-") {
      i = skipLineComment(sql, i);
      continue;
    }
    if (ch === "/" && next === "*") {
      i = skipBlockComment(sql, i);
      continue;
    }
    if (ch === ";") {
      push(start, i);
      start = i + 1;
      i++;
      continue;
    }
    i++;
  }
  push(start, sql.length);
  return statements;
}

/**
 * The statement containing the cursor, or the next statement when the cursor is
 * in trailing whitespace, or the last statement as a final fallback.
 */
export function activeStatement(
  sql: string,
  cursor: number,
): SqlStatement | null {
  const statements = splitSqlStatements(sql);
  if (statements.length === 0) return null;
  for (const statement of statements) {
    if (cursor >= statement.from && cursor <= statement.to) return statement;
  }
  for (const statement of statements) {
    if (cursor < statement.from) return statement;
  }
  return statements[statements.length - 1];
}

/**
 * Decide what to execute: the current selection when non-empty, otherwise the
 * active statement, otherwise the whole document.
 */
export function executableSql(
  sql: string,
  selection: { from: number; to: number },
): string {
  if (selection.from !== selection.to) {
    const selected = sql.slice(selection.from, selection.to).trim();
    if (selected.length > 0) return selected;
  }
  const statement = activeStatement(sql, selection.from);
  return statement ? statement.text : sql.trim();
}
