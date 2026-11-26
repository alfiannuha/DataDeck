import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  createConnectionGroup,
  deleteConnection,
  listConnectionGroups,
  listConnections,
  reorderConnections,
  setConnectionGroup,
  updateConnectionGroup,
} from "@/lib/api/endpoints";
import { ApiClientError } from "@/lib/api-client";
import { renderWithProviders } from "@/test/render";
import { useConnectionStore } from "@/store/useConnectionStore";

import { ConnectionList } from "./connection-list";

vi.mock("@/lib/api/endpoints", () => ({
  listConnections: vi.fn(),
  createConnection: vi.fn(),
  updateConnection: vi.fn(),
  testConnection: vi.fn(),
  deleteConnection: vi.fn(),
  reorderConnections: vi.fn(),
  listConnectionGroups: vi.fn().mockResolvedValue([]),
  createConnectionGroup: vi.fn(),
  updateConnectionGroup: vi.fn(),
  deleteConnectionGroup: vi.fn(),
  reorderConnectionGroups: vi.fn(),
  setConnectionGroup: vi.fn(),
}));

const savedConnection = {
  id: "c1",
  name: "Alpha",
  driver: "postgres",
  host: "db.internal",
  port: 5432,
  database_name: "app",
  username: "appuser",
  ssl_mode: "disable",
};

beforeEach(() => {
  vi.clearAllMocks();
  useConnectionStore.setState({ activeConnectionId: null });
});

