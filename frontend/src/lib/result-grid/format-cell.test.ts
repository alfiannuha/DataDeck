import { describe, expect, it } from "vitest";

import {
  formatCellValue,
  isNullValue,
  rawCellValue,
} from "./format-cell";

describe("formatCellValue", () => {
  it("renders NULL explicitly", () => {
    expect(formatCellValue(null)).toBe("NULL");
    expect(formatCellValue(undefined)).toBe("NULL");
    expect(isNullValue(null)).toBe(true);
  });

  it("renders booleans and numbers", () => {
    expect(formatCellValue(true)).toBe("true");
    expect(formatCellValue(false)).toBe("false");
    expect(formatCellValue(42)).toBe("42");
  });

  it("preserves BIGINT strings without numeric coercion", () => {
    const bigint = "9007199254740993";
    const formatted = formatCellValue(bigint);
    expect(formatted).toBe(bigint);
    expect(typeof formatted).toBe("string");
  });

  it("renders JSON compactly", () => {
    expect(formatCellValue({ role: "admin" })).toBe('{"role":"admin"}');
    expect(formatCellValue([1, 2, 3])).toBe("[1,2,3]");
  });
});

describe("rawCellValue", () => {
  it("returns empty string for NULL", () => {
    expect(rawCellValue(null)).toBe("");
  });

  it("keeps the exact BIGINT string", () => {
    expect(rawCellValue("9007199254740993")).toBe("9007199254740993");
  });

  it("keeps JSON structure", () => {
    expect(rawCellValue({ a: 1 })).toBe('{"a":1}');
  });
});
