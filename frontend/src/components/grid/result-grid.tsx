"use client";

import {
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table";
import { useVirtualizer } from "@tanstack/react-virtual";
import { ArrowDown, ArrowUp, ArrowUpDown, Check, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState, type KeyboardEvent as ReactKeyboardEvent, type ReactNode } from "react";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { copyText } from "@/lib/clipboard";
import {
  formatCellValue,
  isNullValue,
  rawCellValue,
} from "@/lib/result-grid/format-cell";
import {
  buildRowDraft,
  columnCategory,
  draftChanges,
  primaryKeyColumns,
  rowToRecord,
  updatableColumns,
  type DraftField,
  type RowDraft,
} from "@/lib/row-edit";
import { mutationErrorMessage } from "@/lib/mutation-errors";
import { validateRowField } from "@/lib/row-field-validation";
import { cn } from "@/lib/utils";
import type { QueryResult, TableColumnInfo } from "@/types/api";

const ROW_HEIGHT = 32;
const DEFAULT_COLUMN_WIDTH = 180;
const MIN_COLUMN_WIDTH = 80;
const OVERSCAN = 10;
const LEADING_COLUMN_WIDTH = 44;
/** Used when the scroll element has no measured height yet (first paint / jsdom). */
const FALLBACK_VIEWPORT_HEIGHT = 400;

type Row = unknown[];

/** Page-scoped row selection (indices into the current page's rows). */
export interface GridSelectionControl {
  selected: ReadonlySet<number>;
  onToggleRow: (rowIndex: number) => void;
  onToggleAll: (checked: boolean) => void;
}

/** Inline editing configuration. Only enabled when the row has a safe identity. */
export interface GridInlineEdit {
  /** Column metadata aligned with `result.columns` (data columns only). */
  columns: TableColumnInfo[];
  enabled: boolean;
  reason?: string;
  onSave: (
    rowIndex: number,
    payload: {
      identity: Record<string, unknown>;
      expected: Record<string, unknown>;
      changes: Record<string, unknown>;
    },
  ) => Promise<void>;
}

/**
 * Virtualized result grid. TanStack Table owns the column model (headers,
 * sizing, resize); TanStack Virtual owns row virtualization. Rows stay as
 * positional arrays — they are never expanded into per-row objects, and only
 * the visible rows (+ overscan) are mounted.
 *
 * Inline editing (PRF-05) keeps its draft in this component (keyed by column
 * name, for the single edited row), so it survives a virtualized row being
 * unmounted and remounted while scrolling.
 */
export function ResultGrid({
  result,
  ariaLabel = "Query result grid",
  emptyText = "No rows returned.",
  sortColumn = null,
  sortDirection = null,
  onSortColumn,
  rowMenu,
  selection,
  edit,
  editingIndex = null,
  onCancelEdit,
  onDirtyChange,
}: {
  result: QueryResult;
  ariaLabel?: string;
  emptyText?: string;
  /** Controlled server-side sort state (Table Data). Null when not sorting. */
  sortColumn?: string | null;
  sortDirection?: "asc" | "desc" | null;
  /** When provided, headers become sort controls (server-side only). */
  onSortColumn?: (column: string) => void;
  rowMenu?: {
    openIndex: number | null;
    onOpenChange: (rowIndex: number, open: boolean) => void;
    onRowContextMenu: (rowIndex: number) => void;
    renderItems: (row: unknown[]) => ReactNode;
  };
  selection?: GridSelectionControl;
  edit?: GridInlineEdit;
  /** Row currently in edit mode (null = none). */
  editingIndex?: number | null;
  onCancelEdit?: () => void;
  /** Notifies the parent whenever unsaved edits exist (for leave-guards). */
  onDirtyChange?: (dirty: boolean) => void;
}) {
  const rows = (result.rows ?? []) as Row[];
  const scrollRef = useRef<HTMLDivElement>(null);
  const hasRowMenu = Boolean(rowMenu);
  const hasSelection = Boolean(selection);
  const editable = Boolean(edit?.enabled);
  const prefixCount = (hasSelection ? 1 : 0) + (hasRowMenu ? 1 : 0);

  const columns = useMemo<ColumnDef<Row, unknown>[]>(() => {
    const base = (result.columns ?? []).map((column, index) => ({
      id: String(index),
      header: column.name ?? `column_${index}`,
      size: DEFAULT_COLUMN_WIDTH,
      minSize: MIN_COLUMN_WIDTH,
    }));
    const leading: ColumnDef<Row, unknown>[] = [];
    if (hasSelection) {
      leading.push({ id: "__select", header: "", size: LEADING_COLUMN_WIDTH, minSize: LEADING_COLUMN_WIDTH, enableResizing: false });
    }
    if (hasRowMenu) {
      leading.push({ id: "__actions", header: "", size: LEADING_COLUMN_WIDTH, minSize: LEADING_COLUMN_WIDTH, enableResizing: false });
    }
    return [...leading, ...base];
  }, [result.columns, hasRowMenu, hasSelection]);

  const table = useReactTable({
    data: rows,
    columns,
    getCoreRowModel: getCoreRowModel(),
    enableColumnResizing: true,
    columnResizeMode: "onChange",
    defaultColumn: { size: DEFAULT_COLUMN_WIDTH, minSize: MIN_COLUMN_WIDTH },
  });

  const headers = table.getFlatHeaders();
  const gridTemplateColumns = headers
    .map((header) => `${header.getSize()}px`)
    .join(" ");
  const totalWidth = headers.reduce((sum, header) => sum + header.getSize(), 0);

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: OVERSCAN,
    initialRect: { width: 800, height: FALLBACK_VIEWPORT_HEIGHT },
    observeElementRect: (instance, cb) => {
      const element = instance.scrollElement;
      cb({
        width: element?.clientWidth ?? 0,
        height: element?.clientHeight || FALLBACK_VIEWPORT_HEIGHT,
      });
      return () => {};
    },
  });
  const virtualRows = virtualizer.getVirtualItems();

  // --- Inline editing state (one row at a time) ------------------------------
  const [draft, setDraft] = useState<RowDraft>({});
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const savingRef = useRef(false);

  const editingRow =
    edit && editingIndex != null ? rows[editingIndex] : undefined;
  const originalRecord = useMemo(
    () => (edit && editingRow ? rowToRecord(edit.columns, editingRow) : {}),
    [edit, editingRow],
  );

  useEffect(() => {
    if (!edit || editingIndex == null) {
      setDraft({});
      setFieldErrors({});
      setSaveError(null);
      return;
    }
    setDraft(buildRowDraft(edit.columns, rowToRecord(edit.columns, rows[editingIndex] ?? [])));
    setFieldErrors({});
    setSaveError(null);
    // Reset only when the edited row changes; refetches must not wipe input.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editingIndex]);

  const currentChanges = useMemo(
    () => (edit && editingIndex != null ? draftChanges(edit.columns, draft, originalRecord) : {}),
    [edit, editingIndex, draft, originalRecord],
  );
  const dirty = Object.keys(currentChanges).length > 0;
  useEffect(() => {
    onDirtyChange?.(dirty);
  }, [dirty, onDirtyChange]);

  function patchField(name: string, patch: Partial<DraftField>) {
    setDraft((current) => ({ ...current, [name]: { ...current[name], ...patch } }));
    setFieldErrors((current) => {
      if (!(name in current)) return current;
      const next = { ...current };
      delete next[name];
      return next;
    });
  }

  async function saveEditing() {
    if (!edit || editingIndex == null || savingRef.current) return;
    const errors: Record<string, string> = {};
    for (const column of updatableColumns(edit.columns)) {
      const name = column.name ?? "";
      const field = draft[name];
      if (!field) continue;
      const message = validateRowField(column, field.mode, field.value);
      if (message) errors[name] = message;
    }
    setFieldErrors(errors);
    if (Object.keys(errors).length > 0) {
      setSaveError(Object.values(errors)[0] ?? "Check the highlighted fields.");
      return;
    }
    const changes = draftChanges(edit.columns, draft, originalRecord);
    if (Object.keys(changes).length === 0) {
      onCancelEdit?.();
      return;
    }
    const identity = Object.fromEntries(
      primaryKeyColumns(edit.columns).map((name) => [name, originalRecord[name]]),
    );
    const expected: Record<string, unknown> = {};
    for (const column of edit.columns) {
      const name = column.name ?? "";
      if (!name || column.primary_key) continue;
      expected[name] = originalRecord[name] ?? null;
    }
    savingRef.current = true;
    setSaving(true);
    setSaveError(null);
    try {
      await edit.onSave(editingIndex, { identity, expected, changes });
      onCancelEdit?.();
    } catch (cause) {
      setSaveError(mutationErrorMessage(cause, "Update failed."));
    } finally {
      savingRef.current = false;
      setSaving(false);
    }
  }

  const allSelected =
    hasSelection && rows.length > 0 && rows.every((_, index) => selection?.selected.has(index));
  const someSelected =
    hasSelection && rows.some((_, index) => selection?.selected.has(index));

  return (
    <div
      ref={scrollRef}
      role="grid"
      aria-label={ariaLabel}
      aria-rowcount={rows.length}
      aria-colcount={headers.length}
      className="h-full overflow-auto"
    >
      <div className="relative" style={{ width: totalWidth, minWidth: "100%" }}>
        <div
          role="row"
          className="sticky top-0 z-10 grid border-b border-border bg-panel"
          style={{ gridTemplateColumns }}
        >
          {headers.map((header) => {
            if (header.id === "__select") {
              return (
                <div
                  key={header.id}
                  role="columnheader"
                  aria-label="Row selection"
                  className="flex items-center justify-center border-r border-border"
                  style={{ width: header.getSize() }}
                >
                  <HeaderCheckbox
                    checked={allSelected}
                    indeterminate={someSelected && !allSelected}
                    onChange={(checked) => selection?.onToggleAll(checked)}
                  />
                </div>
              );
            }
            if (header.id === "__actions") {
              return (
                <div
                  key={header.id}
                  role="columnheader"
                  aria-label="Row actions"
                  className="border-r border-border"
                  style={{ width: header.getSize() }}
                />
              );
            }
            const label = String(header.column.columnDef.header);
            const sortable = Boolean(onSortColumn);
            const active = sortable && sortColumn === label;
            const ariaSort = !sortable
              ? undefined
              : active
                ? sortDirection === "asc"
                  ? "ascending"
                  : "descending"
                : "none";
            return (
              <div
                key={header.id}
                role="columnheader"
                aria-sort={ariaSort}
                className="relative flex items-center border-r border-border px-2 py-1.5 text-xs font-medium text-muted-foreground"
                style={{ width: header.getSize() }}
              >
                {sortable ? (
                  <button
                    type="button"
                    aria-label={`Sort by ${label}`}
                    onClick={() => onSortColumn?.(label)}
                    className="group flex min-w-0 cursor-pointer items-center gap-1 rounded text-left hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
                  >
                    <span className="truncate">{label}</span>
                    {active ? (
                      sortDirection === "asc" ? (
                        <ArrowUp size={11} aria-hidden="true" />
                      ) : (
                        <ArrowDown size={11} aria-hidden="true" />
                      )
                    ) : (
                      <ArrowUpDown
                        size={11}
                        aria-hidden="true"
                        className="shrink-0 opacity-40 transition-opacity group-hover:opacity-100"
                      />
                    )}
                  </button>
                ) : (
                  <span className="truncate">{label}</span>
                )}
                <div
                  aria-hidden="true"
                  onMouseDown={header.getResizeHandler()}
                  onTouchStart={header.getResizeHandler()}
                  className={cn(
                    "absolute right-0 top-0 h-full w-1 cursor-col-resize touch-none select-none hover:bg-accent/50",
                    header.column.getIsResizing() && "bg-accent",
                  )}
                />
              </div>
            );
          })}
        </div>

        {rows.length === 0 ? (
          <div className="px-3 py-6 text-center text-xs text-muted-foreground">
            {emptyText}
          </div>
        ) : (
          <div className="relative" style={{ height: virtualizer.getTotalSize() }}>
            {virtualRows.map((virtualRow) => {
              const row = rows[virtualRow.index];
              const isEditing = editable && editingIndex === virtualRow.index;
              const isSelected = Boolean(selection?.selected.has(virtualRow.index));
              return (
                <div
                  key={virtualRow.key}
                  role="row"
                  aria-selected={hasSelection ? isSelected : undefined}
                  onContextMenu={
                    rowMenu
                      ? (event) => {
                          event.preventDefault();
                          rowMenu.onRowContextMenu(virtualRow.index);
                        }
                      : undefined
                  }
                  onKeyDown={
                    isEditing
                      ? (event) => {
                          if (event.key === "Escape") {
                            event.preventDefault();
                            onCancelEdit?.();
                          }
                        }
                      : undefined
                  }
                  className={cn(
                    "absolute left-0 top-0 grid border-b border-border hover:bg-panel-raised/60",
                    isSelected && "bg-accent/10",
                    isEditing && "bg-accent/10 ring-1 ring-inset ring-[var(--color-focus-ring)]",
                  )}
                  style={{
                    gridTemplateColumns,
                    width: totalWidth,
                    height: ROW_HEIGHT,
                    transform: `translateY(${virtualRow.start}px)`,
                  }}
                >
                  {headers.map((header, index) => {
                    if (header.id === "__select") {
                      return (
                        <div
                          key={header.id}
                          role="gridcell"
                          className="flex items-center justify-center border-r border-border"
                        >
                          <input
                            type="checkbox"
                            aria-label={`Select row ${virtualRow.index + 1}`}
                            checked={isSelected}
                            onChange={() => selection?.onToggleRow(virtualRow.index)}
                            className="h-3.5 w-3.5 accent-[var(--color-focus-ring)]"
                          />
                        </div>
                      );
                    }
                    if (header.id === "__actions") {
                      return (
                        <div
                          key={header.id}
                          role="gridcell"
                          className="flex items-center justify-center gap-0.5 border-r border-border"
                        >
                          {isEditing ? (
                            <>
                              <button
                                type="button"
                                aria-label="Save row"
                                title="Save (Enter)"
                                data-testid="inline-save"
                                disabled={saving || !dirty}
                                onClick={() => void saveEditing()}
                                className="rounded p-0.5 text-emerald-500 hover:bg-accent disabled:opacity-40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
                              >
                                <Check size={13} aria-hidden="true" />
                              </button>
                              <button
                                type="button"
                                aria-label="Cancel editing"
                                title="Cancel (Esc)"
                                data-testid="inline-cancel"
                                disabled={saving}
                                onClick={() => onCancelEdit?.()}
                                className="rounded p-0.5 text-muted-foreground hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
                              >
                                <X size={13} aria-hidden="true" />
                              </button>
                            </>
                          ) : rowMenu ? (
                            <DropdownMenu
                              open={rowMenu.openIndex === virtualRow.index}
                              onOpenChange={(open) =>
                                rowMenu.onOpenChange(virtualRow.index, open)
                              }
                            >
                              <DropdownMenuTrigger
                                aria-label={`Row actions, row ${virtualRow.index + 1}`}
                                data-testid={`row-menu-trigger-${virtualRow.index + 1}`}
                                className="rounded px-1.5 text-sm leading-none text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
                              >
                                ⋯
                              </DropdownMenuTrigger>
                              <DropdownMenuContent align="start">
                                {row ? rowMenu.renderItems(row) : null}
                              </DropdownMenuContent>
                            </DropdownMenu>
                          ) : null}
                        </div>
                      );
                    }
                    const dataIndex = index - prefixCount;
                    const columnMeta = edit?.columns[dataIndex];
                    if (isEditing && columnMeta?.updatable) {
                      return (
                        <div
                          key={header.id}
                          role="gridcell"
                          className="flex items-center gap-1 overflow-hidden border-r border-border px-1"
                        >
                          <InlineEditor
                            column={columnMeta}
                            field={draft[columnMeta.name ?? ""] ?? { mode: "value", value: "" }}
                            error={fieldErrors[columnMeta.name ?? ""]}
                            saving={saving}
                            onChange={(patch) => patchField(columnMeta.name ?? "", patch)}
                            onSave={() => void saveEditing()}
                            onCancel={() => onCancelEdit?.()}
                          />
                        </div>
                      );
                    }
                    return (
                      <ResultCell
                        key={header.id}
                        value={row?.[dataIndex]}
                        readOnly={isEditing}
                      />
                    );
                  })}
                </div>
              );
            })}
          </div>
        )}
      </div>

      {saveError ? (
        <div
          role="alert"
          data-testid="inline-save-error"
          className="sticky bottom-0 border-t border-destructive/40 bg-destructive/10 px-3 py-1 text-xs text-destructive"
        >
          {saveError}
        </div>
      ) : null}
    </div>
  );
}

function HeaderCheckbox({
  checked,
  indeterminate,
  onChange,
}: {
  checked: boolean;
  indeterminate: boolean;
  onChange: (checked: boolean) => void;
}) {
  const ref = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (ref.current) ref.current.indeterminate = indeterminate;
  }, [indeterminate]);
  return (
    <input
      ref={ref}
      type="checkbox"
      aria-label="Select all rows on this page"
      checked={checked}
      onChange={(event) => onChange(event.target.checked)}
      className="h-3.5 w-3.5 accent-[var(--color-focus-ring)]"
    />
  );
}

