import type { Metadata, Viewport } from "next";

/**
 * PWA application metadata. The manifest itself is a plain static file at
 * `public/manifest.json` (PRD §7.1); this module only points at it so Next does
 * not also generate a conflicting manifest.
 */
export const metadata: Metadata = {
  title: "DataDeck",
  description: "High performance local-first Web/PWA Database GUI",
  applicationName: "DataDeck",
  manifest: "/manifest.json",
  appleWebApp: {
    capable: true,
    title: "DataDeck",
    statusBarStyle: "black-translucent",
  },
  icons: {
    icon: [
      {
        url: "/icons/icon-192x192.png",
        sizes: "192x192",
        type: "image/png",
      },
      {
        url: "/icons/icon-512x512.png",
        sizes: "512x512",
        type: "image/png",
      },
    ],
    apple: [
      {
        url: "/icons/icon-192x192.png",
        sizes: "192x192",
        type: "image/png",
      },
    ],
  },
};

/** `themeColor` and `colorScheme` live in the viewport export in Next 15. */
export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  themeColor: "#09090b",
  colorScheme: "dark",
};
