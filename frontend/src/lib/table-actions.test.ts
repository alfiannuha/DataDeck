import { describe, expect, it } from "vitest";

import { getTableActions, isReadOnlyObject } from "./table-actions";

describe("getTableActions (PRF-02/T12 capability descriptor)", () => {
  it("exposes all table actions in display order for a base table", () => {
    expect(
      getTableActions({ objectType: "BASE TABLE" }).map((action) => action.id),
    ).toEqual([
      "view-data",
      "new-query",
      "structure",
      "refresh",
      "export",
      "import",
      "copy-name",
      "copy-qualified",
    ]);
  });

  it("keeps browse/query/copy/export/import enabled for a writable table", () => {
    const actions = getTableActions({ objectType: "BASE TABLE" });
    for (const id of [
      "view-data",
      "new-query",
      "structure",
      "refresh",
      "export",
      "import",
      "copy-name",
      "copy-qualified",
    ]) {
      expect(actions.find((action) => action.id === id)?.disabled).toBe(false);
    }
    expect(
      actions.find((action) => action.id === "import")?.reason,
    ).toBeUndefined();
  });

  it("explains Import as read-only for views instead of enabling it", () => {
    for (const objectType of ["VIEW", "MATERIALIZED VIEW", "FOREIGN TABLE"]) {
      const importAction = getTableActions({ objectType }).find(
        (action) => action.id === "import",
      );
      expect(importAction?.disabled).toBe(true);
      expect(importAction?.reason).toBe("This object is read-only.");
      // Export is a read operation: it stays enabled for views too.
      const exportAction = getTableActions({ objectType }).find(
        (action) => action.id === "export",
      );
      expect(exportAction?.disabled).toBe(false);
    }
  });

  it("never marks read browse/query/copy actions disabled for views", () => {
    const actions = getTableActions({ objectType: "VIEW" });
    for (const id of ["view-data", "new-query", "structure", "refresh"]) {
      expect(actions.find((action) => action.id === id)?.disabled).toBe(false);
    }
  });
});

describe("isReadOnlyObject", () => {
  it("treats views, materialized views and foreign tables as read-only", () => {
    expect(isReadOnlyObject("VIEW")).toBe(true);
    expect(isReadOnlyObject("MATERIALIZED VIEW")).toBe(true);
    expect(isReadOnlyObject("FOREIGN TABLE")).toBe(true);
    expect(isReadOnlyObject("BASE TABLE")).toBe(false);
    expect(isReadOnlyObject("PARTITIONED TABLE")).toBe(false);
    expect(isReadOnlyObject(undefined)).toBe(false);
  });
});