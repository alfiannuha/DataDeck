import { fireEvent, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { browseTableData, deleteRow, insertRow, updateRow } from "@/lib/api/endpoints";
import { ApiClientError } from "@/lib/api-client";
import { renderWithProviders } from "@/test/render";
import { useWorkspaceStore, type TableDataTab } from "@/store/useWorkspaceStore";

import { TableDataView } from "./table-data-view";

vi.mock("@/lib/api/endpoints", () => ({
  browseTableData: vi.fn(),
  insertRow: vi.fn(),
  updateRow: vi.fn(),
  deleteRow: vi.fn(),
}));

function tableTab(overrides: Partial<TableDataTab> = {}): TableDataTab {
  return {
    kind: "table-data",
    id: "td1",
    title: "users",
    connectionId: "c1",
    database: "alpha",
    schema: "public",
    table: "users",
    filters: [],
    sort: [],
    page: 1,
    pageSize: 100,
    ...overrides,
  };
}

function pageResult(overrides: Record<string, unknown> = {}) {
  return {
    database: "alpha",
    schema: "public",
    table: "users",
    object_type: "BASE TABLE",
    columns: [
      { name: "id", database_type: "bigint", primary_key: true, nullable: false, insertable: false, updatable: false },
      { name: "name", database_type: "text", nullable: true },
    ],
    rows: [["1", "Alfie"]],
    pagination: { page: 1, page_size: 100, has_more: false },
    truncated: false,
    ...overrides,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(browseTableData).mockResolvedValue(pageResult() as never);
  useWorkspaceStore.setState({ tabs: [tableTab()], activeTabId: "td1" });
});

/** Opens the row menu for the 1-based row (Radix trigger opens on ArrowDown). */
function openRowMenu(rowNumber: number) {
  fireEvent.keyDown(screen.getByTestId(`row-menu-trigger-${rowNumber}`), {
    key: "ArrowDown",
  });
}

describe("TableDataView", () => {
  it("requests the tab's explicit binding (never global state)", async () => {
    renderWithProviders(<TableDataView />);

    await waitFor(() =>
      expect(browseTableData).toHaveBeenCalledWith(
        "c1",
        expect.objectContaining({
          database: "alpha",
          schema: "public",
          table: "users",
          page: 1,
          pageSize: 100,
        }),
        expect.anything(),
      ),
    );
  });

  it("shows a bounded loading state while the first page loads", async () => {
    vi.mocked(browseTableData).mockImplementation(() => new Promise(() => {}));
    renderWithProviders(<TableDataView />);
    expect(await screen.findByTestId("table-data-loading")).toBeInTheDocument();
  });

  it("renders dynamic columns, exact BIGINT and NULL", async () => {
    vi.mocked(browseTableData).mockResolvedValue(
      pageResult({
        columns: [
          { name: "id", database_type: "bigint", primary_key: true },
          { name: "name", database_type: "text", nullable: true },
          { name: "big", database_type: "bigint", nullable: true },
        ],
        rows: [
          ["1", "Alfie", "9007199254740993"],
          ["2", null, null],
        ],
      }) as never,
    );
    renderWithProviders(<TableDataView />);

    expect(await screen.findByRole("columnheader", { name: "id" })).toBeInTheDocument();
    expect(screen.getByText("9007199254740993")).toBeInTheDocument();
    expect(screen.getAllByText("NULL").length).toBeGreaterThan(0);
    expect(screen.getByRole("grid", { name: "Table data grid" })).toBeInTheDocument();
  });

  it("shows an explicit empty state for a valid table with no rows", async () => {
    vi.mocked(browseTableData).mockResolvedValue(pageResult({ rows: [] }) as never);
    renderWithProviders(<TableDataView />);
    expect(await screen.findByText("No rows found.")).toBeInTheDocument();
  });

  it("shows a sanitized error with Retry that refetches", async () => {
    vi.mocked(browseTableData).mockRejectedValueOnce(
      new ApiClientError("the requested table was not found", "TABLE_NOT_FOUND", 404),
    );
    renderWithProviders(<TableDataView />);

    const error = await screen.findByTestId("table-data-error");
    expect(error).toHaveTextContent("TABLE_NOT_FOUND");
    expect(error).not.toHaveTextContent(/password|dsn/i);

    vi.mocked(browseTableData).mockResolvedValue(pageResult() as never);
    const before = vi.mocked(browseTableData).mock.calls.length;
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() =>
      expect(vi.mocked(browseTableData).mock.calls.length).toBeGreaterThan(before),
    );
  });

  it("Refresh reloads the same binding and page", async () => {
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });

    const before = vi.mocked(browseTableData).mock.calls.length;
    fireEvent.click(screen.getByRole("button", { name: "Refresh table data" }));
    await waitFor(() =>
      expect(vi.mocked(browseTableData).mock.calls.length).toBeGreaterThan(before),
    );
    const lastCall = vi.mocked(browseTableData).mock.calls.at(-1);
    expect(lastCall?.[1]).toEqual(
      expect.objectContaining({ database: "alpha", schema: "public", table: "users", page: 1 }),
    );
  });

  it("paginates with has_more and keeps page state on the tab", async () => {
    vi.mocked(browseTableData).mockImplementation((_id, params) =>
      Promise.resolve(
        pageResult({
          pagination: { page: params.page, page_size: params.pageSize, has_more: params.page === 1 },
        }) as never,
      ),
    );
    renderWithProviders(<TableDataView />);

    await screen.findByRole("grid", { name: "Table data grid" });
    expect(screen.getByRole("button", { name: "Previous page" })).toBeDisabled();
    const next = screen.getByRole("button", { name: "Next page" });
    expect(next).toBeEnabled();

    fireEvent.click(next);
    await waitFor(() =>
      expect(useWorkspaceStore.getState().tabs[0]).toMatchObject({ page: 2 }),
    );
    await waitFor(() =>
      expect(
        vi.mocked(browseTableData).mock.calls.at(-1)?.[1].page,
      ).toBe(2),
    );
  });

  it("disables Next when has_more is false", async () => {
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });
    expect(screen.getByRole("button", { name: "Next page" })).toBeDisabled();
  });

  it("resets to page 1 when the page size changes", async () => {
    useWorkspaceStore.setState({ tabs: [tableTab({ page: 3 })], activeTabId: "td1" });
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });

    fireEvent.change(screen.getByLabelText("Page size"), {
      target: { value: "50" },
    });

    const tab = useWorkspaceStore.getState().tabs[0];
    expect(tab.kind === "table-data" && tab.page).toBe(1);
    expect(tab.kind === "table-data" && tab.pageSize).toBe(50);
  });

  it("has no client-only sorting or filtering controls", async () => {
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });
    // Sorting is only via the server-driven header controls; no separate
    // client-side sort/filter UI exists.
    expect(screen.queryByRole("button", { name: "Sort" })).not.toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: /filter/i })).not.toBeInTheDocument();
  });

  it("cycles header sorting none → asc → desc → none server-side", async () => {
    renderWithProviders(<TableDataView />);
    const header = await screen.findByRole("button", { name: "Sort by name" });

    fireEvent.click(header);
    expect(tabSort()).toEqual([{ column: "name", direction: "asc" }]);
    await waitFor(() =>
      expect(vi.mocked(browseTableData).mock.calls.at(-1)?.[1].sort).toEqual({
        column: "name",
        direction: "asc",
      }),
    );

    fireEvent.click(header);
    await waitFor(() =>
      expect(tabSort()).toEqual([{ column: "name", direction: "desc" }]),
    );

    fireEvent.click(header);
    await waitFor(() => expect(tabSort()).toEqual([]));
  });

  it("exposes aria-sort for the active column", async () => {
    useWorkspaceStore.setState({
      tabs: [tableTab({ sort: [{ column: "name", direction: "asc" }] })],
      activeTabId: "td1",
    });
    renderWithProviders(<TableDataView />);
    const header = await screen.findByRole("columnheader", { name: /name/ });
    await waitFor(() => expect(header).toHaveAttribute("aria-sort", "ascending"));
  });

  it("resets to page 1 when the sort changes", async () => {
    useWorkspaceStore.setState({
      tabs: [tableTab({ page: 4 })],
      activeTabId: "td1",
    });
    renderWithProviders(<TableDataView />);
    fireEvent.click(await screen.findByRole("button", { name: "Sort by name" }));
    expect(useWorkspaceStore.getState().tabs[0]).toMatchObject({ page: 1 });
  });

  it("preserves sort across page navigation and refresh", async () => {
    vi.mocked(browseTableData).mockImplementation((_id, params) =>
      Promise.resolve(
        pageResult({
          pagination: { page: params.page, page_size: params.pageSize, has_more: true },
        }) as never,
      ),
    );
    useWorkspaceStore.setState({
      tabs: [tableTab({ sort: [{ column: "name", direction: "desc" }] })],
      activeTabId: "td1",
    });
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });

    fireEvent.click(screen.getByRole("button", { name: "Next page" }));
    await waitFor(() =>
      expect(vi.mocked(browseTableData).mock.calls.at(-1)?.[1]).toEqual(
        expect.objectContaining({
          page: 2,
          sort: { column: "name", direction: "desc" },
        }),
      ),
    );

    fireEvent.click(screen.getByRole("button", { name: "Refresh table data" }));
    await waitFor(() =>
      expect(vi.mocked(browseTableData).mock.calls.at(-1)?.[1].sort).toEqual({
        column: "name",
        direction: "desc",
      }),
    );
    expect(useWorkspaceStore.getState().tabs[0]).toMatchObject({
      page: 2,
      sort: [{ column: "name", direction: "desc" }],
    });
  });

  it("keeps sorting tab-local", async () => {
    useWorkspaceStore.setState({
      tabs: [
        tableTab({ id: "t1", database: "alpha" }),
        tableTab({ id: "t2", database: "beta", sort: [{ column: "id", direction: "desc" }] }),
      ],
      activeTabId: "t1",
    });
    renderWithProviders(<TableDataView />);

    fireEvent.click(await screen.findByRole("button", { name: "Sort by name" }));

    const [t1, t2] = useWorkspaceStore.getState().tabs;
    expect(t1.kind === "table-data" && t1.sort).toEqual([{ column: "name", direction: "asc" }]);
    expect(t2.kind === "table-data" && t2.sort).toEqual([{ column: "id", direction: "desc" }]);
  });
});

