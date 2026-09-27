"use client";

import { useEffect } from "react";

import { registerServiceWorker } from "@/lib/pwa/service-worker";

/**
 * Registers the DataDeck service worker in production only. Dev and test runs
 * never register so stale caches cannot mask local changes.
 */
export function ServiceWorkerRegistration() {
  useEffect(() => {
    if (process.env.NODE_ENV !== "production") return;
    registerServiceWorker();
  }, []);

  return null;
}
