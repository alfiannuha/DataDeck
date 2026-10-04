import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiClientError } from "@/lib/api-client";
import {
  createSavedQuery,
  executeQuery,
  getSchemas,
  listConnections,
  listDatabases,
} from "@/lib/api/endpoints";
import { renderWithProviders } from "@/test/render";
import { useConnectionStore } from "@/store/useConnectionStore";
import { useExecutionStore } from "@/store/useExecutionStore";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

import { EditorPanel } from "./editor-panel";
import { asQueryTab } from "@/test/query-tab";

vi.mock("@/lib/api/endpoints", () => ({
  executeQuery: vi.fn(),
  getSchemas: vi.fn(),
  getQueryHistory: vi.fn(),
  listConnections: vi.fn(),
  createConnection: vi.fn(),
  testConnection: vi.fn(),
  deleteConnection: vi.fn(),
  getHealth: vi.fn(),
  listSavedQueries: vi.fn(),
  listDatabases: vi.fn().mockResolvedValue([]),
  createSavedQuery: vi.fn(),
  updateSavedQuery: vi.fn(),
  deleteSavedQuery: vi.fn(),
}));

vi.mock("@/components/editor/sql-editor", async () => {
  const React = await import("react");
  return {
    SqlEditor: React.forwardRef(function MockEditor(props: {
      value: string;
      dialect?: string;
      onExecute: (sql: string) => void;
      onSave?: () => void;
    }) {
      return (
        <div>
          <span data-testid="editor-value">{props.value}</span>
          <span data-testid="editor-dialect">{props.dialect}</span>
          <button type="button" onClick={() => props.onExecute("SELECT 2;")}>
            editor-execute
          </button>
          <button type="button" onClick={() => props.onSave?.()}>
            editor-save
          </button>
        </div>
      );
    }),
  };
});

const result = {
  columns: [],
  rows: [],
  rows_affected: 1,
  execution_time_ms: 5,
  truncated: false,
};

const connections = [
  { id: "c1", name: "PG One", driver: "postgres", host: "h", port: 5432, database_name: "app" },
  { id: "c2", name: "PG Two", driver: "postgres", host: "h", port: 5432, database_name: "app2" },
];

function tab(overrides: Partial<ReturnType<typeof baseTab>> = {}) {
  return { ...baseTab(), ...overrides };
}
function baseTab() {
  return {
    kind: "query" as const,
    id: "t1",
    title: "Query 1",
    sql: "SELECT 1;",
    connectionId: "c1" as string | null,
    database: null as string | null,
    dirty: false,
  };
}

function setOnline(online: boolean) {
  Object.defineProperty(window.navigator, "onLine", {
    configurable: true,
    value: online,
  });
  act(() => {
    window.dispatchEvent(new Event(online ? "online" : "offline"));
  });
}

afterEach(() => setOnline(true));

beforeEach(() => {
  vi.clearAllMocks();
  useConnectionStore.setState({ activeConnectionId: "c1" });
  useExecutionStore.getState().reset();
  useWorkspaceStore.setState({ tabs: [tab()], activeTabId: "t1" });
  vi.mocked(getSchemas).mockResolvedValue([]);
  vi.mocked(listConnections).mockResolvedValue(connections as never);
});

