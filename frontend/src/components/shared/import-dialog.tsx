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
import { Input } from "@/components/ui/input";
import {
  executeTableImport,
  type ImportTargetBody,
  uploadTableImport,
  validateTableImport,
} from "@/lib/api/endpoints";
import { mutationErrorMessage } from "@/lib/mutation-errors";
import { isReadOnlyObject } from "@/lib/table-actions";
import { useImportStore, type ImportTarget } from "@/store/useImportStore";
import type {
  ImportCell,
  ImportMappingResolved,
  TableImportFormat,
  TableImportSession,
  TableImportValidation,
} from "@/types/api";

const FORMATS: { value: TableImportFormat; label: string; hint: string }[] = [
  { value: "csv", label: "CSV", hint: "RFC 4180, first row may be a header" },
  { value: "sql", label: "SQL", hint: "INSERT script (base tables only)" },
];

/** Render one cell for the preview grid. */
function cellText(cell: ImportCell): string {
  if (cell.is_null) return "NULL";
  return cell.value ?? "";
}

function cellStyle(cell: ImportCell): string {
  return cell.is_null ? "italic text-muted-foreground" : "text-foreground";
}

/**
 * Table import dialog (PRF-02/T15). The target is the explicit binding captured
 * when the dialog was opened — never the active tab or Explorer selection. Flow:
 * upload a CSV/SQL file (frozen session + preview) → review the resolved mapping
 * and validation errors → execute inside one all-or-nothing transaction.
 */
export function ImportDialog() {
  const target = useImportStore((state) => state.target);
  if (!target) return null;
  return <ImportDialogBody target={target} />;
}

