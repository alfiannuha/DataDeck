"use client";

import {
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useMemo, useRef, useState } from "react";

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
export function ResultGrid({ result }: { result: QueryResult }) {
  const rows = (result.rows ?? []) as Row[];
  const scrollRef = useRef<HTMLDivElement>(null);

  const columns = useMemo<ColumnDef<Row, unknown>[]>(
    () =>
      (result.columns ?? []).map((column, index) => ({
        id: String(index),
        header: column.name ?? `column_${index}`,
        size: DEFAULT_COLUMN_WIDTH,
        minSize: MIN_COLUMN_WIDTH,
      })),
    [result.columns],
  );

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
      aria-label="Query result grid"
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
          {headers.map((header) => (
            <div
              key={header.id}
              role="columnheader"
              className="relative flex items-center border-r border-border px-2 py-1.5 text-xs font-medium text-muted-foreground"
              style={{ width: header.getSize() }}
            >
              <span className="truncate">
                {String(header.column.columnDef.header)}
              </span>
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
          ))}
        </div>

        {rows.length === 0 ? (
          <div className="px-3 py-6 text-center text-xs text-muted-foreground">
            No rows returned.
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
                  className="absolute left-0 top-0 grid border-b border-border hover:bg-panel-raised/60"
                  style={{
                    gridTemplateColumns,
                    width: totalWidth,
                    height: ROW_HEIGHT,
                    transform: `translateY(${virtualRow.start}px)`,
                  }}
                >
                  {headers.map((header, index) => (
                    <ResultCell key={header.id} value={row?.[index]} />
                  ))}
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
