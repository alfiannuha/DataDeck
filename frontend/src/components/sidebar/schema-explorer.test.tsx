import { fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiClientError } from "@/lib/api-client";
import { executeQuery, getSchemas, listConnections, listDatabases } from "@/lib/api/endpoints";
import { renderWithProviders } from "@/test/render";
import { useConnectionStore } from "@/store/useConnectionStore";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";
import type { DatabaseSchemaTree } from "@/types/api";

import { SchemaExplorer } from "./schema-explorer";

vi.mock("@/lib/api/endpoints", () => ({
  listConnections: vi.fn(),
  createConnection: vi.fn(),
  testConnection: vi.fn(),
  deleteConnection: vi.fn(),
  getSchemas: vi.fn(),
  listDatabases: vi.fn().mockResolvedValue([]),
  executeQuery: vi.fn(),
  getQueryHistory: vi.fn(),
  getHealth: vi.fn(),
}));

const sampleSchema: DatabaseSchemaTree[] = [
  {
    name: "app",
    schemas: [
      {
        name: "public",
        tables: [
          {
            schema: "public",
            name: "users",
            type: "BASE TABLE",
            columns: [
              {
                name: "id",
                data_type: "bigint",
                nullable: false,
                ordinal_position: 1,
              },
              {
                name: "email",
                data_type: "character varying(255)",
                nullable: true,
                ordinal_position: 2,
              },
            ],
            primary_key: { name: "users_pkey", columns: ["id"] },
            foreign_keys: [
              {
                name: "users_role_fk",
                columns: ["role_id"],
                referenced_schema: "public",
                referenced_table: "roles",
                referenced_columns: ["id"],
              },
            ],
            indexes: [
              {
                name: "idx_users_email",
                unique: false,
                primary: false,
                columns: ["email"],
              },
            ],
          },
        ],
      },
    ],
  },
];

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("SchemaExplorer", () => {
  it("prompts to select a connection when none is active", () => {
    renderWithProviders(<SchemaExplorer connectionId={null} />);
    expect(
      screen.getByText("Select a connection to browse its schema."),
    ).toBeInTheDocument();
    expect(getSchemas).not.toHaveBeenCalled();
  });

  it("shows a loading state", () => {
    vi.mocked(getSchemas).mockReturnValue(new Promise(() => {}));
    renderWithProviders(<SchemaExplorer connectionId="c1" />);
    expect(screen.getByText("Loading schema…")).toBeInTheDocument();
  });

  it("shows a normalized error with retry", async () => {
    vi.mocked(getSchemas).mockRejectedValue(
      new ApiClientError("failed to read the database schema", "INTROSPECTION_ERROR", 502),
    );
    renderWithProviders(<SchemaExplorer connectionId="c1" />);

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("INTROSPECTION_ERROR");
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });

  it("shows an empty state when there are no objects", async () => {
    vi.mocked(getSchemas).mockResolvedValue([]);
    renderWithProviders(<SchemaExplorer connectionId="c1" />);
    expect(
      await screen.findByText("No schema objects found."),
    ).toBeInTheDocument();
  });

  it("renders the hierarchy lazily and shows metadata when expanded", async () => {
    vi.mocked(getSchemas).mockResolvedValue(sampleSchema);
    renderWithProviders(<SchemaExplorer connectionId="c1" />);

    // Database is visible; tables are not rendered until expanded.
    expect(await screen.findByText("app")).toBeInTheDocument();
    expect(screen.queryByText("users")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /^public/ }));
    expect(screen.getByRole("button", { name: /^users/ })).toBeInTheDocument();

    // Columns remain unmounted until the table is expanded.
    expect(screen.queryByText("email")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /^users/ }));

    expect(screen.getByText("email")).toBeInTheDocument();
    expect(screen.getByText(/bigint/)).toBeInTheDocument();
    expect(screen.getByText(/NOT NULL/)).toBeInTheDocument();
    expect(screen.getByText("PK")).toBeInTheDocument();
    expect(screen.getByText("FK")).toBeInTheDocument();
    expect(screen.getByText(/idx_users_email/)).toBeInTheDocument();
  });

  it("refreshes the schema", async () => {
    vi.mocked(getSchemas).mockResolvedValue(sampleSchema);
    renderWithProviders(<SchemaExplorer connectionId="c1" />);

    await screen.findByText("app");
    fireEvent.click(screen.getByRole("button", { name: "Refresh schema" }));

    await waitFor(() => expect(getSchemas).toHaveBeenCalledTimes(2));
  });

  it("switches schema state when the active connection changes", async () => {
    vi.mocked(getSchemas).mockResolvedValue(sampleSchema);
    const { rerender } = renderWithProviders(
      <SchemaExplorer connectionId="c1" />,
    );
    await screen.findByText("app");
    fireEvent.click(screen.getByRole("button", { name: /^public/ }));
    expect(screen.getByRole("button", { name: /^users/ })).toBeInTheDocument();

    rerender(<SchemaExplorer connectionId="c2" />);

    await waitFor(() => expect(getSchemas).toHaveBeenCalledWith("c2", expect.anything()));
    // Expansion resets for the new connection: tables hidden again.
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: /^users/ })).not.toBeInTheDocument(),
    );
  });

  it("copies table and column names to the clipboard", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    vi.mocked(getSchemas).mockResolvedValue(sampleSchema);

    renderWithProviders(<SchemaExplorer connectionId="c1" />);
    await screen.findByText("app");
    fireEvent.click(screen.getByRole("button", { name: /^public/ }));

    fireEvent.click(
      screen.getByRole("button", { name: "Copy table name users" }),
    );
    await waitFor(() => expect(writeText).toHaveBeenCalledWith("public.users"));

    fireEvent.click(screen.getByRole("button", { name: /^users/ }));
    fireEvent.click(
      screen.getByRole("button", { name: "Copy column name id" }),
    );
    await waitFor(() => expect(writeText).toHaveBeenCalledWith("id"));
  });

  it("inserts a safe Select Top 100 statement into a query tab", async () => {
    vi.mocked(getSchemas).mockResolvedValue(sampleSchema);
    useWorkspaceStore.setState({
      tabs: [{ id: "t1", title: "Query 1", sql: "", connectionId: null, dirty: false }],
      activeTabId: "t1",
    });

    renderWithProviders(<SchemaExplorer connectionId="c1" />);
    await screen.findByText("app");
    fireEvent.click(screen.getByRole("button", { name: /^public/ }));
    fireEvent.click(
      screen.getByRole("button", { name: "Select top 100 from users" }),
    );

    const state = useWorkspaceStore.getState();
    const active = state.tabs.find((tab) => tab.id === state.activeTabId);
    expect(active?.sql).toContain('FROM "public"."users"');
    expect(active?.sql).toContain("LIMIT 100;");
    expect(active?.connectionId).toBe("c1");
  });

  it("renders database-level tables when the engine has no schemas", async () => {
    vi.mocked(getSchemas).mockResolvedValue([
      {
        name: "main.db",
        tables: [
          {
            name: "widgets",
            type: "BASE TABLE",
            columns: [{ name: "id", data_type: "integer" }],
          },
        ],
      },
    ] as never);

    renderWithProviders(<SchemaExplorer connectionId="c1" />);
    await screen.findByText("main.db");

    // Database-level tables render directly (no schema level).
    expect(
      screen.getByRole("button", { name: /^widgets/ }),
    ).toBeInTheDocument();
  });

  function connection(driver: string, database_name: string) {
    return {
      id: "c1",
      name: "Conn",
      driver,
      database_name,
      database: database_name,
      ssl_mode: "disable",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    } as never;
  }

  const sqliteSchema: DatabaseSchemaTree[] = [
    {
      name: "main.db",
      tables: [
        {
          schema: "",
          name: "widgets",
          type: "BASE TABLE",
          columns: [{ name: "id", data_type: "integer" }],
        },
      ],
    },
  ];

  it("inserts a driver-aware Count Rows statement without executing it", async () => {
    vi.mocked(getSchemas).mockResolvedValue(sampleSchema);
    useWorkspaceStore.setState({
      tabs: [{ id: "t1", title: "Query 1", sql: "", connectionId: null, dirty: false }],
      activeTabId: "t1",
    });

    renderWithProviders(<SchemaExplorer connectionId="c1" />);
    await screen.findByText("app");
    fireEvent.click(screen.getByRole("button", { name: /^public/ }));
    fireEvent.click(screen.getByRole("button", { name: "Count rows in users" }));

    const state = useWorkspaceStore.getState();
    const active = state.tabs.find((tab) => tab.id === state.activeTabId);
    expect(active?.sql).toBe('SELECT COUNT(*)\nFROM "public"."users";');
    expect(active?.connectionId).toBe("c1");
    expect(executeQuery).not.toHaveBeenCalled();
  });

  it("quotes MySQL Count Rows with the connection database", async () => {
    vi.mocked(listConnections).mockResolvedValue([connection("mysql", "app")]);
    vi.mocked(getSchemas).mockResolvedValue(sqliteSchema);
    useWorkspaceStore.setState({
      tabs: [{ id: "t1", title: "Query 1", sql: "", connectionId: null, dirty: false }],
      activeTabId: "t1",
    });

    renderWithProviders(<SchemaExplorer connectionId="c1" />);
    await screen.findByText("main.db");
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Count rows in widgets" }),
      ).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "Count rows in widgets" }));

    const state = useWorkspaceStore.getState();
    const active = state.tabs.find((tab) => tab.id === state.activeTabId);
    expect(active?.sql).toBe("SELECT COUNT(*)\nFROM `app`.`widgets`;");
  });

  it("hides Copy DDL for PostgreSQL (unsupported, never faked)", async () => {
    vi.mocked(listConnections).mockResolvedValue([connection("postgres", "app")]);
    vi.mocked(getSchemas).mockResolvedValue(sampleSchema);

    renderWithProviders(<SchemaExplorer connectionId="c1" />);
    await screen.findByText("app");
    fireEvent.click(screen.getByRole("button", { name: /^public/ }));

    expect(
      screen.getByRole("button", { name: "Count rows in users" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Copy DDL for users" }),
    ).not.toBeInTheDocument();
  });

  it("copies accurate MySQL DDL via SHOW CREATE TABLE", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    vi.mocked(listConnections).mockResolvedValue([connection("mysql", "app")]);
    vi.mocked(getSchemas).mockResolvedValue(sqliteSchema);
    vi.mocked(executeQuery).mockResolvedValue({
      columns: [
        { name: "Table", type: "text" },
        { name: "Create Table", type: "text" },
      ],
      rows: [["widgets", "CREATE TABLE `widgets` (id int)"]],
      rows_affected: 0,
      execution_time_ms: 1,
      truncated: false,
    } as never);

    renderWithProviders(<SchemaExplorer connectionId="c1" />);
    await screen.findByText("main.db");
    fireEvent.click(
      await screen.findByRole("button", { name: "Copy DDL for widgets" }),
    );

    await waitFor(() =>
      expect(executeQuery).toHaveBeenCalledWith(
        expect.objectContaining({ sql: "SHOW CREATE TABLE `app`.`widgets`;" }),
      ),
    );
    await waitFor(() =>
      expect(writeText).toHaveBeenCalledWith("CREATE TABLE `widgets` (id int)"),
    );
    expect(screen.getByRole("status")).toHaveTextContent("DDL copied");
  });

  it("copies SQLite DDL from sqlite_master and reports retrieval errors", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    vi.mocked(listConnections).mockResolvedValue([connection("sqlite", "main.db")]);
    vi.mocked(getSchemas).mockResolvedValue(sqliteSchema);
    vi.mocked(executeQuery).mockResolvedValue({
      columns: [{ name: "sql", type: "text" }],
      rows: [["CREATE TABLE widgets (id int)"]],
      rows_affected: 0,
      execution_time_ms: 1,
      truncated: false,
    } as never);

    renderWithProviders(<SchemaExplorer connectionId="c1" />);
    await screen.findByText("main.db");
    fireEvent.click(
      await screen.findByRole("button", { name: "Copy DDL for widgets" }),
    );

    await waitFor(() =>
      expect(executeQuery).toHaveBeenCalledWith(
        expect.objectContaining({
          sql: "SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'widgets';",
        }),
      ),
    );
    await waitFor(() =>
      expect(writeText).toHaveBeenCalledWith("CREATE TABLE widgets (id int)"),
    );
  });

  it("shows an error notice when DDL retrieval fails", async () => {
    vi.mocked(listConnections).mockResolvedValue([connection("sqlite", "main.db")]);
    vi.mocked(getSchemas).mockResolvedValue(sqliteSchema);
    vi.mocked(executeQuery).mockRejectedValue(
      new ApiClientError("no such table", "SQL_ERROR", 400),
    );

    renderWithProviders(<SchemaExplorer connectionId="c1" />);
    await screen.findByText("main.db");
    fireEvent.click(
      await screen.findByRole("button", { name: "Copy DDL for widgets" }),
    );

    expect(await screen.findByRole("alert")).toHaveTextContent("no such table");
  });

});

