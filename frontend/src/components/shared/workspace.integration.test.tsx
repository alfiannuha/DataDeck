import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AppShell } from "@/components/shared/app-shell";
import {
  executeQuery,
  getSchemas,
  listConnections,
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
    tabs: [{ id: "t1", title: "Query 1", sql: "", connectionId: null, dirty: false }],
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
});
