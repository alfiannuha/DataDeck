import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AppShell } from "@/components/shared/app-shell";
import {
  executeQuery,
  getSchemas,
  listConnections,
  listDatabases,
} from "@/lib/api/endpoints";
import { ApiClientError } from "@/lib/api-client";
import { renderWithProviders } from "@/test/render";
import { useConnectionStore } from "@/store/useConnectionStore";
import { useExecutionStore } from "@/store/useExecutionStore";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

vi.mock("@/lib/api/endpoints", () => ({
  listConnections: vi.fn(),
  createConnection: vi.fn(),
  testConnection: vi.fn(),
  deleteConnection: vi.fn(),
  getSchemas: vi.fn(),
  executeQuery: vi.fn(),
  getQueryHistory: vi.fn(),
  getHealth: vi.fn(),
  listDatabases: vi.fn().mockResolvedValue([]),
  browseTableData: vi.fn().mockResolvedValue({
    database: "app",
    schema: "public",
    table: "users",
    object_type: "BASE TABLE",
    columns: [{ name: "id", database_type: "bigint", primary_key: true }],
    rows: [],
    pagination: { page: 1, page_size: 100, has_more: false },
    truncated: false,
  }),
}));

const downloadExport = vi.fn();
vi.mock("@/lib/export/build-export", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@/lib/export/build-export")>();
  return {
    ...actual,
    downloadExport: (...args: unknown[]) => downloadExport(...args),
  };
});

// CodeMirror needs layout APIs jsdom lacks; a light stub keeps the integration
// test focused on workspace flow rather than editor internals.
vi.mock("@/components/editor/sql-editor", async () => {
  const React = await import("react");
  return {
    SqlEditor: React.forwardRef(function StubEditor(props: { value: string }) {
      return <div data-testid="editor-value">{props.value}</div>;
    }),
  };
});

const connections = [
  { id: "c1", name: "Grid Demo", driver: "postgres", host: "127.0.0.1", port: 5432, database_name: "app" },
  { id: "c2", name: "Other", driver: "postgres", host: "127.0.0.1", port: 5432, database_name: "other" },
];

const schema = [
  {
    name: "app",
    schemas: [
      {
        name: "public",
        tables: [
          { schema: "public", name: "users", type: "BASE TABLE", columns: [{ name: "id", data_type: "bigint" }] },
        ],
      },
    ],
  },
];

beforeEach(() => {
  vi.clearAllMocks();
  useConnectionStore.setState({ activeConnectionId: null });
  useExecutionStore.getState().reset();
  useWorkspaceStore.setState({
    tabs: [{ kind: "query", id: "t1", title: "Query 1", sql: "", connectionId: null, database: null, dirty: false }],
    activeTabId: "t1",
  });
  downloadExport.mockReset();
  vi.mocked(listConnections).mockResolvedValue(connections as never);
  vi.mocked(getSchemas).mockResolvedValue(schema as never);
});

