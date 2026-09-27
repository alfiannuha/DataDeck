"use client";

import { usePwaStore } from "@/store/usePwaStore";

let waitingWorker: ServiceWorker | null = null;
let reloadOnControllerChange = false;

/**
 * User-controlled update activation. Posts SKIP_WAITING to the waiting worker;
 * the page reloads only after the controller actually changes, so an update is
 * never applied behind the user's back.
 */
export function activateUpdate(): boolean {
  if (!waitingWorker) return false;
  reloadOnControllerChange = true;
  waitingWorker.postMessage({ type: "SKIP_WAITING" });
  return true;
}

/** Test seam: forget any tracked waiting worker between tests. */
export function resetServiceWorkerState() {
  waitingWorker = null;
  reloadOnControllerChange = false;
}

function trackRegistration(registration: ServiceWorkerRegistration) {
  if (registration.waiting && navigator.serviceWorker.controller) {
    waitingWorker = registration.waiting;
    usePwaStore.getState().setUpdateReady(true);
  }

  registration.addEventListener("updatefound", () => {
    const installing = registration.installing;
    if (!installing) return;
    installing.addEventListener("statechange", () => {
      // A worker that finished installing while one already controls the page
      // is an available update, not the first install.
      if (installing.state === "installed" && navigator.serviceWorker.controller) {
        waitingWorker = registration.waiting ?? installing;
        usePwaStore.getState().setUpdateReady(true);
      }
    });
  });
}

/**
 * Register the service worker and surface available updates. Failures are
 * swallowed: DataDeck works without offline/update support.
 */
export function registerServiceWorker() {
  if (typeof navigator === "undefined" || !("serviceWorker" in navigator)) {
    return;
  }

  navigator.serviceWorker.addEventListener("controllerchange", () => {
    if (reloadOnControllerChange) {
      window.location.reload();
    }
  });

  navigator.serviceWorker
    .register("/sw.js")
    .then((registration) => {
      trackRegistration(registration);
      // Re-check for a new version when the app regains focus.
      document.addEventListener("visibilitychange", () => {
        if (document.visibilityState === "visible") {
          void registration.update().catch(() => {});
        }
      });
    })
    .catch(() => {
      // Registration is best-effort.
    });
}
