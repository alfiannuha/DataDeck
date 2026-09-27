"use client";

import { API_BASE_URL } from "@/lib/api-client";
import { useOnline } from "@/hooks/use-online";
import { useExecutionStore } from "@/store/useExecutionStore";

/**
 * Bottom status bar. Surfaces connection context plus the latest execution
 * metadata (status, duration, rows, truncation).
 */
export function StatusBar() {
  const status = useExecutionStore((state) => state.status);
  const result = useExecutionStore((state) => state.result);
  const error = useExecutionStore((state) => state.error);
  const online = useOnline();

  let summary = "Ready";
  if (!online) {
    summary = "Offline — backend unavailable";
  } else if (status === "running") {
    summary = "Running query…";
  } else if (status === "canceled") {
    summary = "Query canceled";
  } else if (status === "error" && error) {
    summary = `Error · ${error.code}`;
  } else if (status === "success" && result) {
    summary = `Success · ${result.execution_time_ms} ms · ${result.rows_affected} row${
      result.rows_affected === 1 ? "" : "s"
    }`;
  }

  return (
    <footer
      aria-label="Status bar"
      className="flex h-7 shrink-0 items-center justify-between gap-3 border-t border-border bg-panel px-3 text-xs text-muted-foreground"
    >
      <div className="flex min-w-0 items-center gap-3">
        <span role="status" aria-live="polite" className="truncate">
          {summary}
        </span>
        {result?.truncated ? (
          <span
            className="shrink-0 text-amber-400"
            title="Only part of the result is available because of the 50 MB safety limit."
          >
            Partial result (50 MB limit)
          </span>
        ) : null}
      </div>
      <span className="truncate" title={API_BASE_URL || "same origin"}>
        {API_BASE_URL || "same origin"}
      </span>
    </footer>
  );
}
