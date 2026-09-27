import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  deleteSavedQuery,
  executeQuery,
  listConnections,
  listSavedQueries,
} from "@/lib/api/endpoints";
import { ApiClientError } from "@/lib/api-client";
import { renderWithProviders } from "@/test/render";
import { useExecutionStore } from "@/store/useExecutionStore";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

import { SavedQueriesDialog } from "./saved-queries-dialog";

vi.mock("@/lib/api/endpoints", () => ({
  listSavedQueries: vi.fn(),
  deleteSavedQuery: vi.fn(),
  listConnections: vi.fn(),
  createSavedQuery: vi.fn(),
  updateSavedQuery: vi.fn(),
  executeQuery: vi.fn(),
}));

const connections = [
  { id: "c1", name: "PG One", driver: "postgres", database_name: "app" },
];

function entry(overrides: Record<string, unknown> = {}) {
  return {
    id: "s1",
    connection_id: "c1",
    title: "Active users",
    sql_text: "SELECT 1",
    tags: "users",
    updated_at: "2026-09-25T14:32:00Z",
    created_at: "2026-09-25T14:32:00Z",
    ...overrides,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  useWorkspaceStore.setState({
    tabs: [{ id: "t1", title: "Query 1", sql: "", connectionId: null, dirty: false }],
    activeTabId: "t1",
  });
  vi.mocked(listConnections).mockResolvedValue(connections as never);
});

describe("SavedQueriesDialog", () => {
  it("shows an empty state", async () => {
    vi.mocked(listSavedQueries).mockResolvedValue({ items: [], meta: { page: 1, page_size: 50, total: 0, total_pages: 0 } });
    renderWithProviders(<SavedQueriesDialog open onOpenChange={vi.fn()} />);
    expect(
      await screen.findByText(/No saved queries yet/),
    ).toBeInTheDocument();
  });

  it("lists saved queries with tags and connection", async () => {
    vi.mocked(listSavedQueries).mockResolvedValue({ items: [entry()], meta: { page: 1, page_size: 50, total: 1, total_pages: 1 } } as never);
    renderWithProviders(<SavedQueriesDialog open onOpenChange={vi.fn()} />);

    const list = await screen.findByRole("list", { name: "Saved query entries" });
    expect(within(list).getByText("Active users")).toBeInTheDocument();
    expect(within(list).getByText(/#users/)).toBeInTheDocument();
    expect(within(list).getByText("PG One")).toBeInTheDocument();
  });

  it("opens a saved query into a bound tab without executing", async () => {
    vi.mocked(listSavedQueries).mockResolvedValue({ items: [entry()], meta: { page: 1, page_size: 50, total: 1, total_pages: 1 } } as never);
    const onOpenChange = vi.fn();
    renderWithProviders(<SavedQueriesDialog open onOpenChange={onOpenChange} />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Open in editor" }),
    );

    const state = useWorkspaceStore.getState();
    const tab = state.tabs.find((candidate) => candidate.id === state.activeTabId);
    expect(tab?.sql).toBe("SELECT 1");
    expect(tab?.connectionId).toBe("c1");
    expect(tab?.savedQueryId).toBe("s1");
    expect(tab?.dirty).toBe(false);
    expect(executeQuery).not.toHaveBeenCalled();
  });

  it("opens unbound with a warning when the connection is gone", async () => {
    vi.mocked(listSavedQueries).mockResolvedValue({
      items: [entry({ connection_id: "gone" })],
      meta: { page: 1, page_size: 50, total: 1, total_pages: 1 },
    } as never);
    renderWithProviders(<SavedQueriesDialog open onOpenChange={vi.fn()} />);

    expect(await screen.findByText("connection removed")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Open in editor" }));

    const state = useWorkspaceStore.getState();
    const tab = state.tabs.find((candidate) => candidate.id === state.activeTabId);
    expect(tab?.connectionId).toBeNull();
    expect(executeQuery).not.toHaveBeenCalled();
  });

  it("deletes a saved query after confirmation without touching the connection", async () => {
    vi.mocked(listSavedQueries).mockResolvedValue({ items: [entry()], meta: { page: 1, page_size: 50, total: 1, total_pages: 1 } } as never);
    vi.mocked(deleteSavedQuery).mockResolvedValue({ id: "s1" } as never);
    renderWithProviders(<SavedQueriesDialog open onOpenChange={vi.fn()} />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Delete Active users" }),
    );
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() => expect(deleteSavedQuery).toHaveBeenCalledWith("s1"));
  });

  it("opens an update dialog for editing", async () => {
    vi.mocked(listSavedQueries).mockResolvedValue({ items: [entry()], meta: { page: 1, page_size: 50, total: 1, total_pages: 1 } } as never);
    renderWithProviders(<SavedQueriesDialog open onOpenChange={vi.fn()} />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Edit Active users" }),
    );
    expect(
      await screen.findByRole("heading", { name: "Update saved query" }),
    ).toBeInTheDocument();
  });
});

describe("SavedQueriesDialog pagination and state", () => {
  it("pages through results", async () => {
    vi.mocked(listSavedQueries).mockResolvedValue({
      items: [entry()],
      meta: { page: 1, page_size: 50, total: 120, total_pages: 3 },
    } as never);
    renderWithProviders(<SavedQueriesDialog open onOpenChange={vi.fn()} />);

    await screen.findByText("Page 1 of 3");
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    await waitFor(() =>
      expect(listSavedQueries).toHaveBeenLastCalledWith(
        expect.objectContaining({ page: 2 }),
        expect.anything(),
      ),
    );
  });

  it("clears stale execution state when opening an entry", async () => {
    useExecutionStore.getState().resolve({
      columns: [],
      rows: [["1"]],
      rows_affected: 1,
      execution_time_ms: 5,
      truncated: false,
    });
    vi.mocked(listSavedQueries).mockResolvedValue({
      items: [entry()],
      meta: { page: 1, page_size: 50, total: 1, total_pages: 1 },
    } as never);
    renderWithProviders(<SavedQueriesDialog open onOpenChange={vi.fn()} />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Open in editor" }),
    );
    expect(useExecutionStore.getState().status).toBe("idle");
    expect(useExecutionStore.getState().result).toBeNull();
  });

  it("terminates loading and reports a saved-query load failure", async () => {
    vi.mocked(listSavedQueries).mockRejectedValue(
      new ApiClientError("failed to load saved queries", "INTERNAL_ERROR", 500),
    );
    renderWithProviders(<SavedQueriesDialog open onOpenChange={vi.fn()} />);

    expect(await screen.findByRole("alert")).toHaveTextContent("INTERNAL_ERROR");
    expect(
      screen.queryByText("Loading saved queries…"),
    ).not.toBeInTheDocument();
  });
});
