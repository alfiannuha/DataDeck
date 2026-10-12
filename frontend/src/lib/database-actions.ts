/**
 * Capability descriptor for database-level actions (PRF-02/T16).
 *
 * A single source of truth for what a database context menu may offer. The menu
 * renders exactly these items; deferred actions (Import SQL / Export SQL, owned
 * by T17) stay visible, disabled and focusable with an inline explanation —
 * never enabled no-ops. "Delete Database" (T18) is intentionally absent rather
 * than shown as an inert button.
 *
 * Reconnect is supported by all three engines (PostgreSQL: per-database pool;
 * MySQL/SQLite: the profile's single pool), so this menu is engine-agnostic and
 * contains no engine-name checks.
 */

export type DatabaseActionId =
  | "new-query"
  | "refresh"
  | "reconnect"
  | "import-sql"
  | "export-sql"
  | "copy-name"
  | "delete-database";

export interface DatabaseActionDescriptor {
  id: DatabaseActionId;
  label: string;
  disabled: boolean;
  /** Why the action is unavailable; shown under disabled items. */
  reason?: string;
  /** Render a group separator above this item. */
  separatorBefore?: boolean;
  /** Destructive styling and placement in the final section. */
  destructive?: boolean;
}

/**
 * Menu items in display order, grouped as in the ADR-010 §11 design:
 * navigation/connection actions, then the SQL import/export group, then
 * clipboard. Export SQL is available for every engine; Import SQL / Restore is
 * disabled for engines without a safe atomic restore (MySQL — DDL implicitly
 * commits), shown disabled with an explanation rather than as an enabled no-op.
 */
export function getDatabaseActions(
  target: { engine?: string } = {},
): DatabaseActionDescriptor[] {
  const restoreSupported = target.engine !== "mysql";
  const deleteSupported =
    target.engine === "postgres" || target.engine === "mysql";
  return [
    { id: "new-query", label: "New Query", disabled: false },
    { id: "refresh", label: "Refresh Database", disabled: false },
    { id: "reconnect", label: "Reconnect", disabled: false },
    {
      id: "export-sql",
      label: "Export SQL",
      disabled: false,
      separatorBefore: true,
    },
    {
      id: "import-sql",
      label: "Import SQL (Restore)",
      disabled: !restoreSupported,
      reason: restoreSupported
        ? undefined
        : "Restore is not supported for MySQL.",
    },
    {
      id: "copy-name",
      label: "Copy Database Name",
      disabled: false,
      separatorBefore: true,
    },
    {
      id: "delete-database",
      label: "Delete Database",
      disabled: !deleteSupported,
      reason: deleteSupported
        ? undefined
        : "Delete is not supported for this engine.",
      separatorBefore: true,
      destructive: true,
    },
  ];
}
