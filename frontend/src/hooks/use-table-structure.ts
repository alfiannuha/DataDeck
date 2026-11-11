"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { applyStructure, getTableStructure } from "@/lib/api/endpoints";
import { queryKeys } from "@/lib/query-keys";
import type { StructureChangeRequest } from "@/types/api";

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

/**
 * Applies one schema change (PRF-04/Phase 2) and refetches the bound table's
 * structure on success. The schema tree is also invalidated so the Explorer
 * reflects renames/added columns without a manual refresh.
 */
export function useApplyStructure() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      target,
      body,
    }: {
      target: TableStructureTarget;
      body: StructureChangeRequest;
    }) => applyStructure(target.connectionId as string, body),
    onSuccess: (_result, { target }) => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.tableStructure(
          target.connectionId ?? "",
          target.database,
          target.schema,
          target.table,
        ),
      });
      if (target.connectionId) {
        void queryClient.invalidateQueries({
          queryKey: queryKeys.schemaRoot(target.connectionId),
        });
      }
    },
  });
}