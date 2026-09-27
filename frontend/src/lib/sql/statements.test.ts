import { describe, expect, it } from "vitest";

import {
  activeStatement,
  executableSql,
  splitSqlStatements,
} from "./statements";

describe("splitSqlStatements", () => {
  it("splits simple statements and drops empty trailing ones", () => {
    const statements = splitSqlStatements("SELECT 1; SELECT 2;   ");
    expect(statements.map((s) => s.text)).toEqual(["SELECT 1", "SELECT 2"]);
  });

  it("does not split inside single-quoted strings", () => {
    const statements = splitSqlStatements("SELECT ';' AS a; SELECT 2;");
    expect(statements).toHaveLength(2);
    expect(statements[0].text).toBe("SELECT ';' AS a");
  });

  it("does not split inside dollar-quoted strings", () => {
    const statements = splitSqlStatements("SELECT $$ a;b $$; SELECT 2;");
    expect(statements.map((s) => s.text)).toEqual([
      "SELECT $$ a;b $$",
      "SELECT 2",
    ]);
  });

  it("does not split inside line comments", () => {
    const statements = splitSqlStatements("SELECT 1 -- ; not a split\n; SELECT 2;");
    expect(statements).toHaveLength(2);
    expect(statements[1].text).toBe("SELECT 2");
  });

  it("does not split inside (nested) block comments", () => {
    const statements = splitSqlStatements("SELECT 1 /* ; /* ; */ */; SELECT 2;");
    expect(statements).toHaveLength(2);
    expect(statements[1].text).toBe("SELECT 2");
  });

  it("reports trimmed offsets", () => {
    const [first] = splitSqlStatements("  SELECT 1  ;");
    expect(first.text).toBe("SELECT 1");
    expect("  SELECT 1  ;".slice(first.from, first.to)).toBe("SELECT 1");
  });
});

describe("activeStatement", () => {
  const sql = "SELECT 1;\nSELECT 2;";

  it("returns the statement containing the cursor", () => {
    expect(activeStatement(sql, 12)?.text).toBe("SELECT 2");
  });

  it("returns the first statement when the cursor is in it", () => {
    expect(activeStatement(sql, 2)?.text).toBe("SELECT 1");
  });

  it("returns null for an empty document", () => {
    expect(activeStatement("   ", 0)).toBeNull();
  });
});

describe("executableSql", () => {
  it("prefers a non-empty selection", () => {
    const sql = "SELECT 1; SELECT 2;";
    expect(executableSql(sql, { from: 10, to: 18 })).toBe("SELECT 2");
  });

  it("falls back to the active statement when there is no selection", () => {
    const sql = "SELECT 1; SELECT 2;";
    expect(executableSql(sql, { from: 12, to: 12 })).toBe("SELECT 2");
  });

  it("returns the whole document when there is no statement structure", () => {
    expect(executableSql("SELECT 1", { from: 0, to: 0 })).toBe("SELECT 1");
  });
});
