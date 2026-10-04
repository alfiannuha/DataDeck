"use client";

import { PanelPlaceholder } from "@/components/shared/panel-placeholder";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

/**
 * Table Structure tab surface. PRF02-T01 establishes the typed tab architecture
 * only; structure rendering arrives in a later PRF-02 task. The tab carries the
 * explicit connection/database/schema/table binding.
 */
export function TableStructurePanel() {
  const tab = useWorkspaceStore((state) =>
    state.tabs.find(
      (candidate) =>
        candidate.id === state.activeTabId &&
        candidate.kind === "table-structure",
    ),
  );

  if (tab?.kind !== "table-structure") return null;

  const qualified = tab.schema ? `${tab.schema}.${tab.table}` : tab.table;
  return (
    <section
      aria-label="Table structure"
      data-testid="table-structure-panel"
      className="flex min-h-0 flex-1 flex-col overflow-hidden"
    >
      <header className="flex h-8 shrink-0 items-center gap-2 border-b border-border bg-panel px-3 text-xs text-muted-foreground">
        <span className="font-medium text-foreground">{qualified}</span>
        <span aria-hidden="true">·</span>
        <span data-testid="table-structure-binding">
          {tab.database ?? "default database"}
        </span>
      </header>
      <PanelPlaceholder
        title="Table Structure"
        description="Table Structure — implementation pending"
      />
    </section>
  );
}
