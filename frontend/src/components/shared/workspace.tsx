import { EditorPanel } from "./editor-panel";
import { ResultsPanel } from "./results-panel";
import { TableDataPanel } from "./table-data-panel";
import { TableStructurePanel } from "./table-structure-panel";
import { TabsBar } from "./tabs-bar";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

/**
 * Central workspace. Renders the surface for the active tab kind: the SQL
 * editor + results for Query tabs, or the table data/structure surface for the
 * PRF-02 tab kinds. Tabs are transient UI state (never persisted).
 */
export function Workspace() {
  const activeKind = useWorkspaceStore((state) => {
    const tab = state.tabs.find((candidate) => candidate.id === state.activeTabId);
    return tab?.kind ?? "query";
  });

  return (
    <main className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
      <TabsBar />
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        {activeKind === "table-data" ? (
          <TableDataPanel />
        ) : activeKind === "table-structure" ? (
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
