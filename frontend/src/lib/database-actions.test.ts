import { describe, expect, it } from "vitest";

import { getDatabaseActions } from "./database-actions";

describe("getDatabaseActions (PRF-02/T16 database context menu)", () => {
  it("lists the T16 actions in the documented order", () => {
    expect(getDatabaseActions().map((action) => action.id)).toEqual([
      "new-query",
      "refresh",
      "reconnect",
      "import-sql",
      "export-sql",
      "copy-name",
    ]);
  });

  it("enables navigation/connection/copy actions", () => {
    const actions = getDatabaseActions();
    for (const id of ["new-query", "refresh", "reconnect", "copy-name"]) {
      expect(actions.find((action) => action.id === id)?.disabled).toBe(false);
    }
  });

  it("defers Import SQL and Export SQL (T17) with an explanation", () => {
    const actions = getDatabaseActions();
    for (const id of ["import-sql", "export-sql"]) {
      const action = actions.find((candidate) => candidate.id === id);
      expect(action?.disabled).toBe(true);
      expect(action?.reason).toMatch(/later update/);
    }
  });

  it("never exposes Delete Database (T18)", () => {
    expect(
      getDatabaseActions().some((action) => action.id.includes("delete")),
    ).toBe(false);
  });
});
