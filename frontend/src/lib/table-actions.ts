/**
 * Capability descriptor for table-level actions (PRF-02/T12).
 *
 * A single source of truth for what a table context menu may offer. The menu
 * renders exactly these items; unavailable actions stay visible, disabled and
 * focusable with an inline explanation (never hidden, never silently failed).
 * Backend capabilities remain source of truth for row mutations; this
 * descriptor only drives the table menu's own surface.
 */

export type TableActionId =
  | "view-data"
  | "new-query"
  | "structure"
  | "refresh"
  | "export"
  | "import"
  | "copy-name"
  | "copy-qualified";

export interface TableActionDescriptor {
  id: TableActionId;
  label: string;
  disabled: boolean;
  /** Why the action is unavailable; shown under disabled items. */
  reason?: string;
}

/** Views / foreign tables are read-only objects for UX wording (backend decides). */
export function isReadOnlyObject(objectType: string | undefined): boolean {
  const type = (objectType ?? "").toUpperCase();
  return type.includes("VIEW") || type.includes("FOREIGN");
}

/**
 * Import lands in PRF-02/T15: enabled for writable objects only, disabled with
 * an explanation for views/foreign tables. Export (T14) is live.
 */

/**
 * Menu items in display order. `import` is gated on the object being writable:
 * importing into a view/foreign table is meaningless. `export` is a read
 * operation, so it stays enabled for views too (the dialog disables SQL there).
 */
export function getTableActions(target: {
  objectType?: string;
}): TableActionDescriptor[] {
  const readOnly = isReadOnlyObject(target.objectType);
  return [
    { id: "view-data", label: "View Data", disabled: false },
    { id: "new-query", label: "New Query", disabled: false },
    { id: "structure", label: "Structure", disabled: false },
    { id: "refresh", label: "Refresh", disabled: false },
    { id: "export", label: "Export", disabled: false },
    {
      id: "import",
      label: "Import",
      disabled: readOnly,
      reason: readOnly ? "This object is read-only." : undefined,
    },
    { id: "copy-name", label: "Copy Table Name", disabled: false },
    { id: "copy-qualified", label: "Copy Qualified Name", disabled: false },
  ];
}