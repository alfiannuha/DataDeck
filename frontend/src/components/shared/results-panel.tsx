"use client";

import { Download } from "lucide-react";
import { useRef, useState } from "react";

import { ResultGrid } from "@/components/grid/result-grid";
import { PanelPlaceholder } from "@/components/shared/panel-placeholder";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { useConnections } from "@/hooks/use-connections";
import { buildExport, downloadExport, type ExportFormat } from "@/lib/export/build-export";
import { useConnectionStore } from "@/store/useConnectionStore";
import { useExecutionStore } from "@/store/useExecutionStore";

/**
 * Result region. Renders the virtualized grid for the latest execution result
 * and offers a CSV export of that result (never re-running SQL). A truncated
 * result warns before exporting.
 */
export function ResultsPanel() {
  const result = useExecutionStore((state) => state.result);
  const status = useExecutionStore((state) => state.status);
  const activeConnectionId = useConnectionStore(
    (state) => state.activeConnectionId,
  );
  const { data: connections } = useConnections();

  const [feedback, setFeedback] = useState<string | null>(null);
  const [pendingFormat, setPendingFormat] = useState<ExportFormat | null>(null);
  const feedbackTimer = useRef<number | null>(null);

  function doExport(format: ExportFormat) {
    if (!result) return;
    try {
      const connectionName =
        connections?.find((c) => c.id === activeConnectionId)?.name ?? null;
      const artifact = buildExport(result, { format, connectionName });
      downloadExport(artifact);
      setFeedback(`Exported ${artifact.rowCount} rows`);
    } catch {
      setFeedback("Export failed");
    }
    if (feedbackTimer.current) window.clearTimeout(feedbackTimer.current);
    feedbackTimer.current = window.setTimeout(() => setFeedback(null), 3000);
  }

  function requestExport(format: ExportFormat) {
    if (!result) return;
    if (result.truncated) {
      setPendingFormat(format);
      return;
    }
    doExport(format);
  }

  return (
    <section
      aria-label="Query results"
      className="flex min-h-0 flex-1 flex-col overflow-hidden border-t border-border"
    >
      <div className="flex h-8 shrink-0 items-center justify-between border-b border-border bg-panel px-3">
        <span className="text-xs font-medium uppercase tracking-wide text-subtle-foreground">
          Results
        </span>
        <div className="flex items-center gap-2">
          {result ? (
            <span className="text-[10px] text-muted-foreground">
              {result.rows?.length ?? 0} rows
              {result.truncated ? " · partial" : ""}
            </span>
          ) : null}
          {feedback && (
            <span
              role="status"
              className="text-[10px] text-emerald-400"
            >
              {feedback}
            </span>
          )}
          <Button
            variant="ghost"
            size="sm"
            aria-label="Export CSV"
            title="Export the current result as CSV"
            disabled={!result}
            onClick={() => requestExport("csv")}
          >
            <Download size={12} aria-hidden="true" />
            Export CSV
          </Button>
          <Button
            variant="ghost"
            size="sm"
            aria-label="Export JSON"
            title="Export the current result as JSON"
            disabled={!result}
            onClick={() => requestExport("json")}
          >
            <Download size={12} aria-hidden="true" />
            Export JSON
          </Button>
        </div>
      </div>
      <div className="min-h-0 flex-1 bg-result-surface">
        {result ? (
          <ResultGrid result={result} />
        ) : (
          <PanelPlaceholder
            title="Results"
            description={
              status === "error"
                ? "Fix the error above and run again."
                : "Execute a query to see a virtualized result grid."
            }
          />
        )}
      </div>

      <AlertDialog
        open={pendingFormat !== null}
        onOpenChange={(next) => {
          if (!next) setPendingFormat(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Export partial result?</AlertDialogTitle>
            <AlertDialogDescription>
              This result was truncated at the 50 MB safety limit, so the{" "}
              {pendingFormat === "json" ? "JSON" : "CSV"} will contain only the{" "}
              {result?.rows?.length ?? 0} returned rows — not the complete query
              result.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                const format = pendingFormat;
                setPendingFormat(null);
                if (format) doExport(format);
              }}
            >
              Export anyway
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </section>
  );
}
