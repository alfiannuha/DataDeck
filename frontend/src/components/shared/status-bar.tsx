"use client";

import { useConnections } from "@/hooks/use-connections";
import { useOnline } from "@/hooks/use-online";
import { API_BASE_URL } from "@/lib/api-client";
import { useConnectionStore } from "@/store/useConnectionStore";
import { useExecutionStore } from "@/store/useExecutionStore";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

/**
 * Bottom status bar. Surfaces connection context plus the latest execution
 * metadata (status, duration, rows, truncation).
 *
 * The execution context shows the ACTIVE TAB's bound connection/database — the
 * authoritative target for SQL — not the explorer's browsing selection (PRF-01).
 */
export function StatusBar() {
  const status = useExecutionStore((state) => state.status);
  const result = useExecutionStore((state) => state.result);
  const error = useExecutionStore((state) => state.error);
  const online = useOnline();

  const activeConnectionId = useConnectionStore(
    (state) => state.activeConnectionId,
  );
  const activeTab = useWorkspaceStore((state) =>
    state.tabs.find((tab) => tab.id === state.activeTabId),
  );
  const { data: connections } = useConnections();

  const tabConnectionId = activeTab?.connectionId ?? activeConnectionId;
  const connectionName =
    connections?.find((candidate) => candidate.id === tabConnectionId)?.name ??
    null;
  const databaseLabel =
    activeTab?.database ?? (connectionName ? "default database" : null);
  const context =
    connectionName && databaseLabel
      ? `${connectionName} / ${databaseLabel}`
      : (connectionName ?? null);

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
      <div className="flex min-w-0 shrink-0 items-center gap-3">
        {context ? (
          <span
            aria-label="Execution context"
            title="Where SQL from the active tab executes"
            className="max-w-[280px] truncate text-subtle-foreground"
          >
            {context}
          </span>
        ) : null}
        <span className="truncate" title={API_BASE_URL || "same origin"}>
          {API_BASE_URL || "same origin"}
        </span>
      </div>
    </footer>
  );
}