function ImportDialogBody({ target }: { target: ImportTarget }) {
  const close = useImportStore((state) => state.close);
  const [file, setFile] = useState<File | null>(null);
  const [format, setFormat] = useState<TableImportFormat>("csv");
  const [csvHeader, setCsvHeader] = useState(true);
  const [stage, setStage] = useState<"upload" | "review" | "done">("upload");
  const [session, setSession] = useState<TableImportSession | null>(null);
  const [validation, setValidation] = useState<TableImportValidation | null>(null);
  const [inserted, setInserted] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const busyRef = useRef(false);

  const body: ImportTargetBody = {
    database: target.database,
    schema: target.schema,
    table: target.table,
  };
  const readOnly = isReadOnlyObject(target.objectType);
  const qualified = target.schema
    ? `${target.schema}.${target.table}`
    : target.table;
  const resetError = () => setError(null);

  async function upload() {
    if (busyRef.current) return;
    busyRef.current = true;
    setBusy(true);
    resetError();
    try {
      const s = await uploadTableImport(target.connectionId, {
        database: target.database,
        schema: target.schema,
        table: target.table,
        format,
        csvHeader,
        file: file as File,
      });
      setSession(s);
      const v = await validateTableImport(target.connectionId, s.import_id ?? "", body);
      setValidation(v);
      setStage("review");
    } catch (cause) {
      setError(mutationErrorMessage(cause, "Upload failed."));
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  }

  async function execute() {
    if (busyRef.current || !session) return;
    busyRef.current = true;
    setBusy(true);
    resetError();
    try {
      const result = await executeTableImport(
        target.connectionId,
        session.import_id ?? "",
        body,
      );
      setInserted(result.inserted ?? 0);
      setStage("done");
    } catch (cause) {
      setError(mutationErrorMessage(cause, "Import failed."));
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        // Never close mid-request: the import must settle before the dialog goes.
        if (!open && !busyRef.current) close();
      }}
    >
      <DialogContent aria-labelledby="import-title" data-testid="import-dialog">
        <DialogHeader>
          <DialogTitle id="import-title">Import — {qualified}</DialogTitle>
          <DialogDescription>
            {readOnly
              ? "This object is read-only; no data can be imported into it."
              : stage === "done"
                ? "The import was applied atomically."
                : "Upload a CSV or SQL INSERT script, review the mapping, then import."}
          </DialogDescription>
        </DialogHeader>

        {stage === "upload" ? (
          <UploadStep
            file={file}
            onFileChange={(next) => {
              setFile(next);
              resetError();
            }}
            format={format}
            onFormatChange={(value) => {
              setFormat(value);
              resetError();
            }}
            csvHeader={csvHeader}
            onCsvHeaderChange={(next) => {
              setCsvHeader(next);
              resetError();
            }}
            readOnly={readOnly}
          />
        ) : null}

        {stage === "review" && session && validation ? (
          <ReviewStep session={session} validation={validation} />
        ) : null}

        {stage === "done" ? (
          <p className="text-xs text-foreground" data-testid="import-done-message">
            Imported {inserted} row{inserted === 1 ? "" : "s"} into{" "}
            {qualified}. The existing rows were left untouched (all-or-nothing).
          </p>
        ) : null}

        {error ? (
          <p role="alert" className="text-xs text-destructive" data-testid="import-error">
            {error}
          </p>
        ) : null}

        <DialogFooter>
          {stage === "review" ? (
            <Button
              variant="ghost"
              onClick={() => {
                setStage("upload");
                resetError();
              }}
              disabled={busy}
            >
              Back
            </Button>
          ) : null}
          <Button
            variant="ghost"
            onClick={close}
            disabled={busy}
            data-testid="import-dismiss"
          >
            {stage === "done" ? "Close" : "Cancel"}
          </Button>
          {stage === "upload" ? (
            <Button
              onClick={() => void upload()}
              disabled={busy || !file || readOnly}
              data-testid="import-upload"
            >
              {busy ? "Uploading…" : "Upload & Review"}
            </Button>
          ) : null}
          {stage === "review" ? (
            <Button
              onClick={() => void execute()}
              disabled={busy || validation?.valid === false}
              data-testid="import-execute"
            >
              {busy ? "Importing…" : "Execute Import"}
            </Button>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function UploadStep({
  file,
  onFileChange,
  format,
  onFormatChange,
  csvHeader,
  onCsvHeaderChange,
  readOnly,
}: {
  file: File | null;
  onFileChange: (file: File | null) => void;
  format: TableImportFormat;
  onFormatChange: (value: TableImportFormat) => void;
  csvHeader: boolean;
  onCsvHeaderChange: (value: boolean) => void;
  readOnly: boolean;
}) {
  return (
    <div className="flex flex-col gap-3">
      <label className="flex flex-col gap-1">
        <span className="text-xs font-medium text-foreground">File</span>
        <Input
          type="file"
          accept={format === "csv" ? ".csv,text/csv" : ".sql,text/plain"}
          onChange={(event) => onFileChange(event.target.files?.[0] ?? null)}
          disabled={readOnly}
          data-testid="import-file"
        />
        {file ? (
          <span className="text-[10px] text-muted-foreground">
            {file.name} · {file.size.toLocaleString()} bytes
          </span>
        ) : null}
      </label>

      <fieldset className="flex flex-col gap-1">
        <legend className="text-xs font-medium text-foreground">Format</legend>
        {FORMATS.map((option) => (
          <label key={option.value} className="flex items-center gap-2 text-xs text-foreground">
            <input
              type="radio"
              name="import-format"
              value={option.value}
              checked={format === option.value}
              disabled={readOnly}
              onChange={() => onFormatChange(option.value)}
              data-testid={`import-format-${option.value}`}
            />
            <span className="font-medium">{option.label}</span>
            <span className="text-[10px] text-muted-foreground">{option.hint}</span>
          </label>
        ))}
      </fieldset>

      {format === "csv" ? (
        <label className="flex items-center gap-2 text-xs text-foreground">
          <input
            type="checkbox"
            checked={csvHeader}
            disabled={readOnly}
            onChange={(event) => onCsvHeaderChange(event.target.checked)}
            data-testid="import-csv-header"
          />
          First row is a header
        </label>
      ) : null}
    </div>
  );
}

function ReviewStep({
  session,
  validation,
}: {
  session: TableImportSession;
  validation: TableImportValidation;
}) {
  const preview = session.preview ?? [];
  const errors = validation.errors ?? [];
  const mapping = validation.mapping ?? [];

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
        <span>
          <strong className="text-foreground">{session.row_count ?? 0}</strong> rows
        </span>
        <span>
          <strong className="text-foreground">{mapping.length}</strong> mapped columns
        </span>
        {session.filename ? (
          <span className="truncate">file: {session.filename}</span>
        ) : null}
      </div>

      {errors.length > 0 ? (
        <div
          role="alert"
          className="max-h-32 overflow-y-auto rounded border border-destructive/40 bg-destructive/5 p-2 text-xs text-destructive"
          data-testid="import-validation-errors"
        >
          {errors.map((err, index) => (
            <div key={index} className="flex gap-2">
              <span>row {err.row}:</span>
              <span>
                {err.column ? `${err.column} — ` : ""}
                {err.message}
              </span>
            </div>
          ))}
        </div>
      ) : (
        <p className="text-xs text-emerald-600" data-testid="import-validation-ok">
          Target validated — {validation.row_count ?? 0} rows ready to import.
        </p>
      )}

      {mapping.length > 0 ? (
        <div className="flex flex-wrap gap-2" data-testid="import-mapping">
          {mapping.map((m: ImportMappingResolved, index) => (
            <span
              key={index}
              className="rounded bg-panel-raised px-2 py-1 text-[10px] text-subtle-foreground"
            >
              {session.columns?.[m.source ?? 0] ?? `col ${m.source}`} → {m.target}
            </span>
          ))}
        </div>
      ) : null}

      {preview.length > 0 ? (
        <div className="max-h-48 overflow-auto">
          <table className="w-full border-collapse text-[11px]" data-testid="import-preview">
            <thead>
              <tr>
                {(session.columns ?? []).map((name, index) => (
                  <th
                    key={`${index}-${name}`}
                    className="sticky top-0 border-b border-border bg-background px-2 py-1 text-left font-medium text-subtle-foreground"
                  >
                    {name}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {preview.map((row, rowIndex) => (
                <tr key={rowIndex}>
                  {row.map((cell, cellIndex) => (
                    <td
                      key={cellIndex}
                      className="border-b border-border/50 px-2 py-1 text-foreground"
                    >
                      <span className={cellStyle(cell)}>{cellText(cell)}</span>
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}

      <p className="text-[10px] text-muted-foreground">
        Unquoted empty CSV fields and SQL NULLs import as NULL; an empty string
        must be wrapped in double quotes to survive.
      </p>
    </div>
  );
}