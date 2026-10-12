"use client";

import { create } from "zustand";

/**
 * Explicit delete-database target (PRF-02/T18). Captured by the database node's
 * context menu — never resolved from the Explorer selection or the active tab.
 * The typed name in the dialog is only proof of intent; the backend validates
 * the authoritative target.
 */
export interface DeleteDatabaseTarget {
  connectionId: string;
  database: string;
  engine: string;
}

interface DeleteDatabaseStore {
  target: DeleteDatabaseTarget | null;
  open: (target: DeleteDatabaseTarget) => void;
  close: () => void;
}

/** Transient UI state for the delete-database dialog (never persisted). */
export const useDeleteDatabaseStore = create<DeleteDatabaseStore>((set) => ({
  target: null,
  open: (target) => set({ target }),
  close: () => set({ target: null }),
}));
