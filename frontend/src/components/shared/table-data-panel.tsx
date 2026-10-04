"use client";

import { PanelPlaceholder } from "@/components/shared/panel-placeholder";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

/**
 * Table Data tab surface. PRF02-T01 establishes the typed tab architecture only;
 * the real browser (pagination, filtering, sorting, CRUD) arrives in later
 * PRF-02 tasks. This placeholder shows the tab's authoritative binding so the
 * PRF-01 connection/database context is visible and testable.
 */
export function TableDataPanel() {
  const tab = useWorkspaceStore((state) =>
    state.tabs.find(
      (candidate) =>
        candidate.id === state.activeTabId && candidate.kind === "table-data",
    ),
  );

  if (tab?.kind !== "table-data") return null;

  const qualified = tab.schema ? `${tab.schema}.${tab.table}` : tab.table;
  return (
    <section
      aria-label="Table data"
      data-testid="table-data-panel"
      className="flex min-h-0 flex-1 flex-col overflow-hidden"
    >
      <header className="flex h-8 shrink-0 items-center gap-2 border-b border-border bg-panel px-3 text-xs text-muted-foreground">
        <span className="font-medium text-foreground">{qualified}</span>
        <span aria-hidden="true">·</span>
        <span data-testid="table-data-binding">
          {tab.database ?? "default database"}
        </span>
      </header>
      <PanelPlaceholder
        title="Table Data"
        description="Table Data — implementation pending PRF02-T04"
      />
    </section>
  );
}
