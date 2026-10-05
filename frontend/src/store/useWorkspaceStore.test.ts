import { beforeEach, describe, expect, it } from "vitest";

import { useWorkspaceStore } from "./useWorkspaceStore";
import { asQueryTab } from "@/test/query-tab";

beforeEach(() => {
  useWorkspaceStore.setState({
    sidebarCollapsed: false,
    tabs: [
      { kind: "query", id: "t1", title: "Query 1", sql: "", connectionId: null, database: null, dirty: false },
    ],
    activeTabId: "t1",
  });
});

describe("useWorkspaceStore", () => {
  it("supports an empty workspace with zero tabs", () => {
    useWorkspaceStore.setState({ tabs: [], activeTabId: null });
    const state = useWorkspaceStore.getState();
    expect(state.tabs).toHaveLength(0);
    expect(state.activeTabId).toBeNull();
  });

  it("updates the active tab SQL and marks it dirty", () => {
    useWorkspaceStore.getState().updateActiveSql("SELECT 1");
    const tab = asQueryTab(useWorkspaceStore.getState().tabs[0]);
    expect(tab.sql).toBe("SELECT 1");
    expect(tab.dirty).toBe(true);
  });

  it("binds a new tab to the given connection", () => {
    useWorkspaceStore.getState().addTab("c2");
    const state = useWorkspaceStore.getState();
    expect(state.tabs).toHaveLength(2);
    expect(state.activeTabId).toBe(state.tabs[1].id);
    expect(state.tabs[1].connectionId).toBe("c2");
  });

  it("does not rebind existing tabs when a connection is added", () => {
    useWorkspaceStore.getState().addTab("c1");
    useWorkspaceStore.getState().addTab("c2");
    expect(useWorkspaceStore.getState().tabs[1].connectionId).toBe("c1");
    expect(useWorkspaceStore.getState().tabs[2].connectionId).toBe("c2");
  });

  it("inserts SQL bound to the given connection", () => {
    useWorkspaceStore
      .getState()
      .insertQuerySql('SELECT * FROM "public"."users" LIMIT 100;', "c1");
    const state = useWorkspaceStore.getState();
    const active = asQueryTab(
      state.tabs.find((tab) => tab.id === state.activeTabId),
    );
    expect(active.sql).toContain("LIMIT 100;");
    expect(active.connectionId).toBe("c1");
  });

  it("explicitly rebinds a tab only when asked", () => {
    useWorkspaceStore.getState().setTabConnection("t1", "c9");
    expect(useWorkspaceStore.getState().tabs[0].connectionId).toBe("c9");
  });

  it("clears the previous database when the connection changes", () => {
    useWorkspaceStore.setState({
      tabs: [
        {
          kind: "query",
          id: "t1",
          title: "Query 1",
          sql: "SELECT 1;",
          connectionId: "cA",
          database: "alpha",
          dirty: false,
        },
      ],
      activeTabId: "t1",
    });

    useWorkspaceStore.getState().setTabConnection("t1", "cB");

    expect(useWorkspaceStore.getState().tabs[0].connectionId).toBe("cB");
    expect(useWorkspaceStore.getState().tabs[0].database).toBeNull();
  });

  it("binds the provided default database when the connection changes", () => {
    useWorkspaceStore.setState({
      tabs: [
        {
          kind: "query",
          id: "t1",
          title: "Query 1",
          sql: "SELECT 1;",
          connectionId: "cA",
          database: "alpha",
          dirty: false,
        },
      ],
      activeTabId: "t1",
    });

    useWorkspaceStore.getState().setTabConnection("t1", "cB", "beta");

    expect(useWorkspaceStore.getState().tabs[0].connectionId).toBe("cB");
    expect(useWorkspaceStore.getState().tabs[0].database).toBe("beta");
  });

  it("selects a neighbour when the active tab is closed", () => {
    useWorkspaceStore.getState().addTab("c1");
    const secondId = useWorkspaceStore.getState().tabs[1].id;
    useWorkspaceStore.setState({ activeTabId: "t1" });

    useWorkspaceStore.getState().closeTab("t1");

    const state = useWorkspaceStore.getState();
    expect(state.tabs).toHaveLength(1);
    expect(state.activeTabId).toBe(secondId);
  });

  it("returns to an empty workspace when the last tab closes", () => {
    useWorkspaceStore.setState({ tabs: [], activeTabId: null });
    useWorkspaceStore.getState().addTab("c1");
    const id = useWorkspaceStore.getState().activeTabId!;

    useWorkspaceStore.getState().closeTab(id);

    const state = useWorkspaceStore.getState();
    expect(state.tabs).toHaveLength(0);
    expect(state.activeTabId).toBeNull();
  });

  it("returns to an empty workspace when the last table tab closes", () => {
    useWorkspaceStore.setState({ tabs: [], activeTabId: null });
    useWorkspaceStore.getState().openTableData("c1", "ccm", "public", "users");
    const id = useWorkspaceStore.getState().activeTabId!;

    useWorkspaceStore.getState().closeTab(id);

    const state = useWorkspaceStore.getState();
    expect(state.tabs).toHaveLength(0);
    expect(state.activeTabId).toBeNull();
  });

  it("preserves the active tab when closing a non-active tab", () => {
    useWorkspaceStore.getState().addTab("c1");
    const secondId = useWorkspaceStore.getState().tabs[1].id;
    useWorkspaceStore.setState({ activeTabId: secondId });

    useWorkspaceStore.getState().closeTab("t1");

    const state = useWorkspaceStore.getState();
    expect(state.tabs).toHaveLength(1);
    expect(state.activeTabId).toBe(secondId);
  });

  it("clears dirty state", () => {
    useWorkspaceStore.getState().updateActiveSql("SELECT 1");
    useWorkspaceStore.getState().markActiveClean();
    expect(asQueryTab(useWorkspaceStore.getState().tabs[0]).dirty).toBe(false);
  });

  it("keeps generated SQL clean until it is edited", () => {
    useWorkspaceStore
      .getState()
      .insertQuerySql('SELECT COUNT(*) FROM "public"."users";', "c1");
    const generated = asQueryTab(useWorkspaceStore.getState().tabs[0]);
    expect(generated.sql).toContain("COUNT(*)");
    expect(generated.dirty).toBe(false);

    useWorkspaceStore.getState().updateActiveSql('SELECT COUNT(*) FROM t;');
    expect(asQueryTab(useWorkspaceStore.getState().tabs[0]).dirty).toBe(true);
  });

  it("opens a new clean tab for generated SQL when the active tab is in use", () => {
    useWorkspaceStore.getState().updateActiveSql("SELECT 1;");
    useWorkspaceStore.getState().insertQuerySql("SELECT 2;", "c1");
    const state = useWorkspaceStore.getState();
    expect(state.tabs).toHaveLength(2);
    expect(asQueryTab(state.tabs[1]).sql).toBe("SELECT 2;");
    expect(asQueryTab(state.tabs[1]).dirty).toBe(false);
  });

  it("binds generated SQL to a database when provided", () => {
    useWorkspaceStore
      .getState()
      .insertQuerySql('SELECT * FROM "public"."users" LIMIT 100;', "c1", "CCM");
    const tab = asQueryTab(useWorkspaceStore.getState().tabs[0]);
    expect(tab.connectionId).toBe("c1");
    expect(tab.database).toBe("CCM");
  });

  it("keeps database bindings independent per tab", () => {
    useWorkspaceStore.getState().insertQuerySql("SELECT 1;", "c1", "CCM");
    useWorkspaceStore.getState().addTab("c1", "reporting");
    const state = useWorkspaceStore.getState();
    expect(state.tabs[0].database).toBe("CCM");
    expect(state.tabs[1].database).toBe("reporting");

    useWorkspaceStore.getState().setTabDatabase(state.tabs[1].id, "analytics");
    expect(useWorkspaceStore.getState().tabs[0].database).toBe("CCM");
    expect(useWorkspaceStore.getState().tabs[1].database).toBe("analytics");
  });

  it("opens a Table Data tab bound to the explicit connection/database/table", () => {
    useWorkspaceStore.getState().openTableData("c1", "ccm", "public", "users");
    const state = useWorkspaceStore.getState();
    expect(state.tabs).toHaveLength(2);
    const tab = state.tabs[1];
    expect(tab.kind).toBe("table-data");
    if (tab.kind !== "table-data") throw new Error("expected table-data tab");
    expect(tab.connectionId).toBe("c1");
    expect(tab.database).toBe("ccm");
    expect(tab.schema).toBe("public");
    expect(tab.table).toBe("users");
    expect(tab.title).toBe("users");
    expect(tab.filters).toEqual([]);
    expect(tab.sort).toEqual([]);
    expect(tab.page).toBe(1);
    expect(tab.pageSize).toBe(100);
    expect(state.activeTabId).toBe(tab.id);
  });

  it("focuses an existing identical Table Data tab (policy A)", () => {
    useWorkspaceStore.getState().openTableData("c1", "ccm", "public", "users");
    const first = useWorkspaceStore.getState().tabs[1].id;
    useWorkspaceStore.getState().setActiveTab(useWorkspaceStore.getState().tabs[0].id);

    useWorkspaceStore.getState().openTableData("c1", "ccm", "public", "users");
    const state = useWorkspaceStore.getState();
    expect(state.tabs).toHaveLength(2);
    expect(state.activeTabId).toBe(first);
  });

  it("opens distinct Table Data tabs for different databases", () => {
    useWorkspaceStore.getState().openTableData("c1", "alpha", "public", "users");
    useWorkspaceStore.getState().openTableData("c1", "beta", "public", "users");
    const state = useWorkspaceStore.getState();
    expect(state.tabs).toHaveLength(3);
    expect(state.tabs[1].database).toBe("alpha");
    expect(state.tabs[2].database).toBe("beta");
    // Identical table names stay distinguishable; ids remain the identity.
    expect(state.tabs[1].title).toBe("users");
    expect(state.tabs[2].title).toBe("users (2)");
    expect(state.tabs[1].id).not.toBe(state.tabs[2].id);
  });

  it("opens a Table Structure tab with a distinguishable title", () => {
    useWorkspaceStore.getState().openTableStructure("c1", "ccm", "public", "users");
    const tab = useWorkspaceStore.getState().tabs[1];
    if (tab.kind !== "table-structure") throw new Error("expected structure tab");
    expect(tab.title).toBe("users (structure)");
    expect(tab.table).toBe("users");
  });

  it("opens Query For Table as a NEW query tab and never overwrites", () => {
    const originalId = useWorkspaceStore.getState().tabs[0].id;
    useWorkspaceStore
      .getState()
      .openQueryForTable("c1", "ccm", 'SELECT * FROM "public"."users" LIMIT 100;');
    const state = useWorkspaceStore.getState();
    expect(state.tabs).toHaveLength(2);
    const query = asQueryTab(state.tabs[1]);
    expect(query.id).not.toBe(originalId);
    expect(query.connectionId).toBe("c1");
    expect(query.database).toBe("ccm");
    expect(query.sql).toContain("LIMIT 100;");
    expect(query.title).toBe("Query 2");
    expect(state.activeTabId).toBe(query.id);
  });

  it("updates Table Data browse state tab-locally", () => {
    useWorkspaceStore.getState().openTableData("c1", "alpha", "public", "a");
    useWorkspaceStore.getState().openTableData("c1", "beta", "public", "b");
    const [idA] = useWorkspaceStore.getState().tabs
      .filter((tab) => tab.kind === "table-data")
      .map((tab) => tab.id);

    useWorkspaceStore.getState().updateTableData(idA, {
      page: 3,
      pageSize: 50,
      filters: [{ column: "id", operator: "equals", value: 1 }],
      sort: [{ column: "id", direction: "desc" }],
    });

    const [tabA, tabB] = useWorkspaceStore.getState().tabs.filter(
      (tab) => tab.kind === "table-data",
    );
    if (tabA.kind !== "table-data" || tabB.kind !== "table-data") {
      throw new Error("expected table-data tabs");
    }
    expect(tabA.page).toBe(3);
    expect(tabA.pageSize).toBe(50);
    expect(tabA.filters).toHaveLength(1);
    expect(tabA.sort).toEqual([{ column: "id", direction: "desc" }]);
    expect(tabB.page).toBe(1);
    expect(tabB.filters).toEqual([]);
  });

  it("does not let Query-only actions corrupt Table Data tabs", () => {
    useWorkspaceStore.getState().openTableData("c1", "ccm", "public", "users");
    const tableId = useWorkspaceStore.getState().activeTabId!;

    useWorkspaceStore.getState().updateActiveSql("SELECT 1");
    useWorkspaceStore.getState().markActiveClean();
    useWorkspaceStore.getState().setTabSavedQuery(tableId, {
      id: "s1",
      title: "Saved",
      tags: null,
    });

    const tab = useWorkspaceStore.getState().tabs.find((t) => t.id === tableId);
    expect(tab?.kind).toBe("table-data");
    expect(tab && "sql" in tab).toBe(false);
    expect(tab && "savedQueryId" in tab).toBe(false);
  });

  it("closes mixed tab kinds deterministically", () => {
    useWorkspaceStore.getState().openTableData("c1", "ccm", "public", "users");
    useWorkspaceStore.getState().openTableStructure("c1", "ccm", "public", "users");
    const structureId = useWorkspaceStore.getState().activeTabId!;

    useWorkspaceStore.getState().closeTab(structureId);
    const state = useWorkspaceStore.getState();
    expect(state.tabs).toHaveLength(2);
    expect(state.tabs.some((tab) => tab.kind === "table-structure")).toBe(false);
    expect(state.activeTabId).toBe(state.tabs[1].id);
  });

  it("keeps internal tab ids unique across kinds", () => {
    useWorkspaceStore.getState().openQuery("c1", "ccm", "SELECT 1;");
    useWorkspaceStore.getState().openTableData("c1", "ccm", "public", "users");
    useWorkspaceStore.getState().openTableStructure("c1", "ccm", "public", "users");
    useWorkspaceStore.getState().openQueryForTable("c1", "ccm", "SELECT 2;");
    const ids = useWorkspaceStore.getState().tabs.map((tab) => tab.id);
    expect(new Set(ids).size).toBe(ids.length);
  });
});
