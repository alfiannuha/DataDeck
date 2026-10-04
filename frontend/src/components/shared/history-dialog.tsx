"use client";

import { Clock, AlertTriangle, CheckCircle2, Copy } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Select } from "@/components/ui/select";
import { useConnections } from "@/hooks/use-connections";
import { useQueryHistory } from "@/hooks/use-query-execution";
import { ApiClientError } from "@/lib/api-client";
import { copyText } from "@/lib/clipboard";
import { useExecutionStore } from "@/store/useExecutionStore";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";
import type { QueryHistoryRecord } from "@/types/api";

/** Single-line SQL preview. */
function sqlPreview(sql: string, max = 140): string {
  const flat = sql.replace(/\s+/g, " ").trim();
  return flat.length > max ? `${flat.slice(0, max)}…` : flat;
}

/**
 * Query history panel. Reads the backend audit log (no local duplication) and
 * opens an entry into a query tab bound to its original connection — it never
 * executes automatically.
 */
export function HistoryDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const [connectionFilter, setConnectionFilter] = useState("");
  const [page, setPage] = useState(1);
  const { data: connections } = useConnections();
  const insertQuerySql = useWorkspaceStore((state) => state.insertQuerySql);

  const history = useQueryHistory(connectionFilter || undefined, page, open);

  function connectionName(connectionId: string): string | null {
    return connections?.find((c) => c.id === connectionId)?.name ?? null;
  }

  function openEntry(entry: QueryHistoryRecord) {
    const sql = entry.sql_text ?? "";
    if (!sql) return;
    const stillExists = connections?.some((c) => c.id === entry.connection_id);
    // Opening must not imply the SQL already ran against the previous result.
    useExecutionStore.getState().reset();
    // Preserve the original connection when it still exists; otherwise leave
    // the tab unbound so the user must choose before executing.
    insertQuerySql(
      sql,
      stillExists ? entry.connection_id ?? null : null,
      stillExists ? entry.database_name ?? null : null,
    );
    onOpenChange(false);
  }

  const records = history.data?.items ?? [];
  const meta = history.data?.meta;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        aria-labelledby="history-title"
        className="max-w-2xl"
      >
        <DialogHeader>
          <DialogTitle id="history-title">Query history</DialogTitle>
          <DialogDescription>
            Recent executions from the backend audit log. Opening an entry never
            runs it automatically.
          </DialogDescription>
        </DialogHeader>

        <div className="mb-2 flex items-center justify-between gap-2">
          <label className="text-xs text-muted-foreground" htmlFor="history-filter">
            Filter
          </label>
          <Select
            id="history-filter"
            className="max-w-56"
            value={connectionFilter}
            onChange={(event) => {
              setConnectionFilter(event.target.value);
              setPage(1);
            }}
          >
            <option value="">All connections</option>
            {connections?.map((connection) => (
              <option key={connection.id} value={connection.id}>
                {connection.name}
              </option>
            ))}
          </Select>
        </div>

        {history.isLoading ? (
          <p className="py-6 text-center text-xs text-muted-foreground">
            Loading history…
          </p>
        ) : history.isError ? (
          <p role="alert" className="py-6 text-center text-xs text-destructive">
            {history.error instanceof ApiClientError
              ? `${history.error.message} (${history.error.code})`
              : "Could not load history."}
          </p>
        ) : records.length === 0 ? (
          <p className="py-6 text-center text-xs text-muted-foreground">
            No queries have been executed yet.
          </p>
        ) : (
          <ul
            aria-label="Query history entries"
            className="flex max-h-96 flex-col gap-1 overflow-auto pr-1"
          >
            {records.map((entry) => {
              const name = connectionName(entry.connection_id ?? "");
              const missing = !name;
              return (
                <li
                  key={entry.id}
                  className="rounded-md border border-border bg-panel p-2"
                >
                  <div className="flex items-center gap-2 text-[10px] text-muted-foreground">
                    {entry.status === "SUCCESS" ? (
                      <CheckCircle2
                        size={11}
                        aria-hidden="true"
                        className="shrink-0 text-emerald-400"
                      />
                    ) : (
                      <AlertTriangle
                        size={11}
                        aria-hidden="true"
                        className="shrink-0 text-destructive"
                      />
                    )}
                    <span
                      className={
                        entry.status === "SUCCESS"
                          ? "text-emerald-400"
                          : "text-destructive"
                      }
                    >
                      {entry.status}
                    </span>
                    <span className="flex items-center gap-1">
                      <Clock size={10} aria-hidden="true" />
                      {entry.executed_at
                        ? new Date(entry.executed_at).toLocaleString()
                        : "—"}
                    </span>
                    <span>{entry.execution_time_ms ?? 0} ms</span>
                    {(entry.rows_affected ?? 0) > 0 && (
                      <span>{entry.rows_affected} rows</span>
                    )}
                    <span className="ml-auto truncate">
                      {name ?? "connection removed"}
                    </span>
                  </div>

                  <p
                    className="mt-1 truncate font-mono text-xs text-foreground"
                    title={entry.sql_text ?? ""}
                  >
                    {sqlPreview(entry.sql_text ?? "")}
                  </p>

                  <div className="mt-1 flex items-center gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => openEntry(entry)}
                    >
                      Open in editor
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label="Copy SQL"
                      onClick={() => void copyText(entry.sql_text ?? "")}
                    >
                      <Copy size={12} aria-hidden="true" />
                    </Button>
                    {missing && (
                      <span className="text-[10px] text-destructive">
                        Original connection no longer exists — choose a
                        connection before running.
                      </span>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
        )}

        {meta && meta.total_pages > 1 && (
          <div className="mt-2 flex items-center justify-center gap-3 text-xs text-muted-foreground">
            <Button
              variant="outline"
              size="sm"
              disabled={page <= 1}
              onClick={() => setPage((current) => Math.max(1, current - 1))}
            >
              Previous
            </Button>
            <span>
              Page {meta.page} of {meta.total_pages}
            </span>
            <Button
              variant="outline"
              size="sm"
              disabled={page >= meta.total_pages}
              onClick={() => setPage((current) => current + 1)}
            >
              Next
            </Button>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
