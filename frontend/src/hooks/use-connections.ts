"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createConnection,
  createConnectionGroup,
  deleteConnection,
  deleteConnectionGroup,
  listConnectionGroups,
  listConnections,
  reorderConnectionGroups,
  reorderConnections,
  setConnectionGroup,
  testConnection,
  updateConnection,
  updateConnectionGroup,
} from "@/lib/api/endpoints";
import { queryKeys } from "@/lib/query-keys";
import type {
  ConnectionCreateRequest,
  ConnectionResponse,
  ConnectionTestRequest,
  ConnectionUpdateRequest,
  GroupCreateRequest,
  GroupUpdateRequest,
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
 * Persists a manual connection order with an optimistic cache update and
 * rollback on failure (PRF-05 reordering). The backend remains the source of
 * truth; the list is refetched on settle.
 */
export function useReorderConnections() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (ids: string[]) => reorderConnections(ids),
    onMutate: async (ids) => {
      await queryClient.cancelQueries({ queryKey: queryKeys.connections });
      const previous = queryClient.getQueryData<ConnectionResponse[]>(
        queryKeys.connections,
      );
      if (previous) {
        const byId = new Map(previous.map((connection) => [connection.id, connection]));
        const next = ids
          .map((id) => byId.get(id))
          .filter((connection): connection is ConnectionResponse => Boolean(connection));
        // Only apply a full permutation; never drop entries optimistically.
        if (next.length === previous.length) {
          queryClient.setQueryData(queryKeys.connections, next);
        }
      }
      return { previous };
    },
    onError: (_error, _ids, context) => {
      if (context?.previous) {
        queryClient.setQueryData(queryKeys.connections, context.previous);
      }
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.connections });
    },
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

export function useConnectionGroups() {
  return useQuery({
    queryKey: queryKeys.groups,
    queryFn: ({ signal }) => listConnectionGroups(signal),
  });
}

export function useCreateGroup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: GroupCreateRequest) => createConnectionGroup(body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.groups });
    },
  });
}

export function useUpdateGroup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ groupId, body }: { groupId: string; body: GroupUpdateRequest }) =>
      updateConnectionGroup(groupId, body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.groups });
    },
  });
}

export function useDeleteGroup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (groupId: string) => deleteConnectionGroup(groupId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.groups });
      void queryClient.invalidateQueries({ queryKey: queryKeys.connections });
    },
  });
}

export function useReorderGroups() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (groupIds: string[]) => reorderConnectionGroups(groupIds),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.groups });
    },
  });
}

export function useSetConnectionGroup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ connectionId, groupId }: { connectionId: string; groupId: string | null }) =>
      setConnectionGroup(connectionId, groupId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.connections });
      void queryClient.invalidateQueries({ queryKey: queryKeys.groups });
    },
  });
}
