"use client";

import { Database, Plus, Trash2 } from "lucide-react";
import { useState } from "react";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { useConnections, useDeleteConnection } from "@/hooks/use-connections";
import { ApiClientError } from "@/lib/api-client";
import { cn } from "@/lib/utils";
import { useConnectionStore } from "@/store/useConnectionStore";
import type { ConnectionResponse } from "@/types/api";

import { NewConnectionModal } from "./new-connection-modal";

/**
 * Saved connection list. Shows only non-sensitive fields; credentials are never
 * returned by the backend and never rendered here.
 */
export function ConnectionList() {
  const { data, isLoading, isError, error, refetch } = useConnections();
  const activeConnectionId = useConnectionStore(
    (state) => state.activeConnectionId,
  );
  const setActiveConnection = useConnectionStore(
    (state) => state.setActiveConnection,
  );
  const deleteConnection = useDeleteConnection();

  const [modalOpen, setModalOpen] = useState(false);
  const [pendingDelete, setPendingDelete] = useState<ConnectionResponse | null>(
    null,
  );

  async function confirmDelete() {
    const target = pendingDelete;
    if (!target?.id) {
      setPendingDelete(null);
      return;
    }
    try {
      await deleteConnection.mutateAsync(target.id);
      if (activeConnectionId === target.id) {
        setActiveConnection(null);
      }
    } finally {
      setPendingDelete(null);
    }
  }

  return (
    <div className="flex flex-col gap-1 p-2">
      <div className="flex items-center justify-between px-1 py-1">
        <span className="text-xs font-medium uppercase tracking-wide text-subtle-foreground">
          Connections
        </span>
        <Button
          variant="ghost"
          size="icon"
          aria-label="New connection"
          onClick={() => setModalOpen(true)}
        >
          <Plus size={16} aria-hidden="true" />
        </Button>
      </div>

      {isLoading && (
        <p className="px-2 py-1 text-xs text-muted-foreground">
          Loading connections…
        </p>
      )}

      {isError && (
        <div role="alert" className="flex flex-col gap-2 px-2 py-1">
          <p className="text-xs text-destructive">
            {error instanceof ApiClientError
              ? `${error.message} (${error.code})`
              : "Could not load connections."}
          </p>
          <Button variant="outline" size="sm" onClick={() => void refetch()}>
            Retry
          </Button>
        </div>
      )}

      {data && data.length === 0 && (
        <p className="px-2 py-1 text-xs text-muted-foreground">
          No connections yet. Add one to get started.
        </p>
      )}

      {data && data.length > 0 && (
        <ul className="flex flex-col gap-0.5" aria-label="Saved connections">
          {data.map((connection) => {
            const id = connection.id;
            if (!id) return null;
            const isActive = id === activeConnectionId;
            return (
              <li key={id}>
                <div
                  className={cn(
                    "group flex items-center gap-1 rounded-md",
                    isActive && "bg-panel-raised",
                  )}
                >
                  <button
                    type="button"
                    aria-current={isActive ? "true" : undefined}
                    onClick={() => setActiveConnection(id)}
                    className="flex min-w-0 flex-1 flex-col items-start rounded-md px-2 py-1.5 text-left hover:bg-panel-raised focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
                  >
                    <span className="flex w-full items-center gap-1.5">
                      <Database
                        size={13}
                        aria-hidden="true"
                        className="shrink-0 text-muted-foreground"
                      />
                      <span className="truncate text-sm font-medium text-foreground">
                        {connection.name ?? "Untitled"}
                      </span>
                    </span>
                    <span className="w-full truncate text-xs text-muted-foreground">
                      {connection.driver === "sqlite"
                        ? `sqlite · ${connection.database_name}`
                        : `${connection.driver} · ${connection.host ?? "—"}:${connection.port ?? "—"}/${connection.database_name}`}
                    </span>
                  </button>
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={`Delete ${connection.name ?? "connection"}`}
                    onClick={() => setPendingDelete(connection)}
                  >
                    <Trash2 size={14} aria-hidden="true" />
                  </Button>
                </div>
              </li>
            );
          })}
        </ul>
      )}

      <NewConnectionModal
        open={modalOpen}
        onOpenChange={setModalOpen}
        onSaved={(connection) => setActiveConnection(connection.id ?? null)}
      />

      <AlertDialog
        open={pendingDelete !== null}
        onOpenChange={(open) => {
          if (!open) setPendingDelete(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete connection?</AlertDialogTitle>
            <AlertDialogDescription>
              {pendingDelete
                ? `“${pendingDelete.name}” will be removed and its active connection closed. This cannot be undone.`
                : ""}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={() => void confirmDelete()}>
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
