import { expect, test, type APIRequestContext, type Locator, type Page } from "@playwright/test";

/**
 * PRF-02 Table Structure metadata against a real PostgreSQL server.
 *
 * Requires E2E_PG_HOST (and optional E2E_PG_PORT/USER/PASSWORD). The suite
 * seeds its own databases through the DataDeck API, so no external fixtures are
 * needed. Skipped when no PostgreSQL is configured.
 *
 * Covers the forced-different alpha/beta consistency (structure columns must
 * never bleed between tabs with different bindings) and the no-HTML rule
 * (constraint definition text renders as text).
 */
const PG_HOST = process.env.E2E_PG_HOST;
const PG_PORT = Number(process.env.E2E_PG_PORT ?? "5432");
const PG_USER = process.env.E2E_PG_USER ?? "postgres";
const PG_PASSWORD = process.env.E2E_PG_PASSWORD ?? "";
const DB_PREFIX = "datadeck_e2e_ts";
const DB_ALPHA = `${DB_PREFIX}_alpha`;
const DB_BETA = `${DB_PREFIX}_beta`;

async function createProfile(
  request: APIRequestContext,
  data: Record<string, unknown>,
): Promise<string> {
  const response = await request.post("/api/v1/connections", { data });
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  return body.data.id as string;
}

async function exec(
  request: APIRequestContext,
  connectionId: string,
  sql: string,
): Promise<void> {
  const response = await request.post("/api/v1/query/execute", {
    data: { connection_id: connectionId, sql },
  });
  expect(response.ok(), `SQL failed: ${sql}`).toBeTruthy();
}

/** The database node button inside the Databases list. */
function databaseButton(page: Page, database: string): Locator {
  return page
    .getByRole("list", { name: "Databases" })
    .getByRole("button", { name: new RegExp(`^${database}( database)?$`) });
}

/** Expands a database node across its public schema so tables are visible. */
async function expandDatabase(page: Page, database: string): Promise<Locator> {
  const dbButton = databaseButton(page, database);
  if ((await dbButton.getAttribute("aria-expanded")) !== "true") {
    await dbButton.click();
  }
  const section = dbButton.locator("xpath=ancestor::li[1]");
  const publicButton = section.getByRole("button", { name: /^public\b/ }).first();
  await expect(publicButton).toBeVisible();
  if ((await publicButton.getAttribute("aria-expanded")) !== "true") {
    await publicButton.click();
  }
  return section;
}

/** Opens the ⋯ context menu of one table inside the given database. */
async function openTableMenu(
  page: Page,
  database: string,
  table: string,
): Promise<Locator> {
  await expandDatabase(page, database);
  await databaseButton(page, database)
    .locator("xpath=ancestor::li[1]")
    .getByRole("button", { name: `More actions for ${table}` })
    .click();
  const menu = page.getByRole("menu");
  await expect(menu).toBeVisible();
  return menu;
}

test.describe("table structure (PostgreSQL)", () => {
  test.skip(!PG_HOST, "E2E_PG_HOST not set; skipping PostgreSQL table-structure E2E");

  test("alpha/beta isolation, read-only flags and no-HTML constraint text", async ({
    page,
    request,
  }) => {
    const admin = await createProfile(request, {
      name: "E2E TS Admin",
      driver: "postgres",
      host: PG_HOST,
      port: PG_PORT,
      database_name: "postgres",
      username: PG_USER,
      password: PG_PASSWORD,
      ssl_mode: "disable",
    });

    for (const database of [DB_ALPHA, DB_BETA]) {
      await exec(request, admin, `DROP DATABASE IF EXISTS ${database} WITH (FORCE)`);
      await exec(request, admin, `CREATE DATABASE ${database}`);
    }

    const seed = async (database: string, column: string) => {
      const connection = await createProfile(request, {
        name: `E2E TS ${database}`,
        driver: "postgres",
        host: PG_HOST,
        port: PG_PORT,
        database_name: database,
        username: PG_USER,
        password: PG_PASSWORD,
        ssl_mode: "disable",
      });
      await exec(
        request,
        connection,
        `CREATE TABLE public.users (id bigint PRIMARY KEY, ${column} text)`,
      );
      return connection;
    };

    const alpha = await seed(DB_ALPHA, "marker");
    await exec(
      request,
      alpha,
      `ALTER TABLE public.users ADD CONSTRAINT users_marker_html
       CHECK (marker <> '<script>alert("x")</script>')`,
    );
    await seed(DB_BETA, "email");

    await createProfile(request, {
      name: "E2E TS Server",
      driver: "postgres",
      host: PG_HOST,
      port: PG_PORT,
      database_name: "",
      username: PG_USER,
      password: PG_PASSWORD,
      ssl_mode: "disable",
    });

    await page.goto("/");
    await page.getByRole("button", { name: /^E2E TS Server/ }).first().click();

    // ――― A. Alpha users: structure tab bound to the exact target ―――
    await openTableMenu(page, DB_ALPHA, "users");
    await page.getByRole("menuitem", { name: "Structure" }).click();

    const panel = page.getByTestId("table-structure-panel");
    await expect(panel).toBeVisible();
    await expect(page.getByTestId("table-structure-binding")).toHaveText(DB_ALPHA);
    await expect(page.getByText("public.users")).toBeVisible();
    await expect(page.getByTestId("table-structure-columns")).toContainText("marker");
    await expect(page.getByTestId("table-structure-columns")).not.toContainText("email");

    // PK column id is read-only (identity/primary key): insert and update "no".
    const columns = page.getByTestId("table-structure-columns");
    const idRow = columns.locator("tbody tr", { hasText: "id" }).first();
    await expect(idRow).toContainText("PK");
    await expect(idRow).toContainText("no");
    await expect(columns.locator("tbody tr", { hasText: "marker" }).first()).toContainText(
      "yes",
    );

    // ――― B. Constraint definition is plain text, never HTML ―――
    const constraints = page.getByTestId("table-structure-constraints");
    const constraintRow = constraints.locator("tbody tr", { hasText: "users_marker_html" });
    // The definition is rendered as plain text, cast and all; it must never
    // become a live script tag.
    await expect(constraintRow).toContainText('<script>alert("x")</script>');
    await expect(page.locator("script").filter({ hasText: 'alert("x")' })).toHaveCount(0);

    // ――― C. Beta users keep their own structure; alpha tab stays isolated ―――
    await openTableMenu(page, DB_BETA, "users");
    await page.getByRole("menuitem", { name: "Structure" }).click();
    await expect(page.getByTestId("table-structure-binding")).toHaveText(DB_BETA);
    await expect(page.getByTestId("table-structure-columns")).toContainText("email");
    await expect(page.getByTestId("table-structure-columns")).not.toContainText("marker");

    // Back to the alpha tab: still bound to alpha and still shows marker.
    await page.getByRole("button", { name: "users (structure)", exact: true }).click();
    await expect(page.getByTestId("table-structure-binding")).toHaveText(DB_ALPHA);
    await expect(page.getByTestId("table-structure-columns")).toContainText("marker");
    await expect(page.getByTestId("table-structure-columns")).not.toContainText("email");

    // ――― D. Refresh refetches the same binding and keeps the structure ―――
    await page.getByTestId("table-structure-refresh").click();
    await expect(page.getByTestId("table-structure-columns")).toContainText("marker");
  });
});