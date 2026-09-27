import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

import { describe, expect, it } from "vitest";

import { metadata, viewport } from "@/app/pwa-metadata";

const root = process.cwd();
const manifestPath = path.join(root, "public/manifest.json");
const swPath = path.join(root, "public/sw.js");

interface WebManifest {
  name: string;
  short_name: string;
  description: string;
  start_url: string;
  display: string;
  background_color: string;
  theme_color: string;
  icons: { src: string; sizes: string; type: string }[];
}

function manifest(): WebManifest {
  return JSON.parse(readFileSync(manifestPath, "utf8")) as WebManifest;
}

describe("PWA manifest", () => {
  it("is available at the PRD path with required fields", () => {
    const m = manifest();
    expect(m.name).toBe("DataDeck Studio");
    expect(m.short_name).toBe("DataDeck");
    expect(m.start_url).toBe("/");
    expect(m.display).toBe("standalone");
    expect(m.background_color).toBe("#09090b");
    expect(m.theme_color).toBe("#09090b");
  });

  it("references 192 and 512 icons that exist on disk", () => {
    const m = manifest();
    const sizes = m.icons.map((icon) => icon.sizes);
    expect(sizes).toContain("192x192");
    expect(sizes).toContain("512x512");
    for (const icon of m.icons) {
      expect(icon.type).toBe("image/png");
      expect(icon.src.startsWith("/icons/")).toBe(true);
      expect(existsSync(path.join(root, "public", icon.src))).toBe(true);
    }
  });

  it("stays small (no unnecessary large assets)", () => {
    for (const icon of manifest().icons) {
      const bytes = readFileSync(path.join(root, "public", icon.src)).length;
      expect(bytes).toBeLessThan(100_000);
    }
  });
});

describe("Next application metadata", () => {
  it("links the static manifest without generating a conflicting one", () => {
    expect(metadata.manifest).toBe("/manifest.json");
    expect(metadata.applicationName).toBe("DataDeck");
    const apple = metadata.appleWebApp as { capable?: boolean };
    expect(apple?.capable).toBe(true);
  });

  it("references the same icon set", () => {
    const iconsField = metadata.icons as { icon?: unknown[] } | undefined;
    const icons = (iconsField?.icon ?? []) as (string | { url: string | URL })[];
    const iconUrls = icons.map((icon) =>
      typeof icon === "string" ? icon : String(icon.url),
    );
    expect(iconUrls).toContain("/icons/icon-192x192.png");
    expect(iconUrls).toContain("/icons/icon-512x512.png");
  });

  it("sets a standalone theme color", () => {
    expect(viewport.themeColor).toBe("#09090b");
    expect(viewport.colorScheme).toBe("dark");
  });
});

describe("service worker cache boundaries", () => {
  const source = () => readFileSync(swPath, "utf8");

  it("excludes all API traffic from caching", () => {
    const sw = source();
    expect(sw).toContain('url.pathname === "/api"');
    expect(sw).toContain('url.pathname.startsWith("/api/")');
    expect(sw).toContain("isApiRequest(url)) return");
  });

  it("only handles same-origin GET requests", () => {
    const sw = source();
    expect(sw).toContain('request.method !== "GET"');
    expect(sw).toContain("url.origin !== self.location.origin");
  });

  it("uses a versioned shell cache", () => {
    const sw = source();
    expect(sw).toContain('const CACHE_PREFIX = "datadeck-shell-"');
    expect(sw).toContain("CACHE_PREFIX + \"v2\"");
  });

  it("cleans up only its own versioned caches", () => {
    expect(source()).toContain("key.startsWith(CACHE_PREFIX)");
  });

  it("activates a new worker only on explicit user request", () => {
    const sw = source();
    // skipWaiting appears exactly once, inside the SKIP_WAITING message path.
    expect(sw).toContain('event.data.type === "SKIP_WAITING"');
    expect(sw.split("self.skipWaiting()").length - 1).toBe(1);
    expect(sw).toContain("do NOT skipWaiting");
  });

  it("serves navigations from the cached app shell when offline", () => {
    const sw = source();
    expect(sw).toContain('request.mode === "navigate"');
    expect(sw).toContain('caches.match("/")');
  });
});