function InlineEditor({
  column,
  field,
  error,
  saving,
  onChange,
  onSave,
  onCancel,
}: {
  column: TableColumnInfo;
  field: DraftField;
  error?: string;
  saving: boolean;
  onChange: (patch: Partial<DraftField>) => void;
  onSave: () => void;
  onCancel: () => void;
}) {
  const name = column.name ?? "";
  const category = columnCategory(column.database_type);
  const inputClass =
    "min-w-0 flex-1 rounded border border-border bg-background px-1 py-0.5 text-xs text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]";

  function handleKeyDown(event: ReactKeyboardEvent) {
    if (event.key === "Enter") {
      event.preventDefault();
      onSave();
    } else if (event.key === "Escape") {
      event.preventDefault();
      onCancel();
    }
  }

  return (
    <>
      {column.nullable ? (
        <button
          type="button"
          aria-label={`Toggle NULL for ${name}`}
          aria-pressed={field.mode === "null"}
          title="Toggle NULL"
          disabled={saving}
          onClick={() =>
            onChange({ mode: field.mode === "null" ? "value" : "null" })
          }
          className={cn(
            "shrink-0 rounded border border-border px-1 text-[9px] font-semibold uppercase",
            field.mode === "null"
              ? "bg-foreground text-background"
              : "text-muted-foreground hover:text-foreground",
          )}
        >
          ∅
        </button>
      ) : null}
      {field.mode === "null" ? (
        <span className="flex-1 text-xs italic text-subtle-foreground">NULL</span>
      ) : category === "boolean" ? (
        <select
          aria-label={`${name} value`}
          aria-invalid={error ? true : undefined}
          value={field.value}
          disabled={saving}
          onChange={(event) => onChange({ value: event.target.value })}
          onKeyDown={handleKeyDown}
          className={inputClass}
        >
          <option value="true">true</option>
          <option value="false">false</option>
        </select>
      ) : (
        <input
          aria-label={`${name} value`}
          aria-invalid={error ? true : undefined}
          title={error}
          value={field.value}
          disabled={saving}
          type={category === "date" ? "date" : "text"}
          inputMode={category === "integer" || category === "decimal" ? "numeric" : undefined}
          readOnly={category === "binary"}
          onChange={(event) => onChange({ value: event.target.value })}
          onKeyDown={handleKeyDown}
          className={cn(inputClass, error && "border-destructive")}
        />
      )}
    </>
  );
}

function ResultCell({ value, readOnly }: { value: unknown; readOnly?: boolean }) {
  const [copied, setCopied] = useState(false);
  const nullValue = isNullValue(value);
  const text = formatCellValue(value);

  function copy() {
    if (readOnly) return;
    void copyText(rawCellValue(value)).then((ok) => {
      if (ok) {
        setCopied(true);
        window.setTimeout(() => setCopied(false), 1000);
      }
    });
  }

  return (
    <div
      role="gridcell"
      tabIndex={readOnly ? -1 : 0}
      onClick={copy}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          copy();
        }
      }}
      title={text.length <= 200 ? text : undefined}
      className={cn(
        "flex items-center overflow-hidden whitespace-nowrap border-r border-border px-2 text-xs text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]",
        nullValue && "italic text-subtle-foreground",
        copied && "bg-accent/20",
        readOnly && "cursor-default opacity-70",
      )}
    >
      <span className="truncate">{copied ? "Copied" : text}</span>
    </div>
  );
}
