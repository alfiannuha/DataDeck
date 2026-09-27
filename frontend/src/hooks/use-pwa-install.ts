"use client";

import { useSyncExternalStore } from "react";

import {
  getCanInstall,
  initInstallCapture,
  isStandalone,
  promptInstall,
  subscribeInstall,
} from "@/lib/pwa/install";

// Capture beforeinstallprompt as soon as this module loads on the client.
initInstallCapture();

/**
 * Install/standalone state. `canInstall` is true only after the browser fires a
 * deferrable install prompt, so unsupported browsers never see a broken button.
 */
export function usePwaInstall() {
  const canInstall = useSyncExternalStore(
    subscribeInstall,
    getCanInstall,
    () => false,
  );
  const standalone = useSyncExternalStore(
    subscribeInstall,
    isStandalone,
    () => false,
  );

  return {
    canInstall: canInstall && !standalone,
    standalone,
    install: promptInstall,
  };
}
