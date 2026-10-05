"use client";

import { Plus, Trash2 } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import type { TableColumnInfo } from "@/types/api";
import type { FilterOperator, TableFilter } from "@/store/useWorkspaceStore";

type FilterDraft = {
  column: string;
  operator: FilterOperator | "";
  value: string;
  values: string;
};

const OPERATOR_LABELS: Record<FilterOperator, string> = {
  equals: "equals",
  not_equals: "not equals",
  contains: "contains",
  starts_with: "starts with",
  ends_with: "ends with",
  greater_than: ">",
  greater_or_equal: ">=",
  less_than: "<",
  less_or_equal: "<=",
  is_null: "is null",
  is_not_null: "is not null",
  in: "in",
};

const TEXT_OPS: FilterOperator[] = [
  "equals", "not_equals", "contains", "starts_with", "ends_with", "in", "is_null", "is_not_null",
];
const NUMERIC_OPS: FilterOperator[] = [
  "equals", "not_equals", "greater_than", "greater_or_equal", "less_than", "less_or_equal", "in", "is_null", "is_not_null",
];
const BOOLEAN_OPS: FilterOperator[] = ["equals", "not_equals", "is_null", "is_not_null"];
const NULL_OPS: FilterOperator[] = ["is_null", "is_not_null"];

type Category = "text" | "numeric" | "temporal" | "boolean" | "equality" | "unsupported";

