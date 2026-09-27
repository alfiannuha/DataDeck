import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { useExecutionStore } from "@/store/useExecutionStore";

import { StatusBar } from "./status-bar";

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
});

describe("StatusBar", () => {
  it("shows Ready when idle", () => {
    render(<StatusBar />);
    expect(screen.getByText("Ready")).toBeInTheDocument();
  });

  it("shows a running indicator", () => {
    useExecutionStore.getState().start("SELECT 1");
    render(<StatusBar />);
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
    render(<StatusBar />);
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
    render(<StatusBar />);
    expect(screen.getByText("Partial result (50 MB limit)")).toBeInTheDocument();
  });

  it("shows the error code safely", () => {
    useExecutionStore
      .getState()
      .reject({ code: "SQL_SYNTAX_ERROR", message: "syntax error" });
    render(<StatusBar />);
    expect(screen.getByText("Error · SQL_SYNTAX_ERROR")).toBeInTheDocument();
  });

  it("shows cancellation", () => {
    useExecutionStore.getState().canceled();
    render(<StatusBar />);
    expect(screen.getByText("Query canceled")).toBeInTheDocument();
  });

  it("reports the offline backend state", () => {
    setOnline(false);
    render(<StatusBar />);
    expect(
      screen.getByText("Offline — backend unavailable"),
    ).toBeInTheDocument();
  });
});
