import { create } from "zustand";

import type { QueryResult } from "@/types/api";

export interface ExecutionError {
  code: string;
  message: string;
  /** 1-based character position supplied by the backend for SQL errors. */
  position?: number;
}

export type ExecutionStatus =
  | "idle"
  | "running"
  | "success"
  | "error"
  | "canceled";

interface ExecutionState {
  status: ExecutionStatus;
  /** The SQL that was executed (for display/context). */
  sql: string | null;
  result: QueryResult | null;
  error: ExecutionError | null;
  start: (sql: string) => void;
  resolve: (result: QueryResult) => void;
  reject: (error: ExecutionError) => void;
  canceled: () => void;
  /** Dismiss the current error without clearing a successful result. */
  dismissError: () => void;
  reset: () => void;
}

/**
 * Centralized query execution state (client workspace state). Only the latest
 * execution is kept — results are never cached globally.
 */
export const useExecutionStore = create<ExecutionState>((set) => ({
  status: "idle",
  sql: null,
  result: null,
  error: null,
  start: (sql) => set({ status: "running", sql, result: null, error: null }),
  resolve: (result) => set({ status: "success", result, error: null }),
  reject: (error) => set({ status: "error", error, result: null }),
  canceled: () => set({ status: "canceled", result: null, error: null }),
  dismissError: () =>
    set((state) => (state.error ? { status: "idle", error: null } : state)),
  reset: () => set({ status: "idle", sql: null, result: null, error: null }),
}));
