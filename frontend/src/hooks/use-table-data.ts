"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";

import { browseTableData } from "@/lib/api/endpoints";
import { queryKeys } from "@/lib/query-keys";
import type { TableFilter } from "@/store/useWorkspaceStore";

/** The bound target + page state a Table Data tab owns (never global state). */
export interface TableDataTarget {
  connectionId: string | null;
  database: string | null;
  schema: string | null;
  table: string;
  page: number;
  pageSize: number;
  /** Single-column server-side sort (null = unsorted). */
  sort: { column: string; direction: "asc" | "desc" } | null;
  /** ANDed server-side filters (empty = unfiltered). */
  filters: TableFilter[];
}

/**
 * Bounded table-data page for one explicit tab binding. The query key includes
 * the full target + page so rows are never shared or leaked between unrelated
 * tabs, and TanStack Query cancels superseded requests via the request signal.
 *
 * `placeholderData: keepPreviousData` keeps the current page visible while the
 * next page loads instead of blanking the grid.
 */
export function useTableData(target: TableDataTarget, enabled = true) {
  return useQuery({
    queryKey: queryKeys.tableData(
      target.connectionId ?? "",
      target.database,
      target.schema,
      target.table,
      target.page,
      target.pageSize,
      target.sort?.column ?? null,
      target.sort?.direction ?? null,
      target.filters.length > 0 ? JSON.stringify(target.filters) : "",
    ),
    queryFn: ({ signal }) =>
      browseTableData(
        target.connectionId as string,
        {
          database: target.database,
          schema: target.schema,
          table: target.table,
          page: target.page,
          pageSize: target.pageSize,
          sort: target.sort,
          filters: target.filters,
        },
        signal,
      ),
    enabled: enabled && Boolean(target.connectionId) && Boolean(target.table),
    placeholderData: keepPreviousData,
    retry: false,
  });
}
