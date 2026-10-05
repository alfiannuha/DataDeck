import { create } from "zustand";

/** Default bounded page size for table data tabs (ADR-010 §6). */
export const DEFAULT_TABLE_PAGE_SIZE = 100;

/** Column/operator/value filter used by Table Data tabs (ADR-010 §7). */
export type FilterOperator =
  | "equals"
  | "not_equals"
  | "contains"
  | "starts_with"
  | "ends_with"
  | "greater_than"
  | "greater_or_equal"
  | "less_than"
  | "less_or_equal"
  | "is_null"
  | "is_not_null"
  | "in";

export interface TableFilter {
  column: string;
  operator: FilterOperator;
  value?: unknown;
  values?: unknown[];
}

/** Server-side sort descriptor used by Table Data tabs (ADR-010 §8). */
export interface TableSort {
  column: string;
  direction: "asc" | "desc";
}

/**
 * Fields every database-bound tab owns. The PRF-01 invariant: a tab's
 * connection/database binding is explicit and never mutated by explorer or
 * global selection.
 */
export interface TabBase {
  id: string;
  title: string;
  connectionId: string | null;
  database: string | null;
}

/** A SQL query tab (client-only state). */
export interface QueryTab extends TabBase {
  kind: "query";
  sql: string;
  dirty: boolean;
  /** When this tab was saved, the persisted saved-query id/title/tags. */
  savedQueryId?: string | null;
  savedTitle?: string | null;
  savedTags?: string | null;
}

/**
 * A table data browser tab (PRF-02). Stores browse state only — never SQL,
 * credentials, datasets, row identity, or mutation transactions (ADR-010 §3).
 */
export interface TableDataTab extends TabBase {
  kind: "table-data";
  schema: string | null;
  table: string;
  filters: TableFilter[];
  sort: TableSort[];
  page: number;
  pageSize: number;
}

/** A read-only table structure tab (PRF-02). Binding only. */
export interface TableStructureTab extends TabBase {
  kind: "table-structure";
  schema: string | null;
  table: string;
}

export type WorkspaceTab = QueryTab | TableDataTab | TableStructureTab;

/** Type guard: narrow a workspace tab to a Query tab. */
export function isQueryTab(tab: WorkspaceTab): tab is QueryTab {
  return tab.kind === "query";
}

export interface WorkspaceState {
  sidebarCollapsed: boolean;
  toggleSidebar: () => void;
  setSidebarCollapsed: (collapsed: boolean) => void;

  tabs: WorkspaceTab[];
  activeTabId: string | null;
  setActiveTab: (id: string) => void;
  updateActiveSql: (sql: string) => void;
  /**
   * Insert generated SQL (e.g. Select Top 100) without executing it, binding the
   * tab (or a new Query tab) to the given connection and database context.
   */
  insertQuerySql: (
    sql: string,
    connectionId: string | null,
    database?: string | null,
  ) => void;
  /** Open a new Query tab (optionally pre-populated). Never reuses a tab. */
  openQuery: (
    connectionId: string | null,
    database?: string | null,
    sql?: string,
  ) => void;
  /** Add a Query tab inheriting the given binding (kept for compatibility). */
  addTab: (connectionId: string | null, database?: string | null) => void;
  /**
   * Open a Table Data tab bound to the explicit connection/database/schema/table.
   * If an identical Table Data tab is already open, focus it (policy A, ADR-010).
   */
  openTableData: (
    connectionId: string | null,
    database: string | null,
    schema: string | null,
    table: string,
  ) => void;
  /** Open a Table Structure tab (same focus policy as Table Data). */
  openTableStructure: (
    connectionId: string | null,
    database: string | null,
    schema: string | null,
    table: string,
  ) => void;
  /**
   * Create a NEW Query tab bound to the given connection/database, optionally
   * pre-populated with generated SQL. Never overwrites an existing tab and never
   * executes automatically.
   */
  openQueryForTable: (
    connectionId: string | null,
    database: string | null,
    sql?: string,
  ) => void;
  /** Update the browse state of one Table Data tab (tab-local; no other tab moves). */
  updateTableData: (
    id: string,
    patch: Partial<Pick<TableDataTab, "filters" | "sort" | "page" | "pageSize">>,
  ) => void;
  closeTab: (id: string) => void;
  /**
   * Explicitly bind a tab to a connection. The previous database binding is
   * always cleared; pass the new connection's default database (when it has
   * one) so the resulting context is deterministic (PRF01-FIX-01).
   */
  setTabConnection: (
    id: string,
    connectionId: string | null,
    database?: string | null,
  ) => void;
  /** Explicitly bind a tab to a database (PRF-01; never redirects other tabs). */
  setTabDatabase: (id: string, database: string | null) => void;
  /** Record that a Query tab is linked to a persisted saved query. */
  setTabSavedQuery: (
    id: string,
    saved: { id: string | null; title: string | null; tags: string | null },
  ) => void;
  /** Mark the active Query tab as saved/executed (no unsaved changes). */
  markActiveClean: () => void;
}

