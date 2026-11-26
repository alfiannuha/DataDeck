"use client";

import {
  ChevronRight,
  Database,
  FolderPlus,
  GripVertical,
  MoreVertical,
  Pencil,
  Plus,
  Trash2,
} from "lucide-react";
import { useEffect, useState } from "react";

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
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import {
  useConnectionGroups,
  useConnections,
  useCreateGroup,
  useDeleteConnection,
  useDeleteGroup,
  useReorderConnections,
  useReorderGroups,
  useSetConnectionGroup,
  useUpdateGroup,
} from "@/hooks/use-connections";
import { ApiClientError } from "@/lib/api-client";
import { connectionColor } from "@/lib/connection-colors";
import { cn } from "@/lib/utils";
import { useConnectionStore } from "@/store/useConnectionStore";
import type { ConnectionGroup, ConnectionResponse } from "@/types/api";

import { NewConnectionModal } from "./new-connection-modal";

const MAIN_EXPANDED_KEY = "datadeck.connectionsExpanded";

/**
 * Saved connection list with optional grouping (v0.4.2). A master "DATA
 * CONNECTIONS" accordion wraps group accordions (collapsed by default) and the
 * ungrouped connections. Grouping is opt-in; ungrouped connections render as
 * before. Connection ordering, colors and the active connection are unaffected.
 */
