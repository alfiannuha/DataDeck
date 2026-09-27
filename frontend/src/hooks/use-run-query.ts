"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useCallback, useRef } from "react";

import { executeQuery } from "@/lib/api/endpoints";
import { ApiClientError } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";
import { useExecutionStore } from "@/store/useExecutionStore";
import type { ExecutionError } from "@/store/useExecutionStore";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

/** Default server timeout for a query, in seconds. */
const QUERY_TIMEOUT_SECONDS = 30;

function toExecutionError(error: unknown): ExecutionError {
  if (error instanceof ApiClientError) {
    return {
      code: error.code,
      message: error.message,
      position: error.position,
    };
  }
  return { code: "INTERNAL_ERROR", message: "Query execution failed." };
}

function isAbortError(error: unknown): boolean {
  return (
    typeof error === "object" &&
    error !== null &&
    "name" in error &&
    (error as { name?: unknown }).name === "AbortError"
  );
}

/**
 * Single execution pathway used by both the Run button and Cmd/Ctrl+Enter.
 *
 * Only the newest request may write to the shared execution store, and only
 * while its originating tab is still active. A slower older request (or one
 * belonging to a tab/connection the user has since left) is dropped rather than
 * overwriting the current tab's state.
 *
 * `cancel()` aborts the in-flight HTTP request. This is a *frontend* request
 * cancellation; the backend exposes no explicit cancel endpoint, so server-side
 * PostgreSQL cancellation is not separately guaranteed.
 */
export function useRunQuery() {
  const queryClient = useQueryClient();
  const abortRef = useRef<AbortController | null>(null);
  const requestIdRef = useRef(0);

  const run = useCallback(
    async (sqlText: string, connectionId: string | null, tabId?: string) => {
      const trimmed = sqlText.trim();

      if (!connectionId) {
        useExecutionStore.getState().reject({
          code: "NO_ACTIVE_CONNECTION",
          message: "Select a connection before running a query.",
        });
        return;
      }
      if (!trimmed) return;

      abortRef.current?.abort();
      const controller = new AbortController();
      abortRef.current = controller;
      const requestId = ++requestIdRef.current;

      // A request is current only if it is still the newest one and its tab is
      // still active. This prevents stale responses from corrupting the active
      // tab after rapid re-runs, tab switches or connection switches.
      const isCurrent = () =>
        requestIdRef.current === requestId &&
        abortRef.current === controller &&
        (tabId === undefined ||
          useWorkspaceStore.getState().activeTabId === tabId);

      useExecutionStore.getState().start(trimmed);
      try {
        const result = await executeQuery(
          {
            connection_id: connectionId,
            sql: trimmed,
            timeout_seconds: QUERY_TIMEOUT_SECONDS,
          },
          controller.signal,
        );
        if (!isCurrent()) return;
        useExecutionStore.getState().resolve(result);
        void queryClient.invalidateQueries({
          queryKey: queryKeys.queryHistoryRoot,
        });
      } catch (error) {
        if (!isCurrent()) return;
        if (isAbortError(error)) {
          useExecutionStore.getState().canceled();
          return;
        }
        useExecutionStore.getState().reject(toExecutionError(error));
      } finally {
        if (abortRef.current === controller) {
          abortRef.current = null;
        }
      }
    },
    [queryClient],
  );

  const cancel = useCallback(() => {
    abortRef.current?.abort();
  }, []);

  return { run, cancel };
}
