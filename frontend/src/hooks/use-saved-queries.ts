"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createSavedQuery,
  deleteSavedQuery,
  listSavedQueries,
  updateSavedQuery,
} from "@/lib/api/endpoints";
import { queryKeys } from "@/lib/query-keys";
import type { SavedQueryRequest } from "@/types/api";

export function useSavedQueries(
  connectionId?: string,
  page = 1,
  enabled = true,
) {
  return useQuery({
    queryKey: queryKeys.savedQueries(connectionId, page),
    queryFn: ({ signal }) =>
      listSavedQueries({ connectionId, page, pageSize: 50 }, signal),
    enabled,
  });
}

export function useCreateSavedQuery() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: SavedQueryRequest) => createSavedQuery(body),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.savedQueriesRoot,
      });
    },
  });
}

export function useUpdateSavedQuery() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: SavedQueryRequest }) =>
      updateSavedQuery(id, body),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.savedQueriesRoot,
      });
    },
  });
}

export function useDeleteSavedQuery() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteSavedQuery(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.savedQueriesRoot,
      });
    },
  });
}
