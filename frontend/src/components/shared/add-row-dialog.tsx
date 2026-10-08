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
import { insertRow } from "@/lib/api/endpoints";
import { mutationErrorMessage } from "@/lib/mutation-errors";
import { validateRowField, type RowFieldMode } from "@/lib/row-field-validation";
import type { TableColumnInfo } from "@/types/api";

type Mode = RowFieldMode;

interface FieldState {
  mode: Mode;
  value: string;
}

function categoryIsBoolean(dataType: string | undefined): boolean {
  const base = (dataType ?? "").toLowerCase().split(/[( ]/)[0];
  return base === "bool" || base === "boolean";
}

/** Compact metadata chips (T11 §11): identity / nullability / default. */
function ColumnBadges({ column }: { column: TableColumnInfo }) {
  const badges: string[] = [];
  if (column.primary_key) badges.push("PK");
  if (!column.nullable) badges.push("NOT NULL");
  if (column.has_default) badges.push("DEFAULT");
  if (badges.length === 0) return null;
  return (
    <span className="flex shrink-0 gap-1">
      {badges.map((badge) => (
        <span
          key={badge}
          data-testid={`column-badge-${badge === "NOT NULL" ? "not-null" : badge.toLowerCase()}`}
          className="rounded border border-border px-1 text-[9px] text-muted-foreground"
        >
          {badge}
        </span>
      ))}
    </span>
  );
}

/**
 * Explicit Add Row dialog (PRF-02/T08 + T11 §11/§14). Fields are metadata-driven:
 * only insertable columns are editable; each supports VALUE / NULL (when nullable)
 * / DEFAULT. Values are sent as data — never SQL — and the backend is
 * authoritative. Conservative client-side validation blocks obvious mistakes
 * with inline, per-field errors before any request. The grid is never edited inline.
 */
export function AddRowDialog({
  connectionId,
  database,
  schema,
  table,
  columns,
  initialValues,
  title,
  onClose,
  onInserted,
}: {
  connectionId: string;
  database: string | null;
  schema: string | null;
  table: string;
  columns: TableColumnInfo[];
  /** Optional prefill (e.g. Duplicate Row); value modes are preserved. */
  initialValues?: Record<string, { mode: Mode; value?: string }>;
  title?: string;
  onClose: () => void;
  onInserted: () => void;
}) {
  const editable = columns.filter((column) => column.insertable !== false);
  const [fields, setFields] = useState<Record<string, FieldState>>(() =>
    Object.fromEntries(
      editable.map((column) => {
        const name = column.name ?? "";
        if (initialValues?.[name]) {
          return [name, { mode: initialValues[name].mode, value: initialValues[name].value ?? "" }];
        }
        return [
          name,
          {
            // NOT NULL without a default needs an explicit value; everything else
            // can rely on the database default (nullable + no default → NULL).
            mode: (!column.nullable && !column.has_default ? "value" : "default") as Mode,
            value: "",
          },
        ];
      }),
    ),
  );
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const submittingRef = useRef(false);

  function update(column: string, patch: Partial<FieldState>) {
    setFields((current) => ({ ...current, [column]: { ...current[column], ...patch } }));
    setFieldErrors((current) => {
      if (!(column in current)) return current;
      const next = { ...current };
      delete next[column];
      return next;
    });
  }

  async function submit() {
    if (submittingRef.current) return;
    const errors: Record<string, string> = {};
    for (const column of editable) {
      const name = column.name ?? "";
      const field = fields[name];
      if (!field) continue;
      const message = validateRowField(column, field.mode, field.value);
      if (message) errors[name] = message;
    }
    setFieldErrors(errors);
    if (Object.keys(errors).length > 0) return;

    const values: Record<string, { mode: Mode; value?: unknown }> = {};
    for (const [column, field] of Object.entries(fields)) {
      if (field.mode === "value") {
        values[column] = { mode: "value", value: field.value };
      } else if (field.mode === "null") {
        values[column] = { mode: "null" };
      }
      // "default" is omitted so the database applies its default.
    }
    submittingRef.current = true;
    setSubmitting(true);
    setError(null);
    try {
      await insertRow(connectionId, { database, schema, table, values });
      onInserted();
    } catch (cause) {
      setError(mutationErrorMessage(cause, "Insert failed."));
      submittingRef.current = false;
      setSubmitting(false);
    }
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        // Never close mid-submit: the mutation must settle before the dialog goes.
        if (!open && !submittingRef.current) onClose();
      }}
    >
      <DialogContent aria-labelledby="add-row-title" data-testid="add-row-dialog">
        <DialogHeader>
          <DialogTitle id="add-row-title">{title ?? `Add Row — ${table}`}</DialogTitle>
          <DialogDescription>
            Insert one row into {schema ? `${schema}.${table}` : table}. DEFAULT
            leaves the value to the database.
          </DialogDescription>
        </DialogHeader>

        <div className="flex max-h-80 flex-col gap-2 overflow-auto">
          {columns.map((column) => {
            const name = column.name ?? "";
            const isEditable = editable.some((candidate) => candidate.name === name);
            if (!isEditable) {
              return (
                <div
                  key={name}
                  className="flex items-center justify-between gap-2 text-xs text-muted-foreground"
                >
                  <span className="font-medium text-foreground">{name}</span>
                  <span className="flex items-center gap-2">
                    <ColumnBadges column={column} />
                    <span className="text-[10px]">
                      {column.database_type} · database managed
                    </span>
                  </span>
                </div>
              );
            }
            const field = fields[name] ?? { mode: "value", value: "" };
            const boolean = categoryIsBoolean(column.database_type);
            const fieldError = fieldErrors[name];
            const errorId = `${name}-field-error`;
            return (
              <div key={name} className="flex flex-wrap items-center gap-2 text-xs">
                <span className="w-32 truncate font-medium text-foreground" title={name}>
                  {name}
                </span>
                <span className="flex w-24 items-center gap-1 text-[10px] text-muted-foreground">
                  <span className="truncate" title={column.database_type}>
                    {column.database_type}
                  </span>
                  <ColumnBadges column={column} />
                </span>
                <select
                  aria-label={`${name} mode`}
                  value={field.mode}
                  onChange={(event) => update(name, { mode: event.target.value as Mode })}
                  className="rounded border border-border bg-background px-1 py-0.5 text-[11px] text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
                >
                  <option value="value">value</option>
                  {column.nullable ? <option value="null">null</option> : null}
                  <option value="default">default</option>
                </select>
                {field.mode === "value" ? (
                  boolean ? (
                    <select
                      aria-label={`${name} value`}
                      aria-invalid={fieldError ? true : undefined}
                      aria-describedby={fieldError ? errorId : undefined}
                      value={field.value}
                      onChange={(event) => update(name, { value: event.target.value })}
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
                      onChange={(event) => update(name, { value: event.target.value })}
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
          <p role="alert" className="text-xs text-destructive">
            {error}
          </p>
        ) : null}

        <DialogFooter>
          <Button variant="ghost" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button onClick={() => void submit()} disabled={submitting}>
            {submitting ? "Inserting…" : "Insert"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
