import { EditorPanel } from "./editor-panel";
import { ResultsPanel } from "./results-panel";
import { TableDataView } from "./table-data-view";
import { TableStructurePanel } from "./table-structure-panel";
import { TabsBar } from "./tabs-bar";
import { WorkspaceEmptyState } from "./workspace-empty-state";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

/**
 * Central workspace. Renders the surface for the active tab kind: the SQL
 * editor + results for Query tabs, the table data/structure surfaces for the
 * PRF-02 tab kinds, or the empty state when there is no active tab.
 *
 * Tabs are transient UI state (never persisted). A stale `activeTabId` (set but
 * no matching tab) is treated as "no active tab" rather than crashing; the tab
 * strip remains available so the user can recover.
 */
export function Workspace() {
  const hasTabs = useWorkspaceStore((state) => state.tabs.length > 0);
  const activeTab = useWorkspaceStore((state) =>
    state.tabs.find((candidate) => candidate.id === state.activeTabId),
  );

  return (
    <main className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
      {hasTabs ? <TabsBar /> : null}
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        {!activeTab ? (
          <WorkspaceEmptyState />
        ) : activeTab.kind === "table-data" ? (
          <TableDataView />
        ) : activeTab.kind === "table-structure" ? (
          <TableStructurePanel />
        ) : (
          <>
            <EditorPanel />
            <ResultsPanel />
          </>
        )}
      </div>
    </main>
  );
}
