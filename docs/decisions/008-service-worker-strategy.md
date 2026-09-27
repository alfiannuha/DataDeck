# Service Worker Strategy for the PWA Shell

## Status

Accepted

## Context

DataDeck is a Next.js (App Router) frontend that must satisfy the foundational
PWA requirements of PRD §7.1 (manifest, standalone display, icons) without
implementing offline database behavior. Two future constraints bound the choice:

1. The build may move toward a static export (`output: "export"`).
2. The frontend may eventually be embedded in the Go binary via `embed.FS`
   (`CGO_ENABLED=0` single binary, PRD §9.1), where only static files are served.

A service worker is a browser-side static file served from the app origin, so it
must not depend on a Node/server runtime or a build-time asset pipeline that
would break static packaging.

## Decision

Use a small, hand-written service worker at `frontend/public/sw.js`, registered
by a client component in production only.

- The worker precaches a fixed app-shell list (`/`, `/manifest.json`, the two
  icons) and otherwise treats same-origin GET asset requests as cache-first.
- Navigations are network-first with a cached-shell fallback.
- The worker is a plain static file under `public/`, so it is copied verbatim by
  the Next build and is compatible with both `output: "export"` and `embed.FS`.

## Consequences

- No `next-pwa`/Workbox dependency is introduced; the bundle and dependency
  graph stay lean and exportable.
- Caching behavior is explicit and auditable in one file (see the cache
  boundaries below and `docs/security.md` §11 for local data protection).
- The worker must be manually versioned (`CACHE_NAME`) when the shell changes.
- Because the worker is hand-written, advanced strategies (background sync,
  runtime route manifests) are intentionally not available yet.

Cache boundaries enforced by the worker:

- **MAY CACHE:** same-origin GET static assets (scripts, styles, fonts, images)
  and the navigation app shell.
- **MUST NOT CACHE:** any request whose path is `/api` or starts with `/api/`
  (connection tests, schema introspection, query execution, query history,
  saved queries), plus all cross-origin and non-GET requests. Credentials are
  never stored client-side.

## Alternatives Considered

- **`next-pwa` / Workbox:** rejected — adds a build-time plugin and dependency
  weight, and couples caching to the Next build, complicating a future static
  export and `embed.FS` packaging.
- **App Router `app/manifest.ts` dynamic route:** rejected for now because the
  PRD fixes a static `public/manifest.json`; generating both would create
  duplicate/conflicting manifest metadata. A static file also survives static
  export without a route runtime.
- **No service worker:** rejected — installability and the app-shell offline
  fallback are part of the PWA foundation.

## Install & Update Lifecycle (M4-T08)

- **Install:** the browser's `beforeinstallprompt` is captured and the UI shows
  an install action only while a deferred prompt exists. Browsers without
  support (or when already standalone) never render a broken Install button.
  DataDeck does not force installation.
- **Standalone:** `display: "standalone"` in the manifest; the app remains fully
  functional when launched installed (it is the same client code, and offline
  behavior is the M4-T07 shell).
- **Update:** `install` no longer calls `skipWaiting()`. A newly installed worker
  stays waiting, the client surfaces an "Update" affordance, and activation
  happens only when the user posts `SKIP_WAITING`. The page reloads only on the
  resulting `controllerchange`, and only if the update was user-initiated.
- **Dirty-workspace safety:** if any query tab is dirty, the UI requires an
  explicit confirmation before applying an update; nothing reloads automatically
  and editor SQL is never discarded without consent.
- **Cache cleanup scope:** activation deletes only caches whose name starts with
  `datadeck-shell-`, so unrelated caches on the same origin are never removed.
  `CACHE_NAME` is versioned (`datadeck-shell-v2`).

## Constraints

- The worker never caches API responses (sensitive database data must not be
  persisted client-side).
- Registration is production-only so development is never served stale assets.
- Any future offline *database* behavior requires a separate design and must
  respect the cache boundaries above.
