import { expect, test, type APIRequestContext, type Locator, type Page } from "@playwright/test";

/**
 * PRF-02/T12 Table context menu & table-level actions against PostgreSQL.
 *
 * Requires E2E_PG_HOST (and optional E2E_PG_PORT/USER/PASSWORD). Seeds its own
 * databases through the DataDeck API. Skipped when no PostgreSQL is configured.
 */
const PG_HOST = process.env.E2E_PG_HOST;
const PG_PORT = Number(process.env.E2E_PG_PORT ?? "5432");
const PG_USER = process.env.E2E_PG_USER ?? "postgres";
const PG_PASSWORD = process.env.E2E_PG_PASSWORD ?? "";
const DB_PREFIX = "datadeck_e2e_tm";
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

/** The explorer subtree for one database node (scopes table look-ups). */
function databaseSection(page: Page, database: string): Locator {
  return databaseButton(page, database).locator("xpath=ancestor::li[1]");
}

function databaseButton(page: Page, database: string): Locator {
  return page
    .getByRole("list", { name: "Databases" })
    .getByRole("button", { name: new RegExp(`^${database}( database)?$`) });
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

/** Opens the ⋯ context menu of one table inside the given database. */
async function openTableMenu(
  page: Page,
  database: string,
  table: string,
): Promise<Locator> {
  await expandDatabase(page, database);
  await databaseSection(page, database)
    .getByRole("button", { name: `More actions for ${table}` })
    .click();
  const menu = page.getByRole("menu");
  await expect(menu).toBeVisible();
  return menu;
}

test.describe("table context menu (PostgreSQL)", () => {
  test.skip(!PG_HOST, "E2E_PG_HOST not set; skipping PostgreSQL table-menu E2E");
  // Copy flows read the clipboard back through the page.
  test.use({ permissions: ["clipboard-read", "clipboard-write"] });

  test("menu capabilities, bindings, copy, refresh and read-only reasons", async ({
    page,
    request,
  }) => {
    const admin = await createProfile(request, {
      name: "E2E TM Admin",
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
        name: `E2E TM ${database}`,
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
    const alphaDirect = await seed(DB_ALPHA, "ALPHA");
    await exec(request, alphaDirect, "CREATE TABLE public.logs (message text)");
    await exec(
      request,
      alphaDirect,
      "CREATE VIEW public.v_users AS SELECT id, marker FROM public.users",
    );
    await seed(DB_BETA, "BETA");

    const serverId = await createProfile(request, {
      name: "E2E TM Server",
      driver: "postgres",
      host: PG_HOST,
      port: PG_PORT,
      database_name: "",
      username: PG_USER,
      password: PG_PASSWORD,
      ssl_mode: "disable",
    });

    await page.goto("/");
    await page.getByRole("button", { name: /^E2E TM Server/ }).first().click();

    // ――― A. Menu opens via ⋯, right-click and Shift+F10; no tab is created ―――
    const menu = await openTableMenu(page, DB_ALPHA, "users");
    for (const item of ["View Data", "New Query", "Structure", "Refresh"]) {
      await expect(menu.getByRole("menuitem", { name: item })).toBeVisible();
    }
    // PRF-02: Export is live (T14); Import is still a disabled placeholder (T15).
    await expect(page.getByTestId("table-menu-export")).not.toHaveAttribute(
      "aria-disabled",
      "true",
    );
    await expect(page.getByTestId("table-menu-import")).toHaveAttribute(
      "aria-disabled",
      "true",
    );
    await page.keyboard.press("Escape");
    await expect(menu).toHaveCount(0);

    const usersRow = databaseSection(page, DB_ALPHA).getByRole("button", { name: /^users\b/ }).first();
    await usersRow.click({ button: "right" });
    await expect(page.getByRole("menu")).toBeVisible();
    await page.keyboard.press("Escape");

    await usersRow.focus();
    await page.keyboard.press("Shift+F10");
    await expect(page.getByRole("menu")).toBeVisible();
    await page.keyboard.press("Escape");

    // Opening menus is local-only: still zero tabs.
    await expect(page.getByTestId("workspace-empty-state")).toBeVisible();

    // ――― B. View Data binds the exact target; alpha/beta stay isolated ―――
    await openTableMenu(page, DB_ALPHA, "users");
    await page.getByRole("menuitem", { name: "View Data" }).click();
    await expect(
      page.getByRole("gridcell").filter({ hasText: "ALPHA" }).first(),
    ).toBeVisible();

    // Changing the explorer selection must not retarget the alpha tab.
    await databaseButton(page, DB_BETA).click();
    await expect(
      page.getByRole("gridcell").filter({ hasText: "ALPHA" }).first(),
    ).toBeVisible();
    await expect(page.getByRole("gridcell").filter({ hasText: "BETA" })).toHaveCount(0);

    // beta.users from beta's menu opens its own tab showing BETA.
    await openTableMenu(page, DB_BETA, "users");
    await page.getByRole("menuitem", { name: "View Data" }).click();
    await expect(
      page.getByRole("gridcell").filter({ hasText: "BETA" }).first(),
    ).toBeVisible();
    await expect(page.getByRole("gridcell").filter({ hasText: "ALPHA" })).toHaveCount(0);

    // ――― C. New Query opens a fresh tab with a quoted SELECT, never executes ―――
    await openTableMenu(page, DB_ALPHA, "users");
    await page.getByRole("menuitem", { name: "New Query" }).click();
    const editor = page.locator(".cm-content");
    await expect(page.getByRole("region", { name: "SQL editor" })).toBeVisible();
    await expect(editor).toContainText('SELECT *');
    await expect(editor).toContainText('FROM "public"."users"');
    await expect(editor).toContainText("LIMIT 100");
    // No auto-execute: no result grid yet.
    await expect(page.locator("[role=gridcell]")).toHaveCount(0);

    // A second New Query never overwrites the first tab.
    await openTableMenu(page, DB_BETA, "users");
    await page.getByRole("menuitem", { name: "New Query" }).click();
    const queryTabs = page.getByRole("region", { name: "Query tabs" });
    await expect(queryTabs.getByRole("button", { name: "Query 1", exact: true })).toBeVisible();
    await expect(queryTabs.getByRole("button", { name: "Query 2", exact: true })).toBeVisible();

    // ――― D. Structure opens the panel bound to the expanded database ―――
    await openTableMenu(page, DB_BETA, "users");
    await page.getByRole("menuitem", { name: "Structure" }).click();
    const structure = page.getByTestId("table-structure-panel");
    await expect(structure).toBeVisible();
    await expect(structure).toContainText("public.users");
    await expect(page.getByTestId("table-structure-binding")).toContainText(DB_BETA);

    // ――― E. Copy Table Name / Copy Qualified Name (PG dialect quoted) ―――
    await openTableMenu(page, DB_ALPHA, "users");
    await page.getByRole("menuitem", { name: "Copy Table Name" }).click();
    await expect
      .poll(() => page.evaluate(() => navigator.clipboard.readText()))
      .toBe("users");
    await openTableMenu(page, DB_ALPHA, "users");
    await page.getByRole("menuitem", { name: "Copy Qualified Name" }).click();
    await expect
      .poll(() => page.evaluate(() => navigator.clipboard.readText()))
      .toBe('"public"."users"');

    // ――― F. Refresh re-fetches so an externally created table appears ―――
    await exec(
      request,
      alphaDirect,
      "CREATE TABLE public.late (id int PRIMARY KEY)",
    );
    await openTableMenu(page, DB_ALPHA, "users");
    await page.getByRole("menuitem", { name: "Refresh" }).click();
    await expect(
      databaseSection(page, DB_ALPHA).getByRole("button", { name: /^late\b/ }),
    ).toBeVisible();

    // ――― G. Read-only object: Import explains why it is disabled ―――
    await openTableMenu(page, DB_ALPHA, "v_users");
    await expect(page.getByTestId("table-menu-import")).toHaveAttribute(
      "aria-disabled",
      "true",
    );
    await expect(page.getByTestId("table-menu-import")).toContainText(
      "This object is read-only.",
    );

    // Cleanup (best-effort).
    await exec(request, admin, `DROP DATABASE IF EXISTS ${DB_ALPHA} WITH (FORCE)`);
    await exec(request, admin, `DROP DATABASE IF EXISTS ${DB_BETA} WITH (FORCE)`);
  });
});