describe("ConnectionList", () => {
  it("renders saved connections without any sensitive fields", async () => {
    vi.mocked(listConnections).mockResolvedValue([savedConnection as never]);
    renderWithProviders(<ConnectionList />);

    expect(await screen.findByText("Alpha")).toBeInTheDocument();
    expect(screen.getByText(/db\.internal/)).toBeInTheDocument();

    const html = document.body.innerHTML;
    expect(html).not.toMatch(/password/i);
    expect(html).not.toMatch(/encrypted/i);
    expect(html).not.toContain("ssl_mode");
  });

  it("shows an empty state", async () => {
    vi.mocked(listConnections).mockResolvedValue([]);
    renderWithProviders(<ConnectionList />);

    expect(
      await screen.findByText(/No connections yet/i),
    ).toBeInTheDocument();
  });

  it("sets the active connection when selected", async () => {
    vi.mocked(listConnections).mockResolvedValue([savedConnection as never]);
    renderWithProviders(<ConnectionList />);

    fireEvent.click(await screen.findByRole("button", { name: /^Alpha/ }));

    expect(useConnectionStore.getState().activeConnectionId).toBe("c1");
  });

  it("deletes after confirmation and clears the active selection", async () => {
    vi.mocked(listConnections).mockResolvedValue([savedConnection as never]);
    vi.mocked(deleteConnection).mockResolvedValue({ id: "c1" } as never);
    useConnectionStore.setState({ activeConnectionId: "c1" });

    renderWithProviders(<ConnectionList />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Delete Alpha" }),
    );
    expect(await screen.findByRole("alertdialog")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() =>
      expect(deleteConnection).toHaveBeenCalledWith("c1"),
    );
    await waitFor(() =>
      expect(useConnectionStore.getState().activeConnectionId).toBeNull(),
    );
  });

  it("does not delete when the confirmation is cancelled", async () => {
    vi.mocked(listConnections).mockResolvedValue([savedConnection as never]);
    renderWithProviders(<ConnectionList />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Delete Alpha" }),
    );
    expect(await screen.findByRole("alertdialog")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    await waitFor(() =>
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument(),
    );
    expect(deleteConnection).not.toHaveBeenCalled();
  });

  it("shows the file path for SQLite connections without credentials", async () => {
    vi.mocked(listConnections).mockResolvedValue([
      { id: "s1", name: "Local", driver: "sqlite", database_name: "/tmp/user.db" },
    ] as never);
    renderWithProviders(<ConnectionList />);

    expect(await screen.findByText("Local")).toBeInTheDocument();
    expect(screen.getByText(/sqlite · \/tmp\/user\.db/)).toBeInTheDocument();
    expect(document.body.innerHTML).not.toMatch(/password/i);
  });

  it("renders a server-level PostgreSQL profile without a trailing slash", async () => {
    vi.mocked(listConnections).mockResolvedValue([
      { ...savedConnection, id: "srv", name: "Server", database_name: "" },
    ] as never);
    renderWithProviders(<ConnectionList />);

    expect(await screen.findByText("Server")).toBeInTheDocument();
    expect(screen.getByText("postgres · db.internal:5432")).toBeInTheDocument();
    expect(screen.queryByText(/db\.internal:5432\//)).not.toBeInTheDocument();
  });

  it("still renders a legacy PostgreSQL profile with its database", async () => {
    vi.mocked(listConnections).mockResolvedValue([savedConnection as never]);
    renderWithProviders(<ConnectionList />);

    expect(await screen.findByText("Alpha")).toBeInTheDocument();
    expect(screen.getByText("postgres · db.internal:5432/app")).toBeInTheDocument();
  });

  it("shows the stored connection color, defaulting when absent", async () => {
    vi.mocked(listConnections).mockResolvedValue([
      { ...savedConnection, id: "c1", name: "Alpha", color: "#22c55e" },
      { ...savedConnection, id: "c2", name: "Beta", color: "" },
    ] as never);
    renderWithProviders(<ConnectionList />);

    await screen.findByText("Alpha");
    const dots = screen.getAllByTestId("connection-color");
    expect(dots[0]).toHaveStyle({ backgroundColor: "#22c55e" });
    // PRF-03/A: a legacy record without a color falls back to the default.
    expect(dots[1]).toHaveStyle({ backgroundColor: "#3b82f6" });
  });

  it("opens the edit modal prefilled for a connection", async () => {
    vi.mocked(listConnections).mockResolvedValue([savedConnection as never]);
    renderWithProviders(<ConnectionList />);

    fireEvent.click(await screen.findByRole("button", { name: "Edit Alpha" }));

    expect(
      await screen.findByRole("heading", { name: "Edit connection" }),
    ).toBeInTheDocument();
    expect(screen.getByDisplayValue("Alpha")).toBeInTheDocument();
  });
});

const reorderData = [
  { ...savedConnection, id: "c1", name: "Alpha" },
  { ...savedConnection, id: "c2", name: "Beta" },
  { ...savedConnection, id: "c3", name: "Gamma" },
];

function displayedOrder(): string[] {
  const list = screen.getByRole("list", { name: "Saved connections" });
  return within(list)
    .getAllByRole("listitem")
    .map((item) => {
      const text = item.textContent ?? "";
      return text.includes("Alpha") ? "Alpha" : text.includes("Beta") ? "Beta" : "Gamma";
    });
}

async function openReorderMenu(name: string) {
  fireEvent.keyDown(screen.getByRole("button", { name: `Reorder ${name}` }), {
    key: "ArrowDown",
  });
  await screen.findByRole("menu", { name: `Reorder ${name}` });
}

describe("ConnectionList manual ordering", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useConnectionStore.setState({ activeConnectionId: null });
    vi.mocked(listConnections).mockResolvedValue(reorderData as never);
  });

  it("moves a connection down via the menu and persists the full order", async () => {
    vi.mocked(reorderConnections).mockResolvedValue([] as never);
    renderWithProviders(<ConnectionList />);
    await screen.findByText("Alpha");

    await openReorderMenu("Alpha");
    fireEvent.click(await screen.findByTestId("conn-move-down-c1"));

    await waitFor(() =>
      expect(reorderConnections).toHaveBeenCalledWith(["c2", "c1", "c3"]),
    );
  });

  it("moves a connection to the bottom and disables irrelevant actions", async () => {
    vi.mocked(reorderConnections).mockResolvedValue([] as never);
    renderWithProviders(<ConnectionList />);
    await screen.findByText("Alpha");

    await openReorderMenu("Alpha");
    expect(screen.getByTestId("conn-move-up-c1")).toHaveAttribute(
      "aria-disabled",
      "true",
    );
    fireEvent.click(await screen.findByTestId("conn-move-bottom-c1"));
    await waitFor(() =>
      expect(reorderConnections).toHaveBeenCalledWith(["c2", "c3", "c1"]),
    );
  });

  it("applies the order optimistically and rolls back on failure", async () => {
    let rejectOrder: ((error: unknown) => void) | undefined;
    vi.mocked(reorderConnections).mockImplementation(
      () =>
        new Promise((_resolve, reject) => {
          rejectOrder = reject;
        }) as never,
    );
    renderWithProviders(<ConnectionList />);
    await screen.findByText("Alpha");

    await openReorderMenu("Alpha");
    fireEvent.click(await screen.findByTestId("conn-move-down-c1"));
    await waitFor(() =>
      expect(displayedOrder()).toEqual(["Beta", "Alpha", "Gamma"]),
    );

    rejectOrder?.(new ApiClientError("nope", "VALIDATION_ERROR", 400));
    await waitFor(() =>
      expect(displayedOrder()).toEqual(["Alpha", "Beta", "Gamma"]),
    );
  });

  it("reorders via drag and drop (handle → drop before the first row)", async () => {
    vi.mocked(reorderConnections).mockResolvedValue([] as never);
    renderWithProviders(<ConnectionList />);
    await screen.findByText("Alpha");

    const items = within(
      screen.getByRole("list", { name: "Saved connections" }),
    ).getAllByRole("listitem");
    const rows = items.map((item) => item.querySelector("div") as HTMLElement);
    const handle = items[2].querySelector("[data-drag-handle]") as HTMLElement;
    const dataTransfer = {
      effectAllowed: "",
      dropEffect: "",
      setData: vi.fn(),
      getData: () => "c3",
    };

    fireEvent.dragStart(handle, { dataTransfer });
    expect(dataTransfer.setData).toHaveBeenCalledWith("text/plain", "c3");
    fireEvent.dragOver(rows[0], { dataTransfer, clientY: 0 });
    fireEvent.drop(rows[0], { dataTransfer, clientY: 0 });

    await waitFor(() => expect(reorderConnections).toHaveBeenCalled());
    expect(vi.mocked(reorderConnections).mock.calls[0]?.[0]).toEqual([
      "c3",
      "c1",
      "c2",
    ]);
  });
});

