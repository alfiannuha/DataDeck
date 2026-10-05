import { fireEvent, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { browseTableData } from "@/lib/api/endpoints";
import { ApiClientError } from "@/lib/api-client";
import { renderWithProviders } from "@/test/render";
import { useWorkspaceStore, type TableDataTab } from "@/store/useWorkspaceStore";

import { TableDataView } from "./table-data-view";

vi.mock("@/lib/api/endpoints", () => ({
  browseTableData: vi.fn(),
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
      { name: "id", database_type: "bigint", primary_key: true },
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

  it("exposes no client-only sorting or filtering controls", async () => {
    renderWithProviders(<TableDataView />);
    await screen.findByRole("grid", { name: "Table data grid" });
    expect(screen.queryByRole("button", { name: /sort/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: /filter/i })).not.toBeInTheDocument();
  });
});
