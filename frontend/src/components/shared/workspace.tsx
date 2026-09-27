import { EditorPanel } from "./editor-panel";
import { ResultsPanel } from "./results-panel";
import { TabsBar } from "./tabs-bar";

/**
 * Central workspace: query tabs above a vertically split editor/results area.
 * Both child regions are scroll-contained individually.
 */
export function Workspace() {
  return (
    <main className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
      <TabsBar />
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <EditorPanel />
        <ResultsPanel />
      </div>
    </main>
  );
}
