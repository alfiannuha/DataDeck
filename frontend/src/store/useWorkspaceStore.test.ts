import { beforeEach, describe, expect, it } from "vitest";

import { useWorkspaceStore } from "./useWorkspaceStore";

beforeEach(() => {
  useWorkspaceStore.setState({
    sidebarCollapsed: false,
    tabs: [
      { id: "t1", title: "Query 1", sql: "", connectionId: null, dirty: false },
    ],
    activeTabId: "t1",
  });
});

describe("useWorkspaceStore", () => {
  it("starts with a single active tab", () => {
    const state = useWorkspaceStore.getState();
    expect(state.tabs).toHaveLength(1);
    expect(state.activeTabId).toBe("t1");
    expect(state.tabs[0].connectionId).toBeNull();
  });

  it("updates the active tab SQL and marks it dirty", () => {
    useWorkspaceStore.getState().updateActiveSql("SELECT 1");
    const tab = useWorkspaceStore.getState().tabs[0];
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
    const active = state.tabs.find((tab) => tab.id === state.activeTabId);
    expect(active?.sql).toContain("LIMIT 100;");
    expect(active?.connectionId).toBe("c1");
  });

  it("explicitly rebinds a tab only when asked", () => {
    useWorkspaceStore.getState().setTabConnection("t1", "c9");
    expect(useWorkspaceStore.getState().tabs[0].connectionId).toBe("c9");
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

  it("keeps at least one tab when closing the last one", () => {
    useWorkspaceStore.getState().closeTab("t1");
    const state = useWorkspaceStore.getState();
    expect(state.tabs).toHaveLength(1);
    expect(state.activeTabId).not.toBeNull();
  });

  it("clears dirty state", () => {
    useWorkspaceStore.getState().updateActiveSql("SELECT 1");
    useWorkspaceStore.getState().markActiveClean();
    expect(useWorkspaceStore.getState().tabs[0].dirty).toBe(false);
  });

  it("keeps generated SQL clean until it is edited", () => {
    useWorkspaceStore
      .getState()
      .insertQuerySql('SELECT COUNT(*) FROM "public"."users";', "c1");
    const generated = useWorkspaceStore.getState().tabs[0];
    expect(generated.sql).toContain("COUNT(*)");
    expect(generated.dirty).toBe(false);

    useWorkspaceStore.getState().updateActiveSql('SELECT COUNT(*) FROM t;');
    expect(useWorkspaceStore.getState().tabs[0].dirty).toBe(true);
  });

  it("opens a new clean tab for generated SQL when the active tab is in use", () => {
    useWorkspaceStore.getState().updateActiveSql("SELECT 1;");
    useWorkspaceStore.getState().insertQuerySql("SELECT 2;", "c1");
    const state = useWorkspaceStore.getState();
    expect(state.tabs).toHaveLength(2);
    expect(state.tabs[1].sql).toBe("SELECT 2;");
    expect(state.tabs[1].dirty).toBe(false);
  });
});
