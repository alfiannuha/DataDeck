import { fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiClientError } from "@/lib/api-client";
import { createConnection, testConnection } from "@/lib/api/endpoints";
import { renderWithProviders } from "@/test/render";
import { useConnectionStore } from "@/store/useConnectionStore";

import { NewConnectionModal } from "./new-connection-modal";

vi.mock("@/lib/api/endpoints", () => ({
  listConnections: vi.fn(),
  createConnection: vi.fn(),
  testConnection: vi.fn(),
  deleteConnection: vi.fn(),
}));

function setOnline(online: boolean) {
  Object.defineProperty(window.navigator, "onLine", {
    configurable: true,
    value: online,
  });
  fireEvent(window, new Event(online ? "online" : "offline"));
}

afterEach(() => setOnline(true));

beforeEach(() => {
  vi.clearAllMocks();
  useConnectionStore.setState({ activeConnectionId: null });
});

function renderModal() {
  const onOpenChange = vi.fn();
  const onSaved = vi.fn();
  renderWithProviders(
    <NewConnectionModal
      open
      onOpenChange={onOpenChange}
      onSaved={onSaved}
    />,
  );
  return { onOpenChange, onSaved };
}

describe("NewConnectionModal", () => {
  it("validates required fields before calling the API", async () => {
    renderModal();

    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(await screen.findByText("Name is required.")).toBeInTheDocument();
    // PostgreSQL profiles are server-level: database is optional (PRF-01).
    expect(screen.queryByText("Database is required.")).not.toBeInTheDocument();
    expect(screen.getByText("Username is required.")).toBeInTheDocument();
    expect(createConnection).not.toHaveBeenCalled();
  });

  it("reports a successful test", async () => {
    vi.mocked(testConnection).mockResolvedValue({ status: "ok" } as never);
    renderModal();

    fireEvent.change(screen.getByLabelText("Database (optional)"), {
      target: { value: "app" },
    });
    fireEvent.change(screen.getByLabelText("Username"), {
      target: { value: "appuser" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Test connection" }));

    expect(
      await screen.findByText("Connection successful."),
    ).toBeInTheDocument();
    expect(testConnection).toHaveBeenCalledTimes(1);
  });

  it("surfaces a normalized test failure without credentials", async () => {
    vi.mocked(testConnection).mockRejectedValue(
      new ApiClientError(
        "failed to connect with the provided parameters",
        "CONNECTION_ERROR",
        502,
      ),
    );
    renderModal();

    fireEvent.change(screen.getByLabelText("Database (optional)"), {
      target: { value: "app" },
    });
    fireEvent.change(screen.getByLabelText("Username"), {
      target: { value: "appuser" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Test connection" }));

    const failure = await screen.findByText(/CONNECTION_ERROR/);
    expect(failure).toBeInTheDocument();
    expect(failure.textContent).not.toMatch(/password/i);
  });

  it("creates a connection, keeps the password out of client stores, and closes", async () => {
    vi.mocked(createConnection).mockResolvedValue({
      id: "c9",
      name: "New PG",
    } as never);
    const { onOpenChange, onSaved } = renderModal();

    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "New PG" },
    });
    fireEvent.change(screen.getByLabelText("Database (optional)"), {
      target: { value: "app" },
    });
    fireEvent.change(screen.getByLabelText("Username"), {
      target: { value: "appuser" },
    });
    fireEvent.change(screen.getByLabelText("Password"), {
      target: { value: "s3cret" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(createConnection).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "New PG",
          database_name: "app",
          username: "appuser",
          password: "s3cret",
          driver: "postgres",
        }),
      ),
    );

    expect(onSaved).toHaveBeenCalledWith(
      expect.objectContaining({ id: "c9" }),
    );
    expect(onOpenChange).toHaveBeenCalledWith(false);

    // The password must never reach a client store.
    const storeState = JSON.stringify(useConnectionStore.getState());
    expect(storeState).not.toContain("s3cret");
    expect(Object.keys(useConnectionStore.getState())).toEqual([
      "activeConnectionId",
      "setActiveConnection",
    ]);
  });

  it("switches fields for SQLite (no host/port/credentials)", () => {
    renderModal();
    fireEvent.change(screen.getByLabelText("Driver"), {
      target: { value: "sqlite" },
    });

    expect(screen.getByLabelText("Database file path")).toBeInTheDocument();
    expect(screen.queryByLabelText("Host")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Port")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Username")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Password")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("SSL mode")).not.toBeInTheDocument();
  });

  it("uses MySQL default port when the driver changes", () => {
    renderModal();
    fireEvent.change(screen.getByLabelText("Driver"), {
      target: { value: "mysql" },
    });
    expect(screen.getByLabelText("Port")).toHaveValue("3306");
  });

  it("validates a SQLite path and creates without credentials", async () => {
    vi.mocked(createConnection).mockResolvedValue({
      id: "s1",
      name: "Local",
    } as never);
    const { onSaved } = renderModal();

    fireEvent.change(screen.getByLabelText("Driver"), {
      target: { value: "sqlite" },
    });
    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "Local" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(
      await screen.findByText("Database file path is required."),
    ).toBeInTheDocument();
    expect(createConnection).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Database file path"), {
      target: { value: "/tmp/user.db" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(createConnection).toHaveBeenCalledWith(
        expect.objectContaining({
          driver: "sqlite",
          database_name: "/tmp/user.db",
          name: "Local",
        }),
      ),
    );
    const body = vi.mocked(createConnection).mock.calls[0][0] as Record<
      string,
      unknown
    >;
    expect(body).not.toHaveProperty("host");
    expect(body).not.toHaveProperty("password");
    expect(onSaved).toHaveBeenCalled();
  });

  it("tests a SQLite connection through the shared pathway", async () => {
    vi.mocked(testConnection).mockResolvedValue({ status: "ok" } as never);
    renderModal();

    fireEvent.change(screen.getByLabelText("Driver"), {
      target: { value: "sqlite" },
    });
    fireEvent.change(screen.getByLabelText("Database file path"), {
      target: { value: "/tmp/user.db" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Test connection" }));

    await waitFor(() =>
      expect(testConnection).toHaveBeenCalledWith(
        expect.objectContaining({
          driver: "sqlite",
          database_name: "/tmp/user.db",
        }),
      ),
    );
  });

  it("disables test and save while offline", () => {
    setOnline(false);
    renderModal();

    expect(screen.getByRole("button", { name: "Test connection" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    expect(
      screen.getByText(/Backend unavailable — reconnect to test or save/),
    ).toBeInTheDocument();
  });

  it("allows a PostgreSQL profile with no database", async () => {
    vi.mocked(createConnection).mockResolvedValue({ id: "p1", name: "Server" } as never);
    const { onSaved } = renderModal();

    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Server" } });
    fireEvent.change(screen.getByLabelText("Host"), { target: { value: "127.0.0.1" } });
    fireEvent.change(screen.getByLabelText("Username"), { target: { value: "u" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "pw" } });

    // Helper text explains the server-level behaviour.
    expect(
      screen.getByText(/Leave empty to connect to the server/),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(createConnection).toHaveBeenCalledWith(
        expect.objectContaining({ driver: "postgres", database_name: "" }),
      ),
    );
    expect(onSaved).toHaveBeenCalled();
  });

  it("still sends an explicit PostgreSQL database when provided", async () => {
    vi.mocked(createConnection).mockResolvedValue({ id: "p1", name: "CCM" } as never);
    renderModal();

    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "CCM" } });
    fireEvent.change(screen.getByLabelText("Host"), { target: { value: "127.0.0.1" } });
    fireEvent.change(screen.getByLabelText("Database (optional)"), {
      target: { value: "CCM" },
    });
    fireEvent.change(screen.getByLabelText("Username"), { target: { value: "u" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "pw" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(createConnection).toHaveBeenCalledWith(
        expect.objectContaining({ driver: "postgres", database_name: "CCM" }),
      ),
    );
  });

  it("tests a PostgreSQL connection with an empty database", async () => {
    vi.mocked(testConnection).mockResolvedValue({ status: "ok" } as never);
    renderModal();

    fireEvent.change(screen.getByLabelText("Host"), { target: { value: "127.0.0.1" } });
    fireEvent.change(screen.getByLabelText("Username"), { target: { value: "u" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "pw" } });
    fireEvent.click(screen.getByRole("button", { name: "Test connection" }));

    await waitFor(() =>
      expect(testConnection).toHaveBeenCalledWith(
        expect.objectContaining({ driver: "postgres", database_name: "" }),
      ),
    );
    expect(await screen.findByText("Connection successful.")).toBeInTheDocument();
  });

  it("tests a PostgreSQL connection with an explicit database", async () => {
    vi.mocked(testConnection).mockResolvedValue({ status: "ok" } as never);
    renderModal();

    fireEvent.change(screen.getByLabelText("Host"), { target: { value: "127.0.0.1" } });
    fireEvent.change(screen.getByLabelText("Database (optional)"), {
      target: { value: "reporting" },
    });
    fireEvent.change(screen.getByLabelText("Username"), { target: { value: "u" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "pw" } });
    fireEvent.click(screen.getByRole("button", { name: "Test connection" }));

    await waitFor(() =>
      expect(testConnection).toHaveBeenCalledWith(
        expect.objectContaining({ database_name: "reporting" }),
      ),
    );
  });

  it("requires a database for MySQL and labels it without the optional hint", async () => {
    renderModal();
    fireEvent.change(screen.getByLabelText("Driver"), { target: { value: "mysql" } });
    expect(screen.getByLabelText("Database")).toBeInTheDocument();
    expect(screen.queryByLabelText("Database (optional)")).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "MySQL" } });
    fireEvent.change(screen.getByLabelText("Host"), { target: { value: "127.0.0.1" } });
    fireEvent.change(screen.getByLabelText("Username"), { target: { value: "u" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(await screen.findByText("Database is required.")).toBeInTheDocument();
    expect(createConnection).not.toHaveBeenCalled();
  });
});
