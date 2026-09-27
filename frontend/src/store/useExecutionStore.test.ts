import { describe, expect, it } from "vitest";

import { useExecutionStore } from "./useExecutionStore";

const sampleResult = {
  columns: [],
  rows: [],
  rows_affected: 2,
  execution_time_ms: 12,
  truncated: false,
};

describe("useExecutionStore", () => {
  it("tracks a running execution", () => {
    useExecutionStore.getState().start("SELECT 1");
    const state = useExecutionStore.getState();
    expect(state.status).toBe("running");
    expect(state.sql).toBe("SELECT 1");
    expect(state.result).toBeNull();
    expect(state.error).toBeNull();
  });

  it("stores a successful result", () => {
    useExecutionStore.getState().resolve(sampleResult);
    const state = useExecutionStore.getState();
    expect(state.status).toBe("success");
    expect(state.result).toEqual(sampleResult);
    expect(state.error).toBeNull();
  });

  it("stores a normalized error", () => {
    useExecutionStore
      .getState()
      .reject({ code: "SQL_SYNTAX_ERROR", message: "boom", position: 6 });
    const state = useExecutionStore.getState();
    expect(state.status).toBe("error");
    expect(state.error).toEqual({
      code: "SQL_SYNTAX_ERROR",
      message: "boom",
      position: 6,
    });
    expect(state.result).toBeNull();
  });

  it("dismisses an error without disturbing a result", () => {
    useExecutionStore.getState().resolve(sampleResult);
    useExecutionStore
      .getState()
      .reject({ code: "SQL_ERROR", message: "boom" });
    expect(useExecutionStore.getState().status).toBe("error");

    useExecutionStore.getState().dismissError();
    expect(useExecutionStore.getState().status).toBe("idle");
    expect(useExecutionStore.getState().error).toBeNull();
  });

  it("clears a previous error on the next success", () => {
    useExecutionStore
      .getState()
      .reject({ code: "SQL_ERROR", message: "boom" });
    useExecutionStore.getState().resolve(sampleResult);
    expect(useExecutionStore.getState().status).toBe("success");
    expect(useExecutionStore.getState().error).toBeNull();
  });

  it("resets to idle", () => {
    useExecutionStore.getState().resolve(sampleResult);
    useExecutionStore.getState().reset();
    expect(useExecutionStore.getState().status).toBe("idle");
    expect(useExecutionStore.getState().result).toBeNull();
  });
});
