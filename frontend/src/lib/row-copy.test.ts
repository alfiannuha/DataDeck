import { describe, expect, it } from "vitest";

import { rowsToJSON, rowToJSON } from "./row-copy";
import type { TableColumnInfo } from "@/types/api";

const columns: TableColumnInfo[] = [
  { name: "id", database_type: "bigint" },
  { name: "name", database_type: "text" },
  { name: "active", database_type: "boolean" },
  { name: "meta", database_type: "jsonb" },
];

describe("Copy as JSON", () => {
  it("serializes a single row keyed by column with canonical types", () => {
    const out = JSON.parse(rowToJSON(columns, ["9007199254740993", null, true, { a: 1 }]));
    expect(out).toEqual({
      id: "9007199254740993",
      name: null,
      active: true,
      meta: { a: 1 },
    });
    expect(typeof out.id).toBe("string");
    expect(out.active).toBe(true);
    expect(out.name).toBeNull();
  });

  it("serializes multiple rows as a JSON array", () => {
    const out = JSON.parse(
      rowsToJSON(columns, [
        ["1", "A", true, null],
        ["2", "B", false, { x: 2 }],
      ]),
    );
    expect(Array.isArray(out)).toBe(true);
    expect(out).toHaveLength(2);
    expect(out[0]).toEqual({ id: "1", name: "A", active: true, meta: null });
    expect(out[1]).toEqual({ id: "2", name: "B", active: false, meta: { x: 2 } });
  });

  it("maps undefined to null and keeps valid JSON", () => {
    const out = JSON.parse(rowsToJSON(columns, [["1", undefined, false, null]]));
    expect(out[0].name).toBeNull();
    expect(() => JSON.parse(rowsToJSON(columns, []))).not.toThrow();
  });
});
