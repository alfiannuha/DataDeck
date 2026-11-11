import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { SchemaTree, TreeRow } from "./schema-tree";

const LONG_TABLE = "actual_port_loading_detail_transaction_history";

function treeWithLongTable() {
  return [
    {
      name: "public",
      tables: [
        {
          schema: "public",
          name: LONG_TABLE,
          type: "BASE TABLE",
          columns: [{ name: "id", data_type: "integer" }],
        },
      ],
    },
  ] as never;
}

describe("SchemaTree table labels (full names, wrapping)", () => {
  it("renders the full table name without ellipsis truncation", () => {
    render(<SchemaTree databases={treeWithLongTable()} connectionId="c1" />);
    const label = screen.getByTitle(LONG_TABLE);
    expect(label).toHaveTextContent(LONG_TABLE);
    // Wraps instead of truncating.
    expect(label.className).not.toContain("truncate");
    expect(label.className).toContain("break-words");
    expect(label.className).toContain("whitespace-normal");
  });

  it("shows an object-type icon instead of a redundant type label", () => {
    render(<SchemaTree databases={treeWithLongTable()} connectionId="c1" />);
    // PRF-04/Phase 5: the type label is replaced by a leading icon.
    expect(screen.queryByText("BASE TABLE")).not.toBeInTheDocument();
    expect(document.querySelector("svg.lucide-table-2")).toBeInTheDocument();
  });

  it("keeps expand/collapse working with a long name", () => {
    render(<SchemaTree databases={treeWithLongTable()} connectionId="c1" />);
    const button = screen.getByRole("button", {
      name: new RegExp("^" + LONG_TABLE),
    });
    expect(button).toHaveAttribute("aria-expanded", "false");
    fireEvent.click(button);
    expect(button).toHaveAttribute("aria-expanded", "true");
  });
});

describe("TreeRow label rendering", () => {
  it("wraps long labels and exposes the full name via title", () => {
    render(
      <TreeRow
        depth={0}
        label={LONG_TABLE}
        expandable
        open={false}
        onToggle={() => {}}
      />,
    );
    const label = screen.getByTitle(LONG_TABLE);
    expect(label).toHaveTextContent(LONG_TABLE);
    expect(label.className).not.toContain("truncate");
    expect(label.className).toContain("break-words");
  });

  it("marks the selected row", () => {
    render(
      <TreeRow
        depth={0}
        label="alpha"
        expandable
        open={false}
        selected
        onToggle={() => {}}
      />,
    );
    expect(screen.getByTitle("alpha").closest("[aria-current]")).toHaveAttribute(
      "aria-current",
      "true",
    );
  });
});
