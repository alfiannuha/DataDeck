"use client";

interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: "accepted" | "dismissed" }>;
}

let deferredPrompt: BeforeInstallPromptEvent | null = null;
let appInstalled = false;
const listeners = new Set<() => void>();

function emit() {
  for (const listener of listeners) listener();
}

/** Capture the browser install prompt as early as possible (client only). */
export function initInstallCapture() {
  if (typeof window === "undefined") return;
  window.addEventListener("beforeinstallprompt", (event) => {
    event.preventDefault();
    deferredPrompt = event as BeforeInstallPromptEvent;
    emit();
  });
  window.addEventListener("appinstalled", () => {
    deferredPrompt = null;
    appInstalled = true;
    emit();
  });
}

export function subscribeInstall(onChange: () => void) {
  listeners.add(onChange);
  return () => listeners.delete(onChange);
}

export function getCanInstall() {
  return deferredPrompt !== null;
}

export function isStandalone() {
  if (typeof window === "undefined") return false;
  const displayMode = window.matchMedia?.("(display-mode: standalone)")?.matches;
  const iosStandalone =
    (window.navigator as { standalone?: boolean }).standalone === true;
  return appInstalled || Boolean(displayMode) || iosStandalone;
}

export type InstallOutcome = "accepted" | "dismissed" | "unavailable";

/** Test seam: clear captured prompt/installed state between tests. */
export function resetInstallState() {
  deferredPrompt = null;
  appInstalled = false;
  emit();
}

/** Trigger the deferred browser prompt; safe no-op when unavailable. */
export async function promptInstall(): Promise<InstallOutcome> {
  if (!deferredPrompt) return "unavailable";
  const event = deferredPrompt;
  deferredPrompt = null;
  emit();
  await event.prompt();
  const { outcome } = await event.userChoice;
  return outcome;
}
