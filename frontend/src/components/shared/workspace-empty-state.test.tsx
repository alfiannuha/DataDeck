import { fireEvent, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { listConnections } from "@/lib/api/endpoints";
import { renderWithProviders } from "@/test/render";
import { useConnectionStore } from "@/store/useConnectionStore";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

import { WorkspaceEmptyState } from "./workspace-empty-state";

vi.mock("@/lib/api/endpoints", () => ({
  listConnections: vi.fn(),
}));

const connection = {
  id: "c1",
  name: "CCM DEV",
  driver: "postgres",
  host: "127.0.0.1",
  port: 5432,
  database_name: "",
};

beforeEach(() => {
  vi.clearAllMocks();
  useWorkspaceStore.setState({ tabs: [], activeTabId: null });
  useConnectionStore.setState({
    activeConnectionId: null,
    activeDatabaseByConnection: {},
  });
  vi.mocked(listConnections).mockResolvedValue([connection] as never);
});

describe("WorkspaceEmptyState", () => {
  it("shows no-connection guidance and creates an unbound Query tab", async () => {
    renderWithProviders(<WorkspaceEmptyState />);

    expect(screen.getByText("No tabs open")).toBeInTheDocument();
    expect(
      screen.getByText("Select a connection or create a new query to get started."),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "New Query" }));

    const state = useWorkspaceStore.getState();
    expect(state.tabs).toHaveLength(1);
    expect(state.tabs[0].kind).toBe("query");
    expect(state.tabs[0].connectionId).toBeNull();
    expect(state.tabs[0].database).toBeNull();
  });

  it("shows connection-aware guidance", async () => {
    useConnectionStore.setState({ activeConnectionId: "c1" });
    renderWithProviders(<WorkspaceEmptyState />);

    await waitFor(() =>
      expect(screen.getByTestId("empty-state-connection")).toHaveTextContent(
        "CCM DEV",
      ),
    );
    expect(
      screen.getByText(/Double-click a table in the explorer/),
    ).toBeInTheDocument();
  });

  it("binds a new Query tab to the connection and selected database", async () => {
    useConnectionStore.setState({
      activeConnectionId: "c1",
      activeDatabaseByConnection: { c1: "alpha" },
    });
    renderWithProviders(<WorkspaceEmptyState />);

    fireEvent.click(await screen.findByRole("button", { name: "New Query" }));

    const tab = useWorkspaceStore.getState().tabs[0];
    expect(tab.connectionId).toBe("c1");
    expect(tab.database).toBe("alpha");
  });

  it("never binds a server-level tab to a bootstrap database implicitly", async () => {
    useConnectionStore.setState({ activeConnectionId: "c1" });
    renderWithProviders(<WorkspaceEmptyState />);

    fireEvent.click(await screen.findByRole("button", { name: "New Query" }));

    const tab = useWorkspaceStore.getState().tabs[0];
    expect(tab.connectionId).toBe("c1");
    expect(tab.database).toBeNull();
  });
});
