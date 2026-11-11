"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createConnection,
  deleteConnection,
  listConnections,
  testConnection,
  updateConnection,
} from "@/lib/api/endpoints";
import { queryKeys } from "@/lib/query-keys";
import type {
  ConnectionCreateRequest,
  ConnectionTestRequest,
  ConnectionUpdateRequest,
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

/**
 * Edits an existing profile (PRF-03/B). On success the connection list is
 * refetched; any cached schema/database subtrees for that connection are dropped
 * because the backend invalidated its pools and the target may have changed.
 */
export function useUpdateConnection() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      connectionId,
      body,
    }: {
      connectionId: string;
      body: ConnectionUpdateRequest;
    }) => updateConnection(connectionId, body),
    onSuccess: (_result, { connectionId }) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.connections });
      void queryClient.removeQueries({
        queryKey: queryKeys.schemaRoot(connectionId),
      });
      void queryClient.removeQueries({
        queryKey: queryKeys.databases(connectionId),
      });
    },
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
