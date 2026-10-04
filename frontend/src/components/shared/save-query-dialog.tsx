"use client";

import { useEffect, useState } from "react";

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
import { Label } from "@/components/ui/label";
import {
  useCreateSavedQuery,
  useUpdateSavedQuery,
} from "@/hooks/use-saved-queries";
import { ApiClientError } from "@/lib/api-client";
import type { SavedQueryResponse } from "@/types/api";

/**
 * Save (or update) the current editor SQL as a reusable saved query. The SQL is
 * taken from the caller and never executed here.
 */
export function SaveQueryDialog({
  open,
  onOpenChange,
  sql,
  connectionId,
  databaseName,
  existing,
  onSaved,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  sql: string;
  connectionId: string | null;
  /** Database context to persist with the snippet (PRF-01). */
  databaseName?: string | null;
  existing?: { id: string; title: string; tags: string | null } | null;
  onSaved: (saved: SavedQueryResponse) => void;
}) {
  const [title, setTitle] = useState("");
  const [tags, setTags] = useState("");
  const [error, setError] = useState<string | null>(null);

  const createSavedQuery = useCreateSavedQuery();
  const updateSavedQuery = useUpdateSavedQuery();
  const pending = createSavedQuery.isPending || updateSavedQuery.isPending;

  useEffect(() => {
    if (open) {
      setTitle(existing?.title ?? "");
      setTags(existing?.tags ?? "");
      setError(null);
    }
  }, [open, existing?.title, existing?.tags]);

  async function handleSave() {
    const trimmedTitle = title.trim();
    if (!trimmedTitle) {
      setError("Title is required.");
      return;
    }
    if (!sql.trim()) {
      setError("There is no SQL to save.");
      return;
    }
    const body = {
      title: trimmedTitle,
      sql_text: sql,
      tags: tags.trim() ? tags.trim() : undefined,
      connection_id: connectionId ?? undefined,
      database_name: databaseName ?? undefined,
    };
    try {
      const saved = existing
        ? await updateSavedQuery.mutateAsync({ id: existing.id, body })
        : await createSavedQuery.mutateAsync(body);
      onSaved(saved);
      onOpenChange(false);
    } catch (cause) {
      setError(
        cause instanceof ApiClientError
          ? `${cause.message} (${cause.code})`
          : "Could not save the query.",
      );
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent aria-labelledby="save-query-title">
        <DialogHeader>
          <DialogTitle id="save-query-title">
            {existing ? "Update saved query" : "Save query"}
          </DialogTitle>
          <DialogDescription>
            {connectionId
              ? `Linked to connection ${connectionId}.`
              : "Not linked to a connection."}{" "}
            Saving never executes the SQL.
          </DialogDescription>
        </DialogHeader>

        <form
          className="grid grid-cols-2 gap-3"
          onSubmit={(event) => {
            event.preventDefault();
            void handleSave();
          }}
        >
          <div className="col-span-2">
            <Label htmlFor="saved-title" className="mb-1 block">
              Title
            </Label>
            <Input
              id="saved-title"
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              aria-invalid={Boolean(error)}
            />
          </div>
          <div className="col-span-2">
            <Label htmlFor="saved-tags" className="mb-1 block">
              Tags
            </Label>
            <Input
              id="saved-tags"
              placeholder="comma,separated"
              value={tags}
              onChange={(event) => setTags(event.target.value)}
            />
          </div>

          <div className="col-span-2 min-h-5 text-xs" role="status" aria-live="polite">
            {error && <span className="text-destructive">{error}</span>}
          </div>

          <DialogFooter className="col-span-2">
            <Button type="submit" disabled={pending}>
              {pending ? "Saving…" : existing ? "Update" : "Save"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
