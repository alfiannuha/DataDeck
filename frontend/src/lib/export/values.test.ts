import { describe, expect, it } from "vitest";

import {
  csvCellValue,
  csvEscape,
  disambiguateColumns,
  guardFormulaExport,
  jsonCellValue,
} from "./values";

describe("csvCellValue", () => {
  it("renders NULL as an empty field", () => {
    expect(csvCellValue(null)).toBe("");
    expect(csvCellValue(undefined)).toBe("");
  });

  it("renders booleans and numbers", () => {
    expect(csvCellValue(true)).toBe("true");
    expect(csvCellValue(false)).toBe("false");
    expect(csvCellValue(42)).toBe("42");
  });

  it("keeps BIGINT strings exact without numeric coercion", () => {
    expect(csvCellValue("9007199254740993")).toBe("9007199254740993");
  });

  it("renders JSON compactly", () => {
    expect(csvCellValue({ role: "admin" })).toBe('{"role":"admin"}');
  });
});

describe("csvEscape", () => {
  it("leaves plain fields untouched", () => {
    expect(csvEscape("hello")).toBe("hello");
  });

  it("quotes fields containing comma, quote or newline", () => {
    expect(csvEscape("a,b")).toBe('"a,b"');
    expect(csvEscape('say "hi"')).toBe('"say ""hi"""');
    expect(csvEscape("line1\nline2")).toBe('"line1\nline2"');
  });
});

describe("disambiguateColumns", () => {
  it("suffixes duplicate column names", () => {
    expect(
      disambiguateColumns([{ name: "id" }, { name: "name" }, { name: "id" }]),
    ).toEqual(["id", "name", "id_2"]);
  });

  it("falls back for unnamed columns", () => {
    expect(disambiguateColumns([{}, {}])).toEqual(["column_1", "column_2"]);
  });
});

describe("jsonCellValue", () => {
  it("normalizes undefined to null and preserves other values", () => {
    expect(jsonCellValue(undefined)).toBeNull();
    expect(jsonCellValue("9007199254740993")).toBe("9007199254740993");
    expect(jsonCellValue({ a: 1 })).toEqual({ a: 1 });
  });
});

describe("guardFormulaExport", () => {
  it("neutralizes formula-leading text", () => {
    expect(guardFormulaExport("=1+1")).toBe("'=1+1");
    expect(guardFormulaExport("+SUM(A1)")).toBe("'+SUM(A1)");
    expect(guardFormulaExport("@cmd")).toBe("'@cmd");
    expect(guardFormulaExport("-cmd|' /C calc'!A0")).toBe("'-cmd|' /C calc'!A0");
  });

  it("does not corrupt numbers or BIGINT strings", () => {
    expect(guardFormulaExport("42")).toBe("42");
    expect(guardFormulaExport("-5")).toBe("-5");
    expect(guardFormulaExport("-9223372036854775808")).toBe(
      "-9223372036854775808",
    );
    expect(guardFormulaExport("9007199254740993")).toBe("9007199254740993");
  });
});
