import { create } from "zustand";

/**
 * Active connection and explorer selection (client workspace state). Server data
 * lives in TanStack Query; only ids/names are kept here.
 *
 * `activeDatabaseByConnection` is the EXPLORER selection: the database the user
 * is browsing. It is deliberately separate from a query tab's binding
 * (`QueryTab.database`) — selecting a database never redirects an existing tab;
 * it only seeds NEW tabs (PRF-01).
 */
interface ConnectionState {
  activeConnectionId: string | null;
  activeDatabaseByConnection: Record<string, string | null>;
  setActiveConnection: (id: string | null) => void;
  /** Record the database selected while browsing a connection. */
  setActiveDatabase: (connectionId: string, database: string | null) => void;
}

export const useConnectionStore = create<ConnectionState>((set) => ({
  activeConnectionId: null,
  activeDatabaseByConnection: {},
  setActiveConnection: (activeConnectionId) => set({ activeConnectionId }),
  setActiveDatabase: (connectionId, database) =>
    set((state) => ({
      activeDatabaseByConnection: {
        ...state.activeDatabaseByConnection,
        [connectionId]: database,
      },
    })),
}));
