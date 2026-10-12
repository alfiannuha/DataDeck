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
  | "copy-name";

export interface DatabaseActionDescriptor {
  id: DatabaseActionId;
  label: string;
  disabled: boolean;
  /** Why the action is unavailable; shown under disabled items. */
  reason?: string;
  /** Render a group separator above this item. */
  separatorBefore?: boolean;
}

/**
 * Menu items in display order, grouped as in the ADR-010 §11 design:
 * navigation/connection actions, then the (deferred) SQL import/export group,
 * then clipboard.
 */
export function getDatabaseActions(): DatabaseActionDescriptor[] {
  return [
    { id: "new-query", label: "New Query", disabled: false },
    { id: "refresh", label: "Refresh Database", disabled: false },
    { id: "reconnect", label: "Reconnect", disabled: false },
    {
      id: "import-sql",
      label: "Import SQL",
      disabled: true,
      reason: "Database import arrives in a later update.",
      separatorBefore: true,
    },
    {
      id: "export-sql",
      label: "Export SQL",
      disabled: true,
      reason: "Database export arrives in a later update.",
    },
    {
      id: "copy-name",
      label: "Copy Database Name",
      disabled: false,
      separatorBefore: true,
    },
  ];
}
