"use client";

import { WifiOff } from "lucide-react";

import { useOnline } from "@/hooks/use-online";

/**
 * Explicit offline/backend-unavailable notice. Offline DataDeck can only show
 * the cached application shell: connections, schema, query execution and
 * history need the backend. Editor content is never discarded while offline.
 */
export function OfflineBanner() {
  const online = useOnline();
  if (online) return null;

  return (
    <div
      role="status"
      aria-live="polite"
      className="flex shrink-0 items-center gap-2 border-b border-border bg-amber-500/10 px-3 py-1.5 text-xs text-amber-400"
    >
      <WifiOff size={12} aria-hidden="true" className="shrink-0" />
      <span className="truncate">
        Backend unavailable — you are offline. Only the cached app shell is
        available; queries and connections are disabled until you reconnect.
        Your editor content is preserved.
      </span>
    </div>
  );
}
