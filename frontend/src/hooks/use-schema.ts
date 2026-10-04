"use client";

import { useQuery } from "@tanstack/react-query";

import { getSchemas } from "@/lib/api/endpoints";
import { queryKeys } from "@/lib/query-keys";

/**
 * Schema tree for a connection, optionally scoped to one database for
 * server-level PostgreSQL connections (PRF-01). The database is part of the
 * query key so metadata never leaks between databases or connections.
 *
 * `enabled` allows callers to fetch lazily (e.g. only when a database node is
 * expanded).
 */
export function useSchema(
  connectionId: string | null,
  database?: string | null,
  options?: { enabled?: boolean },
) {
  return useQuery({
    queryKey: queryKeys.schema(connectionId ?? "", database ?? ""),
    queryFn: ({ signal }) =>
      getSchemas(connectionId as string, {
        database: database ?? undefined,
        signal,
      }),
    enabled: Boolean(connectionId) && (options?.enabled ?? true),
  });
}