function newId(): string {
  return `tab-${Math.random().toString(36).slice(2, 10)}`;
}

function nextQueryTitle(tabs: WorkspaceTab[]): string {
  const count = tabs.filter(isQueryTab).length;
  return `Query ${count + 1}`;
}

function newQueryTab(
  title: string,
  connectionId: string | null,
  database: string | null = null,
): QueryTab {
  return {
    id: newId(),
    kind: "query",
    title,
    sql: "",
    connectionId,
    database,
    dirty: false,
  };
}

function newTableDataTab(
  title: string,
  connectionId: string | null,
  database: string | null,
  schema: string | null,
  table: string,
): TableDataTab {
  return {
    id: newId(),
    kind: "table-data",
    title,
    connectionId,
    database,
    schema,
    table,
    filters: [],
    sort: [],
    page: 1,
    pageSize: DEFAULT_TABLE_PAGE_SIZE,
  };
}

function newTableStructureTab(
  title: string,
  connectionId: string | null,
  database: string | null,
  schema: string | null,
  table: string,
): TableStructureTab {
  return {
    id: newId(),
    kind: "table-structure",
    title,
    connectionId,
    database,
    schema,
    table,
  };
}

/** Deterministic visible title for a table-bound tab (ADR-010 §3). */
function tableTabTitle(table: string, kind: "data" | "structure"): string {
  return kind === "data" ? table : `${table} (structure)`;
}

/**
 * Deterministic, non-ambiguous visible title. Identical table names opened
 * against different databases get a stable numeric suffix ("users", "users (2)")
 * so tabs remain distinguishable; the internal id is always the identity.
 */
function uniqueTitle(tabs: WorkspaceTab[], base: string): string {
  const used = new Set(tabs.map((tab) => tab.title));
  if (!used.has(base)) return base;
  let suffix = 2;
  while (used.has(`${base} (${suffix})`)) suffix += 1;
  return `${base} (${suffix})`;
}

function sameTableBinding(
  tab: TableDataTab | TableStructureTab,
  connectionId: string | null,
  database: string | null,
  schema: string | null,
  table: string,
): boolean {
  return (
    tab.connectionId === connectionId &&
    tab.database === database &&
    tab.schema === schema &&
    tab.table === table
  );
}

const initialTabs: WorkspaceTab[] = [];

/**
 * Client workspace state: sidebar collapse and the typed tab workspace. Server
 * data lives in TanStack Query; tab state is transient and never persisted.
 *
 * The workspace starts with zero tabs (PRF02-T02); tabs are created only by
 * explicit user actions, never by connection/database selection or at startup.
 */
