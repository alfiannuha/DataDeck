import { describe, expect, it } from "vitest";

import { analyzeSqlRisk } from "./sql-risk";

describe("analyzeSqlRisk (PRF-05/H)", () => {
  it("treats read-only statements as low risk", () => {
    for (const sql of [
      "SELECT * FROM users",
      "EXPLAIN SELECT 1",
      "SHOW TABLES",
      "DESCRIBE users",
      "PRAGMA table_info(users)",
    ]) {
      expect(analyzeSqlRisk(sql).level).toBe("low");
    }
  });

  it("treats INSERT and filtered updates as medium risk", () => {
    expect(analyzeSqlRisk("INSERT INTO users (name) VALUES ('a')").level).toBe("medium");
    expect(analyzeSqlRisk("UPDATE users SET name='x' WHERE id=1").level).toBe("medium");
    expect(analyzeSqlRisk("DELETE FROM users WHERE id=1").level).toBe("medium");
  });

  it("flags unfiltered UPDATE/DELETE as high risk requiring EXECUTE", () => {
    for (const sql of ["UPDATE users SET admin=true", "DELETE FROM users", "DELETE FROM users WHERE 1=1"]) {
      const risk = analyzeSqlRisk(sql);
      expect(risk.level).toBe("high");
      expect(risk.requiresTypedConfirmation).toBe(true);
    }
  });

  it("flags TRUNCATE and DROP as high risk", () => {
    for (const sql of ["TRUNCATE TABLE users", "DROP TABLE users", "DROP DATABASE prod"]) {
      const risk = analyzeSqlRisk(sql);
      expect(risk.level).toBe("high");
      expect(risk.requiresTypedConfirmation).toBe(true);
    }
  });

  it("flags destructive ALTER and CTE drops as high risk", () => {
    expect(analyzeSqlRisk("ALTER TABLE users DROP COLUMN email").level).toBe("high");
    expect(analyzeSqlRisk("ALTER TABLE users ADD COLUMN x int").level).toBe("medium");
    expect(analyzeSqlRisk("WITH x AS (SELECT 1) DROP TABLE users").level).toBe("high");
  });

  it("ignores destructive keywords inside comments and string literals", () => {
    expect(analyzeSqlRisk("SELECT 'DROP TABLE users'").level).toBe("low");
    expect(analyzeSqlRisk("-- DELETE FROM users\nSELECT 1").level).toBe("low");
    expect(analyzeSqlRisk("/* DROP DATABASE prod */ SELECT 1").level).toBe("low");
  });

  it("uses the highest risk across multiple statements", () => {
    const risk = analyzeSqlRisk("DELETE FROM users; DROP TABLE t; INSERT INTO x VALUES (1)");
    expect(risk.level).toBe("high");
    expect(risk.reasons.some((reason) => reason.includes("Multiple"))).toBe(true);
  });
});
