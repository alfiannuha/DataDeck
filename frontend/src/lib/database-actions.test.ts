import { describe, expect, it } from "vitest";

import { getDatabaseActions } from "./database-actions";

describe("getDatabaseActions (PRF-02/T16/T17 database context menu)", () => {
  it("lists the actions in the documented order", () => {
    expect(getDatabaseActions().map((action) => action.id)).toEqual([
      "new-query",
      "refresh",
      "reconnect",
      "export-sql",
      "import-sql",
      "copy-name",
      "delete-database",
    ]);
  });

  it("enables navigation, reconnect, export and copy", () => {
    const actions = getDatabaseActions({ engine: "postgres" });
    for (const id of ["new-query", "refresh", "reconnect", "export-sql", "copy-name"]) {
      expect(actions.find((action) => action.id === id)?.disabled).toBe(false);
    }
  });

  it("enables Import SQL / Restore for PostgreSQL and SQLite", () => {
    for (const engine of ["postgres", "sqlite"]) {
      const action = getDatabaseActions({ engine }).find(
        (candidate) => candidate.id === "import-sql",
      );
      expect(action?.disabled).toBe(false);
    }
  });

  it("disables Import SQL / Restore for MySQL with an explanation", () => {
    const action = getDatabaseActions({ engine: "mysql" }).find(
      (candidate) => candidate.id === "import-sql",
    );
    expect(action?.disabled).toBe(true);
    expect(action?.reason).toMatch(/MySQL/);
  });

  it("never exposes Delete Database for SQLite", () => {
    const action = getDatabaseActions({ engine: "sqlite" }).find(
      (candidate) => candidate.id === "delete-database",
    );
    expect(action?.disabled).toBe(true);
    expect(action?.destructive).toBe(true);
  });

  it("enables Delete Database for PostgreSQL and MySQL", () => {
    for (const engine of ["postgres", "mysql"]) {
      const action = getDatabaseActions({ engine }).find(
        (candidate) => candidate.id === "delete-database",
      );
      expect(action?.disabled).toBe(false);
      expect(action?.destructive).toBe(true);
    }
  });
});
