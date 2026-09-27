import { describe, expect, it } from "vitest";

import type { DatabaseSchemaTree } from "@/types/api";

import {
  buildCompletions,
  extractSchemaCompletions,
  filterCompletions,
  keywordCompletions,
  PG_KEYWORDS,
} from "./autocomplete";

const schema: DatabaseSchemaTree[] = [
  {
    name: "app",
    schemas: [
      {
        name: "public",
        tables: [
          {
            schema: "public",
            name: "users",
            columns: [
              { name: "id", data_type: "bigint" },
              { name: "email", data_type: "character varying(255)" },
            ],
          },
          {
            schema: "public",
            name: "roles",
            columns: [{ name: "id", data_type: "bigint" }],
          },
        ],
      },
    ],
  },
];

describe("extractSchemaCompletions", () => {
  it("produces qualified and bare table/column names", () => {
    const labels = extractSchemaCompletions(schema).map((c) => c.label);
    expect(labels).toContain("users");
    expect(labels).toContain("public.users");
    expect(labels).toContain("users.email");
    expect(labels).toContain("email");
  });

  it("tags tables and columns with their type", () => {
    const completions = extractSchemaCompletions(schema);
    expect(completions.find((c) => c.label === "users")?.type).toBe("table");
    expect(completions.find((c) => c.label === "users.email")?.type).toBe(
      "column",
    );
  });

  it("deduplicates repeated column names", () => {
    const idEntries = extractSchemaCompletions(schema).filter(
      (c) => c.type === "column" && c.label === "id",
    );
    expect(idEntries).toHaveLength(1);
  });

  it("tolerates missing metadata", () => {
    expect(extractSchemaCompletions(undefined)).toEqual([]);
    expect(extractSchemaCompletions([])).toEqual([]);
    expect(extractSchemaCompletions([{ name: "empty" }])).toEqual([]);
  });
});

describe("completion filtering", () => {
  it("includes SQL keywords", () => {
    expect(PG_KEYWORDS).toContain("SELECT");
    expect(buildCompletions(undefined).some((c) => c.label === "SELECT")).toBe(
      true,
    );
  });

  it("filters case-insensitively by prefix", () => {
    expect(
      filterCompletions(buildCompletions(schema), "use").map((c) => c.label),
    ).toContain("users");
    expect(
      filterCompletions(buildCompletions(schema), "se").some(
        (c) => c.label === "SELECT",
      ),
    ).toBe(true);
  });

  it("returns nothing for an empty prefix", () => {
    expect(filterCompletions(buildCompletions(schema), "")).toEqual([]);
  });
});

describe("dialect keywords", () => {
  it("adds dialect-specific keywords", () => {
    const labels = (dialect: "postgres" | "mysql" | "sqlite") =>
      keywordCompletions(dialect).map((c) => c.label);
    expect(labels("postgres")).toContain("JSONB");
    expect(labels("mysql")).toContain("AUTO_INCREMENT");
    expect(labels("sqlite")).toContain("AUTOINCREMENT");
  });

  it("handles flat (database-level) table metadata", () => {
    const flat = [
      {
        name: "main",
        tables: [{ name: "widgets", columns: [{ name: "id" }, { name: "label" }] }],
      },
    ] as never;
    const labels = extractSchemaCompletions(flat).map((c) => c.label);
    expect(labels).toContain("widgets");
    expect(labels).toContain("widgets.label");
  });
});
