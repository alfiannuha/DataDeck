import { fireEvent, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithProviders } from "@/test/render";
import { usePwaStore } from "@/store/usePwaStore";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

import { TopBar } from "./top-bar";

const mocks = vi.hoisted(() => ({
  install: vi.fn(),
  activateUpdate: vi.fn(),
  installState: { canInstall: false, standalone: false },
}));

vi.mock("@/lib/api/endpoints", () => ({
  listConnections: vi.fn().mockResolvedValue([]),
  getQueryHistory: vi.fn(),
  listSavedQueries: vi.fn(),
}));

vi.mock("@/lib/pwa/service-worker", () => ({
  activateUpdate: mocks.activateUpdate,
}));

vi.mock("@/hooks/use-pwa-install", () => ({
  usePwaInstall: () => ({
    canInstall: mocks.installState.canInstall,
    standalone: mocks.installState.standalone,
    install: mocks.install,
  }),
}));

beforeEach(() => {
  vi.clearAllMocks();
  mocks.installState = { canInstall: false, standalone: false };
  usePwaStore.setState({ updateReady: false });
  useWorkspaceStore.setState({
    tabs: [{ id: "t1", title: "Query 1", sql: "", connectionId: null, dirty: false }],
    activeTabId: "t1",
  });
});

describe("TopBar PWA controls", () => {
  it("hides install and update affordances when unavailable", () => {
    renderWithProviders(<TopBar />);
    expect(
      screen.queryByRole("button", { name: "Install DataDeck" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Update available" }),
    ).not.toBeInTheDocument();
  });

  it("offers install only when the browser supports it", () => {
    mocks.installState = { canInstall: true, standalone: false };
    renderWithProviders(<TopBar />);

    fireEvent.click(screen.getByRole("button", { name: "Install DataDeck" }));
    expect(mocks.install).toHaveBeenCalledTimes(1);
  });

  it("activates an update immediately when no tab is dirty", () => {
    usePwaStore.setState({ updateReady: true });
    renderWithProviders(<TopBar />);

    fireEvent.click(screen.getByRole("button", { name: "Update available" }));

    expect(mocks.activateUpdate).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("protects dirty SQL behind an explicit confirmation", () => {
    usePwaStore.setState({ updateReady: true });
    useWorkspaceStore.setState({
      tabs: [{ id: "t1", title: "Query 1", sql: "SELECT 1", connectionId: null, dirty: true }],
      activeTabId: "t1",
    });
    renderWithProviders(<TopBar />);

    fireEvent.click(screen.getByRole("button", { name: "Update available" }));

    expect(mocks.activateUpdate).not.toHaveBeenCalled();
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(screen.getByText(/unsaved SQL/)).toBeInTheDocument();
  });

  it("can cancel a guarded update and keep the workspace", () => {
    usePwaStore.setState({ updateReady: true });
    useWorkspaceStore.setState({
      tabs: [{ id: "t1", title: "Query 1", sql: "SELECT 1", connectionId: null, dirty: true }],
      activeTabId: "t1",
    });
    renderWithProviders(<TopBar />);

    fireEvent.click(screen.getByRole("button", { name: "Update available" }));
    fireEvent.click(screen.getByRole("button", { name: "Not now" }));

    expect(mocks.activateUpdate).not.toHaveBeenCalled();
    expect(useWorkspaceStore.getState().tabs[0].sql).toBe("SELECT 1");
  });

  it("applies the update after the user confirms despite dirty SQL", () => {
    usePwaStore.setState({ updateReady: true });
    useWorkspaceStore.setState({
      tabs: [{ id: "t1", title: "Query 1", sql: "SELECT 1", connectionId: null, dirty: true }],
      activeTabId: "t1",
    });
    renderWithProviders(<TopBar />);

    fireEvent.click(screen.getByRole("button", { name: "Update available" }));
    fireEvent.click(screen.getByRole("button", { name: "Reload and update" }));

    expect(mocks.activateUpdate).toHaveBeenCalledTimes(1);
  });
});