export function ConnectionList() {
  const { data, isLoading, isError, error, refetch } = useConnections();
  const { data: groups } = useConnectionGroups();
  const activeConnectionId = useConnectionStore((s) => s.activeConnectionId);
  const setActiveConnection = useConnectionStore((s) => s.setActiveConnection);
  const deleteConnection = useDeleteConnection();
  const reorder = useReorderConnections();
  const createGroup = useCreateGroup();
  const updateGroup = useUpdateGroup();
  const deleteGroup = useDeleteGroup();
  const reorderGroups = useReorderGroups();
  const setGroup = useSetConnectionGroup();

  const [modalOpen, setModalOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<ConnectionResponse | null>(null);
  const [pendingDelete, setPendingDelete] = useState<ConnectionResponse | null>(null);
  const [pendingDeleteGroup, setPendingDeleteGroup] = useState<ConnectionGroup | null>(null);
  const [groupDialog, setGroupDialog] = useState<{
    mode: "create" | "rename";
    groupId?: string;
    name: string;
    assignConnectionId?: string;
  } | null>(null);
  const [mainExpanded, setMainExpanded] = useState(true);
  const [draggingId, setDraggingId] = useState<string | null>(null);
  const [dropTarget, setDropTarget] = useState<{ id: string; after: boolean } | null>(null);

  const busy = reorder.isPending;

  useEffect(() => {
    try {
      const stored = window.localStorage.getItem(MAIN_EXPANDED_KEY);
      if (stored !== null) setMainExpanded(stored === "true");
    } catch {
      /* ignore storage errors */
    }
  }, []);

  function toggleMain() {
    setMainExpanded((value) => {
      const next = !value;
      try {
        window.localStorage.setItem(MAIN_EXPANDED_KEY, String(next));
      } catch {
        /* ignore */
      }
      return next;
    });
  }

  function orderedIds(): string[] {
    return (data ?? []).map((c) => c.id).filter(Boolean) as string[];
  }

  function commitOrder(next: string[]) {
    const previous = orderedIds();
    if (next.join("\u0000") === previous.join("\u0000")) return;
    reorder.mutate(next);
  }

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
      if (activeConnectionId === target.id) setActiveConnection(null);
    } finally {
      setPendingDelete(null);
    }
  }

  function openCreateGroup(assignConnectionId?: string) {
    setGroupDialog({ mode: "create", name: "", assignConnectionId });
  }

  async function submitGroupDialog() {
    if (!groupDialog) return;
    const name = groupDialog.name.trim();
    if (!name) return;
    if (groupDialog.mode === "create") {
      const created = await createGroup.mutateAsync({ name });
      if (groupDialog.assignConnectionId && created.id) {
        await setGroup.mutateAsync({
          connectionId: groupDialog.assignConnectionId,
          groupId: created.id,
        });
      }
    } else if (groupDialog.groupId) {
      await updateGroup.mutateAsync({
        groupId: groupDialog.groupId,
        body: { name },
      });
    }
    setGroupDialog(null);
  }

  function moveGroup(group: ConnectionGroup, delta: number) {
    const list = groups ?? [];
    const from = list.findIndex((g) => g.id === group.id);
    const to = from + delta;
    if (from < 0 || to < 0 || to >= list.length) return;
    const ids = list.map((g) => g.id as string);
    ids.splice(from, 1);
    ids.splice(to, 0, group.id as string);
    reorderGroups.mutate(ids);
  }

  function renderConnectionRow(connection: ConnectionResponse, index: number, isLast: boolean) {
    const id = connection.id as string;
    const isActive = id === activeConnectionId;
    const isDragging = draggingId === id;
    const indicator = dropTarget?.id === id ? dropTarget : null;
    const isFirst = index === 0;
    const groupId = connection.group_id ?? null;
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
            setDropTarget({ id, after: event.clientY > rect.top + rect.height / 2 });
          }}
          onDrop={(event) => {
            event.preventDefault();
            const dragId = event.dataTransfer.getData("text/plain") || draggingId;
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
              <Database size={13} aria-hidden="true" className="shrink-0 text-muted-foreground" />
              <span className="truncate text-sm font-medium text-foreground">
                {connection.name ?? "Untitled"}
              </span>
            </span>
            <span className="w-full truncate text-xs text-muted-foreground">
              {connection.driver === "sqlite"
                ? `sqlite · ${connection.database_name}`
                : `${connection.driver} · ${connection.host ?? "—"}:${connection.port ?? "—"}${
                    connection.database_name ? `/${connection.database_name}` : ""
                  }`}
            </span>
          </button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                aria-label={`Reorder ${connection.name ?? "connection"}`}
                title="Actions"
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
                onSelect={() => moveToReducedIndex(id, orderedIds().length - 1)}
              >
                Move to Bottom
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              {groupId ? (
                <DropdownMenuItem
                  data-testid={`conn-ungroup-${id}`}
                  onSelect={() =>
                    setGroup.mutate({ connectionId: id, groupId: null })
                  }
                >
                  Remove from group
                </DropdownMenuItem>
              ) : null}
              {(groups ?? [])
                .filter((group) => group.id !== groupId)
                .map((group) => (
                  <DropdownMenuItem
                    key={group.id}
                    data-testid={`conn-move-group-${id}-${group.id}`}
                    onSelect={() =>
                      setGroup.mutate({ connectionId: id, groupId: group.id as string })
                    }
                  >
                    Move to {group.name}
                  </DropdownMenuItem>
                ))}
              <DropdownMenuItem
                data-testid={`conn-new-group-${id}`}
                onSelect={() => openCreateGroup(id)}
              >
                New group…
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
  }

  const groupList = groups ?? [];
  const groupedIds = new Set(groupList.map((g) => g.id));
  const ungrouped = (data ?? []).filter(
    (c) => !c.group_id || !groupedIds.has(c.group_id),
  );
  const all = data ?? [];

  return (
    <div className="flex flex-col gap-1 p-2">
      <div className="flex items-center justify-between px-1 py-1">
        <button
          type="button"
          onClick={toggleMain}
          aria-expanded={mainExpanded}
          data-testid="connections-toggle"
          className="flex min-w-0 items-center gap-1 rounded text-xs font-medium uppercase tracking-wide text-subtle-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
        >
          <ChevronRight
            size={12}
            aria-hidden="true"
            className={cn("shrink-0 transition-transform", mainExpanded && "rotate-90")}
          />
          <span className="truncate">Data Connections</span>
          <span className="text-xs text-muted-foreground">({all.length})</span>
        </button>
        <div className="flex items-center gap-0.5">
          <Button
            variant="ghost"
            size="icon"
            aria-label="New group"
            title="New group"
            onClick={() => openCreateGroup()}
          >
            <FolderPlus size={15} aria-hidden="true" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            aria-label="New connection"
            onClick={() => setModalOpen(true)}
          >
            <Plus size={16} aria-hidden="true" />
          </Button>
        </div>
      </div>

      {!mainExpanded ? null : (
        <>
          {isLoading && (
            <p className="px-2 py-1 text-xs text-muted-foreground">Loading connections…</p>
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

          {groupList.map((group) => {
            const members = all.filter((c) => c.group_id === group.id);
            const groupIndex = groupList.findIndex((g) => g.id === group.id);
            return (
              <div key={group.id as string} data-testid={`group-${group.id}`}>
                <div className="group flex items-center gap-0.5 rounded px-1 py-1 hover:bg-panel-raised">
                  <button
                    type="button"
                    aria-expanded={group.is_expanded}
                    data-testid={`group-toggle-${group.id}`}
                    onClick={() =>
                      updateGroup.mutate({
                        groupId: group.id as string,
                        body: { is_expanded: !group.is_expanded },
                      })
                    }
                    className="flex min-w-0 flex-1 items-center gap-1 rounded text-left text-xs font-medium text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
                  >
                    <ChevronRight
                      size={12}
                      aria-hidden="true"
                      className={cn("shrink-0 text-muted-foreground transition-transform", group.is_expanded && "rotate-90")}
                    />
                    <span className="truncate">{group.name}</span>
                    <span className="text-xs text-muted-foreground">({members.length})</span>
                  </button>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="icon" aria-label={`Group actions ${group.name}`}>
                        <MoreVertical size={13} aria-hidden="true" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem
                        data-testid={`group-rename-${group.id}`}
                        onSelect={() =>
                          setGroupDialog({
                            mode: "rename",
                            groupId: group.id as string,
                            name: group.name ?? "",
                          })
                        }
                      >
                        Rename
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        disabled={groupIndex <= 0}
                        onSelect={() => moveGroup(group, -1)}
                      >
                        Move Up
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        disabled={groupIndex >= groupList.length - 1}
                        onSelect={() => moveGroup(group, 1)}
                      >
                        Move Down
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem
                        data-testid={`group-delete-${group.id}`}
                        className="text-destructive focus:text-destructive"
                        onSelect={() => setPendingDeleteGroup(group)}
                      >
                        Delete group
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
                {group.is_expanded && members.length > 0 ? (
                  <ul className="ml-3 flex flex-col gap-0.5 border-l border-border pl-1">
                    {members.map((connection, index) =>
                      renderConnectionRow(connection, index, index === members.length - 1),
                    )}
                  </ul>
                ) : null}
              </div>
            );
          })}

          {all.length === 0 && groupList.length === 0 ? (
            <p className="px-2 py-1 text-xs text-muted-foreground">
              No connections yet. Add one to get started.
            </p>
          ) : null}

          {ungrouped.length > 0 ? (
            <ul className="flex flex-col gap-0.5" aria-label="Saved connections">
              {ungrouped.map((connection, index) =>
                renderConnectionRow(connection, index, index === ungrouped.length - 1),
              )}
            </ul>
          ) : null}
        </>
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

      <Dialog open={groupDialog !== null} onOpenChange={(open) => { if (!open) setGroupDialog(null); }}>
        <DialogContent aria-labelledby="group-dialog-title">
          <DialogHeader>
            <DialogTitle id="group-dialog-title">
              {groupDialog?.mode === "rename" ? "Rename group" : "New group"}
            </DialogTitle>
          </DialogHeader>
          <Input
            aria-label="Group name"
            value={groupDialog?.name ?? ""}
            onChange={(event) =>
              setGroupDialog((current) =>
                current ? { ...current, name: event.target.value } : current,
              )
            }
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                void submitGroupDialog();
              }
            }}
            autoFocus
          />
          <DialogFooter>
            <Button variant="ghost" onClick={() => setGroupDialog(null)}>
              Cancel
            </Button>
            <Button
              onClick={() => void submitGroupDialog()}
              disabled={!groupDialog?.name.trim() || createGroup.isPending || updateGroup.isPending}
            >
              {groupDialog?.mode === "rename" ? "Save" : "Create"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={pendingDeleteGroup !== null}
        onOpenChange={(open) => { if (!open) setPendingDeleteGroup(null); }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete group?</AlertDialogTitle>
            <AlertDialogDescription>
              “{pendingDeleteGroup?.name}” will be removed. Its connections are
              kept and become ungrouped.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                const target = pendingDeleteGroup;
                setPendingDeleteGroup(null);
                if (target?.id) deleteGroup.mutate(target.id as string);
              }}
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={pendingDelete !== null}
        onOpenChange={(open) => { if (!open) setPendingDelete(null); }}
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
            <AlertDialogAction onClick={() => void confirmDelete()}>Delete</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
