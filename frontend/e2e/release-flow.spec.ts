import { expect, test } from "@playwright/test";

/**
 * Release-critical browser flow against the single binary (embedded frontend):
 *
 *   server → browser shell → connection → query → result → PWA signals
 *
 * The SQLite target is a throwaway file; no external database is required.
 */
test("single-binary release flow: shell, connection, query, result, PWA", async ({
  page,
  request,
}) => {
  // API is served by the same origin as the embedded frontend.
  const health = await request.get("/api/v1/health");
  expect(health.ok()).toBeTruthy();

  // Create a SQLite connection through the real API.
  const target = `/tmp/datadeck-e2e-${Date.now()}.sqlite3`;
  const created = await request.post("/api/v1/connections", {
    data: {
      name: "E2E SQLite",
      driver: "sqlite",
      database_name: target,
    },
  });
  expect(created.ok()).toBeTruthy();

  await page.goto("/");
  await expect(page).toHaveTitle("DataDeck");
  await expect(
    page.getByRole("region", { name: "SQL editor" }),
  ).toBeVisible();

  // Select the connection (activates it for the workspace).
  await page.getByRole("button", { name: /^E2E SQLite/ }).first().click();

  // Run a query through the editor and assert the virtualized result grid.
  const editor = page.locator(".cm-content");
  await editor.click();
  await page.keyboard.press("ControlOrMeta+A");
  await page.keyboard.insertText("SELECT 9007199254740993 AS big;");
  await page.getByRole("button", { name: "Run query" }).click();

  await expect(page.getByRole("gridcell").first()).toHaveText(
    "9007199254740993",
  );

  // PWA signals served from the embedded assets.
  await expect(page.locator('link[rel="manifest"]')).toHaveAttribute(
    "href",
    "/manifest.json",
  );
  const swActive = await page.evaluate(
    async () => (await navigator.serviceWorker.getRegistration())?.active?.scriptURL ?? null,
  );
  expect(swActive).toContain("/sw.js");

  // Cache Storage must not contain API responses.
  const apiCached = await page.evaluate(async () => {
    const hits: string[] = [];
    for (const name of await caches.keys()) {
      const cache = await caches.open(name);
      for (const req of await cache.keys()) {
        if (new URL(req.url).pathname.startsWith("/api")) hits.push(req.url);
      }
    }
    return hits;
  });
  expect(apiCached).toEqual([]);
});

test("unknown static asset is a 404 and API misses stay JSON", async ({
  request,
}) => {
  const asset = await request.get("/does-not-exist.js");
  expect(asset.status()).toBe(404);

  const api = await request.get("/api/v1/does-not-exist");
  expect(api.status()).toBe(404);
  const body = (await api.json()) as { error?: { code?: string } };
  expect(body.error?.code).toBe("NOT_FOUND");
});