/** Mirrors the backend's conservative type classification (for UX only). */
function categoryOf(dataType: string | undefined): Category {
  const base = (dataType ?? "").toLowerCase().split(/[( ]/)[0];
  if (["char", "varchar", "nvarchar", "nchar", "character", "text", "clob", "tinytext", "mediumtext", "longtext", "citext", "enum", "set"].includes(base)) return "text";
  if (["smallint", "int", "int2", "int4", "int8", "integer", "bigint", "serial", "bigserial", "smallserial", "mediumint", "tinyint", "decimal", "numeric", "real", "float", "float4", "float8", "double", "money"].includes(base)) return "numeric";
  if (["date", "time", "timestamp", "timestamptz", "datetime", "year"].includes(base)) return "temporal";
  if (["bool", "boolean"].includes(base)) return "boolean";
  if (["uuid", "uniqueidentifier"].includes(base)) return "equality";
  return "unsupported";
}

export function operatorsFor(category: Category): FilterOperator[] {
  switch (category) {
    case "text":
      return TEXT_OPS;
    case "numeric":
    case "temporal":
      return NUMERIC_OPS;
    case "boolean":
      return BOOLEAN_OPS;
    case "equality":
      return ["equals", "not_equals", "in", "is_null", "is_not_null"];
    default:
      return NULL_OPS;
  }
}

function emptyDraft(columns: TableColumnInfo[]): FilterDraft {
  const first = columns[0];
  const ops = operatorsFor(categoryOf(first?.database_type));
  return { column: first?.name ?? "", operator: ops[0] ?? "", value: "", values: "" };
}

function draftToFilter(draft: FilterDraft): TableFilter | null {
  if (!draft.column || !draft.operator) return null;
  const operator = draft.operator;
  if (operator === "is_null" || operator === "is_not_null") {
    return { column: draft.column, operator };
  }
  if (operator === "in") {
    const values = draft.values
      .split(",")
      .map((part) => part.trim())
      .filter((part) => part !== "");
    return values.length > 0 ? { column: draft.column, operator, values } : null;
  }
  if (draft.value === "") return null;
  return { column: draft.column, operator, value: draft.value };
}

/**
 * Structured filter builder (PRF-02/T06). Drives draft state locally; only
 * Apply writes the applied filters onto the TableDataTab (which triggers the
 * server request). Never sends SQL — the backend builds a parameterized WHERE.
 */
export function TableDataFilterPanel({
  columns,
  applied,
  onApply,
  onClear,
  onClose,
}: {
  columns: TableColumnInfo[];
  applied: TableFilter[];
  onApply: (filters: TableFilter[]) => void;
  onClear: () => void;
  onClose: () => void;
}) {
  const [draft, setDraft] = useState<FilterDraft[]>(() =>
    applied.length > 0
      ? applied.map((filter) => ({
          column: filter.column,
          operator: filter.operator,
          value: filter.value == null ? "" : String(filter.value),
          values: (filter.values ?? []).map((v) => String(v)).join(", "),
        }))
      : [emptyDraft(columns)],
  );
  const [error, setError] = useState<string | null>(null);

  function update(index: number, patch: Partial<FilterDraft>) {
    setDraft((rows) => rows.map((row, i) => (i === index ? { ...row, ...patch } : row)));
  }

  function apply() {
    const filters: TableFilter[] = [];
    for (const row of draft) {
      const hasInput = row.column && row.operator;
      if (!hasInput) continue;
      const filter = draftToFilter(row);
      if (!filter) {
        setError("Each filter needs a value (except is null / is not null).");
        return;
      }
      filters.push(filter);
    }
    setError(null);
    onApply(filters);
  }

  return (
    <div
      data-testid="table-data-filter-panel"
      className="shrink-0 border-b border-border bg-panel px-3 py-2 text-xs"
    >
      <div className="mb-1 flex items-center justify-between">
        <span className="font-medium text-foreground">Filter</span>
        <button
          type="button"
          onClick={onClose}
          aria-label="Close filters"
          className="rounded px-1 text-[10px] text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
        >
          Close
        </button>
      </div>

      <div className="flex flex-col gap-1">
        {draft.map((row, index) => {
          const column = columns.find((c) => c.name === row.column);
          const ops = operatorsFor(categoryOf(column?.database_type));
          const noValue = row.operator === "is_null" || row.operator === "is_not_null";
          return (
            <div key={index} className="flex flex-wrap items-center gap-1">
              <select
                aria-label={`Filter ${index + 1} column`}
                value={row.column}
                onChange={(event) => {
                  const next = columns.find((c) => c.name === event.target.value);
                  const nextOps = operatorsFor(categoryOf(next?.database_type));
                  update(index, {
                    column: event.target.value,
                    operator: nextOps.includes(row.operator as FilterOperator) ? row.operator : (nextOps[0] ?? ""),
                  });
                }}
                className="rounded border border-border bg-background px-1 py-0.5 text-[11px] text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
              >
                <option value="">column</option>
                {columns.map((c) => (
                  <option key={c.name} value={c.name}>
                    {c.name}
                  </option>
                ))}
              </select>

              <select
                aria-label={`Filter ${index + 1} operator`}
                value={row.operator}
                onChange={(event) => update(index, { operator: event.target.value as FilterOperator })}
                className="rounded border border-border bg-background px-1 py-0.5 text-[11px] text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
              >
                {ops.map((op) => (
                  <option key={op} value={op}>
                    {OPERATOR_LABELS[op]}
                  </option>
                ))}
              </select>

              {!noValue &&
                (row.operator === "in" ? (
                  <input
                    aria-label={`Filter ${index + 1} values`}
                    placeholder="value1, value2"
                    value={row.values}
                    onChange={(event) => update(index, { values: event.target.value })}
                    className="w-48 rounded border border-border bg-background px-1 py-0.5 text-[11px] text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
                  />
                ) : (
                  <input
                    aria-label={`Filter ${index + 1} value`}
                    value={row.value}
                    onChange={(event) => update(index, { value: event.target.value })}
                    className="w-48 rounded border border-border bg-background px-1 py-0.5 text-[11px] text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
                  />
                ))}

              <button
                type="button"
                aria-label={`Remove filter ${index + 1}`}
                onClick={() => setDraft((rows) => rows.filter((_, i) => i !== index))}
                className="rounded p-0.5 text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
              >
                <Trash2 size={11} aria-hidden="true" />
              </button>
            </div>
          );
        })}
      </div>

      {error && (
        <p role="alert" className="mt-1 text-[10px] text-destructive">
          {error}
        </p>
      )}

      <div className="mt-2 flex items-center gap-2">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setDraft((rows) => [...rows, emptyDraft(columns)])}
        >
          <Plus size={12} aria-hidden="true" />
          Add filter
        </Button>
        <Button variant="outline" size="sm" onClick={apply}>
          Apply
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => {
            setDraft([emptyDraft(columns)]);
            setError(null);
            onClear();
          }}
        >
          Clear
        </Button>
      </div>
    </div>
  );
}