describe("ConnectionList grouping", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useConnectionStore.setState({ activeConnectionId: null });
    vi.mocked(listConnections).mockResolvedValue(reorderData as never);
    vi.mocked(listConnectionGroups).mockResolvedValue([] as never);
  });

  it("hides and restores all connections via the master accordion", async () => {
    renderWithProviders(<ConnectionList />);
    expect(await screen.findByText("Alpha")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("connections-toggle"));
    expect(screen.queryByText("Alpha")).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("connections-toggle"));
    expect(await screen.findByText("Alpha")).toBeInTheDocument();
  });

  it("lists groups collapsed by default and hides members until expanded", async () => {
    vi.mocked(listConnectionGroups).mockResolvedValue([
      { id: "g1", name: "Dev", sort_order: 0, is_expanded: false },
    ] as never);
    vi.mocked(listConnections).mockResolvedValue([
      { ...savedConnection, id: "c1", name: "Alpha", group_id: "g1" },
    ] as never);
    renderWithProviders(<ConnectionList />);

    const toggle = await screen.findByTestId("group-toggle-g1");
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("Alpha")).not.toBeInTheDocument();

    vi.mocked(updateConnectionGroup).mockResolvedValue({} as never);
    fireEvent.click(toggle);
    await waitFor(() =>
      expect(updateConnectionGroup).toHaveBeenCalledWith("g1", { is_expanded: true }),
    );
  });

  it("creates a group from the header", async () => {
    vi.mocked(createConnectionGroup).mockResolvedValue({ id: "g9", name: "Dev" } as never);
    renderWithProviders(<ConnectionList />);
    await screen.findByText("Alpha");

    fireEvent.click(screen.getByRole("button", { name: "New group" }));
    fireEvent.change(await screen.findByLabelText("Group name"), {
      target: { value: "Dev" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() =>
      expect(createConnectionGroup).toHaveBeenCalledWith({ name: "Dev" }),
    );
  });

  it("moves a connection into a group via the menu", async () => {
    vi.mocked(listConnectionGroups).mockResolvedValue([
      { id: "g1", name: "Dev", sort_order: 0, is_expanded: false },
    ] as never);
    vi.mocked(setConnectionGroup).mockResolvedValue({} as never);
    renderWithProviders(<ConnectionList />);
    await screen.findByText("Alpha");

    fireEvent.keyDown(screen.getByRole("button", { name: "Reorder Alpha" }), {
      key: "ArrowDown",
    });
    fireEvent.click(await screen.findByTestId("conn-move-group-c1-g1"));

    await waitFor(() =>
      expect(setConnectionGroup).toHaveBeenCalledWith("c1", "g1"),
    );
  });
});
