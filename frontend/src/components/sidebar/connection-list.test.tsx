import { fireEvent, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { deleteConnection, listConnections } from "@/lib/api/endpoints";
import { renderWithProviders } from "@/test/render";
import { useConnectionStore } from "@/store/useConnectionStore";

import { ConnectionList } from "./connection-list";

vi.mock("@/lib/api/endpoints", () => ({
  listConnections: vi.fn(),
  createConnection: vi.fn(),
  updateConnection: vi.fn(),
  testConnection: vi.fn(),
  deleteConnection: vi.fn(),
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