function tabSort() {
  const tab = useWorkspaceStore.getState().tabs[0];
  return tab.kind === "table-data" ? tab.sort : [];
}

describe("TableDataView filters", () => {
  it("applies a draft filter only on Apply and resets page to 1", async () => {
    useWorkspaceStore.setState({ tabs: [tableTab({ page: 3 })], activeTabId: "td1" });
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });

    fireEvent.click(screen.getByTestId("table-data-filter-toggle"));
    const before = vi.mocked(browseTableData).mock.calls.length;
    fireEvent.change(screen.getByLabelText("Filter 1 value"), {
      target: { value: "1" },
    });
    // Draft typing must not hit the backend or mutate the tab.
    expect(useWorkspaceStore.getState().tabs[0]).toMatchObject({ filters: [] });
    expect(vi.mocked(browseTableData).mock.calls.length).toBe(before);

    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    const tab = useWorkspaceStore.getState().tabs[0];
    expect(tab.kind === "table-data" && tab.filters).toEqual([
      { column: "id", operator: "equals", value: "1" },
    ]);
    expect(tab.kind === "table-data" && tab.page).toBe(1);
    await waitFor(() =>
      expect(vi.mocked(browseTableData).mock.calls.at(-1)?.[1].filters).toEqual([
        { column: "id", operator: "equals", value: "1" },
      ]),
    );
  });

  it("offers operator choices based on the column type", async () => {
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });
    fireEvent.click(screen.getByTestId("table-data-filter-toggle"));

    // Numeric column: comparison operators, no contains.
    expect(screen.getByRole("option", { name: ">=" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "contains" })).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Filter 1 column"), {
      target: { value: "name" },
    });
    expect(screen.getByRole("option", { name: "contains" })).toBeInTheDocument();
  });

  it("NULL operators apply without a value", async () => {
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });
    fireEvent.click(screen.getByTestId("table-data-filter-toggle"));
    fireEvent.change(screen.getByLabelText("Filter 1 operator"), {
      target: { value: "is_null" },
    });
    expect(screen.queryByLabelText("Filter 1 value")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(useWorkspaceStore.getState().tabs[0]).toMatchObject({
      filters: [{ column: "id", operator: "is_null" }],
    });
  });

  it("keeps BIGINT filter values exact and preserves sort", async () => {
    useWorkspaceStore.setState({
      tabs: [tableTab({ sort: [{ column: "name", direction: "desc" }] })],
      activeTabId: "td1",
    });
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });
    fireEvent.click(screen.getByTestId("table-data-filter-toggle"));
    fireEvent.change(screen.getByLabelText("Filter 1 value"), {
      target: { value: "9223372036854775807" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Apply" }));

    const tab = useWorkspaceStore.getState().tabs[0];
    expect(tab.kind === "table-data" && tab.filters[0].value).toBe("9223372036854775807");
    expect(tab.kind === "table-data" && tab.sort).toEqual([
      { column: "name", direction: "desc" },
    ]);
  });

  it("Clear resets filters to none and keeps the page at 1", async () => {
    useWorkspaceStore.setState({
      tabs: [tableTab({ filters: [{ column: "id", operator: "equals", value: "1" }], page: 2 })],
      activeTabId: "td1",
    });
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });
    fireEvent.click(screen.getByTestId("table-data-filter-toggle"));
    fireEvent.click(screen.getByRole("button", { name: "Clear" }));

    expect(useWorkspaceStore.getState().tabs[0]).toMatchObject({ filters: [], page: 1 });
  });

  it("surfaces a sanitized INVALID_FILTER error without clearing filters", async () => {
    vi.mocked(browseTableData).mockRejectedValueOnce(
      new ApiClientError("the filter is invalid for that column", "INVALID_FILTER", 400),
    );
    useWorkspaceStore.setState({
      tabs: [tableTab({ filters: [{ column: "id", operator: "contains", value: "x" }] })],
      activeTabId: "td1",
    });
    renderWithProviders(<TableDataView />);

    const error = await screen.findByTestId("table-data-error");
    expect(error).toHaveTextContent("INVALID_FILTER");
    expect(useWorkspaceStore.getState().tabs[0]).toMatchObject({
      filters: [{ column: "id", operator: "contains", value: "x" }],
    });
  });
});

