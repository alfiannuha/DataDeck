"use client";

import { Filter, Loader2, RefreshCw } from "lucide-react";
import { useState } from "react";

import { ResultGrid } from "@/components/grid/result-grid";
import { AddRowDialog } from "@/components/shared/add-row-dialog";
import { TableDataFilterPanel } from "@/components/shared/table-data-filter-panel";
import { Button } from "@/components/ui/button";
import { useTableData } from "@/hooks/use-table-data";
import { ApiClientError } from "@/lib/api-client";
import { useWorkspaceStore, type TableFilter, type TableSort } from "@/store/useWorkspaceStore";
import type { QueryResult } from "@/types/api";

/**
 * Table Data browse surface for one Table Data tab (PRF-02/T04).
 *
 * The tab owns its connection/database/schema/table/page binding; this component
 * only reads that binding and never consults the Explorer's global selection.
 * It renders a bounded page through the structured T03 API (no SQL), and stays
 * read-only (sorting/filtering/CRUD arrive in later tasks).
 */
export function TableDataView() {
  const tab = useWorkspaceStore((state) =>
    state.tabs.find(
      (candidate) =>
        candidate.id === state.activeTabId && candidate.kind === "table-data",
    ),
  );
  const updateTableData = useWorkspaceStore((state) => state.updateTableData);
  const [filterOpen, setFilterOpen] = useState(false);
  const [addRowOpen, setAddRowOpen] = useState(false);

  const target = {
    connectionId: tab?.connectionId ?? null,
    database: tab?.database ?? null,
    schema: tab?.kind === "table-data" ? tab.schema : null,
    table: tab?.kind === "table-data" ? tab.table : "",
    page: tab?.kind === "table-data" ? tab.page : 1,
    pageSize: tab?.kind === "table-data" ? tab.pageSize : 100,
    sort:
      tab?.kind === "table-data" && tab.sort.length > 0
        ? { column: tab.sort[0].column, direction: tab.sort[0].direction }
        : null,
    filters: tab?.kind === "table-data" ? tab.filters : [],
  };
  const query = useTableData(target);
  const data = query.data;

  if (tab?.kind !== "table-data") return null;

  const qualified = tab.schema ? `${tab.schema}.${tab.table}` : tab.table;
  const error = query.error;
  const isApiError = error instanceof ApiClientError;
  const showFirstLoad = query.isLoading && !data;
  const showError = query.isError && !data;

  function refresh() {
    void query.refetch();
  }

  function changePage(next: number) {
    if (!tab || tab.kind !== "table-data") return;
    updateTableData(tab.id, { page: Math.max(1, next) });
  }

  function changePageSize(next: number) {
    if (!tab || tab.kind !== "table-data") return;
    updateTableData(tab.id, { pageSize: next, page: 1 });
  }

  // Cycle none → ASC → DESC → none (single column, per ADR-010 §8). Sorting is
  // server-side; the page resets so the new order starts from the top.
  function handleSort(column: string) {
    if (!tab || tab.kind !== "table-data") return;
    const current = tab.sort[0];
    let sort: TableSort[] = [];
    if (!current || current.column !== column) {
      sort = [{ column, direction: "asc" }];
    } else if (current.direction === "asc") {
      sort = [{ column, direction: "desc" }];
    }
    updateTableData(tab.id, { sort, page: 1 });
  }

  // Applying/clearing filters resets to page 1 and keeps sort/pageSize intact.
  function applyFilters(filters: TableFilter[]) {
    if (!tab || tab.kind !== "table-data") return;
    updateTableData(tab.id, { filters, page: 1 });
  }

  function clearFilters() {
    if (!tab || tab.kind !== "table-data") return;
    updateTableData(tab.id, { filters: [], page: 1 });
  }

  // Adapt the structured Table Data page to the shared virtualized grid shape.
  const gridResult: QueryResult | null = data
    ? {
        columns: (data.columns ?? []).map((column) => ({
          name: column.name,
          type: column.database_type,
        })),
        rows: data.rows ?? [],
        rows_affected: (data.rows ?? []).length,
        execution_time_ms: 0,
        truncated: data.truncated ?? false,
      }
    : null;

  return (
    <section
      aria-label="Table data"
      data-testid="table-data-view"
      className="flex min-h-0 flex-1 flex-col overflow-hidden"
    >
      <header className="flex h-9 shrink-0 items-center justify-between gap-3 border-b border-border bg-panel px-3">
        <div className="flex min-w-0 items-center gap-2 text-xs">
          <span className="truncate font-medium text-foreground">{qualified}</span>
          <span aria-hidden="true" className="text-muted-foreground">
            ·
          </span>
          <span
            data-testid="table-data-binding"
            className="truncate text-muted-foreground"
          >
            {tab.database ?? "default database"}
          </span>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <label className="flex items-center gap-1 text-[10px] text-muted-foreground">
            Rows
            <select
              aria-label="Page size"
              data-testid="table-data-page-size"
              value={tab.pageSize}
              onChange={(event) => changePageSize(Number(event.target.value))}
              className="rounded border border-border bg-background px-1 py-0.5 text-[10px] text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
            >
              {[50, 100, 200].map((size) => (
                <option key={size} value={size}>
                  {size}
                </option>
              ))}
            </select>
          </label>
          {data?.row_capabilities?.insert ? (
            <Button
              variant="outline"
              size="sm"
              data-testid="table-data-add-row"
              aria-label="Add row"
              onClick={() => setAddRowOpen(true)}
            >
              Add Row
            </Button>
          ) : null}
          <Button
            variant={filterOpen || tab.filters.length > 0 ? "outline" : "ghost"}
            size="sm"
            data-testid="table-data-filter-toggle"
            aria-label="Filter table data"
            aria-expanded={filterOpen}
            disabled={!data}
            onClick={() => setFilterOpen((open) => !open)}
          >
            <Filter size={12} aria-hidden="true" />
            Filter
            {tab.filters.length > 0 ? ` (${tab.filters.length})` : ""}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            data-testid="table-data-refresh"
            aria-label="Refresh table data"
            onClick={refresh}
            disabled={query.isFetching}
          >
            {query.isFetching ? (
              <Loader2 size={12} aria-hidden="true" className="animate-spin" />
            ) : (
              <RefreshCw size={12} aria-hidden="true" />
            )}
            Refresh
          </Button>
        </div>
      </header>

      {filterOpen && data ? (
        <TableDataFilterPanel
          columns={data.columns ?? []}
          applied={tab.filters}
          onApply={applyFilters}
          onClear={clearFilters}
          onClose={() => setFilterOpen(false)}
        />
      ) : null}

      {addRowOpen && data && tab.connectionId ? (
        <AddRowDialog
          connectionId={tab.connectionId}
          database={tab.database}
          schema={tab.schema}
          table={tab.table}
          columns={data.columns ?? []}
          onClose={() => setAddRowOpen(false)}
          onInserted={() => {
            setAddRowOpen(false);
            void query.refetch();
          }}
        />
      ) : null}

      <div className="min-h-0 flex-1 overflow-hidden bg-result-surface">
        {showError ? (
          <div
            role="alert"
            data-testid="table-data-error"
            className="flex h-full flex-col items-center justify-center gap-2 p-6 text-center"
          >
            <p className="text-sm font-medium text-foreground">
              Unable to load table data.
            </p>
            <p className="text-xs text-destructive">
              {isApiError ? `${error.message} (${error.code})` : "Unexpected error."}
            </p>
            <Button variant="outline" size="sm" onClick={refresh}>
              Retry
            </Button>
          </div>
        ) : showFirstLoad ? (
          <div
            data-testid="table-data-loading"
            role="status"
            className="flex h-full items-center justify-center gap-2 text-xs text-muted-foreground"
          >
            <Loader2 size={14} aria-hidden="true" className="animate-spin" />
            Loading table data…
          </div>
        ) : gridResult ? (
          <ResultGrid
            result={gridResult}
            ariaLabel="Table data grid"
            emptyText="No rows found."
            sortColumn={tab.sort[0]?.column ?? null}
            sortDirection={tab.sort[0]?.direction ?? null}
            onSortColumn={handleSort}
          />
        ) : null}
      </div>

      <footer className="flex h-8 shrink-0 items-center justify-between border-t border-border bg-panel px-3 text-[10px] text-muted-foreground">
        <span data-testid="table-data-page">
          Page {tab.page}
          {data?.pagination?.has_more ? " · more rows available" : ""}
          {data?.truncated ? " · partial (50 MB limit)" : ""}
        </span>
        <div className="flex items-center gap-1">
          <Button
            variant="outline"
            size="sm"
            data-testid="table-data-prev"
            aria-label="Previous page"
            disabled={tab.page <= 1 || query.isFetching}
            onClick={() => changePage(tab.page - 1)}
          >
            Previous
          </Button>
          <Button
            variant="outline"
            size="sm"
            data-testid="table-data-next"
            aria-label="Next page"
            disabled={!data?.pagination?.has_more || query.isFetching}
            onClick={() => changePage(tab.page + 1)}
          >
            Next
          </Button>
        </div>
      </footer>
    </section>
  );
}
