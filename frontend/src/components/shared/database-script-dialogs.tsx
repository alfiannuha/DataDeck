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
  executeDatabaseImport,
  exportDatabase,
  uploadDatabaseImport,
  type DatabaseExportMode,
} from "@/lib/api/endpoints";
import { downloadBlob } from "@/lib/export/build-export";
import { sanitizeFilenamePart } from "@/lib/export/filename";
import { mutationErrorMessage } from "@/lib/mutation-errors";
import {
  useDatabaseScriptStore,
  type DatabaseScriptTarget,
} from "@/store/useDatabaseScriptStore";
import type { DatabaseImportPreview } from "@/types/api";

const MODES: { value: DatabaseExportMode; label: string; hint: string }[] = [
  { value: "schema_data", label: "Schema + Data", hint: "Tables, constraints and rows" },
  { value: "schema", label: "Schema Only", hint: "Tables, constraints and indexes" },
  { value: "data", label: "Data Only", hint: "INSERT statements only" },
];

/**
 * Database-level export/restore dialogs (PRF-02/T17). The target is the explicit
 * database captured by the context menu — never the active tab or Explorer
 * selection. Export streams a dialect SQL script; restore only accepts a
 * DataDeck export artifact into an empty database, with a preflight step.
 */
export function DatabaseScriptDialogs() {
  const request = useDatabaseScriptStore((state) => state.request);
  if (!request) return null;
  return request.kind === "export" ? (
    <DatabaseExportDialogBody target={request.target} />
  ) : (
    <DatabaseRestoreDialogBody target={request.target} />
  );
}

