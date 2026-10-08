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
import { browseTableData, updateRow } from "@/lib/api/endpoints";
import { ApiClientError } from "@/lib/api-client";
import { mutationErrorMessage } from "@/lib/mutation-errors";
import { validateRowField, type RowFieldMode } from "@/lib/row-field-validation";
import type { TableColumnInfo } from "@/types/api";

type Mode = Exclude<RowFieldMode, "default">;

interface FieldState {
  mode: Mode;
  value: string;
}

type Draft = Record<string, FieldState>;

function categoryIsBoolean(dataType: string | undefined): boolean {
  const base = (dataType ?? "").toLowerCase().split(/[( ]/)[0];
  return base === "bool" || base === "boolean";
}

function formatOriginal(value: unknown): string {
  if (value === null || value === undefined) return "NULL";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

function buildDraft(
  updatable: TableColumnInfo[],
  source: Record<string, unknown>,
): Draft {
  return Object.fromEntries(
    updatable.map((column) => {
      const name = column.name ?? "";
      const value = source[name];
      if (value === null || value === undefined) {
        return [name, { mode: column.nullable ? ("null" as Mode) : ("value" as Mode), value: "" }];
      }
      return [name, { mode: "value" as Mode, value: String(value) }];
    }),
  );
}

/**
 * Explicit Edit Row dialog (PRF-02/T09 + T11 §11/§14/§16). The original row
 * snapshot is frozen for optimistic concurrency; only changed updatable columns
 * are submitted. PK, generated and identity columns are read-only. Fields use the
 * same VALUE/NULL mode controls as Add Row, with conservative client-side
 * validation. On ROW_CONFLICT the dialog stays open and offers Reload, which
 * refetches the row and resets the draft — there is no force-save and no merge.
 */
export function EditRowDialog({
  connectionId,
  database,
  schema,
  table,
  columns,
  row,
  onClose,
  onUpdated,
}: {
  connectionId: string;
  database: string | null;
  schema: string | null;
  table: string;
  columns: TableColumnInfo[];
  row: unknown[];
  onClose: () => void;
  onUpdated: () => void;
}) {
  const pkColumns = columns
    .filter((column) => column.primary_key)
    .map((column) => column.name ?? "");
  const updatable = columns.filter((column) => column.updatable === true);
  // Immutable original snapshot (state, not derived props: Reload replaces it).
  const [original, setOriginal] = useState<Record<string, unknown>>(() =>
    Object.fromEntries(columns.map((column, index) => [column.name ?? "", row[index]])),
  );
  const [draft, setDraft] = useState<Draft>(() => buildDraft(updatable, Object.fromEntries(
    columns.map((column, index) => [column.name ?? "", row[index]]),
  )));
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);
  const [conflict, setConflict] = useState(false);
  const [reloading, setReloading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const submittingRef = useRef(false);

  function setField(column: string, patch: Partial<FieldState>) {
    setDraft((current) => ({
      ...current,
      [column]: { ...current[column], ...patch },
    }));
    setFieldErrors((current) => {
      if (!(column in current)) return current;
      const next = { ...current };
      delete next[column];
      return next;
    });
  }

  function computeChanges(): Record<string, unknown> {
    const changes: Record<string, unknown> = {};
    for (const column of updatable) {
      const name = column.name ?? "";
      const field = draft[name];
      if (!field) continue;
      const before = original[name];
      const beforeText = before === null || before === undefined ? null : String(before);
      const after = field.mode === "null" ? null : field.value;
      if (after === beforeText) continue;
      if (after === null) {
        changes[name] = null;
      } else if (categoryIsBoolean(column.database_type)) {
        changes[name] = after === "true";
      } else {
        changes[name] = after;
      }
    }
    return changes;
  }

  const changes = computeChanges();
  const hasChanges = Object.keys(changes).length > 0;

  async function save() {
    if (submittingRef.current || !hasChanges) return;
    const errors: Record<string, string> = {};
    for (const column of updatable) {
      const name = column.name ?? "";
      const field = draft[name];
      if (!field) continue;
      const message = validateRowField(column, field.mode, field.value);
      if (message) errors[name] = message;
    }
    setFieldErrors(errors);
    if (Object.keys(errors).length > 0) return;

    submittingRef.current = true;
    setSubmitting(true);
    setError(null);
    const identity = Object.fromEntries(pkColumns.map((name) => [name, original[name]]));
    const expected: Record<string, unknown> = {};
    for (const column of columns) {
      const name = column.name ?? "";
      if (column.primary_key || name === "") continue;
      expected[name] = original[name] ?? null;
    }
    try {
      await updateRow(connectionId, { database, schema, table, identity, expected, changes });
      onUpdated();
    } catch (cause) {
      const code = cause instanceof ApiClientError ? cause.code : "";
      setConflict(code === "ROW_CONFLICT" || code === "ROW_NOT_FOUND");
      setError(mutationErrorMessage(cause, "Update failed."));
      submittingRef.current = false;
      setSubmitting(false);
    }
  }

  // Conflict recovery (T11 §16): refetch the row by its identity, then replace
  // the snapshot and reset the draft. Never force-saves stale data.
  async function reload() {
    if (reloading) return;
    setReloading(true);
    setError(null);
    try {
      const filters = pkColumns.map((name) => ({
        column: name,
        operator: "equals",
        value: original[name],
      }));
      const page = await browseTableData(connectionId, {
        database,
        schema,
        table,
        page: 1,
        pageSize: 1,
        filters,
      });
      const fresh = page.rows?.[0];
      if (!fresh) {
        setConflict(false);
        setError("This row no longer exists. (ROW_NOT_FOUND)");
        return;
      }
      const next = Object.fromEntries(columns.map((column, index) => [column.name ?? "", fresh[index]]));
      setOriginal(next);
      setDraft(buildDraft(updatable, next));
      setFieldErrors({});
      setConflict(false);
      setError(null);
    } catch (cause) {
      setError(mutationErrorMessage(cause, "Reload failed."));
    } finally {
      setReloading(false);
    }
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !submittingRef.current) onClose();
      }}
    >
      <DialogContent aria-labelledby="edit-row-title" data-testid="edit-row-dialog">
        <DialogHeader>
          <DialogTitle id="edit-row-title">Edit Row — {table}</DialogTitle>
          <DialogDescription>
            Only changed fields are saved. The row is updated only if it has not
            changed since it was loaded.
          </DialogDescription>
        </DialogHeader>

        <div className="flex max-h-80 flex-col gap-2 overflow-auto">
          {columns.map((column) => {
            const name = column.name ?? "";
            const editable = updatable.some((candidate) => candidate.name === name);
            const boolean = categoryIsBoolean(column.database_type);
            if (!editable) {
              return (
                <div key={name} className="flex items-center justify-between gap-2 text-xs">
                  <span className="w-32 truncate font-medium text-foreground" title={name}>
                    {name}
                  </span>
                  <span className="w-24 text-[10px] text-muted-foreground">{column.database_type}</span>
                  <span className="flex-1 truncate text-[11px] text-muted-foreground">
                    {formatOriginal(original[name])}
                  </span>
                  <span className="text-[10px] text-subtle-foreground">read-only</span>
                </div>
              );
            }
            const field = draft[name] ?? { mode: "value" as Mode, value: "" };
            const fieldError = fieldErrors[name];
            const errorId = `${name}-field-error`;
            return (
              <div key={name} className="flex flex-wrap items-center gap-2 text-xs">
                <span className="w-32 truncate font-medium text-foreground" title={name}>
                  {name}
                </span>
                <span className="w-24 text-[10px] text-muted-foreground">{column.database_type}</span>
                <select
                  aria-label={`${name} mode`}
                  value={field.mode}
                  onChange={(event) => setField(name, { mode: event.target.value as Mode })}
                  className="rounded border border-border bg-background px-1 py-0.5 text-[11px] text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
                >
                  <option value="value">value</option>
                  {column.nullable ? <option value="null">null</option> : null}
                </select>
                {field.mode === "value" ? (
                  boolean ? (
                    <select
                      aria-label={`${name} value`}
                      aria-invalid={fieldError ? true : undefined}
                      aria-describedby={fieldError ? errorId : undefined}
                      value={field.value}
                      onChange={(event) => setField(name, { value: event.target.value })}
                      className="w-40 rounded border border-border bg-background px-1 py-0.5 text-[11px] text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
                    >
                      <option value="">true/false</option>
                      <option value="true">true</option>
                      <option value="false">false</option>
                    </select>
                  ) : (
                    <input
                      aria-label={`${name} value`}
                      aria-invalid={fieldError ? true : undefined}
                      aria-describedby={fieldError ? errorId : undefined}
                      value={field.value}
                      onChange={(event) => setField(name, { value: event.target.value })}
                      className="w-40 rounded border border-border bg-background px-1 py-0.5 text-[11px] text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
                    />
                  )
                ) : null}
                {fieldError ? (
                  <p
                    id={errorId}
                    role="alert"
                    data-testid="row-field-error"
                    className="w-full pl-32 text-[10px] text-destructive"
                  >
                    {fieldError}
                  </p>
                ) : null}
              </div>
            );
          })}
        </div>

        {error ? (
          <div className="flex items-center justify-between gap-2">
            <p role="alert" className="text-xs text-destructive">
              {error}
            </p>
            {conflict ? (
              <Button
                variant="outline"
                size="sm"
                data-testid="edit-row-reload"
                disabled={reloading}
                onClick={() => void reload()}
              >
                {reloading ? "Reloading…" : "Reload row"}
              </Button>
            ) : null}
          </div>
        ) : null}

        <DialogFooter>
          <Button variant="ghost" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button onClick={() => void save()} disabled={submitting || !hasChanges}>
            {submitting ? "Saving…" : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
