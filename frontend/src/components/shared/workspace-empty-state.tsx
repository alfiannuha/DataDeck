"use client";

import { FilePlus2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { useConnections } from "@/hooks/use-connections";
import { useNewQueryTab } from "@/hooks/use-new-query-tab";
import { useConnectionStore } from "@/store/useConnectionStore";

/**
 * Workspace empty state shown when there is no active tab (PRF02-T02).
 *
 * State A (no active connection): guidance to select a connection or start a
 * query. State B (active connection): shows the connection/database and points
 * the user at the schema explorer. Both states offer an explicit New Query
 * action; nothing here creates a tab implicitly.
 */
export function WorkspaceEmptyState() {
  const activeConnectionId = useConnectionStore(
    (state) => state.activeConnectionId,
  );
  const activeDatabaseByConnection = useConnectionStore(
    (state) => state.activeDatabaseByConnection,
  );
  const { data: connections } = useConnections();
  const newQuery = useNewQueryTab();

  const connection = activeConnectionId
    ? connections?.find((candidate) => candidate.id === activeConnectionId)
    : undefined;
  const selectedDatabase = activeConnectionId
    ? (activeDatabaseByConnection[activeConnectionId] ?? null)
    : null;
  // Show the database that a new Query tab would bind to: the explorer's
  // selection, else the profile's configured default (legacy/MySQL/SQLite).
  const databaseLabel = selectedDatabase ?? connection?.database_name ?? null;

  return (
    <section
      aria-label="Empty workspace"
      data-testid="workspace-empty-state"
      className="flex min-h-0 flex-1 flex-col items-center justify-center gap-3 overflow-auto p-6 text-center"
    >
      <h2 className="text-sm font-medium text-foreground">No tabs open</h2>

      {connection ? (
        <>
          <p
            data-testid="empty-state-connection"
            className="text-sm font-semibold tracking-tight text-foreground"
          >
            {connection.name}
            {databaseLabel ? (
              <span className="text-muted-foreground"> / {databaseLabel}</span>
            ) : null}
          </p>
          <p className="max-w-sm text-xs text-muted-foreground">
            Double-click a table in the explorer to browse its data, or open a
            new query.
          </p>
        </>
      ) : (
        <p className="max-w-sm text-xs text-muted-foreground">
          Select a connection or create a new query to get started.
        </p>
      )}

      <Button type="button" variant="outline" size="sm" onClick={newQuery}>
        <FilePlus2 size={14} aria-hidden="true" />
        New Query
      </Button>
    </section>
  );
}