function DatabaseExportDialogBody({ target }: { target: DatabaseScriptTarget }) {
  const close = useDatabaseScriptStore((state) => state.close);
  const [mode, setMode] = useState<DatabaseExportMode>("schema_data");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const busyRef = useRef(false);

  async function submit() {
    if (busyRef.current) return;
    busyRef.current = true;
    setBusy(true);
    setError(null);
    try {
      const result = await exportDatabase(target.connectionId, {
        database: target.database,
        mode,
      });
      const filename =
        result.filename ??
        `${sanitizeFilenamePart(target.database, "database")}_${mode}.sql`;
      downloadBlob(result.blob, filename);
      close();
    } catch (cause) {
      setError(mutationErrorMessage(cause, "Export failed."));
      busyRef.current = false;
      setBusy(false);
    }
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !busyRef.current) close();
      }}
    >
      <DialogContent aria-labelledby="database-export-title" data-testid="database-export-dialog">
        <DialogHeader>
          <DialogTitle id="database-export-title">
            Export Database — {target.database}
          </DialogTitle>
          <DialogDescription>
            {target.engine} · base tables only (views and other objects are excluded).
          </DialogDescription>
        </DialogHeader>

        <fieldset className="flex flex-col gap-1">
          <legend className="text-xs font-medium text-foreground">Export Mode</legend>
          {MODES.map((option) => (
            <label key={option.value} className="flex items-center gap-2 text-xs text-foreground">
              <input
                type="radio"
                name="database-export-mode"
                value={option.value}
                checked={mode === option.value}
                disabled={busy}
                onChange={() => setMode(option.value)}
                data-testid={`database-export-mode-${option.value}`}
              />
              <span className="font-medium">{option.label}</span>
              <span className="text-[10px] text-muted-foreground">{option.hint}</span>
            </label>
          ))}
        </fieldset>

        {error ? (
          <p role="alert" className="text-xs text-destructive" data-testid="database-export-error">
            {error}
          </p>
        ) : null}

        <DialogFooter>
          <Button variant="ghost" onClick={close} disabled={busy}>
            Cancel
          </Button>
          <Button onClick={() => void submit()} disabled={busy} data-testid="database-export-submit">
            {busy ? "Exporting…" : "Export SQL"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function DatabaseRestoreDialogBody({ target }: { target: DatabaseScriptTarget }) {
  const close = useDatabaseScriptStore((state) => state.close);
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<DatabaseImportPreview | null>(null);
  const [stage, setStage] = useState<"upload" | "confirm" | "done">("upload");
  const [inserted, setInserted] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const busyRef = useRef(false);

  async function upload() {
    if (busyRef.current || !file) return;
    busyRef.current = true;
    setBusy(true);
    setError(null);
    try {
      const result = await uploadDatabaseImport(target.connectionId, {
        database: target.database,
        file,
      });
      setPreview(result);
      setStage("confirm");
    } catch (cause) {
      setError(mutationErrorMessage(cause, "Validation failed."));
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  }

  async function execute() {
    if (busyRef.current || !preview?.token) return;
    const { token } = preview;
    busyRef.current = true;
    setBusy(true);
    setError(null);
    try {
      const result = await executeDatabaseImport(target.connectionId, token);
      setInserted(result.statements ?? 0);
      setStage("done");
    } catch (cause) {
      setError(mutationErrorMessage(cause, "Restore failed."));
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !busyRef.current) close();
      }}
    >
      <DialogContent aria-labelledby="database-restore-title" data-testid="database-restore-dialog">
        <DialogHeader>
          <DialogTitle id="database-restore-title">
            Restore Database — {target.database}
          </DialogTitle>
          <DialogDescription>
            {stage === "done"
              ? "The restore was applied atomically."
              : "Restores a DataDeck SQL export into an empty compatible database. Nothing is overwritten."}
          </DialogDescription>
        </DialogHeader>

        {stage === "upload" ? (
          <label className="flex flex-col gap-1">
            <span className="text-xs font-medium text-foreground">Artifact</span>
            <Input
              type="file"
              accept=".sql,text/plain"
              onChange={(event) => setFile(event.target.files?.[0] ?? null)}
              disabled={busy}
              data-testid="database-restore-file"
            />
            {file ? (
              <span className="text-[10px] text-muted-foreground">
                {file.name} · {file.size.toLocaleString()} bytes
              </span>
            ) : null}
          </label>
        ) : null}

        {stage === "confirm" && preview ? (
          <div className="flex flex-col gap-2 text-xs text-foreground" data-testid="database-restore-preview">
            <p className="text-muted-foreground">
              {preview.engine} · {preview.mode} · {preview.statements} statements
            </p>
            <ul className="flex flex-col gap-0.5">
              <li>
                <strong>{preview.tables}</strong> tables
              </li>
              <li>
                <strong>{preview.indexes}</strong> indexes
              </li>
              <li>
                <strong>{preview.foreign_keys}</strong> foreign keys
              </li>
            </ul>
            {preview.excluded && preview.excluded.length > 0 ? (
              <p className="text-[10px] text-muted-foreground">
                Excluded: {preview.excluded.join(", ")}
              </p>
            ) : null}
            <p className="text-[10px] text-muted-foreground">
              Restore into an empty database.
            </p>
          </div>
        ) : null}

        {stage === "done" ? (
          <p className="text-xs text-foreground" data-testid="database-restore-done">
            Restored {inserted} statement{inserted === 1 ? "" : "s"} into{" "}
            {target.database}. The database was empty, so nothing was overwritten.
          </p>
        ) : null}

        {error ? (
          <p role="alert" className="text-xs text-destructive" data-testid="database-restore-error">
            {error}
          </p>
        ) : null}

        <DialogFooter>
          {stage === "confirm" ? (
            <Button
              variant="ghost"
              onClick={() => {
                setStage("upload");
                setPreview(null);
                setError(null);
              }}
              disabled={busy}
            >
              Back
            </Button>
          ) : null}
          <Button variant="ghost" onClick={close} disabled={busy} data-testid="database-restore-dismiss">
            {stage === "done" ? "Close" : "Cancel"}
          </Button>
          {stage === "upload" ? (
            <Button
              onClick={() => void upload()}
              disabled={busy || !file}
              data-testid="database-restore-upload"
            >
              {busy ? "Validating…" : "Validate & Preview"}
            </Button>
          ) : null}
          {stage === "confirm" ? (
            <Button onClick={() => void execute()} disabled={busy} data-testid="database-restore-execute">
              {busy ? "Restoring…" : "Restore"}
            </Button>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
