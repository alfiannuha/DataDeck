"use client";

import { useState } from "react";

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
import { ApiClientError } from "@/lib/api-client";
import type { TableColumnInfo } from "@/types/api";

type Mode = "value" | "null" | "default";

interface FieldState {
  mode: Mode;
  value: string;
}

function categoryIsBoolean(dataType: string | undefined): boolean {
  const base = (dataType ?? "").toLowerCase().split(/[( ]/)[0];
  return base === "bool" || base === "boolean";
}

/**
 * Explicit Add Row dialog (PRF-02/T08). Fields are metadata-driven: only
 * insertable columns are editable; each supports VALUE / NULL (when nullable) /
 * DEFAULT. Values are sent as data — never SQL — and the backend is
 * authoritative. The grid is never edited inline.
 */
export function AddRowDialog({
  connectionId,
  database,
  schema,
  table,
  columns,
  onClose,
  onInserted,
}: {
  connectionId: string;
  database: string | null;
  schema: string | null;
  table: string;
  columns: TableColumnInfo[];
  onClose: () => void;
  onInserted: () => void;
}) {
  const editable = columns.filter((column) => column.insertable !== false);
  const [fields, setFields] = useState<Record<string, FieldState>>(() =>
    Object.fromEntries(
      editable.map((column) => [
        column.name ?? "",
        {
          // NOT NULL without a default needs an explicit value; everything else
          // can rely on the database default (nullable + no default → NULL).
          mode: (!column.nullable && !column.has_default ? "value" : "default") as Mode,
          value: "",
        },
      ]),
    ),
  );
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  function update(column: string, patch: Partial<FieldState>) {
    setFields((current) => ({ ...current, [column]: { ...current[column], ...patch } }));
  }

  async function submit() {
    if (submitting) return;
    const values: Record<string, { mode: Mode; value?: unknown }> = {};
    for (const [column, field] of Object.entries(fields)) {
      if (field.mode === "value") {
        values[column] = { mode: "value", value: field.value };
      } else if (field.mode === "null") {
        values[column] = { mode: "null" };
      }
      // "default" is omitted so the database applies its default.
    }
    setSubmitting(true);
    setError(null);
    try {
      await insertRow(connectionId, { database, schema, table, values });
      onInserted();
    } catch (cause) {
      setError(
        cause instanceof ApiClientError
          ? `${cause.message} (${cause.code})`
          : "Insert failed.",
      );
      setSubmitting(false);
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent aria-labelledby="add-row-title" data-testid="add-row-dialog">
        <DialogHeader>
          <DialogTitle id="add-row-title">Add Row — {table}</DialogTitle>
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
                  <span className="text-[10px]">
                    {column.database_type} · database managed
                  </span>
                </div>
              );
            }
            const field = fields[name] ?? { mode: "value", value: "" };
            const boolean = categoryIsBoolean(column.database_type);
            return (
              <div key={name} className="flex flex-wrap items-center gap-2 text-xs">
                <span className="w-32 truncate font-medium text-foreground" title={name}>
                  {name}
                </span>
                <span className="w-24 text-[10px] text-muted-foreground">
                  {column.database_type}
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
                      value={field.value}
                      onChange={(event) => update(name, { value: event.target.value })}
                      className="w-40 rounded border border-border bg-background px-1 py-0.5 text-[11px] text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
                    />
                  )
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
