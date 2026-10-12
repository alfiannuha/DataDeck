"use client";

import { create } from "zustand";

/**
 * Explicit database-level script target (PRF-02/T17). Captured from the
 * originating database node's context menu — never resolved from the Explorer
 * selection or the active tab, so a background database can never leak its
 * binding into an export or restore.
 */
export interface DatabaseScriptTarget {
  connectionId: string;
  database: string;
  engine: string;
}

export type DatabaseScriptKind = "export" | "import";

interface DatabaseScriptStore {
  request: { kind: DatabaseScriptKind; target: DatabaseScriptTarget } | null;
  open: (kind: DatabaseScriptKind, target: DatabaseScriptTarget) => void;
  close: () => void;
}

/** Transient UI state for the database export/restore dialogs (never persisted). */
export const useDatabaseScriptStore = create<DatabaseScriptStore>((set) => ({
  request: null,
  open: (kind, target) => set({ request: { kind, target } }),
  close: () => set({ request: null }),
}));
