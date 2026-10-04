"use client";

import { useCallback } from "react";

import { useConnectionStore } from "@/store/useConnectionStore";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

/**
 * Canonical "New Query" action. Creates a Query tab through the workspace store
 * with the approved PRF-01 binding precedence: the active connection plus the
 * explorer-selected database for that connection (null when none is selected,
 * so a server-level PostgreSQL tab stays database-unbound and never falls back
 * to the bootstrap/maintenance database).
 *
 * This is the single entry point used by the empty state and the tab bar; it
 * never synthesises a different binding policy.
 */
export function useNewQueryTab(): () => void {
  const activeConnectionId = useConnectionStore(
    (state) => state.activeConnectionId,
  );
  const activeDatabaseByConnection = useConnectionStore(
    (state) => state.activeDatabaseByConnection,
  );
  const addTab = useWorkspaceStore((state) => state.addTab);

  const database = activeConnectionId
    ? (activeDatabaseByConnection[activeConnectionId] ?? null)
    : null;

  return useCallback(() => {
    addTab(activeConnectionId, database);
  }, [addTab, activeConnectionId, database]);
}
