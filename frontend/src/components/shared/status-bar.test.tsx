import { fireEvent, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithProviders } from "@/test/render";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";
import { useExecutionStore } from "@/store/useExecutionStore";

import { StatusBar } from "./status-bar";

vi.mock("@/lib/api/endpoints", () => ({
  listConnections: vi.fn().mockResolvedValue([
    { id: "c1", name: "CCM DEV", driver: "postgres", database_name: "" },
  ]),
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
  useExecutionStore.getState().reset();
  useWorkspaceStore.setState({
    tabs: [{ kind: "query", id: "t1", title: "Query 1", sql: "", connectionId: "c1", database: "CCM", dirty: false }],
    activeTabId: "t1",
  });
});

describe("StatusBar", () => {
  it("shows Ready when idle", () => {
    renderWithProviders(<StatusBar />);
    expect(screen.getByText("Ready")).toBeInTheDocument();
  });

  it("shows a running indicator", () => {
    useExecutionStore.getState().start("SELECT 1");
    renderWithProviders(<StatusBar />);
    expect(screen.getByText("Running query…")).toBeInTheDocument();
  });

  it("shows duration and rows on success", () => {
    useExecutionStore.getState().resolve({
      columns: [],
      rows: [],
      rows_affected: 3,
      execution_time_ms: 17,
      truncated: false,
    });
    renderWithProviders(<StatusBar />);
    expect(screen.getByText("Success · 17 ms · 3 rows")).toBeInTheDocument();
  });

  it("communicates truncation without alarm", () => {
    useExecutionStore.getState().resolve({
      columns: [],
      rows: [],
      rows_affected: 1000,
      execution_time_ms: 42,
      truncated: true,
    });
    renderWithProviders(<StatusBar />);
    expect(screen.getByText("Partial result (50 MB limit)")).toBeInTheDocument();
  });

  it("shows the error code safely", () => {
    useExecutionStore
      .getState()
      .reject({ code: "SQL_SYNTAX_ERROR", message: "syntax error" });
    renderWithProviders(<StatusBar />);
    expect(screen.getByText("Error · SQL_SYNTAX_ERROR")).toBeInTheDocument();
  });

  it("shows cancellation", () => {
    useExecutionStore.getState().canceled();
    renderWithProviders(<StatusBar />);
    expect(screen.getByText("Query canceled")).toBeInTheDocument();
  });

  it("reports the offline backend state", () => {
    setOnline(false);
    renderWithProviders(<StatusBar />);
    expect(
      screen.getByText("Offline — backend unavailable"),
    ).toBeInTheDocument();
  });

  it("shows the active tab's execution context (connection / database)", async () => {
    renderWithProviders(<StatusBar />);
    expect(
      await screen.findByLabelText("Execution context"),
    ).toHaveTextContent("CCM DEV / CCM");
  });
});
