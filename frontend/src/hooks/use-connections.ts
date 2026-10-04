"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createConnection,
  deleteConnection,
  listConnections,
  testConnection,
} from "@/lib/api/endpoints";
import { queryKeys } from "@/lib/query-keys";
import type {
  ConnectionCreateRequest,
  ConnectionTestRequest,
} from "@/types/api";

export function useConnections() {
  return useQuery({
    queryKey: queryKeys.connections,
    queryFn: ({ signal }) => listConnections(signal),
  });
}

export function useCreateConnection() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: ConnectionCreateRequest) => createConnection(body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.connections });
    },
  });
}

/** Validates parameters without persisting. Never cached. */
export function useTestConnection() {
  return useMutation({
    mutationFn: (body: ConnectionTestRequest) => testConnection(body),
  });
}

export function useDeleteConnection() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (connectionId: string) => deleteConnection(connectionId),
    onSuccess: (_result, connectionId) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.connections });
      // Drop every cached schema subtree (all databases) and the database list.
      void queryClient.removeQueries({
        queryKey: queryKeys.schemaRoot(connectionId),
      });
      void queryClient.removeQueries({
        queryKey: queryKeys.databases(connectionId),
      });
    },
  });
}
