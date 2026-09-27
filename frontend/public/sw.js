/*
 * DataDeck service worker.
 *
 * Strategy: a small, hand-written cache for the app shell and static assets
 * only. No Workbox/next-pwa dependency, so the build stays exportable and the
 * output can later be packaged into a Go embed.FS single binary unchanged.
 *
 * Cache boundaries (see docs/decisions/008-service-worker-strategy.md):
 *   MAY CACHE  same-origin GET static assets and navigations (app shell).
 *   NEVER CACHE anything under /api/ (connection tests, schema, query
 *   execution, history, saved queries) or cross-origin/opaque responses.
 */

const CACHE_PREFIX = "datadeck-shell-";
const CACHE_NAME = CACHE_PREFIX + "v2";

const SHELL_ASSETS = ["/", "/manifest.json", "/icons/icon-192x192.png", "/icons/icon-512x512.png"];

self.addEventListener("install", (event) => {
  // Precache the new shell but do NOT skipWaiting: activating a new worker must
  // stay user-controlled so a reload never interrupts unsaved SQL.
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(SHELL_ASSETS)),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(
          // Only clean our own versioned caches; never touch caches belonging
          // to other applications on this origin.
          keys
            .filter((key) => key.startsWith(CACHE_PREFIX) && key !== CACHE_NAME)
            .map((key) => caches.delete(key)),
        ),
      )
      .then(() => self.clients.claim()),
  );
});

self.addEventListener("message", (event) => {
  // The client explicitly asks to activate the waiting worker (user chose to
  // update from the UI). This is the only path that skips waiting.
  if (event.data && event.data.type === "SKIP_WAITING") {
    self.skipWaiting();
  }
});

/** API traffic and non-GET requests must always hit the network. */
function isApiRequest(url) {
  return url.pathname === "/api" || url.pathname.startsWith("/api/");
}

self.addEventListener("fetch", (event) => {
  const request = event.request;
  if (request.method !== "GET") return;

  const url = new URL(request.url);
  if (url.origin !== self.location.origin) return;
  if (isApiRequest(url)) return; // never cached, never served from cache

  // Navigations: network-first, falling back to the cached shell when offline.
  if (request.mode === "navigate") {
    event.respondWith(
      fetch(request).catch(() => caches.match("/").then((cached) => cached || Response.error())),
    );
    return;
  }

  // Static assets: cache-first, then network (and populate the cache).
  event.respondWith(
    caches.match(request).then((cached) => {
      if (cached) return cached;
      return fetch(request).then((response) => {
        if (response.ok && response.type === "basic") {
          const copy = response.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(request, copy));
        }
        return response;
      });
    }),
  );
});
