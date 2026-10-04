"use client";

import { Plus, X } from "lucide-react";
import { useRef, useState } from "react";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { useConnections } from "@/hooks/use-connections";
import { useNewQueryTab } from "@/hooks/use-new-query-tab";
import { cn } from "@/lib/utils";
import { useWorkspaceStore, type WorkspaceTab } from "@/store/useWorkspaceStore";

/** Query tab strip: switch, close (with unsaved confirmation) and add tabs. */
export function TabsBar() {
  const tabs = useWorkspaceStore((state) => state.tabs);
  const activeTabId = useWorkspaceStore((state) => state.activeTabId);
  const setActiveTab = useWorkspaceStore((state) => state.setActiveTab);
  const closeTab = useWorkspaceStore((state) => state.closeTab);
  const newQueryTab = useNewQueryTab();
  const { data: connections } = useConnections();

  const [pendingClose, setPendingClose] = useState<WorkspaceTab | null>(null);
  const stripRef = useRef<HTMLElement>(null);

  // After tabs change, keep keyboard focus on the active tab instead of
  // dropping it to the document body when the focused tab is removed.
  function focusActiveTab() {
    window.requestAnimationFrame(() => {
      stripRef.current
        ?.querySelector<HTMLElement>('button[aria-current="true"]')
        ?.focus();
    });
  }

  function connectionLabel(connectionId: string | null): string {
    if (!connectionId) return "No connection";
    const connection = connections?.find((c) => c.id === connectionId);
    return connection?.name ?? connectionId;
  }

  function requestClose(tab: WorkspaceTab) {
    if (tab.kind === "query" && tab.dirty) {
      setPendingClose(tab);
    } else {
      closeTab(tab.id);
      focusActiveTab();
    }
  }

  return (
    <section
      ref={stripRef}
      aria-label="Query tabs"
      className="flex h-9 shrink-0 items-center gap-1 overflow-x-auto border-b border-border bg-panel px-2"
    >
      <ul className="flex items-center gap-1" aria-label="Open query tabs">
        {tabs.map((tab) => {
          const isActive = tab.id === activeTabId;
          return (
            <li key={tab.id}>
              <div
                className={cn(
                  "flex items-center gap-1 rounded-md border px-2 py-0.5",
                  isActive
                    ? "border-border-strong bg-editor-surface"
                    : "border-transparent hover:bg-panel-raised",
                )}
              >
                <button
                  type="button"
                  aria-current={isActive ? "true" : undefined}
                  title={`Connection: ${connectionLabel(tab.connectionId)}${
                    tab.database ? ` / ${tab.database}` : ""
                  }`}
                  onClick={() => setActiveTab(tab.id)}
                  className="flex items-center gap-1.5 text-xs text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
                >
                  {tab.kind === "query" && tab.dirty && (
                    <span
                      aria-label="Unsaved changes"
                      title="Unsaved changes"
                      className="text-[10px] leading-none text-accent"
                    >
                      ●
                    </span>
                  )}
                  <span className="max-w-40 truncate">{tab.title}</span>
                </button>
                <button
                  type="button"
                  aria-label={`Close ${tab.title}`}
                  onClick={() => requestClose(tab)}
                  className="rounded p-0.5 text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]"
                >
                  <X size={11} aria-hidden="true" />
                </button>
              </div>
            </li>
          );
        })}
      </ul>
      <Button
        variant="ghost"
        size="icon"
        aria-label="New query tab"
        onClick={() => {
          // Canonical New Query action: inherits the explorer's selected
          // database for the active connection; existing tabs are never
          // modified (PRF-01).
          newQueryTab();
          focusActiveTab();
        }}
      >
        <Plus size={14} aria-hidden="true" />
      </Button>

      <AlertDialog
        open={pendingClose !== null}
        onOpenChange={(open) => {
          if (!open) setPendingClose(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Close tab with unsaved SQL?</AlertDialogTitle>
            <AlertDialogDescription>
              {pendingClose
                ? `“${pendingClose.title}” has unsaved changes that will be lost.`
                : ""}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (pendingClose) closeTab(pendingClose.id);
                setPendingClose(null);
                focusActiveTab();
              }}
            >
              Close tab
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </section>
  );
}