describe("TableDataView add row", () => {
  it("shows Add Row only when the table reports insert capability", async () => {
    vi.mocked(browseTableData).mockResolvedValue(
      pageResult({ row_capabilities: { insert: true, update: true, delete: true, duplicate: true } }) as never,
    );
    renderWithProviders(<TableDataView />);
    expect(await screen.findByTestId("table-data-add-row")).toBeInTheDocument();
  });

  it("disables Add Row for read-only tables and explains why", async () => {
    vi.mocked(browseTableData).mockResolvedValue(
      pageResult({
        row_capabilities: { insert: false, update: false, delete: false, duplicate: false },
        object_type: "VIEW",
      }) as never,
    );
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });
    const button = screen.getByTestId("table-data-add-row");
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute("title", "This object is read-only.");
    fireEvent.click(button);
    expect(screen.queryByTestId("add-row-dialog")).not.toBeInTheDocument();
  });

  it("explains a non-insertable base table instead of hiding the control", async () => {
    vi.mocked(browseTableData).mockResolvedValue(
      pageResult({ row_capabilities: { insert: false, update: false, delete: false, duplicate: false } }) as never,
    );
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });
    expect(screen.getByTestId("table-data-add-row")).toHaveAttribute(
      "title",
      "This table does not support row insertion.",
    );
  });

  it("submits a structured insert against the tab binding and refreshes without clearing filters/sort", async () => {
    vi.mocked(browseTableData).mockResolvedValue(
      pageResult({ row_capabilities: { insert: true, update: false, delete: false, duplicate: false } }) as never,
    );
    vi.mocked(insertRow).mockResolvedValue({ affected_rows: 1 } as never);
    useWorkspaceStore.setState({
      tabs: [
        tableTab({
          database: "alpha",
          filters: [{ column: "id", operator: "equals", value: "1" }],
          sort: [{ column: "name", direction: "asc" }],
        }),
      ],
      activeTabId: "td1",
    });
    renderWithProviders(<TableDataView />);
    await screen.findByTestId("table-data-add-row");

    fireEvent.click(screen.getByTestId("table-data-add-row"));
    fireEvent.change(await screen.findByLabelText("name mode"), {
      target: { value: "value" },
    });
    fireEvent.change(screen.getByLabelText("name value"), {
      target: { value: "Alfie" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Insert" }));

    await waitFor(() =>
      expect(insertRow).toHaveBeenCalledWith(
        "c1",
        expect.objectContaining({
          database: "alpha",
          schema: "public",
          table: "users",
          values: expect.objectContaining({ name: { mode: "value", value: "Alfie" } }),
        }),
      ),
    );
    // Refresh keeps filters and sort; no optimistic row injection.
    await waitFor(() =>
      expect(vi.mocked(browseTableData).mock.calls.at(-1)?.[1]).toEqual(
        expect.objectContaining({
          database: "alpha",
          filters: [{ column: "id", operator: "equals", value: "1" }],
          sort: { column: "name", direction: "asc" },
        }),
      ),
    );
  });

  it("keeps the dialog usable on backend error", async () => {
    vi.mocked(browseTableData).mockResolvedValue(
      pageResult({ row_capabilities: { insert: true } }) as never,
    );
    vi.mocked(insertRow).mockRejectedValueOnce(
      new ApiClientError("the row violates a database constraint", "CONSTRAINT_VIOLATION", 409),
    );
    renderWithProviders(<TableDataView />);
    fireEvent.click(await screen.findByTestId("table-data-add-row"));
    fireEvent.click(screen.getByRole("button", { name: "Insert" }));

    expect(await screen.findByTestId("add-row-dialog")).toBeInTheDocument();
    expect(await screen.findByRole("alert")).toHaveTextContent("CONSTRAINT_VIOLATION");
  });
});

describe("TableDataView edit row", () => {
  it("shows the Edit action only when update is allowed and submits against the tab binding", async () => {
    vi.mocked(browseTableData).mockResolvedValue(
      pageResult({
        row_capabilities: { insert: false, update: true, delete: false, duplicate: false },
        columns: [
          { name: "id", database_type: "bigint", primary_key: true, updatable: false },
          { name: "name", database_type: "text", nullable: true, updatable: true },
        ],
        rows: [["42", "Alice"]],
      }) as never,
    );
    vi.mocked(updateRow).mockResolvedValue({ affected_rows: 1 } as never);
    useWorkspaceStore.setState({
      tabs: [tableTab({ database: "alpha", filters: [{ column: "id", operator: "equals", value: "42" }] })],
      activeTabId: "td1",
    });
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });

    openRowMenu(1);
    fireEvent.click(await screen.findByRole("menuitem", { name: "Edit Row" }));
    fireEvent.change(await screen.findByLabelText("name value"), { target: { value: "Bob" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(updateRow).toHaveBeenCalledWith(
        "c1",
        expect.objectContaining({
          database: "alpha",
          table: "users",
          identity: { id: "42" },
          changes: { name: "Bob" },
        }),
      ),
    );
    // Refresh preserves filters; no optimistic row patch.
    await waitFor(() =>
      expect(vi.mocked(browseTableData).mock.calls.at(-1)?.[1]).toEqual(
        expect.objectContaining({
          database: "alpha",
          filters: [{ column: "id", operator: "equals", value: "42" }],
        }),
      ),
    );
  });

  it("keeps unavailable row actions focusable with an explanation", async () => {
    vi.mocked(browseTableData).mockResolvedValue(
      pageResult({
        row_capabilities: { insert: true, update: false, delete: false, duplicate: false },
        row_identity: { columns: ["id"], kind: "primary_key" },
        rows: [["42", "Alice"]],
      }) as never,
    );
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });

    openRowMenu(1);
    const edit = await screen.findByRole("menuitem", { name: /Edit Row/ });
    expect(edit).toHaveAttribute("aria-disabled", "true");
    expect(edit).toHaveTextContent("Row editing is unavailable for this table.");
    const remove = screen.getByRole("menuitem", { name: /Delete Row/ });
    expect(remove).toHaveAttribute("aria-disabled", "true");
    expect(remove).toHaveTextContent("Row deletion is unavailable for this table.");
    // Copy never mutates and stays available.
    expect(screen.getByRole("menuitem", { name: "Copy" })).not.toHaveAttribute(
      "aria-disabled",
    );
    fireEvent.click(edit);
    expect(updateRow).not.toHaveBeenCalled();
    expect(screen.queryByTestId("edit-row-dialog")).not.toBeInTheDocument();
  });

  it("names the missing primary key when the object reports no row identity", async () => {
    vi.mocked(browseTableData).mockResolvedValue(
      pageResult({
        row_capabilities: { insert: true, update: false, delete: false, duplicate: false },
        rows: [["42", "Alice"]],
      }) as never,
    );
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });

    openRowMenu(1);
    const edit = await screen.findByRole("menuitem", { name: /Edit Row/ });
    expect(edit).toHaveTextContent("No primary key — row editing is unavailable.");
    expect(screen.getByRole("menuitem", { name: /Delete Row/ })).toHaveTextContent(
      "No primary key — row deletion is unavailable.",
    );
  });

  it("submits against the tab binding captured when the dialog opened", async () => {
    vi.mocked(browseTableData).mockResolvedValue(
      pageResult({
        row_capabilities: { insert: true, update: true, delete: false, duplicate: false },
        columns: [
          { name: "id", database_type: "bigint", primary_key: true, updatable: false },
          { name: "name", database_type: "text", nullable: true, updatable: true },
        ],
        rows: [["42", "Alice"]],
      }) as never,
    );
    vi.mocked(insertRow).mockResolvedValue({ affected_rows: 1 } as never);
    vi.mocked(updateRow).mockResolvedValue({ affected_rows: 1 } as never);
    useWorkspaceStore.setState({
      tabs: [tableTab({ id: "td1", database: "alpha" }), tableTab({ id: "td2", database: "beta", table: "users" })],
      activeTabId: "td1",
    });
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });

    openRowMenu(1);
    fireEvent.click(await screen.findByRole("menuitem", { name: "Edit Row" }));
    fireEvent.change(await screen.findByLabelText("name value"), { target: { value: "Bob" } });

    // Switching tabs while the dialog is open must not retarget the mutation.
    useWorkspaceStore.setState({ activeTabId: "td2" });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(updateRow).toHaveBeenCalledWith(
        "c1",
        expect.objectContaining({ database: "alpha" }),
      ),
    );
  });
});

