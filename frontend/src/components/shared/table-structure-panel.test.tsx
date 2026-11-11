import { fireEvent, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { applyStructure, getTableStructure } from "@/lib/api/endpoints";
import { ApiClientError } from "@/lib/api-client";
import { renderWithProviders } from "@/test/render";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

import { TableStructurePanel } from "./table-structure-panel";

vi.mock("@/lib/api/endpoints", () => ({
  getTableStructure: vi.fn(),
  applyStructure: vi.fn(),
}));

const FULL_CAPS = {
  rename_table: true,
  add_column: true,
  drop_column: true,
  alter_column: true,
  create_index: true,
  drop_index: true,
  add_constraint: true,
  drop_constraint: true,
  partial_index: true,
  index_method: true,
};

function structureResult(overrides: Record<string, unknown> = {}) {
  return {
    database: "alpha",
    schema: "public",
    table: "users",
    object_type: "BASE TABLE",
    columns: [
      { name: "id", database_type: "bigint", normalized_type: "int8", nullable: false, primary_key: true, insertable: false, updatable: false },
      { name: "name", database_type: "text", normalized_type: "text", nullable: true, primary_key: false, insertable: true, updatable: true, default: null },
    ],
    primary_key: { name: "users_pkey", columns: ["id"] },
    indexes: [
      { name: "idx_users_name", unique: false, primary: false, method: "btree", partial: false, columns: ["name"] },
    ],
    constraints: [
      { name: "users_pkey", type: "PRIMARY KEY", columns: ["id"], definition: "<script>alert(1)</script>" },
    ],
    foreign_keys: [
      { name: "fk_users_team", columns: ["team_id"], referenced_schema: "public", referenced_table: "teams", referenced_columns: ["id"], on_update: "CASCADE", on_delete: "RESTRICT" },
    ],
    structure_capabilities: FULL_CAPS,
    ...overrides,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(getTableStructure).mockResolvedValue(structureResult() as never);
  useWorkspaceStore.setState({
    tabs: [
      {
        kind: "table-structure",
        id: "ts1",
        title: "users",
        connectionId: "c1",
        database: "alpha",
        schema: "public",
        table: "users",
      },
    ],
    activeTabId: "ts1",
  });
});

describe("TableStructurePanel", () => {
  it("requests the tab's explicit binding (never global state)", async () => {
    renderWithProviders(<TableStructurePanel />);

    await waitFor(() =>
      expect(getTableStructure).toHaveBeenCalledWith(
        "c1",
        expect.objectContaining({
          database: "alpha",
          schema: "public",
          table: "users",
        }),
        expect.anything(),
      ),
    );
  });

  it("renders the binding header and all structure sections", async () => {
    renderWithProviders(<TableStructurePanel />);

    expect(await screen.findByTestId("table-structure-panel")).toBeInTheDocument();
    expect(screen.getByTestId("table-structure-binding")).toHaveTextContent("alpha");
    expect(screen.getByText("public.users")).toBeInTheDocument();

    expect(await screen.findByTestId("table-structure-columns")).toBeInTheDocument();
    expect(screen.getByText("int8")).toBeInTheDocument();
    expect(screen.getByTestId("table-structure-pk")).toHaveTextContent("users_pkey: id");
    expect(screen.getByText("idx_users_name")).toBeInTheDocument();
  });

  it("renders constraint definition as plain text, never HTML", async () => {
    renderWithProviders(<TableStructurePanel />);

    expect(await screen.findByTestId("table-structure-constraints")).toBeInTheDocument();
    const definition = screen.getByText("<script>alert(1)</script>");
    expect(definition.tagName).toBe("TD");
    expect(document.querySelector("script")).toBeNull();
  });

  it("renders foreign keys with full reference details", async () => {
    renderWithProviders(<TableStructurePanel />);

    expect(await screen.findByTestId("table-structure-foreign-keys")).toBeInTheDocument();
    expect(screen.getByText("fk_users_team")).toBeInTheDocument();
    expect(screen.getByText("public.teams(id)")).toBeInTheDocument();
    expect(screen.getByText("CASCADE")).toBeInTheDocument();
    expect(screen.getByText("RESTRICT")).toBeInTheDocument();
  });

  it("shows a bounded loading state while the first load is pending", async () => {
    vi.mocked(getTableStructure).mockImplementation(() => new Promise(() => {}));
    renderWithProviders(<TableStructurePanel />);
    expect(await screen.findByTestId("table-structure-loading")).toBeInTheDocument();
  });

  it("shows a sanitized error with Retry that refetches", async () => {
    vi.mocked(getTableStructure).mockRejectedValueOnce(
      new ApiClientError("the requested table was not found", "TABLE_NOT_FOUND", 404),
    );
    renderWithProviders(<TableStructurePanel />);

    expect(await screen.findByTestId("table-structure-error")).toBeInTheDocument();
    expect(screen.getByText(/TABLE_NOT_FOUND/)).toBeInTheDocument();
    screen.getByRole("button", { name: "Retry" }).click();

    await waitFor(() => expect(getTableStructure).toHaveBeenCalledTimes(2));
  });

  it("marks PK columns with a badge and reads-only flags from the API", async () => {
    vi.mocked(getTableStructure).mockResolvedValue(
      structureResult({
        columns: [
          { name: "id", database_type: "bigint", normalized_type: "int8", nullable: false, primary_key: true, insertable: false, updatable: false },
          { name: "note", database_type: "text", normalized_type: "text", nullable: true, primary_key: false, insertable: true, updatable: true },
        ],
      }) as never,
    );
    renderWithProviders(<TableStructurePanel />);

    const columns = await screen.findByTestId("table-structure-columns");
    const rows = columns.querySelectorAll("tbody tr");
    expect(rows.length).toBe(2);
    expect(rows[0].textContent).toContain("PK");
    expect(rows[0].textContent).toContain("id");
    expect(rows[0].textContent).toContain("no");
    expect(rows[1].textContent).toContain("note");
    expect(rows[1].textContent).toContain("yes");
  });

  it("gates management controls on the engine capabilities", async () => {
    vi.mocked(getTableStructure).mockResolvedValue(
      structureResult({ structure_capabilities: {} }) as never,
    );
    renderWithProviders(<TableStructurePanel />);

    // Wait for the data to render, then assert no editing affordances exist.
    await screen.findByTestId("table-structure-columns");
    expect(screen.queryByTestId("structure-add-column")).not.toBeInTheDocument();
    expect(screen.queryByTestId("structure-rename-table")).not.toBeInTheDocument();
    expect(screen.queryByTestId("structure-create-index")).not.toBeInTheDocument();
    expect(screen.queryByTestId("structure-add-constraint")).not.toBeInTheDocument();
  });

  it("applies an add_column change through the dialog", async () => {
    vi.mocked(applyStructure).mockResolvedValue({ operation: "add_column", statements: 1 } as never);
    renderWithProviders(<TableStructurePanel />);

    fireEvent.click(await screen.findByTestId("structure-add-column"));
    fireEvent.change(screen.getByLabelText("Column name"), { target: { value: "status" } });
    fireEvent.change(screen.getByLabelText("Type"), { target: { value: "text" } });
    fireEvent.click(screen.getByRole("button", { name: "Apply" }));

    await waitFor(() =>
      expect(applyStructure).toHaveBeenCalledWith(
        "c1",
        expect.objectContaining({
          operation: "add_column",
          table: "users",
          schema: "public",
          column: expect.objectContaining({ name: "status", type: "text" }),
        }),
      ),
    );
  });

  it("drops a column after confirmation with the captured name", async () => {
    vi.mocked(applyStructure).mockResolvedValue({ operation: "drop_column", statements: 1 } as never);
    renderWithProviders(<TableStructurePanel />);

    fireEvent.click(await screen.findByTestId("structure-drop-column-name"));
    fireEvent.click(screen.getByRole("button", { name: "Apply change" }));

    await waitFor(() =>
      expect(applyStructure).toHaveBeenCalledWith(
        "c1",
        expect.objectContaining({ operation: "drop_column", column_name: "name" }),
      ),
    );
  });
});