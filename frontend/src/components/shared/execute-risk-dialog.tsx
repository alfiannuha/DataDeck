"use client";

import { useState } from "react";

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
import type { SqlRisk } from "@/lib/sql-risk";

/**
 * High-risk SQL confirmation (PRF-05/H). The exact SQL shown here is the SQL
 * executed on confirm (captured by the caller); the dialog is modal so it cannot
 * change underneath. Very destructive statements additionally require typing
 * EXECUTE. Red styling marks it as destructive.
 */
export function ExecuteRiskDialog({
  sql,
  risk,
  database,
  onConfirm,
  onCancel,
}: {
  sql: string;
  risk: SqlRisk;
  database?: string | null;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  const [typed, setTyped] = useState("");
  const needTyped = risk.requiresTypedConfirmation;
  const canConfirm = !needTyped || typed.trim().toUpperCase() === "EXECUTE";

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onCancel();
      }}
    >
      <DialogContent aria-labelledby="risk-title" data-testid="execute-risk-dialog">
        <DialogHeader>
          <DialogTitle id="risk-title" className="text-destructive">
            High-risk query
          </DialogTitle>
          <DialogDescription>
            This statement may modify or delete a large amount of data. Review it
            before running.
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-1 text-xs text-foreground">
          <p>
            <span className="text-muted-foreground">Risk level: </span>
            <span className="font-semibold uppercase text-destructive">{risk.level}</span>
          </p>
          <p>
            <span className="text-muted-foreground">Target database: </span>
            {database || "default"}
          </p>
        </div>

        <ul className="list-disc pl-4 text-xs text-destructive">
          {risk.reasons.map((reason, index) => (
            <li key={index}>{reason}</li>
          ))}
        </ul>

        <pre
          data-testid="risk-sql"
          className="max-h-40 overflow-auto whitespace-pre-wrap break-words rounded border border-border bg-panel p-2 font-mono text-xs text-foreground"
        >
          {sql}
        </pre>

        {needTyped ? (
          <label className="flex flex-col gap-1 text-xs text-foreground">
            <span>
              Type <span className="font-mono font-semibold">EXECUTE</span> to confirm
            </span>
            <Input
              value={typed}
              onChange={(event) => setTyped(event.target.value)}
              aria-label="Type EXECUTE to confirm"
              autoFocus
            />
          </label>
        ) : null}

        <DialogFooter>
          <Button variant="ghost" onClick={onCancel}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            disabled={!canConfirm}
            onClick={onConfirm}
            data-testid="risk-confirm"
          >
            Run query
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