describe("TableDataView delete row", () => {
  it("shows Delete only when allowed and confirms with identity and full-row expected", async () => {
    vi.mocked(browseTableData).mockResolvedValue(
      pageResult({
        row_capabilities: { insert: false, update: false, delete: true, duplicate: false },
        rows: [["42", "Alice"]],
      }) as never,
    );
    vi.mocked(deleteRow).mockResolvedValue({ affected_rows: 1 } as never);
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });

    openRowMenu(1);
    fireEvent.click(await screen.findByRole("menuitem", { name: "Delete Row" }));

    const dialog = await screen.findByTestId("delete-row-dialog");
    expect(dialog).toHaveTextContent("id = 42");
    expect(dialog).toHaveTextContent("alpha / public.users");

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    await waitFor(() =>
      expect(deleteRow).toHaveBeenCalledWith("c1", {
        database: "alpha",
        schema: "public",
        table: "users",
        identity: { id: "42" },
        expected: { name: "Alice" },
      }),
    );
    await waitFor(() =>
      expect(vi.mocked(browseTableData).mock.calls.length).toBeGreaterThan(1),
    );
  });

  it("keeps the dialog open on ROW_CONFLICT and does not refetch", async () => {
    vi.mocked(browseTableData).mockResolvedValue(
      pageResult({
        row_capabilities: { insert: false, update: false, delete: true, duplicate: false },
        rows: [["42", "Alice"]],
      }) as never,
    );
    vi.mocked(deleteRow).mockRejectedValueOnce(
      new ApiClientError("the row changed since it was loaded", "ROW_CONFLICT", 409),
    );
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });
    openRowMenu(1);
    fireEvent.click(await screen.findByRole("menuitem", { name: "Delete Row" }));
    fireEvent.click(await screen.findByRole("button", { name: "Delete" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("ROW_CONFLICT");
    expect(screen.getByTestId("delete-row-dialog")).toBeInTheDocument();
    expect(vi.mocked(browseTableData).mock.calls.length).toBe(1);
  });

  it("Cancel closes without calling the API", async () => {
    vi.mocked(browseTableData).mockResolvedValue(
      pageResult({
        row_capabilities: { insert: false, update: false, delete: true, duplicate: false },
        rows: [["42", "Alice"]],
      }) as never,
    );
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });
    openRowMenu(1);
    fireEvent.click(await screen.findByRole("menuitem", { name: "Delete Row" }));
    fireEvent.click(await screen.findByRole("button", { name: "Cancel" }));

    await waitFor(() =>
      expect(screen.queryByTestId("delete-row-dialog")).not.toBeInTheDocument(),
    );
    expect(deleteRow).not.toHaveBeenCalled();
  });
});