describe("SchemaExplorer database discovery (PRF-01)", () => {
  function serverConnection(id = "c1", name = "Server") {
    return {
      id,
      name,
      driver: "postgres",
      database_name: "",
      host: "127.0.0.1",
      port: 5432,
      username: "u",
      ssl_mode: "disable",
    } as never;
  }

  /** A per-database tree with tables directly on the database (no schema level). */
  function databaseTree(database: string) {
    return [
      {
        name: database,
        tables: [
          {
            schema: "",
            name: `table_in_${database}`,
            type: "BASE TABLE",
            columns: [],
          },
        ],
      },
    ];
  }

  it("lists databases and lazy-loads schemas per database with isolation", async () => {
    vi.mocked(listConnections).mockResolvedValue([serverConnection()]);
    vi.mocked(listDatabases).mockResolvedValue([
      { name: "alpha" },
      { name: "beta" },
    ] as never);
    vi.mocked(getSchemas).mockImplementation((_id, options) =>
      Promise.resolve(databaseTree(options?.database ?? "none") as never),
    );

    renderWithProviders(<SchemaExplorer connectionId="c1" />);

    expect(await screen.findByRole("button", { name: /^alpha/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^beta/ })).toBeInTheDocument();
    // No per-database introspection until a database is expanded.
    expect(
      vi.mocked(getSchemas).mock.calls.every((call) => !call[1]?.database),
    ).toBe(true);

    fireEvent.click(screen.getByRole("button", { name: /^alpha/ }));
    await screen.findByText("table_in_alpha");
    expect(getSchemas).toHaveBeenCalledWith(
      "c1",
      expect.objectContaining({ database: "alpha" }),
    );
    expect(screen.queryByText("table_in_beta")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /^beta/ }));
    await screen.findByText("table_in_beta");
    expect(screen.getByText("table_in_alpha")).toBeInTheDocument();
  });

  it("localizes an inaccessible database error without breaking siblings", async () => {
    vi.mocked(listConnections).mockResolvedValue([serverConnection()]);
    vi.mocked(listDatabases).mockResolvedValue([{ name: "denied" }, { name: "ok" }] as never);
    vi.mocked(getSchemas).mockImplementation((_id, options) => {
      if (options?.database === "denied") {
        return Promise.reject(
          new ApiClientError("no CONNECT permission", "DATABASE_CONNECT_DENIED", 400),
        );
      }
      return Promise.resolve(databaseTree(options?.database ?? "none") as never);
    });

    renderWithProviders(<SchemaExplorer connectionId="c1" />);

    fireEvent.click(await screen.findByRole("button", { name: /^denied/ }));
    expect(await screen.findByRole("alert")).toHaveTextContent("DATABASE_CONNECT_DENIED");

    fireEvent.click(screen.getByRole("button", { name: /^ok/ }));
    expect(await screen.findByText("table_in_ok")).toBeInTheDocument();
  });

  it("shows a loading state per database", async () => {
    vi.mocked(listConnections).mockResolvedValue([serverConnection()]);
    vi.mocked(listDatabases).mockResolvedValue([{ name: "alpha" }] as never);
    vi.mocked(getSchemas).mockImplementation((_id, options) =>
      options?.database ? (new Promise(() => {}) as never) : Promise.resolve([] as never),
    );

    renderWithProviders(<SchemaExplorer connectionId="c1" />);
    fireEvent.click(await screen.findByRole("button", { name: /^alpha/ }));
    expect(await screen.findByText("Loading schemas…")).toBeInTheDocument();
  });

  it("shows database loading and empty states", async () => {
    vi.mocked(listConnections).mockResolvedValue([serverConnection()]);
    vi.mocked(listDatabases).mockReturnValue(new Promise(() => {}) as never);
    const { unmount } = renderWithProviders(<SchemaExplorer connectionId="c1" />);
    expect(await screen.findByText("Loading databases…")).toBeInTheDocument();
    unmount();

    vi.mocked(listDatabases).mockResolvedValue([] as never);
    renderWithProviders(<SchemaExplorer connectionId="c1" />);
    expect(await screen.findByText("No databases found.")).toBeInTheDocument();
  });

  it("shows a connection-level discovery error with retry", async () => {
    vi.mocked(listConnections).mockResolvedValue([serverConnection()]);
    vi.mocked(listDatabases).mockRejectedValue(
      new ApiClientError("no bootstrap database", "BOOTSTRAP_DATABASE_UNAVAILABLE", 502),
    );
    renderWithProviders(<SchemaExplorer connectionId="c1" />);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "BOOTSTRAP_DATABASE_UNAVAILABLE",
    );
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });

  it("bounds rendering of very large database lists", { timeout: 20000 }, async () => {
    vi.mocked(listConnections).mockResolvedValue([serverConnection()]);
    vi.mocked(listDatabases).mockResolvedValue(
      Array.from({ length: 120 }, (_, i) => ({ name: `db_${i}` })) as never,
    );
    renderWithProviders(<SchemaExplorer connectionId="c1" />);

    await screen.findByRole("button", { name: /^db_0/ });
    expect(screen.queryByRole("button", { name: /^db_119/ })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Show 50 more/ }));
    expect(await screen.findByRole("button", { name: /^db_99/ })).toBeInTheDocument();
  });

  it("resets expansion when switching to another connection", async () => {
    vi.mocked(listConnections).mockResolvedValue([
      serverConnection("c1", "Server 1"),
      serverConnection("c2", "Server 2"),
    ]);
    vi.mocked(listDatabases).mockResolvedValue([{ name: "alpha" }] as never);
    vi.mocked(getSchemas).mockImplementation((_id, options) =>
      Promise.resolve(databaseTree(options?.database ?? "none") as never),
    );

    const { rerender } = renderWithProviders(<SchemaExplorer connectionId="c1" />);
    fireEvent.click(await screen.findByRole("button", { name: /^alpha/ }));
    await screen.findByText("table_in_alpha");

    rerender(<SchemaExplorer connectionId="c2" />);
    await waitFor(() =>
      expect(listDatabases).toHaveBeenLastCalledWith("c2", expect.anything()),
    );
    await waitFor(() =>
      expect(screen.queryByText("table_in_alpha")).not.toBeInTheDocument(),
    );
  });

  it("does not call discovery for MySQL connections", async () => {
    vi.mocked(listConnections).mockResolvedValue([
      {
        id: "c1",
        name: "MySQL",
        driver: "mysql",
        database_name: "app",
        host: "127.0.0.1",
        port: 3306,
        username: "u",
        ssl_mode: "disable",
      } as never,
    ]);
    vi.mocked(getSchemas).mockResolvedValue(sampleSchema);

    renderWithProviders(<SchemaExplorer connectionId="c1" />);
    await screen.findByText("app");
    expect(listDatabases).not.toHaveBeenCalled();
  });

  it("binds generated SQL to the expanded database's tab", async () => {
    vi.mocked(listConnections).mockResolvedValue([serverConnection()]);
    vi.mocked(listDatabases).mockResolvedValue([{ name: "alpha" }] as never);
    vi.mocked(getSchemas).mockResolvedValue(databaseTree("alpha") as never);
    useWorkspaceStore.setState({
      tabs: [{ id: "t1", title: "Query 1", sql: "", connectionId: null, database: null, dirty: false }],
      activeTabId: "t1",
    });

    renderWithProviders(<SchemaExplorer connectionId="c1" />);
    fireEvent.click(await screen.findByRole("button", { name: /^alpha/ }));
    fireEvent.click(await screen.findByRole("button", { name: "Select top 100 from table_in_alpha" }));

    const tab = useWorkspaceStore.getState().tabs[0];
    expect(tab.connectionId).toBe("c1");
    expect(tab.database).toBe("alpha");
  });

  it("marks the explorer selection without rebinding existing tabs", async () => {
    vi.mocked(listConnections).mockResolvedValue([serverConnection()]);
    vi.mocked(listDatabases).mockResolvedValue([{ name: "alpha" }, { name: "beta" }] as never);
    vi.mocked(getSchemas).mockImplementation((_id, options) =>
      Promise.resolve(databaseTree(options?.database ?? "none") as never),
    );
    useConnectionStore.setState({
      activeConnectionId: "c1",
      activeDatabaseByConnection: {},
    });
    useWorkspaceStore.setState({
      tabs: [{ id: "t1", title: "Query 1", sql: "SELECT 1;", connectionId: "c1", database: "CCM", dirty: true }],
      activeTabId: "t1",
    });

    renderWithProviders(<SchemaExplorer connectionId="c1" />);
    fireEvent.click(await screen.findByRole("button", { name: /^beta/ }));

    expect(useConnectionStore.getState().activeDatabaseByConnection.c1).toBe("beta");
    // The query tab binding is untouched (selection ≠ binding).
    const tab = useWorkspaceStore.getState().tabs[0];
    expect(tab.database).toBe("CCM");
    expect(tab.sql).toBe("SELECT 1;");
    expect(tab.dirty).toBe(true);
  });

  it("offers a database refresh when the selected database disappears", async () => {
    vi.mocked(listConnections).mockResolvedValue([serverConnection()]);
    vi.mocked(listDatabases).mockResolvedValue([{ name: "gone" }] as never);
    vi.mocked(getSchemas).mockRejectedValue(
      new ApiClientError("the requested database does not exist", "DATABASE_NOT_FOUND", 400),
    );

    renderWithProviders(<SchemaExplorer connectionId="c1" />);
    fireEvent.click(await screen.findByRole("button", { name: /^gone/ }));
    expect(await screen.findByRole("alert")).toHaveTextContent("DATABASE_NOT_FOUND");

    const before = vi.mocked(listDatabases).mock.calls.length;
    fireEvent.click(screen.getByRole("button", { name: "Refresh databases" }));
    await waitFor(() =>
      expect(vi.mocked(listDatabases).mock.calls.length).toBeGreaterThan(before),
    );
  });
});