export const useWorkspaceStore = create<WorkspaceState>((set) => ({
  sidebarCollapsed: false,
  toggleSidebar: () =>
    set((state) => ({ sidebarCollapsed: !state.sidebarCollapsed })),
  setSidebarCollapsed: (sidebarCollapsed) => set({ sidebarCollapsed }),

  tabs: initialTabs,
  activeTabId: null,
  setActiveTab: (id) =>
    set((state) =>
      state.tabs.some((tab) => tab.id === id) ? { activeTabId: id } : state,
    ),
  updateActiveSql: (sql) =>
    set((state) => ({
      tabs: state.tabs.map((tab) =>
        tab.id === state.activeTabId && tab.kind === "query"
          ? { ...tab, sql, dirty: true }
          : tab,
      ),
    })),
  insertQuerySql: (sql, connectionId, database = null) =>
    set((state) => {
      const active = state.tabs.find((tab) => tab.id === state.activeTabId);
      // Reuse an untouched empty Query tab that is not already bound elsewhere;
      // otherwise open a new one. Dirty policy for generated SQL: an inserted
      // statement keeps the tab clean because it is reproducible scaffolding.
      // Manual edits flow through updateActiveSql and mark the tab dirty.
      const reusable =
        active !== undefined &&
        active.kind === "query" &&
        !active.dirty &&
        active.sql.trim() === "" &&
        (active.connectionId === null || active.connectionId === connectionId);
      if (reusable && active !== undefined && active.kind === "query") {
        return {
          tabs: state.tabs.map((tab) =>
            tab.id === active.id && tab.kind === "query"
              ? { ...tab, sql, connectionId, database }
              : tab,
          ),
        };
      }
      const tab: QueryTab = {
        ...newQueryTab(nextQueryTitle(state.tabs), connectionId, database),
        sql,
      };
      return { tabs: [...state.tabs, tab], activeTabId: tab.id };
    }),
  openQuery: (connectionId, database = null, sql = "") =>
    set((state) => {
      const tab: QueryTab = {
        ...newQueryTab(nextQueryTitle(state.tabs), connectionId, database),
        sql,
      };
      return { tabs: [...state.tabs, tab], activeTabId: tab.id };
    }),
  addTab: (connectionId, database = null) =>
    set((state) => {
      const tab = newQueryTab(
        nextQueryTitle(state.tabs),
        connectionId,
        database,
      );
      return { tabs: [...state.tabs, tab], activeTabId: tab.id };
    }),
  openTableData: (connectionId, database, schema, table) =>
    set((state) => {
      const existing = state.tabs.find(
        (tab): tab is TableDataTab =>
          tab.kind === "table-data" &&
          sameTableBinding(tab, connectionId, database, schema, table),
      );
      if (existing) return { activeTabId: existing.id };
      const tab = newTableDataTab(
        uniqueTitle(state.tabs, tableTabTitle(table, "data")),
        connectionId,
        database,
        schema,
        table,
      );
      return { tabs: [...state.tabs, tab], activeTabId: tab.id };
    }),
  openTableStructure: (connectionId, database, schema, table) =>
    set((state) => {
      const existing = state.tabs.find(
        (tab): tab is TableStructureTab =>
          tab.kind === "table-structure" &&
          sameTableBinding(tab, connectionId, database, schema, table),
      );
      if (existing) return { activeTabId: existing.id };
      const tab = newTableStructureTab(
        uniqueTitle(state.tabs, tableTabTitle(table, "structure")),
        connectionId,
        database,
        schema,
        table,
      );
      return { tabs: [...state.tabs, tab], activeTabId: tab.id };
    }),
  openQueryForTable: (connectionId, database, sql = "") =>
    set((state) => {
      const tab: QueryTab = {
        ...newQueryTab(nextQueryTitle(state.tabs), connectionId, database),
        sql,
      };
      return { tabs: [...state.tabs, tab], activeTabId: tab.id };
    }),
  updateTableData: (id, patch) =>
    set((state) => ({
      tabs: state.tabs.map((tab) =>
        tab.id === id && tab.kind === "table-data"
          ? { ...tab, ...patch }
          : tab,
      ),
    })),
  setTabConnection: (id, connectionId, database = null) =>
    set((state) => ({
      tabs: state.tabs.map((tab) =>
        tab.id === id ? { ...tab, connectionId, database } : tab,
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
        tab.id === id && tab.kind === "query"
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

      const tabs = state.tabs.filter((tab) => tab.id !== id);
      // Closing the final tab leaves an empty workspace (PRF02-T02): no tab is
      // recreated implicitly.
      if (tabs.length === 0) {
        return { tabs, activeTabId: null };
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
        tab.id === state.activeTabId && tab.kind === "query"
          ? { ...tab, dirty: false }
          : tab,
      ),
    })),
}));
