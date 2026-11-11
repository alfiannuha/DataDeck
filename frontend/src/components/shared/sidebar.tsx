"use client";

import { ConnectionList } from "@/components/sidebar/connection-list";
import { SchemaExplorer } from "@/components/sidebar/schema-explorer";
import { useResizableSidebar } from "@/hooks/use-resizable-sidebar";
import { cn } from "@/lib/utils";
import { useConnectionStore } from "@/store/useConnectionStore";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

/**
 * Explorer rail: saved connections plus the schema tree for the active
 * connection. The rail is user-resizable (drag the right divider; double-click
 * to reset) and the width is persisted to localStorage.
 */
export function Sidebar() {
  const collapsed = useWorkspaceStore((state) => state.sidebarCollapsed);
  const activeConnectionId = useConnectionStore(
    (state) => state.activeConnectionId,
  );
  const { width, resizing, beginResize, moveResize, endResize, resetWidth } =
    useResizableSidebar();

  return (
    <aside
      id="schema-explorer"
      aria-label="Connections and schema"
      hidden={collapsed}
      style={{ width: `${width}px` }}
      className="relative flex shrink-0 flex-col overflow-hidden border-r border-border bg-panel"
    >
      <div className="min-h-0 flex-1 overflow-auto">
        <ConnectionList />
        <div className="border-t border-border">
          <SchemaExplorer connectionId={activeConnectionId} />
        </div>
      </div>
      <div
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize sidebar"
        title="Drag to resize · double-click to reset"
        data-testid="sidebar-resize-handle"
        className={cn(
          "absolute inset-y-0 right-0 z-10 w-1.5 cursor-col-resize touch-none transition-colors hover:bg-accent/40 focus-visible:bg-accent/60 focus-visible:outline-none",
          resizing && "bg-accent/60",
        )}
        onPointerDown={(event) => {
          event.preventDefault();
          event.currentTarget.setPointerCapture?.(event.pointerId);
          beginResize(event.clientX);
        }}
        onPointerMove={(event) => moveResize(event.clientX)}
        onPointerUp={(event) => {
          event.currentTarget.releasePointerCapture?.(event.pointerId);
          endResize();
        }}
        onPointerCancel={endResize}
        onDoubleClick={resetWidth}
      />
    </aside>
  );
}
