"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";

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
import { applyStructure } from "@/lib/api/endpoints";
import { mutationErrorMessage } from "@/lib/mutation-errors";
import { queryKeys } from "@/lib/query-keys";

export interface DeleteTableTarget {
  connectionId: string;
  connectionName?: string;
  database: string;
  schema: string;
  table: string;
}

type Step = "warn" | "confirm" | "deleting" | "deleted";

/**
 * Two-step Delete Table (PRF-05/E). Step 1 warns with the full target; step 2
 * requires typing the exact table name. The DROP runs through the backend DDL
 * layer (quoted identifier, never CASCADE) and the schema explorer is refreshed
 * on success.
 */
export function DeleteTableDialog({
  target,
  onClose,
}: {
  target: DeleteTableTarget;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const [step, setStep] = useState<Step>("warn");
  const [typed, setTyped] = useState("");
  const [error, setError] = useState<string | null>(null);
  const busyRef = useRef(false);

  const targetLabel = target.schema ? `${target.schema}.${target.table}` : target.table;
  const nameMatches = typed.trim() === target.table;
  const canDelete = step === "confirm" && nameMatches;
  const deleting = step === "deleting";

  async function submit() {
    if (busyRef.current || !canDelete) return;
    busyRef.current = true;
    setStep("deleting");
    setError(null);
    try {
      await applyStructure(target.connectionId, {
        operation: "drop_table",
        database: target.database || undefined,
        schema: target.schema || undefined,
        table: target.table,
      });
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.schemaRoot(target.connectionId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.databases(target.connectionId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.tableDataRoot(target.connectionId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.tableStructureRoot(target.connectionId) }),
      ]);
      setStep("deleted");
    } catch (cause) {
      setError(mutationErrorMessage(cause, "Delete failed."));
      setStep("confirm");
      busyRef.current = false;
    }
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && step !== "deleting") onClose();
      }}
    >
      <DialogContent aria-labelledby="delete-table-title" data-testid="delete-table-dialog">
        <DialogHeader>
          <DialogTitle id="delete-table-title">Delete Table</DialogTitle>
          <DialogDescription>
            {step === "warn"
              ? "This permanently deletes the table and all of its data."
              : "Type the exact table name to confirm."}
          </DialogDescription>
        </DialogHeader>

        <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs text-foreground">
          {target.connectionName ? (
            <>
              <dt className="text-muted-foreground">Connection</dt>
              <dd>{target.connectionName}</dd>
            </>
          ) : null}
          <dt className="text-muted-foreground">Database</dt>
          <dd>{target.database || "—"}</dd>
          {target.schema ? (
            <>
              <dt className="text-muted-foreground">Schema</dt>
              <dd>{target.schema}</dd>
            </>
          ) : null}
          <dt className="text-muted-foreground">Table</dt>
          <dd className="font-medium">{target.table}</dd>
        </dl>

        <p className="text-xs font-medium text-destructive">
          Warning: this action cannot be undone.
        </p>

        {step === "confirm" || step === "deleting" ? (
          <label className="flex flex-col gap-1">
            <span className="text-xs text-foreground">
              Type &quot;{target.table}&quot; to confirm deletion
            </span>
            <Input
              value={typed}
              onChange={(event) => setTyped(event.target.value)}
              disabled={deleting}
              data-testid="delete-table-typed"
              autoFocus
            />
          </label>
        ) : null}

        {error ? (
          <p role="alert" className="text-xs text-destructive" data-testid="delete-table-error">
            {error}
          </p>
        ) : null}

        {step === "deleted" ? (
          <p className="text-xs text-foreground" data-testid="delete-table-done">
            {targetLabel} was deleted.
          </p>
        ) : null}

        <DialogFooter>
          <Button
            variant="ghost"
            onClick={onClose}
            disabled={deleting}
            data-testid="delete-table-cancel"
          >
            {step === "deleted" ? "Close" : "Cancel"}
          </Button>
          {step === "warn" ? (
            <Button
              variant="destructive"
              className="text-destructive"
              onClick={() => setStep("confirm")}
              data-testid="delete-table-continue"
            >
              Continue
            </Button>
          ) : null}
          {step === "confirm" || step === "deleting" ? (
            <Button
              variant="destructive"
              onClick={() => void submit()}
              disabled={!canDelete || deleting}
              data-testid="delete-table-submit"
            >
              {deleting ? "Deleting…" : "Delete Table"}
            </Button>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
