"use client";

import { useMemo, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { updateRow } from "@/lib/api/endpoints";
import { ApiClientError } from "@/lib/api-client";
import type { TableColumnInfo } from "@/types/api";

type Draft = Record<string, string | null>;

function categoryIsBoolean(dataType: string | undefined): boolean {
  const base = (dataType ?? "").toLowerCase().split(/[( ]/)[0];
  return base === "bool" || base === "boolean";
}

function formatOriginal(value: unknown): string {
  if (value === null || value === undefined) return "NULL";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

/**
 * Explicit Edit Row dialog (PRF-02/T09). The original row snapshot is frozen for
 * optimistic concurrency; only changed updatable columns are submitted. PK,
 * generated and identity columns are read-only. No inline grid editing.
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
  // Immutable original snapshot (never mutated by the draft).
  const original = useMemo(
    () => Object.fromEntries(columns.map((column, index) => [column.name ?? "", row[index]])),
    [columns, row],
  );
  const pkColumns = useMemo(
    () => columns.filter((column) => column.primary_key).map((column) => column.name ?? ""),
    [columns],
  );
  const updatable = useMemo(
    () => columns.filter((column) => column.updatable === true),
    [columns],
  );
  const [draft, setDraft] = useState<Draft>(() =>
    Object.fromEntries(
      updatable.map((column) => {
        const value = original[column.name ?? ""];
        return [column.name ?? "", value === null || value === undefined ? null : String(value)];
      }),
    ),
  );
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const submittingRef = useRef(false);

  function setField(column: string, value: string | null) {
    setDraft((current) => ({ ...current, [column]: value }));
  }

  function computeChanges(): Record<string, unknown> {
    const changes: Record<string, unknown> = {};
    for (const column of updatable) {
      const name = column.name ?? "";
      const before = original[name];
      const beforeText = before === null || before === undefined ? null : String(before);
      const after = draft[name] ?? null;
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
      setError(
        cause instanceof ApiClientError
          ? `${cause.message} (${cause.code})`
          : "Update failed.",
      );
      submittingRef.current = false;
      setSubmitting(false);
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
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
            return (
              <div key={name} className="flex flex-wrap items-center gap-2 text-xs">
                <span className="w-32 truncate font-medium text-foreground" title={name}>
                  {name}
                </span>
                <span className="w-24 text-[10px] text-muted-foreground">{column.database_type}</span>
                {boolean ? (
                  <select
                    aria-label={`${name} value`}
                    value={draft[name] ?? ""}
                    onChange={(event) => setField(name, event.target.value || null)}
                    disabled={draft[name] === null}
                    className="w-40 rounded border border-border bg-background px-1 py-0.5 text-[11px] text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
                  >
                    <option value="">true/false</option>
                    <option value="true">true</option>
                    <option value="false">false</option>
                  </select>
                ) : (
                  <input
                    aria-label={`${name} value`}
                    value={draft[name] ?? ""}
                    disabled={draft[name] === null}
                    onChange={(event) => setField(name, event.target.value)}
                    className="w-40 rounded border border-border bg-background px-1 py-0.5 text-[11px] text-foreground disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
                  />
                )}
                {column.nullable ? (
                  <label className="flex items-center gap-1 text-[10px] text-muted-foreground">
                    <input
                      type="checkbox"
                      aria-label={`${name} null`}
                      checked={draft[name] === null}
                      onChange={(event) => setField(name, event.target.checked ? null : "")}
                    />
                    NULL
                  </label>
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
          <Button onClick={() => void save()} disabled={submitting || !hasChanges}>
            {submitting ? "Saving…" : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
