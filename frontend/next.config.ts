import type { NextConfig } from "next";

/**
 * Static export mode.
 *
 * Default builds keep the standard Next.js server output (`npm run build`).
 * Setting `DATADECK_STATIC_EXPORT=true` switches to `output: "export"`, which
 * emits `frontend/out/` — a fully static bundle that can be served by any
 * static host (and later embedded with Go `embed.FS`) without a Node runtime.
 *
 * `trailingSlash: true` is used only in static mode so a plain static server
 * maps `/route` to `/route/index.html` deterministically.
 */
const staticExport = process.env.DATADECK_STATIC_EXPORT === "true";

const nextConfig: NextConfig = {
  ...(staticExport ? { output: "export" as const, trailingSlash: true } : {}),
  // The app does not use next/image; disable optimization so no Next server is
  // required to serve images in the static bundle.
  images: { unoptimized: true },
};

export default nextConfig;
