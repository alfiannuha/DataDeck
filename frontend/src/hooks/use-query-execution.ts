"use client";

import { useQuery } from "@tanstack/react-query";

import { getQueryHistory } from "@/lib/api/endpoints";
import { queryKeys } from "@/lib/query-keys";

/**
 * Query audit log, optionally filtered by connection. Execution itself is
 * handled by `useRunQuery`; results are not cached here. Pass `enabled: false`
 * to avoid fetching until a consumer (e.g. the history panel) is visible.
 */
export function useQueryHistory(
  connectionId?: string,
  page = 1,
  enabled = true,
) {
  return useQuery({
    queryKey: queryKeys.queryHistory(connectionId, page),
    queryFn: ({ signal }) =>
      getQueryHistory({ connectionId, page, pageSize: 50 }, signal),
    enabled,
  });
}
