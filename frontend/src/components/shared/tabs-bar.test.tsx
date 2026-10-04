import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithProviders } from "@/test/render";
import { useConnectionStore } from "@/store/useConnectionStore";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

import { TabsBar } from "./tabs-bar";

vi.mock("@/lib/api/endpoints", () => ({
  listConnections: vi.fn().mockResolvedValue([]),
}));

beforeEach(() => {
  useConnectionStore.setState({ activeConnectionId: null });
  useWorkspaceStore.setState({
    tabs: [
      { kind: "query", id: "t1", title: "Query 1", sql: "", connectionId: null, database: null, dirty: false },
    ],
    activeTabId: "t1",
  });
});

describe("TabsBar", () => {
  it("renders the open tabs", () => {
    renderWithProviders(<TabsBar />);
    expect(screen.getByText("Query 1")).toBeInTheDocument();
  });

  it("shows a dirty indicator for unsaved changes", () => {
    renderWithProviders(<TabsBar />);
    expect(screen.queryByLabelText("Unsaved changes")).not.toBeInTheDocument();

    act(() => {
      useWorkspaceStore.getState().updateActiveSql("SELECT 1");
    });

    expect(screen.getByLabelText("Unsaved changes")).toBeInTheDocument();
  });

  it("adds a new tab bound to the active connection", () => {
    useConnectionStore.setState({ activeConnectionId: "c1" });
    renderWithProviders(<TabsBar />);
    fireEvent.click(screen.getByRole("button", { name: "New query tab" }));

    expect(screen.getByText("Query 2")).toBeInTheDocument();
    const state = useWorkspaceStore.getState();
    expect(state.tabs[1].connectionId).toBe("c1");
  });

  it("closes a tab", () => {
    renderWithProviders(<TabsBar />);
    fireEvent.click(screen.getByRole("button", { name: "New query tab" }));
    expect(screen.getByText("Query 2")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Close Query 1" }));

    expect(screen.queryByText("Query 1")).not.toBeInTheDocument();
    expect(screen.getByText("Query 2")).toBeInTheDocument();
  });

  it("confirms before closing a dirty tab", () => {
    useWorkspaceStore.setState({
      tabs: [
        { kind: "query", id: "t1", title: "Query 1", sql: "SELECT 1", connectionId: "c1", database: null, dirty: true },
        { kind: "query", id: "t2", title: "Query 2", sql: "", connectionId: "c1", database: null, dirty: false },
      ],
      activeTabId: "t2",
    });
    renderWithProviders(<TabsBar />);

    fireEvent.click(screen.getByRole("button", { name: "Close Query 1" }));
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Close tab" }));

    expect(screen.queryByText("Query 1")).not.toBeInTheDocument();
  });

  it("keeps the dirty tab when closing is cancelled", () => {
    useWorkspaceStore.setState({
      tabs: [
        { kind: "query", id: "t1", title: "Query 1", sql: "SELECT 1", connectionId: "c1", database: null, dirty: true },
        { kind: "query", id: "t2", title: "Query 2", sql: "", connectionId: "c1", database: null, dirty: false },
      ],
      activeTabId: "t2",
    });
    renderWithProviders(<TabsBar />);

    fireEvent.click(screen.getByRole("button", { name: "Close Query 1" }));
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    expect(screen.getByText("Query 1")).toBeInTheDocument();
    expect(useWorkspaceStore.getState().tabs).toHaveLength(2);
  });

  it("closes a clean tab without a confirmation dialog", () => {
    renderWithProviders(<TabsBar />);
    fireEvent.click(screen.getByRole("button", { name: "Close Query 1" }));
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("moves focus to the active tab after closing", async () => {
    renderWithProviders(<TabsBar />);
    fireEvent.click(screen.getByRole("button", { name: "New query tab" }));

    await waitFor(() =>
      expect(document.activeElement?.textContent).toContain("Query 2"),
    );

    fireEvent.click(screen.getByRole("button", { name: "Close Query 2" }));
    await waitFor(() =>
      expect(document.activeElement?.textContent).toContain("Query 1"),
    );
  });

  it("new tabs inherit the explorer-selected database without touching others", () => {
    useConnectionStore.setState({
      activeConnectionId: "c1",
      activeDatabaseByConnection: { c1: "reporting" },
    });
    useWorkspaceStore.setState({
      tabs: [{ kind: "query", id: "t1", title: "Query 1", sql: "", connectionId: "c1", database: "CCM", dirty: false }],
      activeTabId: "t1",
    });
    renderWithProviders(<TabsBar />);

    fireEvent.click(screen.getByRole("button", { name: "New query tab" }));

    const state = useWorkspaceStore.getState();
    expect(state.tabs[0].database).toBe("CCM");
    expect(state.tabs[1].database).toBe("reporting");
    expect(state.tabs[1].connectionId).toBe("c1");
  });

  it("closes a Table Data tab directly without an unsaved dialog", () => {
    useWorkspaceStore.setState({
      tabs: [
        { kind: "query", id: "t1", title: "Query 1", sql: "", connectionId: "c1", database: null, dirty: false },
        {
          kind: "table-data",
          id: "td1",
          title: "users",
          connectionId: "c1",
          database: "app",
          schema: "public",
          table: "users",
          filters: [],
          sort: [],
          page: 1,
          pageSize: 100,
        },
      ],
      activeTabId: "td1",
    });
    renderWithProviders(<TabsBar />);

    expect(screen.getByRole("button", { name: /^users/ })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Close users" }));

    const state = useWorkspaceStore.getState();
    expect(state.tabs).toHaveLength(1);
    expect(state.tabs[0].kind).toBe("query");
    expect(screen.queryByText(/unsaved changes/i)).not.toBeInTheDocument();
  });

  it("handles zero tabs without crashing or faking a tab", () => {
    useWorkspaceStore.setState({ tabs: [], activeTabId: null });
    renderWithProviders(<TabsBar />);

    // No tab buttons, but the New Query control remains available.
    expect(
      screen.queryByRole("button", { name: /^Query 1/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "New query tab" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "New query tab" }));
    expect(useWorkspaceStore.getState().tabs).toHaveLength(1);
  });
});
