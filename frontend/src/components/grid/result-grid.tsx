"use client";

import {
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table";
import { useVirtualizer } from "@tanstack/react-virtual";
import { ArrowDown, ArrowUp } from "lucide-react";
import { useMemo, useRef, useState, type ReactNode } from "react";

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
import { cn } from "@/lib/utils";
import type { QueryResult } from "@/types/api";

const ROW_HEIGHT = 32;
const DEFAULT_COLUMN_WIDTH = 180;
const MIN_COLUMN_WIDTH = 80;
const OVERSCAN = 10;
/** Used when the scroll element has no measured height yet (first paint / jsdom). */
const FALLBACK_VIEWPORT_HEIGHT = 400;

type Row = unknown[];

/**
 * Virtualized result grid. TanStack Table owns the column model (headers,
 * sizing, resize); TanStack Virtual owns row virtualization. Rows stay as
 * positional arrays — they are never expanded into per-row objects, and only
 * the visible rows (+ overscan) are mounted.
 */
export function ResultGrid({
  result,
  ariaLabel = "Query result grid",
  emptyText = "No rows returned.",
  sortColumn = null,
  sortDirection = null,
  onSortColumn,
  rowMenu,
}: {
  result: QueryResult;
  ariaLabel?: string;
  emptyText?: string;
  /** Controlled server-side sort state (Table Data). Null when not sorting. */
  sortColumn?: string | null;
  sortDirection?: "asc" | "desc" | null;
  /** When provided, headers become sort controls (server-side only). */
  onSortColumn?: (column: string) => void;
  /**
   * When provided, each row gets a compact menu trigger and right-click opens
   * the same menu (PRF-02/T11). The parent owns which row's menu is open and
   * renders the items; the grid only wires the trigger/context target.
   */
  rowMenu?: {
    openIndex: number | null;
    onOpenChange: (rowIndex: number, open: boolean) => void;
    onRowContextMenu: (rowIndex: number) => void;
    renderItems: (row: unknown[]) => ReactNode;
  };
}) {
  const rows = (result.rows ?? []) as Row[];
  const scrollRef = useRef<HTMLDivElement>(null);
  const hasRowMenu = Boolean(rowMenu);

  const columns = useMemo<ColumnDef<Row, unknown>[]>(() => {
    const base = (result.columns ?? []).map((column, index) => ({
      id: String(index),
      header: column.name ?? `column_${index}`,
      size: DEFAULT_COLUMN_WIDTH,
      minSize: MIN_COLUMN_WIDTH,
    }));
    if (!hasRowMenu) return base;
    return [
      { id: "__actions", header: "", size: 40, minSize: 40, enableResizing: false },
      ...base,
    ];
  }, [result.columns, hasRowMenu]);

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
    // Measure the real viewport; fall back to a sane height before layout is
    // available so the first render still mounts the visible window.
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
                    className="flex min-w-0 items-center gap-1 rounded text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
                  >
                    <span className="truncate">{label}</span>
                    {active ? (
                      sortDirection === "asc" ? (
                        <ArrowUp size={11} aria-hidden="true" />
                      ) : (
                        <ArrowDown size={11} aria-hidden="true" />
                      )
                    ) : null}
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
          <div
            className="relative"
            style={{ height: virtualizer.getTotalSize() }}
          >
            {virtualRows.map((virtualRow) => {
              const row = rows[virtualRow.index];
              return (
                <div
                  key={virtualRow.key}
                  role="row"
                  onContextMenu={
                    rowMenu
                      ? (event) => {
                          event.preventDefault();
                          rowMenu.onRowContextMenu(virtualRow.index);
                        }
                      : undefined
                  }
                  className="absolute left-0 top-0 grid border-b border-border hover:bg-panel-raised/60"
                  style={{
                    gridTemplateColumns,
                    width: totalWidth,
                    height: ROW_HEIGHT,
                    transform: `translateY(${virtualRow.start}px)`,
                  }}
                >
                  {headers.map((header, index) =>
                    header.id === "__actions" ? (
                      <div
                        key={header.id}
                        role="gridcell"
                        className="flex items-center justify-center border-r border-border"
                      >
                        <DropdownMenu
                          open={rowMenu?.openIndex === virtualRow.index}
                          onOpenChange={(open) =>
                            rowMenu?.onOpenChange(virtualRow.index, open)
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
                            {row ? rowMenu?.renderItems(row) : null}
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </div>
                    ) : (
                      <ResultCell key={header.id} value={row?.[index - (hasRowMenu ? 1 : 0)]} />
                    ),
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

function ResultCell({ value }: { value: unknown }) {
  const [copied, setCopied] = useState(false);
  const nullValue = isNullValue(value);
  const text = formatCellValue(value);

  function copy() {
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
      tabIndex={0}
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
      )}
    >
      <span className="truncate">{copied ? "Copied" : text}</span>
    </div>
  );
}
