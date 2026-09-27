"use client";

import { create } from "zustand";

/** User-facing PWA lifecycle state (client-only, never persisted). */
interface PwaStore {
  /** True when a new service worker is installed and waiting to activate. */
  updateReady: boolean;
  setUpdateReady: (ready: boolean) => void;
}

export const usePwaStore = create<PwaStore>((set) => ({
  updateReady: false,
  setUpdateReady: (updateReady) => set({ updateReady }),
}));
