import { ServiceWorkerRegistration } from "@/components/shared/service-worker-registration";

import { AppProviders } from "./providers";
import {
  metadata as pwaMetadata,
  viewport as pwaViewport,
} from "./pwa-metadata";
import "./globals.css";

export const metadata = pwaMetadata;
export const viewport = pwaViewport;

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>
        <AppProviders>{children}</AppProviders>
        <ServiceWorkerRegistration />
      </body>
    </html>
  );
}
