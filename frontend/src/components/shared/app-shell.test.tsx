import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { listConnections } from "@/lib/api/endpoints";
import { ApiClientError } from "@/lib/api-client";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";
import { renderWithProviders } from "@/test/render";

import { AppShell } from "./app-shell";

vi.mock("@/lib/api/endpoints", () => ({
  listConnections: vi.fn().mockResolvedValue([]),
  createConnection: vi.fn(),
  testConnection: vi.fn(),
  deleteConnection: vi.fn(),
  getSchemas: vi.fn(),
  executeQuery: vi.fn(),
  getQueryHistory: vi.fn(),
  getHealth: vi.fn(),
}));

// CodeMirror needs layout APIs jsdom lacks; the shell test only exercises layout.
vi.mock("@/components/editor/sql-editor", () => ({
  SqlEditor: () => <div data-testid="sql-editor" />,
}));

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
  useWorkspaceStore.setState({
    sidebarCollapsed: false,
    tabs: [{ id: "t1", title: "Query 1", sql: "", connectionId: null, dirty: false }],
    activeTabId: "t1",
  });
});

describe("AppShell", () => {
  it("renders every workspace region", () => {
    renderWithProviders(<AppShell />);

    expect(screen.getByRole("banner")).toBeInTheDocument();
    expect(screen.getByRole("complementary")).toBeInTheDocument();
    expect(
      screen.getByRole("region", { name: "Query tabs" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("region", { name: "SQL editor" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("region", { name: "Query results" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("contentinfo")).toBeInTheDocument();
  });

  it("collapses and restores the sidebar from an accessible toggle", () => {
    renderWithProviders(<AppShell />);

    const toggle = screen.getByRole("button", {
      name: "Toggle schema explorer",
    });
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(toggle).toHaveAttribute("aria-controls", "schema-explorer");
    expect(screen.getByRole("complementary")).toBeInTheDocument();

    fireEvent.click(toggle);

    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("complementary")).not.toBeInTheDocument();

    fireEvent.click(toggle);

    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("complementary")).toBeInTheDocument();
  });

  it("exposes the toggle as a real button for keyboard users", () => {
    renderWithProviders(<AppShell />);

    const toggle = screen.getByRole("button", {
      name: "Toggle schema explorer",
    });
    expect(toggle.tagName).toBe("BUTTON");
    expect(toggle).toHaveAttribute("type", "button");
  });

  it("shows the offline shell notice and refetches on reconnect", async () => {
    renderWithProviders(<AppShell />);
    await waitFor(() => expect(listConnections).toHaveBeenCalledTimes(1));

    setOnline(false);
    expect(
      screen.getByText(/Backend unavailable — you are offline/),
    ).toBeInTheDocument();

    setOnline(true);
    await waitFor(() => expect(listConnections).toHaveBeenCalledTimes(2));
    expect(
      screen.queryByText(/Backend unavailable — you are offline/),
    ).not.toBeInTheDocument();
  });

  it("stays usable when the backend is unavailable and recovers on retry", async () => {
    vi.mocked(listConnections).mockRejectedValueOnce(
      new ApiClientError("failed to connect", "CONNECTION_ERROR", 0),
    );
    renderWithProviders(<AppShell />);

    // The shell renders and the failure is surfaced without crashing.
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "CONNECTION_ERROR",
    );
    expect(
      screen.getByRole("region", { name: "SQL editor" }),
    ).toBeInTheDocument();

    // Retrying refetches server state and clears the error.
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() =>
      expect(screen.queryByRole("alert")).not.toBeInTheDocument(),
    );
    await waitFor(() => expect(listConnections).toHaveBeenCalledTimes(2));
  });
});
