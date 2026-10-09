"use client";

import { useQuery } from "@tanstack/react-query";

import { getTableStructure } from "@/lib/api/endpoints";
import { queryKeys } from "@/lib/query-keys";

/** The bound target a Table Structure tab owns (never global state). */
export interface TableStructureTarget {
  connectionId: string | null;
  database: string | null;
  schema: string | null;
  table: string;
}

/**
 * Read-only structure for one explicit tab binding (PRF-02/T13). The query key
 * includes the full target, so metadata is never shared or leaked between
 * unrelated tabs, and TanStack Query cancels superseded requests via the
 * request signal.
 */
export function useTableStructure(target: TableStructureTarget, enabled = true) {
  return useQuery({
    queryKey: queryKeys.tableStructure(
      target.connectionId ?? "",
      target.database,
      target.schema,
      target.table,
    ),
    queryFn: ({ signal }) =>
      getTableStructure(
        target.connectionId as string,
        {
          database: target.database,
          schema: target.schema,
          table: target.table,
        },
        signal,
      ),
    enabled: enabled && Boolean(target.connectionId) && Boolean(target.table),
    retry: false,
  });
}