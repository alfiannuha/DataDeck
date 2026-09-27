"use client";

import { useSyncExternalStore } from "react";

function subscribe(onStoreChange: () => void) {
  window.addEventListener("online", onStoreChange);
  window.addEventListener("offline", onStoreChange);
  return () => {
    window.removeEventListener("online", onStoreChange);
    window.removeEventListener("offline", onStoreChange);
  };
}

function getSnapshot() {
  return typeof navigator === "undefined" ? true : navigator.onLine;
}

function getServerSnapshot() {
  return true;
}

/**
 * Reactive browser connectivity state. This reports the *network* browser
 * reachability (navigator.onLine); the backend always remains the source of
 * truth, so an "online" browser can still fail an API call and surface its own
 * error. Used to guard backend-dependent actions, never to claim a database is
 * available offline.
 */
export function useOnline(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}
