"use client";

import { create } from "zustand";

import type { TableFilter, TableSort } from "@/store/useWorkspaceStore";

/**
 * Explicit export target (PRF-02/T14). Captured from the originating Table Data
 * tab or explorer table menu — never resolved from the active tab or the current
 * Explorer selection, so a background tab can never leak its binding (α/β).
 */
export interface ExportTarget {
  connectionId: string;
  database: string | null;
  schema: string | null;
  table: string;
  /** Canonical object type ("BASE TABLE", "VIEW", ...). SQL needs a base table. */
  objectType?: string;
  /** Filter set captured from the Table Data tab (server-side, ANDed). */
  filters?: TableFilter[];
  /** Single-column sort captured from the Table Data tab. */
  sort?: TableSort | null;
}

interface ExportStore {
  target: ExportTarget | null;
  open: (target: ExportTarget) => void;
  close: () => void;
}

/** Transient UI state for the export dialog (never persisted). */
export const useExportStore = create<ExportStore>((set) => ({
  target: null,
  open: (target) => set({ target }),
  close: () => set({ target: null }),
}));
