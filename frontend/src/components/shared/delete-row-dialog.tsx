"use client";

import { useRef, useState } from "react";

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
import { deleteRow } from "@/lib/api/endpoints";
import { mutationErrorMessage } from "@/lib/mutation-errors";

/**
 * Explicit Delete confirmation (PRF-02/T10). Displays the target and canonical
 * row identity, uses optimistic concurrency via the original snapshot, and never
 * deletes without confirmation. No inline grid deletion.
 */
export function DeleteRowDialog({
  connectionId,
  database,
  schema,
  table,
  identity,
  expected,
  onClose,
  onDeleted,
}: {
  connectionId: string;
  database: string | null;
  schema: string | null;
  table: string;
  identity: Record<string, unknown>;
  expected: Record<string, unknown>;
  onClose: () => void;
  onDeleted: () => void;
}) {
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const submittingRef = useRef(false);

  const identityLabel = Object.entries(identity)
    .map(([key, value]) => `${key} = ${value === null ? "NULL" : String(value)}`)
    .join(", ");

  async function confirm() {
    if (submittingRef.current) return;
    submittingRef.current = true;
    setSubmitting(true);
    setError(null);
    try {
      await deleteRow(connectionId, { database, schema, table, identity, expected });
      onDeleted();
    } catch (cause) {
      // Dialog stays open so the user sees the conflict and can cancel (T11 §16);
      // refreshing is explicit — no auto-reload here.
      setError(mutationErrorMessage(cause, "Delete failed."));
      submittingRef.current = false;
      setSubmitting(false);
    }
  }

  return (
    <AlertDialog
      open
      onOpenChange={(open) => {
        if (!open && !submittingRef.current) onClose();
      }}
    >
      <AlertDialogContent data-testid="delete-row-dialog">
        <AlertDialogHeader>
          <AlertDialogTitle>Delete Row?</AlertDialogTitle>
          <AlertDialogDescription>
            Target: {database ?? "default database"} / {schema ? `${schema}.` : ""}
            {table}
            <br />
            Identity: {identityLabel}
            <br />
            This action cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>

        {error ? (
          <p role="alert" className="text-xs text-destructive">
            {error}
          </p>
        ) : null}

        <AlertDialogFooter>
          <AlertDialogCancel disabled={submitting}>Cancel</AlertDialogCancel>
          <AlertDialogAction
            disabled={submitting}
            onClick={(event) => {
              event.preventDefault();
              void confirm();
            }}
            className="bg-destructive text-destructive-foreground hover:opacity-90"
          >
            {submitting ? "Deleting…" : "Delete"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
