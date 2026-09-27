import { describe, expect, it } from "vitest";

import {
  buildCopyDdlQuery,
  buildCountRows,
  buildSelectTop100,
  extractDdl,
  identifierQuoteFor,
  qualifyTable,
  quoteIdentifier,
  quoteStringLiteral,
  supportsCopyDdl,
} from "./identifiers";

describe("quoteIdentifier", () => {
  it("wraps identifiers in double quotes", () => {
    expect(quoteIdentifier("users")).toBe('"users"');
  });

  it("escapes embedded double quotes by doubling them", () => {
    expect(quoteIdentifier('we"ird')).toBe('"we""ird"');
  });
});

describe("qualifyTable", () => {
  it("qualifies schema and table", () => {
    expect(qualifyTable("public", "users")).toBe('"public"."users"');
  });

  it("omits the schema when empty", () => {
    expect(qualifyTable("", "users")).toBe('"users"');
  });
});

describe("buildSelectTop100", () => {
  it("builds a safe LIMIT query with quoted identifiers", () => {
    expect(buildSelectTop100("public", "users")).toBe(
      'SELECT *\nFROM "public"."users"\nLIMIT 100;',
    );
  });

  it("sanitizes the limit and quotes hostile identifiers", () => {
    expect(buildSelectTop100("public", 'users"; DROP TABLE x; --', 100)).toBe(
      'SELECT *\nFROM "public"."users""; DROP TABLE x; --"\nLIMIT 100;',
    );
    expect(buildSelectTop100("public", "users", 0)).toContain("LIMIT 1;");
  });
});

describe("driver-specific quoting", () => {
  it("uses backticks for MySQL identifiers", () => {
    expect(quoteIdentifier("we`ird", "`")).toBe("`we``ird`");
    expect(qualifyTable("app", "users", "`")).toBe("`app`.`users`");
    expect(buildSelectTop100("app", "users", 100, "`")).toBe(
      "SELECT *\nFROM `app`.`users`\nLIMIT 100;",
    );
  });
});

describe("quoteStringLiteral", () => {
  it("escapes embedded single quotes", () => {
    expect(quoteStringLiteral("users")).toBe("'users'");
    expect(quoteStringLiteral("o'brien")).toBe("'o''brien'");
  });
});

describe("driver detection", () => {
  it("maps drivers to quote characters", () => {
    expect(identifierQuoteFor("postgres")).toBe('"');
    expect(identifierQuoteFor("sqlite")).toBe('"');
    expect(identifierQuoteFor("mysql")).toBe("`");
    expect(identifierQuoteFor(undefined)).toBe('"');
  });

  it("reports Copy DDL support honestly (PostgreSQL deferred)", () => {
    expect(supportsCopyDdl("mysql")).toBe(true);
    expect(supportsCopyDdl("sqlite")).toBe(true);
    expect(supportsCopyDdl("postgres")).toBe(false);
    expect(supportsCopyDdl(null)).toBe(false);
  });
});

describe("buildCountRows", () => {
  it("quotes schema-qualified PostgreSQL identifiers", () => {
    expect(buildCountRows("public", "users")).toBe(
      'SELECT COUNT(*)\nFROM "public"."users";',
    );
  });

  it("quotes MySQL database-qualified identifiers with backticks", () => {
    expect(buildCountRows("app", "users", "`")).toBe(
      "SELECT COUNT(*)\nFROM `app`.`users`;",
    );
  });

  it("leaves SQLite unqualified and escapes hostile names", () => {
    expect(buildCountRows("", "users")).toBe('SELECT COUNT(*)\nFROM "users";');
    expect(buildCountRows("", 'we"ird')).toBe(
      'SELECT COUNT(*)\nFROM "we""ird";',
    );
  });
});

describe("buildCopyDdlQuery", () => {
  it("uses SHOW CREATE TABLE for MySQL with backticks", () => {
    expect(buildCopyDdlQuery("mysql", "app", "users")).toEqual({
      sql: "SHOW CREATE TABLE `app`.`users`;",
      ddlColumnIndex: 1,
    });
  });

  it("uses sqlite_master for SQLite with an escaped literal", () => {
    expect(buildCopyDdlQuery("sqlite", "", "o'brien")).toEqual({
      sql: "SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'o''brien';",
      ddlColumnIndex: 0,
    });
  });

  it("returns null for PostgreSQL (deferred, never faked)", () => {
    expect(buildCopyDdlQuery("postgres", "public", "users")).toBeNull();
  });

  it("never concatenates unquoted identifiers", () => {
    const q = buildCopyDdlQuery("mysql", "app", 'users`; DROP TABLE x; --');
    expect(q?.sql).toBe("SHOW CREATE TABLE `app`.`users``; DROP TABLE x; --`;");
  });
});

describe("extractDdl", () => {
  it("extracts the DDL column and rejects empty/missing rows", () => {
    expect(extractDdl([["t", "CREATE TABLE t()"]], 1)).toBe("CREATE TABLE t()");
    expect(extractDdl([["CREATE TABLE t()"]], 0)).toBe("CREATE TABLE t()");
    expect(extractDdl([["t", ""]], 1)).toBeNull();
    expect(extractDdl([], 0)).toBeNull();
    expect(extractDdl(undefined, 0)).toBeNull();
    expect(extractDdl([[42]], 0)).toBeNull();
  });
});
