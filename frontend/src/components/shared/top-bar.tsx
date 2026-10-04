"use client";

import { Bookmark, Download, History, Menu, RefreshCw } from "lucide-react";
import { useState } from "react";

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
import { usePwaInstall } from "@/hooks/use-pwa-install";
import { activateUpdate } from "@/lib/pwa/service-worker";
import { useConnectionStore } from "@/store/useConnectionStore";
import { usePwaStore } from "@/store/usePwaStore";
import { useWorkspaceStore } from "@/store/useWorkspaceStore";

import { HistoryDialog } from "./history-dialog";
import { SavedQueriesDialog } from "./saved-queries-dialog";

/**
 * Top bar / connection context. Owns the sidebar toggle and the query history
 * entry point; the active connection name is derived from the connections query
 * (server state) rather than duplicated into a client store.
 */
export function TopBar() {
  const sidebarCollapsed = useWorkspaceStore((state) => state.sidebarCollapsed);
  const toggleSidebar = useWorkspaceStore((state) => state.toggleSidebar);
  const activeConnectionId = useConnectionStore(
    (state) => state.activeConnectionId,
  );
  const { data: connections } = useConnections();
  const { canInstall, install } = usePwaInstall();
  const updateReady = usePwaStore((state) => state.updateReady);
  const hasDirtyTabs = useWorkspaceStore((state) =>
    state.tabs.some((tab) => tab.kind === "query" && tab.dirty),
  );
  const [historyOpen, setHistoryOpen] = useState(false);
  const [savedOpen, setSavedOpen] = useState(false);
  const [confirmUpdate, setConfirmUpdate] = useState(false);

  function requestUpdate() {
    if (hasDirtyTabs) {
      setConfirmUpdate(true);
      return;
    }
    activateUpdate();
  }

  const activeConnection = connections?.find(
    (connection) => connection.id === activeConnectionId,
  );
  const contextLabel = activeConnection
    ? activeConnection.name
    : activeConnectionId
      ? activeConnectionId
      : "No active connection";

  return (
    <header
      className="flex h-11 shrink-0 items-center justify-between gap-3 border-b border-border bg-panel px-3"
      aria-label="Application toolbar"
    >
      <div className="flex min-w-0 items-center gap-2">
        <Button
          variant="ghost"
          size="icon"
          onClick={toggleSidebar}
          aria-label="Toggle schema explorer"
          aria-controls="schema-explorer"
          aria-expanded={!sidebarCollapsed}
        >
          <Menu size={16} aria-hidden="true" />
        </Button>
        <span className="text-sm font-semibold tracking-tight text-foreground">
          DataDeck
        </span>
      </div>

      <div className="flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
        <span className="truncate" aria-live="polite">
          {contextLabel}
        </span>
        {updateReady && (
          <Button
            variant="ghost"
            size="sm"
            aria-label="Update available"
            title="A new version of DataDeck is available"
            onClick={requestUpdate}
          >
            <RefreshCw size={14} aria-hidden="true" />
            Update
          </Button>
        )}
        {canInstall && (
          <Button
            variant="ghost"
            size="icon"
            aria-label="Install DataDeck"
            title="Install DataDeck as an app"
            onClick={() => void install()}
          >
            <Download size={15} aria-hidden="true" />
          </Button>
        )}
        <Button
          variant="ghost"
          size="icon"
          aria-label="Saved queries"
          title="Saved queries"
          onClick={() => setSavedOpen(true)}
        >
          <Bookmark size={15} aria-hidden="true" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          aria-label="Query history"
          title="Query history"
          onClick={() => setHistoryOpen(true)}
        >
          <History size={15} aria-hidden="true" />
        </Button>
      </div>

      <HistoryDialog open={historyOpen} onOpenChange={setHistoryOpen} />
      <SavedQueriesDialog open={savedOpen} onOpenChange={setSavedOpen} />

      <AlertDialog open={confirmUpdate} onOpenChange={setConfirmUpdate}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Update DataDeck?</AlertDialogTitle>
            <AlertDialogDescription>
              Reloading applies the new version. You have unsaved SQL in one or
              more tabs that will be lost. Save your work first if you need it.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Not now</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                setConfirmUpdate(false);
                activateUpdate();
              }}
            >
              Reload and update
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </header>
  );
}
