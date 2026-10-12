/**
 * Build metadata for the frontend (PRF02-R02). The application version is the
 * canonical release version and must match the backend (`VERSION` file /
 * `internal/version.Version`). Values are resolved at build time so the static
 * export embedded in the Go binary always reports the same version as the
 * backend that serves it. `NEXT_PUBLIC_*` values are inlined by Next.js at build
 * time; the fallbacks are deterministic for an uninjected build.
 */
export type BuildChannel = "development" | "rc" | "release";

export interface BuildInfo {
  application: string;
  version: string;
  channel: BuildChannel;
  commit: string;
}

function resolveChannel(raw: string | undefined): BuildChannel {
  return raw === "release" || raw === "rc" ? raw : "development";
}

export const BUILD_INFO: BuildInfo = {
  application: "DataDeck",
  // Keep in lockstep with the repository VERSION file (validated by scripts/check-version.sh).
  version: process.env.NEXT_PUBLIC_DATADECK_VERSION ?? "0.1.0",
  channel: resolveChannel(process.env.NEXT_PUBLIC_DATADECK_CHANNEL),
  commit: process.env.NEXT_PUBLIC_DATADECK_COMMIT ?? "unknown",
};
