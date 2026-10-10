"use client";

import { useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { exportTableData } from "@/lib/api/endpoints";
import { downloadBlob } from "@/lib/export/build-export";
import { sanitizeFilenamePart } from "@/lib/export/filename";
import { mutationErrorMessage } from "@/lib/mutation-errors";
import { isReadOnlyObject } from "@/lib/table-actions";
import { useExportStore, type ExportTarget } from "@/store/useExportStore";
import type { TableExportFormat } from "@/types/api";

const FORMATS: { value: TableExportFormat; label: string; hint: string }[] = [
  { value: "csv", label: "CSV", hint: "Comma-separated values" },
  { value: "xlsx", label: "Excel (XLSX)", hint: "Spreadsheet workbook" },
  { value: "sql", label: "SQL", hint: "INSERT script (base tables only)" },
];

/** A stable fallback filename when the server omits Content-Disposition. */
function fallbackFilename(target: ExportTarget, format: TableExportFormat): string {
  return `${sanitizeFilenamePart(target.table, "export")}.${format}`;
}

/**
 * Table export dialog (PRF-02/T14). The target is the explicit binding captured
 * when the dialog was opened (explorer menu or Table Data tab) — never the
 * active tab or the current Explorer selection. The backend streams the file as
 * an attachment; the client just saves it. SQL scripts are base-table only.
 */
export function ExportDialog() {
  const target = useExportStore((state) => state.target);
  if (!target) return null;
  return <ExportDialogBody target={target} />;
}

function ExportDialogBody({ target }: { target: ExportTarget }) {
  const close = useExportStore((state) => state.close);
  const [format, setFormat] = useState<TableExportFormat>("csv");
  const [includeHeaders, setIncludeHeaders] = useState(true);
  const [transaction, setTransaction] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const submittingRef = useRef(false);

  const sqlBlocked = isReadOnlyObject(target.objectType);
  const qualified = target.schema ? `${target.schema}.${target.table}` : target.table;
  const filterCount = target.filters?.length ?? 0;

  async function submit() {
    if (submittingRef.current) return;
    submittingRef.current = true;
    setSubmitting(true);
    setError(null);
    try {
      const result = await exportTableData(target.connectionId, {
        database: target.database,
        schema: target.schema,
        table: target.table,
        format,
        sort: target.sort ?? null,
        filters: target.filters,
        includeHeaders: format === "csv" ? includeHeaders : undefined,
        transaction: format === "sql" ? transaction : undefined,
      });
      downloadBlob(result.blob, result.filename ?? fallbackFilename(target, format));
      close();
    } catch (cause) {
      setError(mutationErrorMessage(cause, "Export failed."));
      submittingRef.current = false;
      setSubmitting(false);
    }
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        // Never close mid-export: the request must settle before the dialog goes.
        if (!open && !submittingRef.current) close();
      }}
    >
      <DialogContent aria-labelledby="export-title" data-testid="export-dialog">
        <DialogHeader>
          <DialogTitle id="export-title">Export — {qualified}</DialogTitle>
          <DialogDescription>
            {filterCount > 0
              ? `Exports the ${filterCount} active filter${filterCount === 1 ? "" : "s"} applied in this tab.`
              : "Exports the full table."}
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-2">
          <fieldset className="flex flex-col gap-1">
            <legend className="text-xs font-medium text-foreground">Format</legend>
            {FORMATS.map((option) => {
              const disabled = option.value === "sql" && sqlBlocked;
              return (
                <label
                  key={option.value}
                  className={`flex items-center gap-2 text-xs ${
                    disabled ? "text-muted-foreground" : "text-foreground"
                  }`}
                >
                  <input
                    type="radio"
                    name="export-format"
                    value={option.value}
                    checked={format === option.value}
                    disabled={disabled}
                    onChange={() => setFormat(option.value)}
                    data-testid={`export-format-${option.value}`}
                  />
                  <span className="font-medium">{option.label}</span>
                  <span className="text-[10px] text-muted-foreground">
                    {disabled ? "Not available for views" : option.hint}
                  </span>
                </label>
              );
            })}
          </fieldset>

          {format === "csv" ? (
            <label className="flex items-center gap-2 text-xs text-foreground">
              <input
                type="checkbox"
                checked={includeHeaders}
                onChange={(event) => setIncludeHeaders(event.target.checked)}
                data-testid="export-include-headers"
              />
              Include column headers
            </label>
          ) : null}

          {format === "sql" ? (
            <label className="flex items-center gap-2 text-xs text-foreground">
              <input
                type="checkbox"
                checked={transaction}
                onChange={(event) => setTransaction(event.target.checked)}
                data-testid="export-transaction"
              />
              Wrap in a transaction
            </label>
          ) : null}
        </div>

        {error ? (
          <p role="alert" className="text-xs text-destructive" data-testid="export-error">
            {error}
          </p>
        ) : null}

        <DialogFooter>
          <Button variant="ghost" onClick={close} disabled={submitting}>
            Cancel
          </Button>
          <Button onClick={() => void submit()} disabled={submitting} data-testid="export-submit">
            {submitting ? "Exporting…" : "Export"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
