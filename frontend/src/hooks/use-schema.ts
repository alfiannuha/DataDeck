"use client";

import { useQuery } from "@tanstack/react-query";

import { getSchemas } from "@/lib/api/endpoints";
import { queryKeys } from "@/lib/query-keys";

/** Schema tree for a connection. Disabled until a connection id is selected. */
export function useSchema(connectionId: string | null) {
  return useQuery({
    queryKey: queryKeys.schema(connectionId ?? ""),
    queryFn: ({ signal }) => getSchemas(connectionId as string, signal),
    enabled: Boolean(connectionId),
  });
}
