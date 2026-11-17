import { describe, expect, it } from "vitest";

import {
  buildRowDraft,
  columnCategory,
  draftChanges,
  primaryKeyColumns,
  rowToRecord,
} from "./row-edit";
import type { TableColumnInfo } from "@/types/api";

const columns: TableColumnInfo[] = [
  { name: "id", database_type: "bigint", primary_key: true, updatable: false },
  { name: "name", database_type: "text", nullable: true, updatable: true },
  { name: "age", database_type: "integer", nullable: true, updatable: true },
  { name: "active", database_type: "boolean", nullable: false, updatable: true },
  { name: "meta", database_type: "jsonb", nullable: true, updatable: true },
];

describe("columnCategory", () => {
  it("maps common database types to editing categories", () => {
    expect(columnCategory("varchar(255)")).toBe("text");
    expect(columnCategory("text")).toBe("text");
    expect(columnCategory("bigint")).toBe("integer");
    expect(columnCategory("numeric(10,2)")).toBe("decimal");
    expect(columnCategory("boolean")).toBe("boolean");
    expect(columnCategory("date")).toBe("date");
    expect(columnCategory("timestamp with time zone")).toBe("timestamp");
    expect(columnCategory("jsonb")).toBe("json");
    expect(columnCategory("uuid")).toBe("uuid");
    expect(columnCategory("bytea")).toBe("binary");
    expect(columnCategory(undefined)).toBe("text");
  });
});

describe("primaryKeyColumns", () => {
  it("returns the declared primary-key columns", () => {
    expect(primaryKeyColumns(columns)).toEqual(["id"]);
  });
});

describe("buildRowDraft", () => {
  it("keeps NULL distinct, normalizes booleans, and stringifies JSON", () => {
    const record = rowToRecord(columns, ["42", null, 30, true, { a: 1 }]);
    const draft = buildRowDraft(columns, record);

    // Primary key is not part of the editable draft.
    expect(draft.id).toBeUndefined();
    // A nullable NULL stays NULL, not an empty string.
    expect(draft.name).toEqual({ mode: "null", value: "" });
    expect(draft.age).toEqual({ mode: "value", value: "30" });
    expect(draft.active).toEqual({ mode: "value", value: "true" });
    expect(draft.meta).toEqual({ mode: "value", value: '{"a":1}' });
  });
});

describe("draftChanges", () => {
  const original = rowToRecord(columns, ["42", "Alice", 30, true, { a: 1 }]);

  it("emits only changed columns with typed values", () => {
    const draft = buildRowDraft(columns, original);
    draft.name = { mode: "value", value: "Bob" };
    expect(draftChanges(columns, draft, original)).toEqual({ name: "Bob" });
  });

  it("emits null when a value is cleared to NULL", () => {
    const draft = buildRowDraft(columns, original);
    draft.name = { mode: "null", value: "" };
    expect(draftChanges(columns, draft, original)).toEqual({ name: null });
  });

  it("emits a boolean for a boolean change", () => {
    const draft = buildRowDraft(columns, original);
    draft.active = { mode: "value", value: "false" };
    expect(draftChanges(columns, draft, original)).toEqual({ active: false });
  });

  it("emits nothing when the draft matches the original", () => {
    const draft = buildRowDraft(columns, original);
    expect(draftChanges(columns, draft, original)).toEqual({});
  });

  it("does not emit a NULL change when the original was already NULL", () => {
    const nullOriginal = rowToRecord(columns, ["42", null, 30, true, null]);
    const draft = buildRowDraft(columns, nullOriginal);
    expect(draftChanges(columns, draft, nullOriginal)).toEqual({});
  });
});
