/**
 * Conservative SQL risk analysis for the execution guard (PRF-05/H).
 *
 * This is intentionally conservative, not a full SQL parser. It strips
 * comments and string literals, splits statements, and classifies by leading
 * keyword + presence of a meaningful WHERE clause. It can UNDER-estimate edge
 * cases (e.g. always-true predicates written as string comparisons, dialect
 * extensions); the backend remains authoritative for execution. When unsure it
 * errs toward a higher level. Documented limitations:
 *   - only the first keyword of each statement drives the base class;
 *   - `WITH` bodies are scanned for DROP/TRUNCATE/unfiltered DELETE/UPDATE;
 *   - always-true detection covers `1=1` and `TRUE` (not every tautology).
 */
export type RiskLevel = "low" | "medium" | "high";

export interface SqlRisk {
  level: RiskLevel;
  reasons: string[];
  /** Require the user to type EXECUTE (very destructive statements). */
  requiresTypedConfirmation: boolean;
}

/** Remove comments and blank out string / quoted-identifier contents. */
function stripCommentsAndStrings(sql: string): string {
  let out = "";
  let i = 0;
  while (i < sql.length) {
    const c = sql[i];
    if (c === "-" && sql[i + 1] === "-") {
      while (i < sql.length && sql[i] !== "\n") i++;
    } else if (c === "/" && sql[i + 1] === "*") {
      i += 2;
      while (i < sql.length && !(sql[i] === "*" && sql[i + 1] === "/")) i++;
      i += 2;
    } else if (c === "'") {
      i++;
      while (i < sql.length) {
        if (sql[i] === "'" && sql[i + 1] === "'") i += 2;
        else if (sql[i] === "'") {
          i++;
          break;
        } else i++;
      }
      out += " '' ";
    } else if (c === '"') {
      i++;
      while (i < sql.length && sql[i] !== '"') i++;
      i++;
      out += ' "" ';
    } else if (c === "`") {
      i++;
      while (i < sql.length && sql[i] !== "`") i++;
      i++;
      out += " `` ";
    } else {
      out += c;
      i++;
    }
  }
  return out;
}

function hasAlwaysTrueWhere(s: string): boolean {
  const match = s.match(/\bwhere\b([\s\S]*)$/i);
  if (!match) return false;
  const clause = match[1]
    .replace(/[()]/g, " ")
    .replace(/\s+/g, " ")
    .trim()
    .toLowerCase();
  return /^1\s*=\s*1$/.test(clause) || /^(true|1=1|1 = 1)$/.test(clause);
}

export function analyzeSqlRisk(sql: string): SqlRisk {
  const clean = stripCommentsAndStrings(sql);
  const statements = clean
    .split(";")
    .map((statement) => statement.trim())
    .filter(Boolean);

  if (statements.length === 0) {
    return { level: "low", reasons: [], requiresTypedConfirmation: false };
  }

  let high = 0;
  let medium = 0;
  let requiresTyped = false;
  const reasons: string[] = [];

  for (const raw of statements) {
    const statement = raw.toLowerCase();
    const first = statement.match(/^[a-z]+/)?.[0] ?? "";

    switch (first) {
      case "select":
      case "explain":
      case "show":
      case "describe":
      case "desc":
      case "pragma":
      case "values":
      case "begin":
      case "commit":
      case "rollback":
      case "set":
        continue;
      case "insert":
      case "replace":
      case "create":
      case "grant":
      case "revoke":
      case "comment":
      case "merge":
      case "analyze":
      case "vacuum":
        medium++;
        continue;
      case "update":
      case "delete": {
        const hasWhere = /\bwhere\b/.test(statement);
        if (!hasWhere || hasAlwaysTrueWhere(statement)) {
          high++;
          requiresTyped = true;
          reasons.push(
            `${first.toUpperCase()} without a meaningful WHERE (modifies every row)`,
          );
        } else {
          medium++;
        }
        continue;
      }
      case "truncate":
        high++;
        requiresTyped = true;
        reasons.push("TRUNCATE removes all rows");
        continue;
      case "drop":
        high++;
        requiresTyped = true;
        reasons.push("DROP removes a database object");
        continue;
      case "alter":
        if (/\bdrop\b/.test(statement) || /\btruncate\b/.test(statement)) {
          high++;
          reasons.push("ALTER TABLE with a destructive clause");
        } else {
          medium++;
        }
        continue;
      case "with":
        if (/\b(drop|truncate)\b/.test(statement)) {
          high++;
          requiresTyped = true;
          reasons.push("Destructive statement inside a WITH/CTE");
        } else if (
          /\b(delete|update)\b/.test(statement) &&
          !/\bwhere\b/.test(statement)
        ) {
          high++;
          requiresTyped = true;
          reasons.push("CTE modifies rows without a WHERE");
        } else if (/\b(delete|update)\b/.test(statement)) {
          medium++;
        } else {
          continue;
        }
        continue;
      default:
        // Unknown / dialect-specific statement: err toward medium.
        medium++;
    }
  }

  if (high > 1) {
    reasons.push("Multiple high-risk statements in one execution");
    requiresTyped = true;
  }

  return {
    level: high > 0 ? "high" : medium > 0 ? "medium" : "low",
    reasons,
    requiresTypedConfirmation: requiresTyped,
  };
}
