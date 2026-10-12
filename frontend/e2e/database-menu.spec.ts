import {
  expect,
  test,
  type APIRequestContext,
  type Locator,
  type Page,
} from "@playwright/test";

/**
 * PRF-02/T16 database context menu & database-scoped reconnect against
 * PostgreSQL. Requires E2E_PG_HOST (and optional E2E_PG_PORT/USER/PASSWORD).
 * Seeds its own databases through the DataDeck API; skipped without PostgreSQL.
 *
 * Mandatory alpha/beta isolation: reconnect targets the database whose node
 * opened the menu — never the Explorer selection or the active tab — and leaves
 * the other database's pool and tabs intact.
 */
const PG_HOST = process.env.E2E_PG_HOST;
const PG_PORT = Number(process.env.E2E_PG_PORT ?? "5432");
const PG_USER = process.env.E2E_PG_USER ?? "postgres";
const PG_PASSWORD = process.env.E2E_PG_PASSWORD ?? "";
const DB_PREFIX = "datadeck_e2e_dm";
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

function databaseButton(page: Page, database: string): Locator {
  return page
    .getByRole("list", { name: "Databases" })
    .getByRole("button", { name: new RegExp(`^${database}( database)?$`) });
}

function databaseSection(page: Page, database: string): Locator {
  return databaseButton(page, database).locator("xpath=ancestor::li[1]");
}

/** Opens a database node's ⋯ context menu. */
async function openDatabaseMenu(page: Page, database: string): Promise<Locator> {
  await page
    .getByRole("list", { name: "Databases" })
    .getByRole("button", { name: `More actions for ${database}` })
    .click();
  const menu = page.getByRole("menu", { name: `More actions for ${database}` });
  await expect(menu).toBeVisible();
  return menu;
}

/** Expands a database's public schema so its table nodes are visible. */
async function expandDatabase(page: Page, database: string): Promise<void> {
  const dbButton = databaseButton(page, database);
  if ((await dbButton.getAttribute("aria-expanded")) !== "true") {
    await dbButton.click();
  }
  const section = databaseSection(page, database);
  const publicButton = section.getByRole("button", { name: /^public\b/ }).first();
  await expect(publicButton).toBeVisible();
  if ((await publicButton.getAttribute("aria-expanded")) !== "true") {
    await publicButton.click();
  }
}

/** Double-clicks a table to open its Table Data tab. */
async function openTableData(
  page: Page,
  database: string,
  table: string,
): Promise<void> {
  await expandDatabase(page, database);
  await databaseSection(page, database)
    .getByRole("button", { name: new RegExp(`^${table}\\b`) })
    .first()
    .dblclick();
}

/** Clicks a workspace tab by its exact visible title. */
async function selectTab(page: Page, title: string): Promise<void> {
  await page.getByRole("button", { name: title, exact: true }).click();
}

test.describe("database context menu (PostgreSQL)", () => {
  test.skip(!PG_HOST, "E2E_PG_HOST not set; skipping PostgreSQL database-menu E2E");
  test.use({ permissions: ["clipboard-read", "clipboard-write"] });

  test("reconnect stays bound to the menu's database (alpha/beta isolation)", async ({
    page,
    request,
  }) => {
    const admin = await createProfile(request, {
      name: "E2E DM Admin",
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

    const seed = async (database: string, marker: string) => {
      const connection = await createProfile(request, {
        name: `E2E DM ${database}`,
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
        "CREATE TABLE public.users (id int PRIMARY KEY, marker text)",
      );
      await exec(
        request,
        connection,
        `INSERT INTO public.users VALUES (1, '${marker}')`,
      );
      return connection;
    };
    await seed(DB_ALPHA, "ALPHA");
    await seed(DB_BETA, "BETA");

    await createProfile(request, {
      name: "E2E DM Server",
      driver: "postgres",
      host: PG_HOST,
      port: PG_PORT,
      database_name: "",
      username: PG_USER,
      password: PG_PASSWORD,
      ssl_mode: "disable",
    });

    await page.goto("/");
    await page.getByRole("button", { name: /^E2E DM Server/ }).first().click();
    await expect(databaseButton(page, DB_ALPHA)).toBeVisible();

    // ――― Open a beta tab first, then two alpha tabs ―――
    await openTableData(page, DB_BETA, "users");
    await expect(
      page.getByRole("gridcell").filter({ hasText: "BETA" }).first(),
    ).toBeVisible();

    await openTableData(page, DB_ALPHA, "users");
    await expect(
      page.getByRole("gridcell").filter({ hasText: "ALPHA" }).first(),
    ).toBeVisible();

    await expandDatabase(page, DB_ALPHA);
    await databaseSection(page, DB_ALPHA)
      .getByRole("button", { name: "More actions for users" })
      .click();
    await page.getByRole("menuitem", { name: "Structure" }).click();
    await expect(page.getByTestId("table-structure-binding")).toContainText(
      DB_ALPHA,
    );

    const tabStrip = page.locator('ul[aria-label="Open query tabs"] > li');
    const tabsBefore = await tabStrip.count();

    // ――― Reconnect alpha, then switch the Explorer selection to beta ―――
    await openDatabaseMenu(page, DB_ALPHA);
    await page.getByRole("menuitem", { name: "Reconnect" }).click();
    await databaseButton(page, DB_BETA).click();

    await expect(
      page.getByRole("status").filter({ hasText: `Reconnected ${DB_ALPHA}` }),
    ).toBeVisible();

    // Reconnect never opened, closed or rebound a tab.
    expect(await tabStrip.count()).toBe(tabsBefore);

    // The alpha tabs still read alpha; the beta tab still reads beta.
    await selectTab(page, "users (2)"); // alpha table data
    await expect(
      page.getByRole("gridcell").filter({ hasText: "ALPHA" }).first(),
    ).toBeVisible();
    await selectTab(page, "users (structure)"); // alpha structure
    await expect(page.getByTestId("table-structure-binding")).toContainText(
      DB_ALPHA,
    );
    await selectTab(page, "users"); // beta table data
    await expect(
      page.getByRole("gridcell").filter({ hasText: "BETA" }).first(),
    ).toBeVisible();

    // Cleanup (best-effort).
    await exec(request, admin, `DROP DATABASE IF EXISTS ${DB_ALPHA} WITH (FORCE)`);
    await exec(request, admin, `DROP DATABASE IF EXISTS ${DB_BETA} WITH (FORCE)`);
  });
});
