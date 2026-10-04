"use client";

import { useQuery } from "@tanstack/react-query";

import { listDatabases } from "@/lib/api/endpoints";
import { queryKeys } from "@/lib/query-keys";

/**
 * Selectable databases for a server-level connection (PostgreSQL; PRF-01).
 * Disabled until a connection id is selected or when explicitly disabled (e.g.
 * a driver without multiple databases).
 */
export function useDatabases(connectionId: string | null, enabled = true) {
  return useQuery({
    queryKey: queryKeys.databases(connectionId ?? ""),
    queryFn: ({ signal }) => listDatabases(connectionId as string, signal),
    enabled: Boolean(connectionId) && enabled,
  });
}
