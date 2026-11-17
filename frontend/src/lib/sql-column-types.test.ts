import { describe, expect, it } from "vitest";

import {
  baseTypeName,
  columnTypeGroups,
  composeColumnType,
  typeTakesLength,
  typeTakesPrecision,
} from "./sql-column-types";

function allTypes(driver: string | undefined): string[] {
  return columnTypeGroups(driver).flatMap((group) => group.types);
}

describe("columnTypeGroups (engine-aware, PRF-05/L)", () => {
  it("returns engine-specific catalogues", () => {
    expect(allTypes("postgres")).toEqual(expect.arrayContaining(["uuid", "jsonb", "bytea"]));
    expect(allTypes("mysql")).toEqual(expect.arrayContaining(["enum", "set", "mediumint"]));
    expect(allTypes("sqlite")).toEqual(["integer", "real", "text", "blob", "numeric"]);
  });

  it("does not leak PostgreSQL types into MySQL/SQLite", () => {
    expect(allTypes("mysql")).not.toContain("jsonb");
    expect(allTypes("sqlite")).not.toContain("varchar");
  });

  it("falls back to PostgreSQL for an unknown driver", () => {
    expect(allTypes(undefined)).toContain("jsonb");
  });
});

describe("composeColumnType", () => {
  it("appends a length for length-bearing types", () => {
    expect(typeTakesLength("varchar")).toBe(true);
    expect(composeColumnType("varchar", "255", "", "")).toBe("varchar(255)");
    expect(composeColumnType("varchar", "", "", "")).toBe("varchar");
  });

  it("appends precision and optional scale for numeric types", () => {
    expect(typeTakesPrecision("numeric")).toBe(true);
    expect(composeColumnType("numeric", "", "10", "")).toBe("numeric(10)");
    expect(composeColumnType("numeric", "", "10", "2")).toBe("numeric(10,2)");
  });

  it("returns the bare base type when no params apply", () => {
    expect(composeColumnType("uuid", "10", "10", "2")).toBe("uuid");
    expect(composeColumnType("", "10", "", "")).toBe("");
  });
});

describe("baseTypeName", () => {
  it("strips a trailing parameter list", () => {
    expect(baseTypeName("varchar(255)")).toBe("varchar");
    expect(baseTypeName("numeric(10,2)")).toBe("numeric");
    expect(baseTypeName("uuid")).toBe("uuid");
    expect(baseTypeName(undefined)).toBe("");
  });
});
