import { create } from "zustand";

/** A single SQL query tab in the workspace (client-only state). */
export interface QueryTab {
  id: string;
  title: string;
  sql: string;
  /** Connection this tab is bound to. Null means "not yet bound". */
  connectionId: string | null;
  /**
   * Database this tab executes against (PRF-01). For server-level PostgreSQL
   * connections this is the explicitly selected database; for MySQL/SQLite and
   * legacy profiles null lets the backend use the profile's database. Never
   * inherited from global UI state once a tab is bound.
   */
  database?: string | null;
  dirty: boolean;
  /** When this tab was saved, the persisted saved-query id/title/tags. */
  savedQueryId?: string | null;
  savedTitle?: string | null;
  savedTags?: string | null;
}

interface WorkspaceState {
  sidebarCollapsed: boolean;
  toggleSidebar: () => void;
  setSidebarCollapsed: (collapsed: boolean) => void;

  tabs: QueryTab[];
  activeTabId: string | null;
  setActiveTab: (id: string) => void;
  updateActiveSql: (sql: string) => void;
  /**
   * Insert generated SQL (e.g. Select Top 100) without executing it, binding the
   * tab (or a new tab) to the given connection and database context.
   */
  insertQuerySql: (
    sql: string,
    connectionId: string | null,
    database?: string | null,
  ) => void;
  addTab: (connectionId: string | null, database?: string | null) => void;
  closeTab: (id: string) => void;
  /** Explicitly bind a tab to a connection (never done implicitly on switch). */
  setTabConnection: (id: string, connectionId: string | null) => void;
  /** Explicitly bind a tab to a database (PRF-01; never redirects other tabs). */
  setTabDatabase: (id: string, database: string | null) => void;
  /** Record that a tab is linked to a persisted saved query. */
  setTabSavedQuery: (
    id: string,
    saved: { id: string | null; title: string | null; tags: string | null },
  ) => void;
  /** Mark the active tab as saved/executed (no unsaved changes). */
  markActiveClean: () => void;
}

function newTab(
  title: string,
  connectionId: string | null,
  database: string | null = null,
): QueryTab {
  return {
    id: `tab-${Math.random().toString(36).slice(2, 10)}`,
    title,
    sql: "",
    connectionId,
    database,
    dirty: false,
  };
}

const initialTab = newTab("Query 1", null);

/**
 * Client workspace state: sidebar collapse and the query tab workspace. Server
 * data lives in TanStack Query; SQL text is transient and never persisted.
 */
export const useWorkspaceStore = create<WorkspaceState>((set) => ({
  sidebarCollapsed: false,
  toggleSidebar: () =>
    set((state) => ({ sidebarCollapsed: !state.sidebarCollapsed })),
  setSidebarCollapsed: (sidebarCollapsed) => set({ sidebarCollapsed }),

  tabs: [initialTab],
  activeTabId: initialTab.id,
  setActiveTab: (id) =>
    set((state) =>
      state.tabs.some((tab) => tab.id === id) ? { activeTabId: id } : state,
    ),
  updateActiveSql: (sql) =>
    set((state) => ({
      tabs: state.tabs.map((tab) =>
        tab.id === state.activeTabId ? { ...tab, sql, dirty: true } : tab,
      ),
    })),
  insertQuerySql: (sql, connectionId, database = null) =>
    set((state) => {
      const active = state.tabs.find((tab) => tab.id === state.activeTabId);
      // Reuse an untouched empty tab that is not already bound elsewhere;
      // otherwise open a new one. Dirty policy for generated SQL: an inserted
      // statement keeps the tab clean because it is reproducible scaffolding.
      // Manual edits flow through updateActiveSql and mark the tab dirty.
      const reusable =
        active &&
        !active.dirty &&
        active.sql.trim() === "" &&
        (active.connectionId === null || active.connectionId === connectionId);
      if (reusable && active) {
        return {
          tabs: state.tabs.map((tab) =>
            tab.id === active.id
              ? { ...tab, sql, connectionId, database }
              : tab,
          ),
        };
      }
      const tab = {
        ...newTab(`Query ${state.tabs.length + 1}`, connectionId, database),
        sql,
      };
      return { tabs: [...state.tabs, tab], activeTabId: tab.id };
    }),
  addTab: (connectionId, database = null) =>
    set((state) => {
      const tab = newTab(`Query ${state.tabs.length + 1}`, connectionId, database);
      return { tabs: [...state.tabs, tab], activeTabId: tab.id };
    }),
  setTabConnection: (id, connectionId) =>
    set((state) => ({
      tabs: state.tabs.map((tab) =>
        tab.id === id ? { ...tab, connectionId } : tab,
      ),
    })),
  setTabDatabase: (id, database) =>
    set((state) => ({
      tabs: state.tabs.map((tab) =>
        tab.id === id ? { ...tab, database } : tab,
      ),
    })),
  setTabSavedQuery: (id, saved) =>
    set((state) => ({
      tabs: state.tabs.map((tab) =>
        tab.id === id
          ? {
              ...tab,
              savedQueryId: saved.id,
              savedTitle: saved.title,
              savedTags: saved.tags,
            }
          : tab,
      ),
    })),
  closeTab: (id) =>
    set((state) => {
      const index = state.tabs.findIndex((tab) => tab.id === id);
      if (index === -1) return state;

      let tabs = state.tabs.filter((tab) => tab.id !== id);
      if (tabs.length === 0) {
        tabs = [newTab("Query 1", null)];
      }

      let activeTabId = state.activeTabId;
      if (activeTabId === id) {
        const next = tabs[Math.min(index, tabs.length - 1)];
        activeTabId = next.id;
      }
      return { tabs, activeTabId };
    }),
  markActiveClean: () =>
    set((state) => ({
      tabs: state.tabs.map((tab) =>
        tab.id === state.activeTabId ? { ...tab, dirty: false } : tab,
      ),
    })),
}));
