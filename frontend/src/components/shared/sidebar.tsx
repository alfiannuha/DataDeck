"use client";

import { ConnectionList } from "@/components/sidebar/connection-list";
import { SchemaExplorer } from "@/components/sidebar/schema-explorer";
import { useConnectionStore } from "@/store/useConnectionStore";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

/**
 * Explorer rail: saved connections plus the schema tree for the active
 * connection.
 */
export function Sidebar() {
  const collapsed = useWorkspaceStore((state) => state.sidebarCollapsed);
  const activeConnectionId = useConnectionStore(
    (state) => state.activeConnectionId,
  );

  return (
    <aside
      id="schema-explorer"
      aria-label="Connections and schema"
      hidden={collapsed}
      className="flex w-64 shrink-0 flex-col overflow-hidden border-r border-border bg-panel"
    >
      <div className="min-h-0 flex-1 overflow-auto">
        <ConnectionList />
        <div className="border-t border-border">
          <SchemaExplorer connectionId={activeConnectionId} />
        </div>
      </div>
    </aside>
  );
}