describe("integrated workspace flow", () => {
  it("runs a query end to end: connection → schema → select top 100 → execute → grid → status", { timeout: 20000 }, async () => {
    vi.mocked(executeQuery).mockResolvedValue({
      columns: [{ name: "id", type: "bigint" }],
      rows: [["1"]],
      rows_affected: 1,
      execution_time_ms: 4,
      truncated: false,
    } as never);

    renderWithProviders(<AppShell />);

    // Connections list loads; select the connection.
    fireEvent.click(await screen.findByRole(
      "button",
      { name: /^Grid Demo/ },
      { timeout: 5000 },
    ));
    expect(useConnectionStore.getState().activeConnectionId).toBe("c1");

    // Schema loads for the active connection.
    fireEvent.click(await screen.findByRole("button", { name: /^public/ }, { timeout: 5000 }));
    fireEvent.click(screen.getByRole("button", { name: "Select top 100 from users" }));

    // SQL is inserted into the query tab (not executed).
    await waitFor(() =>
      expect(screen.getByTestId("editor-value").textContent).toContain(
        'FROM "public"."users"',
      ),
    );
    expect(executeQuery).not.toHaveBeenCalled();

    // Run through the toolbar; execution uses the active connection and SQL.
    fireEvent.click(screen.getByRole("button", { name: "Run query" }));
    await waitFor(() =>
      expect(executeQuery).toHaveBeenCalledWith(
        expect.objectContaining({ connection_id: "c1" }),
        expect.anything(),
      ),
    );
    expect(
      vi.mocked(executeQuery).mock.calls[0][0].sql,
    ).toContain('FROM "public"."users"');

    // Result grid and status bar reflect the execution.
    expect(await screen.findByRole("gridcell")).toHaveTextContent("1");
    expect(screen.getByText(/^Success ·/)).toBeInTheDocument();
  });

  it("clears execution state and reloads schema when switching connections", { timeout: 20000 }, async () => {
    vi.mocked(executeQuery).mockResolvedValue({
      columns: [{ name: "id", type: "bigint" }],
      rows: [["1"]],
      rows_affected: 1,
      execution_time_ms: 1,
      truncated: false,
    } as never);

    renderWithProviders(<AppShell />);

    fireEvent.click(await screen.findByRole(
      "button",
      { name: /^Grid Demo/ },
      { timeout: 5000 },
    ));
    fireEvent.click(await screen.findByRole("button", { name: /^public/ }, { timeout: 5000 }));
    fireEvent.click(screen.getByRole("button", { name: "Select top 100 from users" }));
    fireEvent.click(screen.getByRole("button", { name: "Run query" }));
    expect(await screen.findByRole("gridcell")).toBeInTheDocument();

    // Switch to another connection.
    fireEvent.click(screen.getByRole("button", { name: /^Other/ }));

    await waitFor(() =>
      expect(getSchemas).toHaveBeenCalledWith("c2", expect.anything()),
    );
    // Previous result is cleared; status returns to Ready.
    expect(screen.getByText("Ready")).toBeInTheDocument();
    expect(screen.queryByRole("gridcell")).not.toBeInTheDocument();

    // The tab stays bound to its originating connection (c1), not the newly
    // active one (c2): no silent rebinding.
    vi.mocked(executeQuery).mockClear();
    fireEvent.click(screen.getByRole("button", { name: "Run query" }));
    await waitFor(() =>
      expect(executeQuery).toHaveBeenCalledWith(
        expect.objectContaining({ connection_id: "c1" }),
        expect.anything(),
      ),
    );
  });

  it("inserts a safe Count Rows statement without executing", async () => {
    renderWithProviders(<AppShell />);

    fireEvent.click(await screen.findByRole(
      "button",
      { name: /^Grid Demo/ },
      { timeout: 5000 },
    ));
    fireEvent.click(await screen.findByRole("button", { name: /^public/ }, { timeout: 5000 }));
    fireEvent.click(screen.getByRole("button", { name: "Count rows in users" }));

    await waitFor(() =>
      expect(screen.getByTestId("editor-value").textContent).toBe(
        'SELECT COUNT(*)\nFROM "public"."users";',
      ),
    );
    expect(executeQuery).not.toHaveBeenCalled();
  });

  it("exports the executed result as exact CSV and JSON without re-running SQL", async () => {
    vi.mocked(executeQuery).mockResolvedValue({
      columns: [{ name: "id", type: "bigint" }, { name: "name", type: "text" }],
      rows: [
        ["9007199254740993", "a,b"],
        [null, "=1+1"],
      ],
      rows_affected: 2,
      execution_time_ms: 3,
      truncated: false,
    } as never);

    renderWithProviders(<AppShell />);

    fireEvent.click(await screen.findByRole(
      "button",
      { name: /^Grid Demo/ },
      { timeout: 5000 },
    ));
    fireEvent.click(await screen.findByRole("button", { name: /^public/ }, { timeout: 5000 }));
    fireEvent.click(screen.getByRole("button", { name: "Select top 100 from users" }));
    fireEvent.click(screen.getByRole("button", { name: "Run query" }));

    await screen.findAllByRole("gridcell");
    vi.mocked(executeQuery).mockClear();

    fireEvent.click(screen.getByRole("button", { name: "Export CSV" }));
    const csv = downloadExport.mock.calls[0][0] as { content: string };
    expect(csv.content).toContain("9007199254740993");
    expect(csv.content).toContain('"a,b"');
    expect(csv.content).toContain("\n,");
    expect(csv.content).toContain("'=1+1");

    fireEvent.click(screen.getByRole("button", { name: "Export JSON" }));
    const json = downloadExport.mock.calls[1][0] as { content: string };
    expect(JSON.parse(json.content)).toEqual([
      { id: "9007199254740993", name: "a,b" },
      { id: null, name: "=1+1" },
    ]);

    expect(executeQuery).not.toHaveBeenCalled();
  });

  it("drops an in-flight result after the user switches tabs", async () => {
    let release: (value: unknown) => void = () => {};
    vi.mocked(executeQuery).mockImplementation(
      () =>
        new Promise((resolve) => {
          release = resolve;
        }) as never,
    );

    renderWithProviders(<AppShell />);

    fireEvent.click(await screen.findByRole(
      "button",
      { name: /^Grid Demo/ },
      { timeout: 5000 },
    ));
    fireEvent.click(await screen.findByRole("button", { name: /^public/ }, { timeout: 5000 }));
    fireEvent.click(screen.getByRole("button", { name: "Select top 100 from users" }));
    await waitFor(() =>
      expect(screen.getByTestId("editor-value").textContent).toContain(
        'FROM "public"."users"',
      ),
    );
    fireEvent.click(screen.getByRole("button", { name: "Run query" }));

    // The user moves to a new tab while the query is still in flight.
    fireEvent.click(screen.getByRole("button", { name: "New query tab" }));

    await act(async () => {
      release({
        columns: [{ name: "id", type: "bigint" }],
        rows: [["1"]],
        rows_affected: 1,
        execution_time_ms: 5,
        truncated: false,
      });
    });

    // The stale result must not be applied to the newly active tab.
    await waitFor(() => expect(screen.getByText("Ready")).toBeInTheDocument());
    expect(screen.queryByRole("gridcell")).not.toBeInTheDocument();
  });

  it("terminates the running state when execution fails", async () => {
    vi.mocked(executeQuery).mockRejectedValue(
      new ApiClientError("failed to connect", "CONNECTION_ERROR", 502),
    );

    renderWithProviders(<AppShell />);

    fireEvent.click(await screen.findByRole(
      "button",
      { name: /^Grid Demo/ },
      { timeout: 5000 },
    ));
    fireEvent.click(await screen.findByRole("button", { name: /^public/ }, { timeout: 5000 }));
    fireEvent.click(screen.getByRole("button", { name: "Select top 100 from users" }));
    await waitFor(() =>
      expect(screen.getByTestId("editor-value").textContent).toContain(
        'FROM "public"."users"',
      ),
    );

    fireEvent.click(screen.getByRole("button", { name: "Run query" }));

    expect(
      await screen.findByText("Error · CONNECTION_ERROR"),
    ).toBeInTheDocument();
    // No permanent spinner: Run is usable again and no running indicator remains.
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Run query" })).toBeEnabled(),
    );
    expect(screen.queryByText("Running…")).not.toBeInTheDocument();
  });

  it("keeps database-bound tabs independent and never redirects them", async () => {
    vi.mocked(listConnections).mockResolvedValue([
      {
        id: "c1",
        name: "PG Server",
        driver: "postgres",
        host: "127.0.0.1",
        port: 5432,
        database_name: "",
      },
    ] as never);
    vi.mocked(executeQuery).mockImplementation((body) =>
      Promise.resolve({
        columns: [{ name: "db", type: "text" }],
        rows: [[body.database ?? "none"]],
        rows_affected: 1,
        execution_time_ms: 1,
        truncated: false,
      } as never),
    );
    useWorkspaceStore.setState({
      tabs: [
        { kind: "query", id: "tA", title: "A", sql: "SELECT 1;", connectionId: "c1", database: "CCM", dirty: false },
        { kind: "query", id: "tB", title: "B", sql: "SELECT 1;", connectionId: "c1", database: "reporting", dirty: false },
      ],
      activeTabId: "tA",
      sidebarCollapsed: false,
    });
    useConnectionStore.setState({ activeConnectionId: "c1" });

    renderWithProviders(<AppShell />);

    const runButton = await screen.findByRole("button", { name: "Run query" });
    await waitFor(() => expect(runButton).toBeEnabled());
    fireEvent.click(runButton);
    await waitFor(() =>
      expect(executeQuery).toHaveBeenLastCalledWith(
        expect.objectContaining({ database: "CCM" }),
        expect.anything(),
      ),
    );

    // Switch to tab B: it must execute against reporting, untouched by tab A.
    fireEvent.click(screen.getByRole("button", { name: /^B/ }));
    await waitFor(() =>
      expect(screen.getByTestId("query-context")).toHaveTextContent(
        "PG Server / reporting",
      ),
    );
    fireEvent.click(screen.getByRole("button", { name: "Run query" }));
    await waitFor(() =>
      expect(executeQuery).toHaveBeenLastCalledWith(
        expect.objectContaining({ database: "reporting" }),
        expect.anything(),
      ),
    );

    // Changing another tab's database binding never redirects the active tab.
    act(() => {
      useWorkspaceStore.getState().setTabDatabase("tA", "analytics");
    });
    expect(executeQuery).toHaveBeenCalledTimes(2);
  });

  it("renders a Table Data tab and preserves the query tab's SQL and dirty state", async () => {
    useWorkspaceStore.setState({
      tabs: [
        {
          kind: "query",
          id: "t1",
          title: "Query 1",
          sql: "SELECT 1;",
          connectionId: "c1",
          database: "app",
          dirty: true,
        },
      ],
      activeTabId: "t1",
    });

    renderWithProviders(<AppShell />);
    expect(await screen.findByTestId("editor-value")).toHaveTextContent("SELECT 1;");

    act(() => {
      useWorkspaceStore.getState().openTableData("c1", "app", "public", "users");
    });

    expect(await screen.findByTestId("table-data-view")).toBeInTheDocument();
    expect(screen.getByTestId("table-data-binding")).toHaveTextContent("app");
    expect(screen.queryByTestId("editor-value")).not.toBeInTheDocument();

    // Switching back restores the query tab with its SQL and dirty flag intact.
    act(() => {
      useWorkspaceStore.getState().setActiveTab("t1");
    });
    expect(await screen.findByTestId("editor-value")).toHaveTextContent("SELECT 1;");
    const query = useWorkspaceStore.getState().tabs.find((tab) => tab.id === "t1");
    expect(query?.kind === "query" && query.dirty).toBe(true);

    // The table tab kept its own independent binding.
    const table = useWorkspaceStore.getState().tabs.find(
      (tab) => tab.kind === "table-data",
    );
    expect(table?.connectionId).toBe("c1");
    expect(table?.database).toBe("app");
  });

  it("renders a Table Structure tab with its explicit binding", async () => {
    renderWithProviders(<AppShell />);
    await screen.findByTestId("editor-value");

    act(() => {
      useWorkspaceStore.getState().openTableStructure("c2", "other", "public", "orders");
    });

    expect(await screen.findByTestId("table-structure-panel")).toBeInTheDocument();
    expect(screen.getByTestId("table-structure-binding")).toHaveTextContent("other");
    expect(screen.getByText("public.orders")).toBeInTheDocument();
  });

  it("starts with zero tabs, hides the tab strip, and opens a Query on demand", async () => {
    useWorkspaceStore.setState({ tabs: [], activeTabId: null });
    useConnectionStore.setState({
      activeConnectionId: "c1",
      activeDatabaseByConnection: { c1: "app" },
    });

    renderWithProviders(<AppShell />);

    expect(await screen.findByTestId("workspace-empty-state")).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "Query tabs" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "New Query" }));

    expect(await screen.findByTestId("editor-value")).toBeInTheDocument();
    const tab = useWorkspaceStore.getState().tabs[0];
    expect(tab.connectionId).toBe("c1");
    expect(tab.database).toBe("app");
  });

  it("closing the final tab returns to the empty workspace", async () => {
    useConnectionStore.setState({ activeConnectionId: "c1" });
    renderWithProviders(<AppShell />);
    await screen.findByTestId("editor-value");

    fireEvent.click(screen.getByRole("button", { name: "Close Query 1" }));

    expect(await screen.findByTestId("workspace-empty-state")).toBeInTheDocument();
    const state = useWorkspaceStore.getState();
    expect(state.tabs).toHaveLength(0);
    expect(state.activeTabId).toBeNull();
  });

  it("selecting a connection does not create a workspace tab", async () => {
    useWorkspaceStore.setState({ tabs: [], activeTabId: null });
    useConnectionStore.setState({ activeConnectionId: null });

    renderWithProviders(<AppShell />);
    fireEvent.click(
      await screen.findByRole("button", { name: /^Grid Demo/ }),
    );

    await waitFor(() =>
      expect(useConnectionStore.getState().activeConnectionId).toBe("c1"),
    );
    expect(useWorkspaceStore.getState().tabs).toHaveLength(0);
    expect(useWorkspaceStore.getState().activeTabId).toBeNull();
  });

  it("renders the empty state instead of crashing on a stale activeTabId", async () => {
    useWorkspaceStore.setState({
      tabs: [
        {
          kind: "query",
          id: "t1",
          title: "Query 1",
          sql: "SELECT 1;",
          connectionId: "c1",
          database: "app",
          dirty: false,
        },
      ],
      activeTabId: "does-not-exist",
    });

    renderWithProviders(<AppShell />);

    expect(await screen.findByTestId("workspace-empty-state")).toBeInTheDocument();
    // The strip remains so the user can recover to the existing tab.
    expect(screen.getByRole("button", { name: /^Query 1/ })).toBeInTheDocument();
  });

  it("binds New Query to the selected database and never rebinds on explorer changes (PRF-01)", async () => {
    vi.mocked(listConnections).mockResolvedValue([
      { id: "srv", name: "Server", driver: "postgres", host: "127.0.0.1", port: 5432, database_name: "" },
    ] as never);
    vi.mocked(listDatabases).mockResolvedValue([
      { name: "alpha" },
      { name: "beta" },
    ] as never);
    useWorkspaceStore.setState({ tabs: [], activeTabId: null });
    useConnectionStore.setState({
      activeConnectionId: "srv",
      activeDatabaseByConnection: {},
    });

    renderWithProviders(<AppShell />);

    // Select a database in the explorer; the workspace stays empty.
    fireEvent.click(await screen.findByRole("button", { name: /^alpha/ }));
    expect(useWorkspaceStore.getState().tabs).toHaveLength(0);

    // Explicit New Query binds the selected database.
    fireEvent.click(screen.getByRole("button", { name: "New Query" }));
    const tab = useWorkspaceStore.getState().tabs[0];
    expect(tab.connectionId).toBe("srv");
    expect(tab.database).toBe("alpha");

    // Changing the explorer selection must not rebind the existing tab.
    fireEvent.click(await screen.findByRole("button", { name: /^beta/ }));
    expect(useConnectionStore.getState().activeDatabaseByConnection.srv).toBe("beta");
    expect(useWorkspaceStore.getState().tabs[0].database).toBe("alpha");
  });
});
