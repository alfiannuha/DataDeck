"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  deleteDatabase,
  deleteDatabasePreflight,
} from "@/lib/api/endpoints";
import { mutationErrorMessage } from "@/lib/mutation-errors";
import { queryKeys } from "@/lib/query-keys";
import {
  useDeleteDatabaseStore,
  type DeleteDatabaseTarget,
} from "@/store/useDeleteDatabaseStore";

/**
 * Delete-database dialog (PRF-02/T18). Two-phase: preflight (obtains a
 * short-lived, single-use token bound to the exact target) then execute with a
 * typed database-name confirmation and an acknowledgment. Success is shown only
 * after the backend verifies the database is gone. The target is the explicit
 * database captured by the context menu — never the active tab or Explorer
 * selection.
 */
export function DeleteDatabaseDialog() {
  const target = useDeleteDatabaseStore((state) => state.target);
  if (!target) return null;
  return <DeleteDatabaseDialogBody target={target} />;
}

type Stage = "preflight" | "ready" | "blocked" | "deleting" | "deleted" | "unknown";

function DeleteDatabaseDialogBody({ target }: { target: DeleteDatabaseTarget }) {
  const close = useDeleteDatabaseStore((state) => state.close);
  const queryClient = useQueryClient();
  const [token, setToken] = useState<string | null>(null);
  const [stage, setStage] = useState<Stage>("preflight");
  const [blockedReason, setBlockedReason] = useState<string | null>(null);
  const [typed, setTyped] = useState("");
  const [acknowledged, setAcknowledged] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const busyRef = useRef(false);

  useEffect(() => {
    let cancelled = false;
    deleteDatabasePreflight(target.connectionId, target.database)
      .then((preflight) => {
        if (cancelled) return;
        setToken(preflight.token ?? null);
        setStage("ready");
      })
      .catch((cause) => {
        if (cancelled) return;
        setBlockedReason(mutationErrorMessage(cause, "This database cannot be deleted."));
        setStage("blocked");
      });
    return () => {
      cancelled = true;
    };
  }, [target.connectionId, target.database]);

  const canDelete =
    stage === "ready" && token !== null && typed === target.database && acknowledged;

  async function submit() {
    if (busyRef.current || !canDelete || !token) return;
    busyRef.current = true;
    setStage("deleting");
    setError(null);
    try {
      await deleteDatabase(target.connectionId, target.database, {
        confirmationToken: token,
        typedDatabaseName: typed,
        acknowledged,
      });
      // Verified deleted: refresh the Explorer so the database disappears.
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.databases(target.connectionId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.schemaRoot(target.connectionId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.tableDataRoot(target.connectionId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.tableStructureRoot(target.connectionId) }),
      ]);
      setStage("deleted");
    } catch (cause) {
      const code = cause && typeof cause === "object" && "code" in cause ? String(cause.code) : "";
      setError(mutationErrorMessage(cause, "Delete failed."));
      setStage(code === "DATABASE_DELETE_OUTCOME_UNKNOWN" ? "unknown" : "blocked");
      busyRef.current = false;
    }
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && stage !== "deleting") close();
      }}
    >
      <DialogContent aria-labelledby="delete-database-title" data-testid="delete-database-dialog">
        <DialogHeader>
          <DialogTitle id="delete-database-title">Delete Database</DialogTitle>
          <DialogDescription>
            This permanently deletes the database and its contents.
          </DialogDescription>
        </DialogHeader>

        <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs text-foreground">
          <dt className="text-muted-foreground">Database</dt>
          <dd className="font-medium">{target.database}</dd>
          <dt className="text-muted-foreground">Engine</dt>
          <dd>{target.engine}</dd>
        </dl>

        {stage === "preflight" ? (
          <p className="text-xs text-muted-foreground" data-testid="delete-database-preflight">
            Checking the target…
          </p>
        ) : null}

        {stage === "blocked" ? (
          <p role="alert" className="text-xs text-destructive" data-testid="delete-database-blocked">
            {error ?? blockedReason}
          </p>
        ) : null}

        {stage === "ready" || stage === "deleting" ? (
          <div className="flex flex-col gap-2">
            <p className="text-xs font-medium text-destructive">
              WARNING: this action cannot be undone.
            </p>
            <label className="flex flex-col gap-1">
              <span className="text-xs text-foreground">
                Type &quot;{target.database}&quot; to confirm
              </span>
              <Input
                value={typed}
                onChange={(event) => setTyped(event.target.value)}
                disabled={stage === "deleting"}
                data-testid="delete-database-typed"
              />
            </label>
            <label className="flex items-center gap-2 text-xs text-foreground">
              <input
                type="checkbox"
                checked={acknowledged}
                onChange={(event) => setAcknowledged(event.target.checked)}
                disabled={stage === "deleting"}
                data-testid="delete-database-ack"
              />
              I understand this action cannot be undone.
            </label>
          </div>
        ) : null}

        {stage === "deleted" ? (
          <p className="text-xs text-foreground" data-testid="delete-database-done">
            Database {target.database} was deleted and verified gone.
          </p>
        ) : null}

        {stage === "unknown" ? (
          <p role="alert" className="text-xs text-destructive" data-testid="delete-database-unknown">
            {error} The deletion outcome is unknown — verify the database state before retrying.
          </p>
        ) : null}

        {error && stage !== "blocked" && stage !== "unknown" ? (
          <p role="alert" className="text-xs text-destructive" data-testid="delete-database-error">
            {error}
          </p>
        ) : null}

        <DialogFooter>
          <Button
            variant="ghost"
            onClick={close}
            disabled={stage === "deleting"}
            data-testid="delete-database-cancel"
          >
            {stage === "deleted" ? "Close" : "Cancel"}
          </Button>
          {stage !== "deleted" && stage !== "blocked" && stage !== "unknown" ? (
            <Button
              variant="destructive"
              onClick={() => void submit()}
              disabled={!canDelete}
              data-testid="delete-database-submit"
            >
              {stage === "deleting" ? "Deleting…" : "Delete Database"}
            </Button>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
