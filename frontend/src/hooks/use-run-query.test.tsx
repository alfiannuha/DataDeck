import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiClientError } from "@/lib/api-client";
import { executeQuery } from "@/lib/api/endpoints";
import { useExecutionStore } from "@/store/useExecutionStore";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

import { useRunQuery } from "./use-run-query";

vi.mock("@/lib/api/endpoints", () => ({
  executeQuery: vi.fn(),
}));

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

const result = {
  columns: [{ name: "id", type: "INT8" }],
  rows: [["1"]],
  rows_affected: 1,
  execution_time_ms: 8,
  truncated: false,
};

beforeEach(() => {
  vi.clearAllMocks();
  useExecutionStore.getState().reset();
  useWorkspaceStore.setState({
    tabs: [
      { kind: "query", id: "t1", title: "Query 1", sql: "", connectionId: "c1", database: null, dirty: false },
    ],
    activeTabId: "t1",
  });
});

describe("useRunQuery", () => {
  it("executes against the supplied connection", async () => {
    vi.mocked(executeQuery).mockResolvedValue(result as never);
    const { result: hook } = renderHook(() => useRunQuery(), { wrapper });

    await act(async () => {
      await hook.current.run("SELECT 1", "c1");
    });

    expect(executeQuery).toHaveBeenCalledWith(
      { connection_id: "c1", sql: "SELECT 1", timeout_seconds: 30 },
      expect.anything(),
    );
    expect(useExecutionStore.getState().status).toBe("success");
  });

  it("refuses to run without a connection id", async () => {
    const { result: hook } = renderHook(() => useRunQuery(), { wrapper });

    await act(async () => {
      await hook.current.run("SELECT 1", null);
    });

    expect(executeQuery).not.toHaveBeenCalled();
    expect(useExecutionStore.getState().error?.code).toBe(
      "NO_ACTIVE_CONNECTION",
    );
  });

  it("reports a running status while pending", async () => {
    let release: (value: unknown) => void = () => {};
    vi.mocked(executeQuery).mockImplementation(
      () =>
        new Promise((resolve) => {
          release = resolve;
        }) as never,
    );
    const { result: hook } = renderHook(() => useRunQuery(), { wrapper });

    let pending: Promise<void> = Promise.resolve();
    act(() => {
      pending = hook.current.run("SELECT 1", "c1");
    });
    expect(useExecutionStore.getState().status).toBe("running");

    await act(async () => {
      release(result);
      await pending;
    });
    expect(useExecutionStore.getState().status).toBe("success");
  });

  it("exposes SQL error position", async () => {
    vi.mocked(executeQuery).mockRejectedValue(
      new ApiClientError("syntax error", "SQL_SYNTAX_ERROR", 400, 62),
    );
    const { result: hook } = renderHook(() => useRunQuery(), { wrapper });

    await act(async () => {
      await hook.current.run("SELECT WHERRE", "c1");
    });

    expect(useExecutionStore.getState().error).toEqual({
      code: "SQL_SYNTAX_ERROR",
      message: "syntax error",
      position: 62,
    });
  });

  it("surfaces timeout errors", async () => {
    vi.mocked(executeQuery).mockRejectedValue(
      new ApiClientError("query exceeded the timeout", "QUERY_TIMEOUT", 504),
    );
    const { result: hook } = renderHook(() => useRunQuery(), { wrapper });

    await act(async () => {
      await hook.current.run("SELECT pg_sleep(999)", "c1");
    });

    expect(useExecutionStore.getState().error?.code).toBe("QUERY_TIMEOUT");
  });

  it("keeps rows affected and truncation in the result", async () => {
    vi.mocked(executeQuery).mockResolvedValue({
      ...result,
      rows_affected: 5000,
      truncated: true,
    } as never);
    const { result: hook } = renderHook(() => useRunQuery(), { wrapper });

    await act(async () => {
      await hook.current.run("SELECT * FROM big", "c1");
    });

    const state = useExecutionStore.getState();
    expect(state.result?.rows_affected).toBe(5000);
    expect(state.result?.truncated).toBe(true);
  });

  it("marks the request canceled when aborted", async () => {
    vi.mocked(executeQuery).mockImplementation((_body, signal) => {
      return new Promise((_resolve, reject) => {
        signal?.addEventListener("abort", () => {
          const error = new Error("aborted");
          error.name = "AbortError";
          reject(error);
        });
      }) as never;
    });
    const { result: hook } = renderHook(() => useRunQuery(), { wrapper });

    let pending: Promise<void> = Promise.resolve();
    act(() => {
      pending = hook.current.run("SELECT 1", "c1");
    });
    act(() => {
      hook.current.cancel();
    });

    await act(async () => {
      await pending;
    });

    expect(useExecutionStore.getState().status).toBe("canceled");
  });

  it("drops a stale response whose tab is no longer active", async () => {
    let release: (value: unknown) => void = () => {};
    vi.mocked(executeQuery).mockImplementation(
      () =>
        new Promise((resolve) => {
          release = resolve;
        }) as never,
    );
    useWorkspaceStore.setState({
      tabs: [
        { kind: "query", id: "t1", title: "Query 1", sql: "SELECT 1", connectionId: "c1", database: null, dirty: false },
        { kind: "query", id: "t2", title: "Query 2", sql: "", connectionId: "c1", database: null, dirty: false },
      ],
      activeTabId: "t1",
    });
    const { result: hook } = renderHook(() => useRunQuery(), { wrapper });

    let pending: Promise<void> = Promise.resolve();
    act(() => {
      pending = hook.current.run("SELECT 1", "c1", "t1");
    });

    // The user switches to another tab before the query resolves.
    act(() => {
      useWorkspaceStore.getState().setActiveTab("t2");
      useExecutionStore.getState().reset();
    });

    await act(async () => {
      release(result);
      await pending;
    });

    // The stale response must not populate the now-active tab.
    expect(useExecutionStore.getState().result).toBeNull();
    expect(useExecutionStore.getState().status).toBe("idle");
  });

  it("keeps only the newest run when requests overlap", async () => {
    const first = { ...result, rows: [["first"]] };
    const second = { ...result, rows: [["second"]] };
    let releaseFirst: (value: unknown) => void = () => {};
    vi.mocked(executeQuery)
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            releaseFirst = resolve;
          }) as never,
      )
      .mockResolvedValueOnce(second as never);

    const { result: hook } = renderHook(() => useRunQuery(), { wrapper });

    let pendingFirst: Promise<void> = Promise.resolve();
    act(() => {
      pendingFirst = hook.current.run("SELECT first", "c1");
    });
    // A second run supersedes the first while it is still in flight.
    await act(async () => {
      await hook.current.run("SELECT second", "c1");
    });

    expect(useExecutionStore.getState().result?.rows?.[0]?.[0]).toBe("second");

    // The slow first response arriving later must be ignored.
    await act(async () => {
      releaseFirst(first);
      await pendingFirst;
    });
    expect(useExecutionStore.getState().result?.rows?.[0]?.[0]).toBe("second");
  });

  it("sends the tab's database with the request", async () => {
    vi.mocked(executeQuery).mockResolvedValue(result as never);
    useWorkspaceStore.setState({
      tabs: [
        { kind: "query", id: "t1", title: "Query 1", sql: "SELECT 1", connectionId: "c1", database: "CCM", dirty: false },
      ],
      activeTabId: "t1",
    });
    const { result: hook } = renderHook(() => useRunQuery(), { wrapper });

    await act(async () => {
      await hook.current.run("SELECT 1", "c1", "t1", "CCM");
    });

    expect(executeQuery).toHaveBeenCalledWith(
      expect.objectContaining({ connection_id: "c1", database: "CCM" }),
      expect.anything(),
    );
  });

  it("drops a response when the tab was rebound to another database", async () => {
    let release: (value: unknown) => void = () => {};
    vi.mocked(executeQuery).mockImplementation(
      () =>
        new Promise((resolve) => {
          release = resolve;
        }) as never,
    );
    useWorkspaceStore.setState({
      tabs: [
        { kind: "query", id: "t1", title: "Query 1", sql: "SELECT 1", connectionId: "c1", database: "CCM", dirty: false },
      ],
      activeTabId: "t1",
    });
    const { result: hook } = renderHook(() => useRunQuery(), { wrapper });

    let pending: Promise<void> = Promise.resolve();
    act(() => {
      pending = hook.current.run("SELECT 1", "c1", "t1", "CCM");
    });
    // The user rebinds the same tab to another database before it resolves.
    act(() => {
      useWorkspaceStore.getState().setTabDatabase("t1", "reporting");
    });

    await act(async () => {
      release(result);
      await pending;
    });

    expect(useExecutionStore.getState().result).toBeNull();
  });
});
