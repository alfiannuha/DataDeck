import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { DatabaseSchemaTree } from "@/types/api";

import { SchemaTree } from "./schema-tree";

const TABLE_COUNT = 1_000;

function table(name: string, schema = "") {
  return {
    schema,
    name,
    type: "BASE TABLE",
    columns: [{ name: "id", data_type: "integer" }],
  };
}

function databaseWithSchemaLevel(): DatabaseSchemaTree[] {
  return [
    {
      name: "app",
      schemas: [
        {
          name: "public",
          tables: Array.from({ length: TABLE_COUNT }, (_, i) =>
            table(`table_${i}`, "public"),
          ),
        },
      ],
    },
  ];
}

function databaseWithTableLevel(): DatabaseSchemaTree[] {
  return [
    {
      name: "main.db",
      tables: Array.from({ length: TABLE_COUNT }, (_, i) => table(`table_${i}`)),
    },
  ];
}

describe("SchemaExplorer large-schema rendering", () => {
  it("keeps schema-level tables lazy (collapsed schema)", () => {
    const started = performance.now();
    const { container, unmount } = render(
      <SchemaTree databases={databaseWithSchemaLevel()} connectionId="c1" />,
    );
    const elapsed = Math.round(performance.now() - started);
    const domNodes = container.querySelectorAll("li").length;

    process.stdout.write(
      `\n[schema] mode=schema-level tables=${TABLE_COUNT} domNodes=${domNodes} mountMs=${elapsed}`,
    );

    // The schema node is collapsed by default, so no table rows are mounted.
    expect(screen.queryByRole("button", { name: /^table_0/ })).toBeNull();
    expect(domNodes).toBeLessThan(10);
    unmount();
  });

  it("measures database-level table rendering (engines without a schema level)", () => {
    const started = performance.now();
    const { container, unmount } = render(
      <SchemaTree databases={databaseWithTableLevel()} connectionId="c1" />,
    );
    const elapsed = Math.round(performance.now() - started);
    const domNodes = container.querySelectorAll("li").length;

    process.stdout.write(
      `\n[schema] mode=table-level tables=${TABLE_COUNT} domNodes=${domNodes} mountMs=${elapsed}`,
    );

    // Rendering is bounded to one batch of tables rather than all 1000.
    expect(domNodes).toBeLessThan(200);
    expect(domNodes).toBeGreaterThan(0);
    unmount();
  });

  it("offers incremental rendering for large table-level schemas", { timeout: 20000 }, () => {
    render(
      <SchemaTree
        databases={databaseWithTableLevel()}
        connectionId="c1"
        onSelectTop100={() => {}}
      />,
    );
    const more = screen.getByRole("button", { name: /Show 100 more/ });
    expect(more).toBeInTheDocument();

    // Context actions remain available on rendered tables.
    expect(
      screen.getByRole("button", { name: "Select top 100 from table_0" }),
    ).toBeInTheDocument();

    fireEvent.click(more);
    expect(
      screen.getByRole("button", { name: "Select top 100 from table_199" }),
    ).toBeInTheDocument();
  });
});
