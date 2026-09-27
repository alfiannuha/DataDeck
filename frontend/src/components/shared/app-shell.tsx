"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";

import { useOnline } from "@/hooks/use-online";
import { useConnectionStore } from "@/store/useConnectionStore";
import { useExecutionStore } from "@/store/useExecutionStore";

import { OfflineBanner } from "./offline-banner";
import { Sidebar } from "./sidebar";
import { StatusBar } from "./status-bar";
import { TopBar } from "./top-bar";
import { Workspace } from "./workspace";

/**
 * Root application shell. Full-viewport flex column: top bar, a middle row of
 * sidebar + workspace, and a status bar. The shell itself never scrolls; each
 * inner region contains its own overflow.
 *
 * Switching connections clears the previous execution result so the status bar
 * and result grid never show data from a different connection. Reconnecting
 * refetches server state (never auto-executes SQL).
 */
export function AppShell() {
  const activeConnectionId = useConnectionStore(
    (state) => state.activeConnectionId,
  );
  const previousConnectionId = useRef(activeConnectionId);
  const online = useOnline();
  const queryClient = useQueryClient();
  const wasOffline = useRef(false);

  useEffect(() => {
    if (previousConnectionId.current !== activeConnectionId) {
      previousConnectionId.current = activeConnectionId;
      useExecutionStore.getState().reset();
    }
  }, [activeConnectionId]);

  // Recovery: when connectivity returns, safely refetch server state. This is
  // a data refresh only — it never runs a query on the user's behalf.
  useEffect(() => {
    if (!online) {
      wasOffline.current = true;
      return;
    }
    if (wasOffline.current) {
      wasOffline.current = false;
      void queryClient.invalidateQueries();
    }
  }, [online, queryClient]);

  return (
    <div className="flex h-dvh w-full flex-col overflow-hidden bg-background text-foreground">
      <TopBar />
      <OfflineBanner />
      <div className="flex min-h-0 flex-1">
        <Sidebar />
        <Workspace />
      </div>
      <StatusBar />
    </div>
  );
}
