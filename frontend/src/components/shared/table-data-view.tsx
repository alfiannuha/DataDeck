"use client";

import { Download, Filter, Loader2, RefreshCw } from "lucide-react";
import { useEffect, useRef, useState, type ReactNode } from "react";

import { ResultGrid } from "@/components/grid/result-grid";
import { AddRowDialog } from "@/components/shared/add-row-dialog";
import { DeleteRowDialog } from "@/components/shared/delete-row-dialog";
import { EditRowDialog } from "@/components/shared/edit-row-dialog";
import { TableDataFilterPanel } from "@/components/shared/table-data-filter-panel";
import { Button } from "@/components/ui/button";
import {
  DropdownMenuHint,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import { useTableData } from "@/hooks/use-table-data";
import { copyText } from "@/lib/clipboard";
import { ApiClientError } from "@/lib/api-client";
import { isReadOnlyObject } from "@/lib/table-actions";
import { rowToJSON, rowToTSV } from "@/lib/row-copy";
import { useExportStore } from "@/store/useExportStore";
import {
  useWorkspaceStore,
  type TableFilter,
  type TableSort,
} from "@/store/useWorkspaceStore";
import type { QueryResult, TableColumnInfo } from "@/types/api";

/** The immutable target a mutation dialog was opened with (never re-read live). */
interface DialogTarget {
  connectionId: string;
  database: string | null;
  schema: string | null;
  table: string;
}

type DialogState =
  | { kind: "add"; target: DialogTarget; columns: TableColumnInfo[] }
  | {
      kind: "duplicate";
      target: DialogTarget;
      columns: TableColumnInfo[];
      prefill: Record<string, { mode: "default" | "value" | "null"; value?: string }>;
    }
  | { kind: "edit"; target: DialogTarget; columns: TableColumnInfo[]; row: unknown[] }
  | {
      kind: "delete";
      target: DialogTarget;
      identity: Record<string, unknown>;
      expected: Record<string, unknown>;
    };

/**
 * Table Data browse surface for one Table Data tab (PRF-02/T04 + T11).
 *
 * The tab owns its connection/database/schema/table/page binding; this component
 * only reads that binding and never consults the Explorer's global selection.
 * Mutation dialogs capture their target when opened, so switching tabs or the
 * Explorer selection can never retarget an in-flight action (T11 §21).
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
  const [dialog, setDialog] = useState<DialogState | null>(null);
  const [menuOpenIndex, setMenuOpenIndex] = useState<number | null>(null);
  const [status, setStatus] = useState<string | null>(null);
  const statusTimer = useRef<number | null>(null);

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

  useEffect(
    () => () => {
      if (statusTimer.current !== null) window.clearTimeout(statusTimer.current);
    },
    [],
  );

  if (tab?.kind !== "table-data") return null;

  const qualified = tab.schema ? `${tab.schema}.${tab.table}` : tab.table;
  const error = query.error;
  const isApiError = error instanceof ApiClientError;
  const showFirstLoad = query.isLoading && !data;
  const showError = query.isError && !data;

  function flashStatus(message: string) {
    setStatus(message);
    if (statusTimer.current !== null) window.clearTimeout(statusTimer.current);
    statusTimer.current = window.setTimeout(() => setStatus(null), 2500);
  }

  function refresh() {
    void query.refetch();
  }

  function closeDialogAndRefresh() {
    setDialog(null);
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

  // Snapshot the tab binding at the moment a dialog opens (T11 §21): later
  // tab/Explorer switches can never change the mutation target.
  function dialogTarget(): DialogTarget | null {
    if (!tab || tab.kind !== "table-data" || !tab.connectionId) return null;
    return {
      connectionId: tab.connectionId,
      database: tab.database,
      schema: tab.schema,
      table: tab.table,
    };
  }

  const columns = data?.columns ?? [];

  // Identity = PK values from the snapshot; expected = every non-PK column, so
  // the backend's optimistic concurrency compares the whole row (PRF-02).
  function rowIdentity(row: unknown[]): Record<string, unknown> {
    return Object.fromEntries(
      columns
        .map((column, index) => [column, row[index]] as const)
        .filter(([column]) => column.primary_key)
        .map(([column, value]) => [column.name ?? "", value]),
    );
  }

  function rowExpected(row: unknown[]): Record<string, unknown> {
    return Object.fromEntries(
      columns
        .map((column, index) => [column, row[index]] as const)
        .filter(([column]) => !column.primary_key)
        .map(([column, value]) => [column.name ?? "", value ?? null]),
    );
  }

  // Duplicate prefill: PKs and generated/identity columns are skipped (DB
  // assigns them); normal values are copied; NULL stays NULL when nullable.
  function duplicatePrefill(row: unknown[]): Record<
    string,
    { mode: "default" | "value" | "null"; value?: string }
  > {
    const prefill: Record<string, { mode: "default" | "value" | "null"; value?: string }> = {};
    for (const [index, column] of columns.entries()) {
      const name = column.name ?? "";
      if (!name || column.insertable === false || column.primary_key) continue;
      const value = row[index];
      if (value === null || value === undefined) {
        prefill[name] = { mode: column.nullable ? "null" : "default" };
        continue;
      }
      prefill[name] = { mode: "value", value: String(value) };
    }
    return prefill;
  }

  async function copyRow(row: unknown[], kind: "tsv" | "json") {
    setMenuOpenIndex(null);
    const text = kind === "tsv" ? rowToTSV(columns, row) : rowToJSON(columns, row);
    const ok = await copyText(text);
    flashStatus(
      !ok
        ? "Copy failed — clipboard unavailable."
        : kind === "tsv"
          ? "Row copied."
          : "Row copied as JSON.",
    );
  }

  // The row context menu (T11 §2): capability-gated actions with explanations
  // on unavailable items; Copy/Copy as JSON are always available and never
  // mutate. Disabled items stay focusable (aria-disabled) so the reason is
  // announced (T11 §10).
  function rowMenuItems(row: unknown[]): ReactNode {
    const capabilities = data?.row_capabilities;
    const readOnlyObject = isReadOnlyObject(data?.object_type);
    const hasIdentity = (data?.row_identity?.columns?.length ?? 0) > 0;
    const editReason = readOnlyObject
      ? "This object is read-only."
      : !hasIdentity
        ? "No primary key — row editing is unavailable."
        : "Row editing is unavailable for this table.";
    const deleteReason = readOnlyObject
      ? "This object is read-only."
      : !hasIdentity
        ? "No primary key — row deletion is unavailable."
        : "Row deletion is unavailable for this table.";
    const duplicateReason = readOnlyObject
      ? "This object is read-only."
      : "Duplication is unavailable for this table.";

    return (
      <>
        <DropdownMenuItem
          disabled={!capabilities?.update}
          data-testid="row-menu-edit"
          onSelect={() => {
            setMenuOpenIndex(null);
            const target = dialogTarget();
            if (target) setDialog({ kind: "edit", target, columns, row });
          }}
        >
          Edit Row
          {!capabilities?.update ? <DropdownMenuHint>{editReason}</DropdownMenuHint> : null}
        </DropdownMenuItem>
        <DropdownMenuItem
          disabled={!capabilities?.duplicate}
          data-testid="row-menu-duplicate"
          onSelect={() => {
            setMenuOpenIndex(null);
            const target = dialogTarget();
            if (target) {
              setDialog({ kind: "duplicate", target, columns, prefill: duplicatePrefill(row) });
            }
          }}
        >
          Duplicate Row
          {!capabilities?.duplicate ? (
            <DropdownMenuHint>{duplicateReason}</DropdownMenuHint>
          ) : null}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          data-testid="row-menu-copy"
          onSelect={() => void copyRow(row, "tsv")}
        >
          Copy
        </DropdownMenuItem>
        <DropdownMenuItem
          data-testid="row-menu-copy-json"
          onSelect={() => void copyRow(row, "json")}
        >
          Copy as JSON
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          disabled={!capabilities?.delete}
          data-testid="row-menu-delete"
          onSelect={() => {
            setMenuOpenIndex(null);
            const target = dialogTarget();
            const identity = rowIdentity(row);
            if (target && Object.keys(identity).length > 0) {
              setDialog({ kind: "delete", target, identity, expected: rowExpected(row) });
            }
          }}
        >
          Delete Row
          {!capabilities?.delete ? <DropdownMenuHint>{deleteReason}</DropdownMenuHint> : null}
        </DropdownMenuItem>
      </>
    );
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

  const addRowDisabledReason = data
    ? data.row_capabilities?.insert
      ? null
      : isReadOnlyObject(data.object_type)
        ? "This object is read-only."
        : "This table does not support row insertion."
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
          {status ? (
            <span
              data-testid="table-data-status"
              aria-live="polite"
              className="truncate text-[10px] text-muted-foreground"
            >
              {status}
            </span>
          ) : null}
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
          {data ? (
            <Button
              variant="outline"
              size="sm"
              data-testid="table-data-add-row"
              aria-label="Add row"
              disabled={Boolean(addRowDisabledReason)}
              title={addRowDisabledReason ?? undefined}
              onClick={() => {
                const target = dialogTarget();
                if (target) setDialog({ kind: "add", target, columns });
              }}
            >
              Add Row
            </Button>
          ) : null}
          <Button
            variant="ghost"
            size="sm"
            data-testid="table-data-export"
            aria-label="Export table data"
            disabled={!data || !tab.connectionId}
            onClick={() => {
              if (!tab.connectionId) return;
              useExportStore.getState().open({
                connectionId: tab.connectionId,
                database: tab.database,
                schema: tab.schema,
                table: tab.table,
                objectType: data?.object_type,
                filters: tab.filters,
                sort: target.sort,
              });
            }}
          >
            <Download size={12} aria-hidden="true" />
            Export
          </Button>
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

      {dialog?.kind === "add" ? (
        <AddRowDialog
          connectionId={dialog.target.connectionId}
          database={dialog.target.database}
          schema={dialog.target.schema}
          table={dialog.target.table}
          columns={dialog.columns}
          onClose={() => setDialog(null)}
          onInserted={closeDialogAndRefresh}
        />
      ) : null}

      {dialog?.kind === "duplicate" ? (
        <AddRowDialog
          connectionId={dialog.target.connectionId}
          database={dialog.target.database}
          schema={dialog.target.schema}
          table={dialog.target.table}
          columns={dialog.columns}
          initialValues={dialog.prefill}
          title={`Duplicate Row — ${dialog.target.table}`}
          onClose={() => setDialog(null)}
          onInserted={closeDialogAndRefresh}
        />
      ) : null}

      {dialog?.kind === "edit" ? (
        <EditRowDialog
          connectionId={dialog.target.connectionId}
          database={dialog.target.database}
          schema={dialog.target.schema}
          table={dialog.target.table}
          columns={dialog.columns}
          row={dialog.row}
          onClose={() => setDialog(null)}
          onUpdated={closeDialogAndRefresh}
        />
      ) : null}

      {dialog?.kind === "delete" ? (
        <DeleteRowDialog
          connectionId={dialog.target.connectionId}
          database={dialog.target.database}
          schema={dialog.target.schema}
          table={dialog.target.table}
          identity={dialog.identity}
          expected={dialog.expected}
          onClose={() => setDialog(null)}
          onDeleted={closeDialogAndRefresh}
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
            rowMenu={{
              openIndex: menuOpenIndex,
              onOpenChange: (rowIndex, open) =>
                setMenuOpenIndex(open ? rowIndex : null),
              onRowContextMenu: (rowIndex) => setMenuOpenIndex(rowIndex),
              renderItems: rowMenuItems,
            }}
          />
        ) : null}
      </div>

      <footer className="flex h-8 shrink-0 items-center justify-between border-b border-t border-border bg-panel px-3 text-[10px] text-muted-foreground">
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