describe("TableDataView duplicate row", () => {
  it("prefills Add Row from the snapshot, skipping PK columns, then inserts", async () => {
    vi.mocked(browseTableData).mockResolvedValue(
      pageResult({
        row_capabilities: { insert: true, update: false, delete: false, duplicate: true },
        columns: [
          { name: "id", database_type: "bigint", primary_key: true, insertable: false },
          { name: "name", database_type: "text", nullable: false, insertable: true },
          { name: "nickname", database_type: "text", nullable: true, insertable: true },
        ],
        rows: [["42", "Alice", null]],
      }) as never,
    );
    vi.mocked(insertRow).mockResolvedValue({ affected_rows: 1 } as never);
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });

    openRowMenu(1);
    fireEvent.click(await screen.findByRole("menuitem", { name: "Duplicate Row" }));
    const dialog = await screen.findByTestId("add-row-dialog");
    expect(dialog).toHaveTextContent("Duplicate Row — users");
    // Value columns copy the source; NULL nullable stays NULL; PK is skipped.
    expect(screen.getByLabelText("name value")).toHaveValue("Alice");
    expect(screen.getByLabelText("nickname mode")).toHaveTextContent("null");
    expect(screen.queryByLabelText("id mode")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Insert" }));
    await waitFor(() =>
      expect(insertRow).toHaveBeenCalledWith(
        "c1",
        expect.objectContaining({
          values: {
            name: { mode: "value", value: "Alice" },
            nickname: { mode: "null" },
          },
        }),
      ),
    );
  });

  it("disables Duplicate when the table reports no duplication capability", async () => {
    vi.mocked(browseTableData).mockResolvedValue(
      pageResult({
        row_capabilities: { insert: true, update: true, delete: true, duplicate: false },
        rows: [["42", "Alice"]],
      }) as never,
    );
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });

    openRowMenu(1);
    const duplicate = await screen.findByRole("menuitem", { name: /Duplicate Row/ });
    expect(duplicate).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(duplicate);
    expect(screen.queryByTestId("add-row-dialog")).not.toBeInTheDocument();
    // Edit and Delete remain available (capabilities allow them).
    expect(screen.getByRole("menuitem", { name: /Edit Row/ })).not.toHaveAttribute(
      "aria-disabled",
    );
    expect(screen.getByRole("menuitem", { name: /Delete Row/ })).not.toHaveAttribute(
      "aria-disabled",
    );
  });
});

