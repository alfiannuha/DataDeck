import type { QueryTab, WorkspaceTab } from "@/store/useWorkspaceStore";

/**
 * Test helper: narrow a workspace tab to a Query tab, failing the test when the
 * tab is missing or of another kind. Keeps tests honest instead of casting.
 */
export function asQueryTab(tab: WorkspaceTab | undefined): QueryTab {
  if (!tab || tab.kind !== "query") {
    throw new Error("expected a query tab");
  }
  return tab;
}
