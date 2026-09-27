import { defineConfig } from "@playwright/test";

/**
 * Browser E2E configuration.
 *
 * The release-flow suite runs against an already-started DataDeck server (the
 * single binary with the embedded frontend). The server URL is provided via
 * `E2E_BASE_URL` (default http://127.0.0.1:8099); the CI workflow builds and
 * launches the binary, so no `webServer` is configured here.
 */
export default defineConfig({
  testDir: "./e2e",
  timeout: 60_000,
  expect: { timeout: 15_000 },
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["list"], ["github"]] : "list",
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://127.0.0.1:8099",
    trace: "retain-on-failure",
  },
});