describe("TableDataView row copy", () => {
  async function openCopyFixture() {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      configurable: true,
    });
    vi.mocked(browseTableData).mockResolvedValue(
      pageResult({
        row_capabilities: { insert: false, update: false, delete: false, duplicate: false },
        columns: [
          { name: "id", database_type: "bigint", primary_key: true },
          { name: "name", database_type: "text", nullable: true },
          { name: "big", database_type: "bigint", nullable: true },
        ],
        rows: [["9223372036854775807", "Alfie", null]],
      }) as never,
    );
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });
    openRowMenu(1);
    return writeText;
  }

  it("copies the row as TSV in column order with exact BIGINT and NULL", async () => {
    const writeText = await openCopyFixture();
    fireEvent.click(await screen.findByRole("menuitem", { name: "Copy" }));

    await waitFor(() =>
      expect(writeText).toHaveBeenCalledWith("9223372036854775807\tAlfie\tNULL"),
    );
    expect(await screen.findByTestId("table-data-status")).toHaveTextContent("Row copied.");
    // Copy is pure: no refetch and no mutation.
    expect(vi.mocked(browseTableData).mock.calls.length).toBe(1);
    expect(insertRow).not.toHaveBeenCalled();
    expect(updateRow).not.toHaveBeenCalled();
    expect(deleteRow).not.toHaveBeenCalled();
  });

  it("copies the row as canonical JSON with BIGINT kept as a string", async () => {
    const writeText = await openCopyFixture();
    fireEvent.click(await screen.findByRole("menuitem", { name: "Copy as JSON" }));

    await waitFor(() => expect(writeText).toHaveBeenCalledTimes(1));
    const payload = JSON.parse(writeText.mock.calls[0][0] as string) as Record<string, unknown>;
    expect(payload).toEqual({
      id: "9223372036854775807",
      name: "Alfie",
      big: null,
    });
    expect(vi.mocked(browseTableData).mock.calls.length).toBe(1);
  });

  it("reports a copy failure without breaking the grid", async () => {
    Object.defineProperty(navigator, "clipboard", {
      value: undefined,
      configurable: true,
    });
    vi.mocked(browseTableData).mockResolvedValue(
      pageResult({ rows: [["42", "Alice"]] }) as never,
    );
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });
    openRowMenu(1);
    fireEvent.click(await screen.findByRole("menuitem", { name: "Copy" }));

    expect(await screen.findByTestId("table-data-status")).toHaveTextContent(
      "Copy failed",
    );
    expect(screen.getByRole("grid", { name: "Table data grid" })).toBeInTheDocument();
  });
});
