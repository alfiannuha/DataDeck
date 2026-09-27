import { create } from "zustand";

/**
 * Active connection selection (client workspace state). The connection itself
 * is server state and is fetched through TanStack Query; only the selected id
 * is kept here.
 */
interface ConnectionState {
  activeConnectionId: string | null;
  setActiveConnection: (id: string | null) => void;
}

export const useConnectionStore = create<ConnectionState>((set) => ({
  activeConnectionId: null,
  setActiveConnection: (activeConnectionId) => set({ activeConnectionId }),
}));
