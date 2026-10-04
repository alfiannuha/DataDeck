"use client";

import { Bookmark, Pencil, Trash2 } from "lucide-react";
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
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useConnections } from "@/hooks/use-connections";
import { useDeleteSavedQuery, useSavedQueries } from "@/hooks/use-saved-queries";
import { ApiClientError } from "@/lib/api-client";
import { useExecutionStore } from "@/store/useExecutionStore";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";
import type { SavedQueryResponse } from "@/types/api";

import { SaveQueryDialog } from "./save-query-dialog";

/**
 * Saved Queries panel: list, open into a tab (bound to its connection when it
 * still exists), edit and delete. Opening never executes SQL.
 */
export function SavedQueriesDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { data: connections } = useConnections();
  const [page, setPage] = useState(1);
  const saved = useSavedQueries(undefined, page, open);
  const deleteSavedQuery = useDeleteSavedQuery();
  const insertQuerySql = useWorkspaceStore((state) => state.insertQuerySql);
  const setTabSavedQuery = useWorkspaceStore((state) => state.setTabSavedQuery);
  const markActiveClean = useWorkspaceStore((state) => state.markActiveClean);

  const [editing, setEditing] = useState<SavedQueryResponse | null>(null);
  const [pendingDelete, setPendingDelete] = useState<SavedQueryResponse | null>(
    null,
  );

  function connectionName(connectionId: string | null | undefined) {
    if (!connectionId) return null;
    return connections?.find((c) => c.id === connectionId)?.name ?? null;
  }

  function openEntry(entry: SavedQueryResponse) {
    const sql = entry.sql_text ?? "";
    if (!sql) return;
    useExecutionStore.getState().reset();
    const stillExists = connections?.some((c) => c.id === entry.connection_id);
    insertQuerySql(
      sql,
      stillExists ? entry.connection_id ?? null : null,
      stillExists ? entry.database_name ?? null : null,
    );
    const activeTabId = useWorkspaceStore.getState().activeTabId;
    if (activeTabId) {
      setTabSavedQuery(activeTabId, {
        id: entry.id ?? null,
        title: entry.title ?? null,
        tags: entry.tags ?? null,
      });
      markActiveClean();
    }
    onOpenChange(false);
  }

  const records = saved.data?.items ?? [];
  const meta = saved.data?.meta;

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent aria-labelledby="saved-queries-title" className="max-w-2xl">
          <DialogHeader>
            <DialogTitle id="saved-queries-title">Saved queries</DialogTitle>
            <DialogDescription>
              Reusable SQL snippets. Opening one loads it into a tab without
              running it.
            </DialogDescription>
          </DialogHeader>

          {saved.isLoading ? (
            <p className="py-6 text-center text-xs text-muted-foreground">
              Loading saved queries…
            </p>
          ) : saved.isError ? (
            <p role="alert" className="py-6 text-center text-xs text-destructive">
              {saved.error instanceof ApiClientError
                ? `${saved.error.message} (${saved.error.code})`
                : "Could not load saved queries."}
            </p>
          ) : records.length === 0 ? (
            <p className="py-6 text-center text-xs text-muted-foreground">
              No saved queries yet. Use Cmd/Ctrl+S in the editor to save one.
            </p>
          ) : (
            <ul
              aria-label="Saved query entries"
              className="flex max-h-96 flex-col gap-1 overflow-auto pr-1"
            >
              {records.map((entry) => {
                const name = connectionName(entry.connection_id);
                const missing = Boolean(entry.connection_id) && !name;
                return (
                  <li
                    key={entry.id}
                    className="rounded-md border border-border bg-panel p-2"
                  >
                    <div className="flex items-center gap-2">
                      <Bookmark
                        size={12}
                        aria-hidden="true"
                        className="shrink-0 text-muted-foreground"
                      />
                      <span className="truncate text-xs font-medium text-foreground">
                        {entry.title}
                      </span>
                      <span className="ml-auto shrink-0 text-[10px] text-muted-foreground">
                        {entry.updated_at
                          ? new Date(entry.updated_at).toLocaleDateString()
                          : "—"}
                      </span>
                    </div>
                    <div className="mt-0.5 flex items-center gap-2 text-[10px] text-muted-foreground">
                      <span>
                        {entry.connection_id
                          ? (name ?? "connection removed")
                          : "no connection"}
                      </span>
                      {entry.tags && <span>· #{entry.tags}</span>}
                    </div>
                    <p
                      className="mt-1 truncate font-mono text-xs text-foreground"
                      title={entry.sql_text ?? ""}
                    >
                      {(entry.sql_text ?? "").replace(/\s+/g, " ").slice(0, 140)}
                    </p>
                    <div className="mt-1 flex items-center gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => openEntry(entry)}
                      >
                        Open in editor
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label={`Edit ${entry.title}`}
                        onClick={() => setEditing(entry)}
                      >
                        <Pencil size={12} aria-hidden="true" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label={`Delete ${entry.title}`}
                        onClick={() => setPendingDelete(entry)}
                      >
                        <Trash2 size={12} aria-hidden="true" />
                      </Button>
                      {missing && (
                        <span className="text-[10px] text-destructive">
                          Original connection no longer exists — choose one
                          before running.
                        </span>
                      )}
                    </div>
                  </li>
                );
              })}
            </ul>
          )}

          {meta && meta.total_pages > 1 && (
            <div className="mt-2 flex items-center justify-center gap-3 text-xs text-muted-foreground">
              <Button
                variant="outline"
                size="sm"
                disabled={page <= 1}
                onClick={() => setPage((current) => Math.max(1, current - 1))}
              >
                Previous
              </Button>
              <span>
                Page {meta.page} of {meta.total_pages}
              </span>
              <Button
                variant="outline"
                size="sm"
                disabled={page >= meta.total_pages}
                onClick={() => setPage((current) => current + 1)}
              >
                Next
              </Button>
            </div>
          )}
        </DialogContent>
      </Dialog>

      {editing && (
        <SaveQueryDialog
          open
          onOpenChange={(next) => {
            if (!next) setEditing(null);
          }}
          sql={editing.sql_text ?? ""}
          connectionId={editing.connection_id ?? null}
          existing={{
            id: editing.id ?? "",
            title: editing.title ?? "",
            tags: editing.tags ?? null,
          }}
          onSaved={() => setEditing(null)}
        />
      )}

      <AlertDialog
        open={pendingDelete !== null}
        onOpenChange={(next) => {
          if (!next) setPendingDelete(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete saved query?</AlertDialogTitle>
            <AlertDialogDescription>
              {pendingDelete
                ? `“${pendingDelete.title}” will be removed. The underlying database connection is not affected.`
                : ""}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                const target = pendingDelete;
                setPendingDelete(null);
                if (target?.id) void deleteSavedQuery.mutateAsync(target.id);
              }}
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
