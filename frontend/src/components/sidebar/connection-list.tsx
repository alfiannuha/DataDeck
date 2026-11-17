"use client";

import {
  Database,
  GripVertical,
  MoreVertical,
  Pencil,
  Plus,
  Trash2,
} from "lucide-react";
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  useConnections,
  useDeleteConnection,
  useReorderConnections,
} from "@/hooks/use-connections";
import { ApiClientError } from "@/lib/api-client";
import { connectionColor } from "@/lib/connection-colors";
import { cn } from "@/lib/utils";
import { useConnectionStore } from "@/store/useConnectionStore";
import type { ConnectionResponse } from "@/types/api";

import { NewConnectionModal } from "./new-connection-modal";

interface DropTarget {
  id: string;
  after: boolean;
}

/**
 * Saved connection list. Shows only non-sensitive fields; credentials are never
 * returned by the backend and never rendered here.
 *
 * Manual ordering (PRF-05): drag a row by its handle, or use the three-dot menu
 * (Move to Top/Up/Down/Bottom). Reordering never changes the active connection
 * and is persisted optimistically (rolled back on failure).
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
  const reorder = useReorderConnections();

  const [modalOpen, setModalOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<ConnectionResponse | null>(null);
  const [pendingDelete, setPendingDelete] = useState<ConnectionResponse | null>(
    null,
  );
  const [draggingId, setDraggingId] = useState<string | null>(null);
  const [dropTarget, setDropTarget] = useState<DropTarget | null>(null);

  const busy = reorder.isPending;

  function orderedIds(): string[] {
    return (data ?? []).map((connection) => connection.id).filter(Boolean) as string[];
  }

  function commitOrder(next: string[]) {
    const previous = orderedIds();
    if (next.join("\u0000") === previous.join("\u0000")) return;
    reorder.mutate(next);
  }

  /** Move `id` to `reducedIndex` in the list with `id` removed (0..n). */
  function moveToReducedIndex(id: string, reducedIndex: number) {
    if (busy) return;
    const ids = orderedIds();
    const reduced = ids.filter((candidate) => candidate !== id);
    const index = Math.max(0, Math.min(reducedIndex, reduced.length));
    reduced.splice(index, 0, id);
    commitOrder(reduced);
  }

  function moveStep(id: string, delta: number) {
    const ids = orderedIds();
    const from = ids.indexOf(id);
    if (from < 0) return;
    moveToReducedIndex(id, from + delta);
  }

  function drop(dragId: string, targetId: string, after: boolean) {
    if (busy || dragId === targetId) return;
    const reduced = orderedIds().filter((candidate) => candidate !== dragId);
    let index = reduced.indexOf(targetId);
    if (index < 0) return;
    if (after) index += 1;
    reduced.splice(index, 0, dragId);
    commitOrder(reduced);
  }

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
          {data.map((connection, index) => {
            const id = connection.id;
            if (!id) return null;
            const isActive = id === activeConnectionId;
            const isFirst = index === 0;
            const isLast = index === data.length - 1;
            const isDragging = draggingId === id;
            const indicator = dropTarget?.id === id ? dropTarget : null;
            return (
              <li key={id} className="relative">
                {indicator ? (
                  <span
                    aria-hidden="true"
                    className={cn(
                      "pointer-events-none absolute inset-x-1 z-20 h-0.5 rounded bg-[var(--color-focus-ring)]",
                      indicator.after ? "bottom-0" : "top-0",
                    )}
                  />
                ) : null}
                <div
                  onDragOver={(event) => {
                    if (!draggingId) return;
                    event.preventDefault();
                    event.dataTransfer.dropEffect = "move";
                    const rect = event.currentTarget.getBoundingClientRect();
                    setDropTarget({
                      id,
                      after: event.clientY > rect.top + rect.height / 2,
                    });
                  }}
                  onDrop={(event) => {
                    event.preventDefault();
                    // Prefer the dragged id carried by the drag data; fall back
                    // to the tracked id. Position comes from the drop pointer if
                    // the hover state did not update in time.
                    const dragId =
                      event.dataTransfer.getData("text/plain") || draggingId;
                    if (dragId) {
                      let after = dropTarget?.after;
                      if (after === undefined) {
                        const rect = event.currentTarget.getBoundingClientRect();
                        after = event.clientY > rect.top + rect.height / 2;
                      }
                      drop(dragId, id, after);
                    }
                    setDraggingId(null);
                    setDropTarget(null);
                  }}
                  className={cn(
                    "group flex items-center gap-0.5 rounded-md",
                    isActive && "bg-panel-raised",
                    isDragging && "opacity-50",
                  )}
                >
                  <span
                    data-drag-handle
                    draggable={!busy}
                    aria-hidden="true"
                    title="Drag to reorder"
                    onDragStart={(event) => {
                      if (busy) {
                        event.preventDefault();
                        return;
                      }
                      setDraggingId(id);
                      event.dataTransfer.effectAllowed = "move";
                      event.dataTransfer.setData("text/plain", id);
                    }}
                    onDragEnd={() => {
                      setDraggingId(null);
                      setDropTarget(null);
                    }}
                    className="shrink-0 cursor-grab pl-1 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100 active:cursor-grabbing"
                  >
                    <GripVertical size={13} aria-hidden="true" />
                  </span>
                  <button
                    type="button"
                    aria-current={isActive ? "true" : undefined}
                    onClick={() => setActiveConnection(id)}
                    className="flex min-w-0 flex-1 flex-col items-start rounded-md px-1.5 py-1.5 text-left hover:bg-panel-raised focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
                  >
                    <span className="flex w-full items-center gap-1.5">
                      <span
                        data-testid="connection-color"
                        aria-hidden="true"
                        title={connection.name ?? undefined}
                        className="h-2.5 w-2.5 shrink-0 rounded-full border border-border"
                        style={{ backgroundColor: connectionColor(connection.color) }}
                      />
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
                        : `${connection.driver} · ${connection.host ?? "—"}:${connection.port ?? "—"}${
                            connection.database_name
                              ? `/${connection.database_name}`
                              : ""
                          }`}
                    </span>
                  </button>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label={`Reorder ${connection.name ?? "connection"}`}
                        title="Reorder"
                        disabled={busy}
                      >
                        <MoreVertical size={14} aria-hidden="true" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem
                        data-testid={`conn-move-top-${id}`}
                        disabled={busy || isFirst}
                        onSelect={() => moveToReducedIndex(id, 0)}
                      >
                        Move to Top
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        data-testid={`conn-move-up-${id}`}
                        disabled={busy || isFirst}
                        onSelect={() => moveStep(id, -1)}
                      >
                        Move Up
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        data-testid={`conn-move-down-${id}`}
                        disabled={busy || isLast}
                        onSelect={() => moveStep(id, 1)}
                      >
                        Move Down
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        data-testid={`conn-move-bottom-${id}`}
                        disabled={busy || isLast}
                        onSelect={() => moveToReducedIndex(id, data.length - 1)}
                      >
                        Move to Bottom
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={`Edit ${connection.name ?? "connection"}`}
                    onClick={() => setEditTarget(connection)}
                  >
                    <Pencil size={14} aria-hidden="true" />
                  </Button>
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

      <NewConnectionModal
        open={editTarget !== null}
        connection={editTarget ?? undefined}
        onOpenChange={(open) => {
          if (!open) setEditTarget(null);
        }}
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
