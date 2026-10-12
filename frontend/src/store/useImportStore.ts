"use client";

import { create } from "zustand";

/**
 * Explicit import target (PRF-02/T15). Captured from the originating Table Data
 * tab or explorer table menu — never resolved from the active tab or the
 * current Explorer selection, so a background tab can never leak its binding.
 */
export interface ImportTarget {
  connectionId: string;
  database: string | null;
  schema: string | null;
  table: string;
  /** Canonical object type ("BASE TABLE", "VIEW", ...). Import needs a base table. */
  objectType?: string;
}

interface ImportStore {
  target: ImportTarget | null;
  open: (target: ImportTarget) => void;
  close: () => void;
}

/** Transient UI state for the import dialog (never persisted). */
export const useImportStore = create<ImportStore>((set) => ({
  target: null,
  open: (target) => set({ target }),
  close: () => set({ target: null }),
}));