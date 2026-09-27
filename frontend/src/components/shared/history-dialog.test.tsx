import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  executeQuery,
  getQueryHistory,
  listConnections,
} from "@/lib/api/endpoints";
import { ApiClientError } from "@/lib/api-client";
import { renderWithProviders } from "@/test/render";
import { useExecutionStore } from "@/store/useExecutionStore";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

import { HistoryDialog } from "./history-dialog";

vi.mock("@/lib/api/endpoints", () => ({
  listConnections: vi.fn(),
  getQueryHistory: vi.fn(),
  executeQuery: vi.fn(),
}));

const connections = [
  { id: "c1", name: "PG One", driver: "postgres", database_name: "app" },
];

function entry(overrides: Record<string, unknown> = {}) {
  return {
    id: "h1",
    connection_id: "c1",
    sql_text: "SELECT id FROM users",
    status: "SUCCESS",
    execution_time_ms: 12,
    rows_affected: 2,
    executed_at: "2026-09-25T14:32:00Z",
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

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("HistoryDialog", () => {
  it("shows a loading state", () => {
    vi.mocked(getQueryHistory).mockReturnValue(new Promise(() => {}));
    renderWithProviders(<HistoryDialog open onOpenChange={vi.fn()} />);
    expect(screen.getByText("Loading history…")).toBeInTheDocument();
  });

  it("shows an empty state", async () => {
    vi.mocked(getQueryHistory).mockResolvedValue({ items: [], meta: { page: 1, page_size: 50, total: 0, total_pages: 0 } });
    renderWithProviders(<HistoryDialog open onOpenChange={vi.fn()} />);
    expect(
      await screen.findByText("No queries have been executed yet."),
    ).toBeInTheDocument();
  });

  it("renders success and error entries with metadata", async () => {
    vi.mocked(getQueryHistory).mockResolvedValue({
      items: [
        entry(),
        entry({
          id: "h2",
          status: "ERROR",
          execution_time_ms: 3,
          rows_affected: 0,
          sql_text: "SELECT WHERRE",
        }),
      ],
      meta: { page: 1, page_size: 50, total: 2, total_pages: 1 },
    } as never);
    renderWithProviders(<HistoryDialog open onOpenChange={vi.fn()} />);

    expect(await screen.findByText("SELECT id FROM users")).toBeInTheDocument();
    expect(screen.getByText("SUCCESS")).toBeInTheDocument();
    expect(screen.getByText("ERROR")).toBeInTheDocument();
    expect(screen.getByText("12 ms")).toBeInTheDocument();
    expect(screen.getByText("2 rows")).toBeInTheDocument();
    const list = screen.getByRole("list", { name: "Query history entries" });
    expect(within(list).getAllByText("PG One")).toHaveLength(2);
  });

  it("flags an entry whose connection no longer exists", async () => {
    vi.mocked(getQueryHistory).mockResolvedValue({
      items: [entry({ connection_id: "gone" })],
      meta: { page: 1, page_size: 50, total: 1, total_pages: 1 },
    } as never);
    renderWithProviders(<HistoryDialog open onOpenChange={vi.fn()} />);

    expect(await screen.findByText("connection removed")).toBeInTheDocument();
    expect(
      screen.getByText(/Original connection no longer exists/),
    ).toBeInTheDocument();
  });

  it("opens an entry into a tab bound to its connection without executing", async () => {
    vi.mocked(getQueryHistory).mockResolvedValue({ items: [entry()], meta: { page: 1, page_size: 50, total: 1, total_pages: 1 } } as never);
    const onOpenChange = vi.fn();
    renderWithProviders(
      <HistoryDialog open onOpenChange={onOpenChange} />,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "Open in editor" }),
    );

    const state = useWorkspaceStore.getState();
    const tab = state.tabs.find((candidate) => candidate.id === state.activeTabId);
    expect(tab?.sql).toBe("SELECT id FROM users");
    expect(tab?.connectionId).toBe("c1");
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(executeQuery).not.toHaveBeenCalled();
  });

  it("opens with no connection binding when the original is gone", async () => {
    vi.mocked(getQueryHistory).mockResolvedValue({
      items: [entry({ connection_id: "gone" })],
      meta: { page: 1, page_size: 50, total: 1, total_pages: 1 },
    } as never);
    renderWithProviders(<HistoryDialog open onOpenChange={vi.fn()} />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Open in editor" }),
    );

    const state = useWorkspaceStore.getState();
    const tab = state.tabs.find((candidate) => candidate.id === state.activeTabId);
    expect(tab?.connectionId).toBeNull();
    expect(executeQuery).not.toHaveBeenCalled();
  });

  it("filters by connection", async () => {
    vi.mocked(getQueryHistory).mockResolvedValue({ items: [entry()], meta: { page: 1, page_size: 50, total: 1, total_pages: 1 } } as never);
    renderWithProviders(<HistoryDialog open onOpenChange={vi.fn()} />);

    await screen.findByText("SELECT id FROM users");
    fireEvent.change(screen.getByLabelText("Filter"), {
      target: { value: "c1" },
    });

    await waitFor(() =>
      expect(getQueryHistory).toHaveBeenCalledWith(expect.objectContaining({ connectionId: "c1" }), expect.anything()),
    );
  });

  it("copies the SQL of an entry", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    vi.mocked(getQueryHistory).mockResolvedValue({ items: [entry()], meta: { page: 1, page_size: 50, total: 1, total_pages: 1 } } as never);
    renderWithProviders(<HistoryDialog open onOpenChange={vi.fn()} />);

    fireEvent.click(await screen.findByRole("button", { name: "Copy SQL" }));
    await waitFor(() =>
      expect(writeText).toHaveBeenCalledWith("SELECT id FROM users"),
    );
  });
});

describe("HistoryDialog pagination and state", () => {
  it("pages through results", async () => {
    vi.mocked(getQueryHistory).mockResolvedValue({
      items: [entry()],
      meta: { page: 1, page_size: 50, total: 120, total_pages: 3 },
    } as never);
    renderWithProviders(<HistoryDialog open onOpenChange={vi.fn()} />);

    await screen.findByText("Page 1 of 3");
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    await waitFor(() =>
      expect(getQueryHistory).toHaveBeenLastCalledWith(
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
    vi.mocked(getQueryHistory).mockResolvedValue({
      items: [entry()],
      meta: { page: 1, page_size: 50, total: 1, total_pages: 1 },
    } as never);
    renderWithProviders(<HistoryDialog open onOpenChange={vi.fn()} />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Open in editor" }),
    );
    expect(useExecutionStore.getState().status).toBe("idle");
    expect(useExecutionStore.getState().result).toBeNull();
  });

  it("terminates loading and reports a history load failure", async () => {
    vi.mocked(getQueryHistory).mockRejectedValue(
      new ApiClientError("failed to load history", "INTERNAL_ERROR", 500),
    );
    renderWithProviders(<HistoryDialog open onOpenChange={vi.fn()} />);

    expect(await screen.findByRole("alert")).toHaveTextContent("INTERNAL_ERROR");
    expect(screen.queryByText("Loading history…")).not.toBeInTheDocument();
  });
});