describe("EditorPanel execution", () => {
  it("runs the active statement from the Run button", async () => {
    vi.mocked(executeQuery).mockResolvedValue(result as never);
    renderWithProviders(<EditorPanel />);

    const runButton = await screen.findByRole("button", { name: "Run query" });
    await waitFor(() => expect(runButton).toBeEnabled());
    fireEvent.click(runButton);

    await waitFor(() =>
      expect(executeQuery).toHaveBeenCalledWith(
        { connection_id: "c1", sql: "SELECT 1", timeout_seconds: 30 },
        expect.anything(),
      ),
    );
  });

  it("runs through the editor execute pathway", async () => {
    vi.mocked(executeQuery).mockResolvedValue(result as never);
    renderWithProviders(<EditorPanel />);

    // Wait until the connection list has loaded so the guard allows execution.
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Run query" })).toBeEnabled(),
    );
    fireEvent.click(screen.getByRole("button", { name: "editor-execute" }));

    await waitFor(() =>
      expect(executeQuery).toHaveBeenCalledWith(
        expect.objectContaining({ sql: "SELECT 2;" }),
        expect.anything(),
      ),
    );
  });

  it("uses the tab's bound connection, not the active connection", async () => {
    vi.mocked(executeQuery).mockResolvedValue(result as never);
    useConnectionStore.setState({ activeConnectionId: "c2" });
    renderWithProviders(<EditorPanel />);

    const runButton = await screen.findByRole("button", { name: "Run query" });
    await waitFor(() => expect(runButton).toBeEnabled());
    fireEvent.click(runButton);

    await waitFor(() =>
      expect(executeQuery).toHaveBeenCalledWith(
        expect.objectContaining({ connection_id: "c1" }),
        expect.anything(),
      ),
    );
  });

  it("disables Run without a connection context", async () => {
    useConnectionStore.setState({ activeConnectionId: null });
    useWorkspaceStore.setState({
      tabs: [tab({ connectionId: null })],
      activeTabId: "t1",
    });
    renderWithProviders(<EditorPanel />);

    const runButton = await screen.findByRole("button", { name: "Run query" });
    expect(runButton).toBeDisabled();

    // Keyboard path still honours the guard and surfaces a message.
    fireEvent.click(screen.getByRole("button", { name: "editor-execute" }));
    expect(await screen.findByText(/NO_ACTIVE_CONNECTION/)).toBeInTheDocument();
  });

  it("blocks execution when the bound connection no longer exists", async () => {
    vi.mocked(listConnections).mockResolvedValue([]);
    useWorkspaceStore.setState({
      tabs: [tab({ connectionId: "gone" })],
      activeTabId: "t1",
    });
    renderWithProviders(<EditorPanel />);

    const runButton = await screen.findByRole("button", { name: "Run query" });
    await waitFor(() => expect(runButton).toBeDisabled());

    fireEvent.click(screen.getByRole("button", { name: "editor-execute" }));
    expect(
      await screen.findByText(/CONNECTION_NOT_AVAILABLE/),
    ).toBeInTheDocument();
    expect(executeQuery).not.toHaveBeenCalled();
  });

  it("shows a running indicator and disables Run while executing", async () => {
    vi.mocked(executeQuery).mockImplementation(
      () => new Promise(() => {}) as never,
    );
    renderWithProviders(<EditorPanel />);

    const runButton = await screen.findByRole("button", { name: "Run query" });
    await waitFor(() => expect(runButton).toBeEnabled());
    fireEvent.click(runButton);

    expect(await screen.findByText("Running…")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Run query" })).toBeDisabled();
  });

  it("allows dismissing a query error", async () => {
    vi.mocked(executeQuery).mockRejectedValue(
      new ApiClientError("syntax error", "SQL_SYNTAX_ERROR", 400),
    );
    renderWithProviders(<EditorPanel />);

    const runButton = await screen.findByRole("button", { name: "Run query" });
    await waitFor(() => expect(runButton).toBeEnabled());
    fireEvent.click(runButton);

    expect(await screen.findByText(/SQL_SYNTAX_ERROR/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Dismiss error" }));
    expect(screen.queryByText(/SQL_SYNTAX_ERROR/)).not.toBeInTheDocument();
  });

  it("clears a previous error after a successful run", async () => {
    vi.mocked(executeQuery).mockRejectedValueOnce(
      new ApiClientError("boom", "SQL_ERROR", 400),
    );
    renderWithProviders(<EditorPanel />);

    const runButton = await screen.findByRole("button", { name: "Run query" });
    await waitFor(() => expect(runButton).toBeEnabled());
    fireEvent.click(runButton);
    expect(await screen.findByText(/SQL_ERROR/)).toBeInTheDocument();

    vi.mocked(executeQuery).mockResolvedValue(result as never);
    fireEvent.click(screen.getByRole("button", { name: "Run query" }));

    await waitFor(() =>
      expect(screen.queryByText(/SQL_ERROR/)).not.toBeInTheDocument(),
    );
  });

  it("scopes schema and dialect to the tab's connection, not the active one", async () => {
    useConnectionStore.setState({ activeConnectionId: "c2" });
    useWorkspaceStore.setState({
      tabs: [tab({ connectionId: "c1" })],
      activeTabId: "t1",
    });
    renderWithProviders(<EditorPanel />);

    await waitFor(() =>
      expect(getSchemas).toHaveBeenCalledWith("c1", expect.anything()),
    );
    expect(screen.getByTestId("editor-dialect")).toHaveTextContent("postgres");
  });

  it("uses the MySQL dialect for a MySQL-bound tab", async () => {
    vi.mocked(listConnections).mockResolvedValue([
      { id: "m1", name: "MySQL", driver: "mysql", host: "h", port: 3306, database_name: "app" },
    ] as never);
    useConnectionStore.setState({ activeConnectionId: "m1" });
    useWorkspaceStore.setState({
      tabs: [tab({ connectionId: "m1" })],
      activeTabId: "t1",
    });
    renderWithProviders(<EditorPanel />);

    await waitFor(() =>
      expect(screen.getByTestId("editor-dialect")).toHaveTextContent("mysql"),
    );
  });

  it("opens the save dialog from the editor save shortcut (Cmd+S)", async () => {
    renderWithProviders(<EditorPanel />);
    fireEvent.click(screen.getByRole("button", { name: "editor-save" }));
    expect(
      await screen.findByRole("heading", { name: "Save query" }),
    ).toBeInTheDocument();
  });

  it("clears dirty state and links the tab after saving", async () => {
    vi.mocked(createSavedQuery).mockResolvedValue({
      id: "s1",
      title: "My query",
      tags: null,
    } as never);
    useWorkspaceStore.setState({
      tabs: [tab({ connectionId: "c1", sql: "SELECT 1;", dirty: true })],
      activeTabId: "t1",
    });
    renderWithProviders(<EditorPanel />);

    fireEvent.click(screen.getByRole("button", { name: "Save query" }));
    fireEvent.change(await screen.findByLabelText("Title"), {
      target: { value: "My query" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(asQueryTab(useWorkspaceStore.getState().tabs[0]).savedQueryId).toBe("s1"),
    );
    expect(asQueryTab(useWorkspaceStore.getState().tabs[0]).dirty).toBe(false);
    expect(executeQuery).not.toHaveBeenCalled();
  });

  it("explicitly rebinds a tab without rewriting its SQL", async () => {
    useWorkspaceStore.setState({
      tabs: [tab({ connectionId: "c1", sql: "SELECT 1;", database: "alpha" })],
      activeTabId: "t1",
    });
    renderWithProviders(<EditorPanel />);

    const select = await screen.findByLabelText("Tab connection");
    await screen.findByRole("option", { name: "PG Two" });
    fireEvent.change(select, { target: { value: "c2" } });

    const bound = asQueryTab(useWorkspaceStore.getState().tabs[0]);
    expect(bound.connectionId).toBe("c2");
    // The old connection's database is cleared and the new connection's default
    // database (c2 -> app2) is bound deterministically.
    expect(bound.database).toBe("app2");
    expect(bound.sql).toBe("SELECT 1;");
    await waitFor(() =>
      expect(getSchemas).toHaveBeenCalledWith(
        "c2",
        expect.objectContaining({ database: "app2" }),
      ),
    );
    expect(executeQuery).not.toHaveBeenCalled();
  });

  it("clears the database on a server-level connection and blocks execution until bound", async () => {
    vi.mocked(listConnections).mockResolvedValue([
      { id: "c1", name: "PG One", driver: "postgres", host: "h", port: 5432, database_name: "app" },
      { id: "c3", name: "PG Server", driver: "postgres", host: "h", port: 5432, database_name: "" },
    ] as never);
    useWorkspaceStore.setState({
      tabs: [tab({ connectionId: "c1", database: "alpha" })],
      activeTabId: "t1",
    });
    renderWithProviders(<EditorPanel />);

    const select = await screen.findByLabelText("Tab connection");
    await screen.findByRole("option", { name: "PG Server" });
    fireEvent.change(select, { target: { value: "c3" } });

    const bound = asQueryTab(useWorkspaceStore.getState().tabs[0]);
    expect(bound.connectionId).toBe("c3");
    expect(bound.database).toBeNull();

    // No database bound yet: the Run button is disabled and a keyboard run is
    // rejected locally without contacting the backend.
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Run query" })).toBeDisabled(),
    );
    fireEvent.keyDown(window, { key: "Enter", metaKey: true });
    expect(executeQuery).not.toHaveBeenCalled();
    expect(await screen.findByText(/DATABASE_REQUIRED/)).toBeInTheDocument();
  });

  it("clears stale execution state when switching tabs", async () => {
    useWorkspaceStore.setState({
      tabs: [tab({ kind: "query", id: "t1" }), tab({ id: "t2", title: "Query 2" })],
      activeTabId: "t1",
    });
    renderWithProviders(<EditorPanel />);

    act(() => {
      useExecutionStore.getState().resolve(result as never);
    });
    expect(useExecutionStore.getState().status).toBe("success");

    act(() => {
      useWorkspaceStore.getState().setActiveTab("t2");
    });

    await waitFor(() => expect(useExecutionStore.getState().status).toBe("idle"));
    expect(useExecutionStore.getState().result).toBeNull();
  });

  it("runs with Mod+Enter when focus is outside the editor", async () => {
    vi.mocked(executeQuery).mockResolvedValue(result as never);
    renderWithProviders(<EditorPanel />);

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Run query" })).toBeEnabled(),
    );
    fireEvent.keyDown(window, { key: "Enter", metaKey: true });

    await waitFor(() =>
      expect(executeQuery).toHaveBeenCalledWith(
        expect.objectContaining({ sql: "SELECT 1" }),
        expect.anything(),
      ),
    );
  });

  it("opens the save dialog with Mod+S when focus is outside the editor", async () => {
    renderWithProviders(<EditorPanel />);

    fireEvent.keyDown(window, { key: "s", metaKey: true });

    expect(
      await screen.findByRole("heading", { name: "Save query" }),
    ).toBeInTheDocument();
    expect(executeQuery).not.toHaveBeenCalled();
  });

  it("guards Run while offline and preserves editor content", async () => {
    setOnline(false);
    useWorkspaceStore.setState({
      tabs: [tab({ sql: "SELECT 1;" })],
      activeTabId: "t1",
    });
    renderWithProviders(<EditorPanel />);

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Run query" })).toBeDisabled(),
    );
    expect(screen.getByTestId("editor-value")).toHaveTextContent("SELECT 1;");

    fireEvent.click(screen.getByRole("button", { name: "editor-execute" }));
    expect(await screen.findByText(/OFFLINE/)).toBeInTheDocument();
    expect(executeQuery).not.toHaveBeenCalled();
  });

  it("recovers when connectivity returns without auto-executing", async () => {
    setOnline(false);
    renderWithProviders(<EditorPanel />);

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Run query" })).toBeDisabled(),
    );

    act(() => setOnline(true));

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Run query" })).toBeEnabled(),
    );
    expect(executeQuery).not.toHaveBeenCalled();
  });

  it("shows where the tab will execute (connection / database)", async () => {
    useWorkspaceStore.setState({
      tabs: [tab({ database: "CCM" })],
      activeTabId: "t1",
    });
    renderWithProviders(<EditorPanel />);

    const context = await screen.findByTestId("query-context");
    await waitFor(() => expect(context).toHaveTextContent("PG One / CCM"));
  });

  it("executes against the tab's bound database", async () => {
    vi.mocked(executeQuery).mockResolvedValue(result as never);
    useWorkspaceStore.setState({
      tabs: [tab({ database: "CCM" })],
      activeTabId: "t1",
    });
    renderWithProviders(<EditorPanel />);

    const runButton = await screen.findByRole("button", { name: "Run query" });
    await waitFor(() => expect(runButton).toBeEnabled());
    fireEvent.click(runButton);

    await waitFor(() =>
      expect(executeQuery).toHaveBeenCalledWith(
        expect.objectContaining({ connection_id: "c1", database: "CCM" }),
        expect.anything(),
      ),
    );
  });

  it("rebinds only the active tab's database without executing", async () => {
    vi.mocked(listConnections).mockResolvedValue([
      {
        id: "c1",
        name: "PG Server",
        driver: "postgres",
        host: "h",
        port: 5432,
        database_name: "",
      },
    ] as never);
    vi.mocked(listDatabases).mockResolvedValue([
      { name: "CCM" },
      { name: "reporting" },
    ] as never);
    useWorkspaceStore.setState({
      tabs: [tab(), { ...tab({ id: "t2", title: "Query 2", database: "reporting" }) }],
      activeTabId: "t1",
    });
    renderWithProviders(<EditorPanel />);

    const select = await screen.findByLabelText("Tab database");
    await screen.findByRole("option", { name: "reporting" });
    fireEvent.change(select, { target: { value: "CCM" } });

    const state = useWorkspaceStore.getState();
    expect(state.tabs.find((t) => t.id === "t1")?.database).toBe("CCM");
    expect(state.tabs.find((t) => t.id === "t2")?.database).toBe("reporting");
    expect(executeQuery).not.toHaveBeenCalled();
  });

  it("scopes schema/autocomplete metadata to the tab's database", async () => {
    useWorkspaceStore.setState({
      tabs: [tab({ database: "CCM" })],
      activeTabId: "t1",
    });
    renderWithProviders(<EditorPanel />);

    await waitFor(() =>
      expect(getSchemas).toHaveBeenCalledWith(
        "c1",
        expect.objectContaining({ database: "CCM" }),
      ),
    );
  });

  it("saves the tab's database context with the query", async () => {
    vi.mocked(createSavedQuery).mockResolvedValue({ kind: "query", id: "s1", title: "CCM query", tags: null } as never);
    useWorkspaceStore.setState({
      tabs: [tab({ database: "CCM", sql: "SELECT 1;" })],
      activeTabId: "t1",
    });
    renderWithProviders(<EditorPanel />);

    fireEvent.click(await screen.findByRole("button", { name: "Save query" }));
    fireEvent.change(await screen.findByLabelText("Title"), {
      target: { value: "CCM query" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(createSavedQuery).toHaveBeenCalledWith(
        expect.objectContaining({ database_name: "CCM" }),
      ),
    );
  });

  it("keyboard run uses the tab's database, not the explorer selection", async () => {
    vi.mocked(executeQuery).mockResolvedValue(result as never);
    useConnectionStore.setState({
      activeConnectionId: "c1",
      activeDatabaseByConnection: { c1: "reporting" },
    });
    useWorkspaceStore.setState({
      tabs: [tab({ database: "CCM" })],
      activeTabId: "t1",
    });
    renderWithProviders(<EditorPanel />);

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Run query" })).toBeEnabled(),
    );
    fireEvent.keyDown(window, { key: "Enter", metaKey: true });

    await waitFor(() =>
      expect(executeQuery).toHaveBeenCalledWith(
        expect.objectContaining({ database: "CCM" }),
        expect.anything(),
      ),
    );
  });
});
