"use client";

import { MoreHorizontal } from "lucide-react";
import { Fragment } from "react";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuHint,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  getDatabaseActions,
  type DatabaseActionId,
} from "@/lib/database-actions";

const triggerClass =
  "shrink-0 rounded p-0.5 text-muted-foreground opacity-0 transition-opacity hover:text-foreground focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)] group-hover:opacity-100";

/**
 * Database context-menu dispatch. `database` is the explicit target captured at
 * the node that owns the menu, so an action never resolves its target from the
 * Explorer's current selection or the active tab.
 */
export type DatabaseMenuHandler = (
  action: DatabaseActionId,
  database: string,
) => void;

/**
 * Database node context menu (PRF-02/T16). Shared by the server-level database
 * explorer (lazy per-database nodes) and the single-database tree so both node
 * kinds expose the same actions. Opening the menu performs no network request;
 * the target database is captured by the owning node.
 */
export function DatabaseMenu({
  database,
  engine,
  open,
  onOpenChange,
  onAction,
}: {
  database: string;
  engine?: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Receives an enabled action id; disabled items never dispatch. */
  onAction: (action: DatabaseActionId) => void;
}) {
  const actions = getDatabaseActions({ engine });
  return (
    <DropdownMenu open={open} onOpenChange={onOpenChange} modal={false}>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={`More actions for ${database}`}
          title={`Actions for ${database} (Shift+F10)`}
          className={triggerClass}
        >
          <MoreHorizontal size={12} aria-hidden="true" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start">
        {actions.map((action) => (
          <Fragment key={action.id}>
            {action.separatorBefore ? <DropdownMenuSeparator /> : null}
            <DropdownMenuItem
              disabled={action.disabled}
              data-testid={`database-menu-${action.id}`}
              onSelect={() => onAction(action.id)}
            >
              {action.label}
              {action.reason ? (
                <DropdownMenuHint>{action.reason}</DropdownMenuHint>
              ) : null}
            </DropdownMenuItem>
          </Fragment>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
