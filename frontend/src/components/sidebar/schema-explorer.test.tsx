import { fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiClientError } from "@/lib/api-client";
import { executeQuery, getSchemas, listConnections } from "@/lib/api/endpoints";
import { renderWithProviders } from "@/test/render";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";
import type { DatabaseSchemaTree } from "@/types/api";

import { SchemaExplorer } from "./schema-explorer";

vi.mock("@/lib/api/endpoints", () => ({
  listConnections: vi.fn(),
  createConnection: vi.fn(),
  testConnection: vi.fn(),
  deleteConnection: vi.fn(),
  getSchemas: vi.fn(),
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
