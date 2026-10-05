import { expect, test, type APIRequestContext, type Locator, type Page } from "@playwright/test";

/**
 * PRF-02 Table Data browsing against a real PostgreSQL server.
 *
 * Requires E2E_PG_HOST (and optional E2E_PG_PORT/USER/PASSWORD). The suite
 * seeds its own databases through the DataDeck API, so no external fixtures are
 * needed. Skipped when no PostgreSQL is configured.
 */
const PG_HOST = process.env.E2E_PG_HOST;
const PG_PORT = Number(process.env.E2E_PG_PORT ?? "5432");
const PG_USER = process.env.E2E_PG_USER ?? "postgres";
const PG_PASSWORD = process.env.E2E_PG_PASSWORD ?? "";
const DB_PREFIX = "datadeck_e2e_td";
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

/**
 * The database node button inside the Databases list. Its accessible name is
 * "<db> database" when collapsed and "<db>" when expanded, so both are matched;
 * scoping to the Databases list avoids the connection-profile buttons that also
 * contain the database name.
 */
function databaseButton(page: Page, database: string): Locator {
  return page
    .getByRole("list", { name: "Databases" })
    .getByRole("button", { name: new RegExp(`^${database}( database)?$`) });
}

/** Expands a database node and opens one table inside its public schema. */
async function openTable(page: Page, database: string, table: string): Promise<void> {
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
  await section
    .getByRole("button", { name: new RegExp(`^${table}\\b`) })
    .first()
    .dblclick();
}

test.describe("table data browsing (PostgreSQL)", () => {
  test.skip(!PG_HOST, "E2E_PG_HOST not set; skipping PostgreSQL Table Data E2E");

  test("alpha/beta isolation, pagination and refresh", async ({ page, request }) => {
    const admin = await createProfile(request, {
      name: "E2E TD Admin",
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
        name: `E2E TD ${database}`,
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
        "CREATE TABLE public.users (id int PRIMARY KEY, marker text, note text)",
      );
      await exec(
        request,
        connection,
        `INSERT INTO public.users VALUES (1, '${marker}', NULL), (2, '${marker}', 'x')`,
      );
      await exec(
        request,
        connection,
        "CREATE TABLE public.many AS SELECT g AS id, 'row-' || g AS label FROM generate_series(1, 150) g",
      );
    };
    await seed(DB_ALPHA, "ALPHA");
    await seed(DB_BETA, "BETA");

    await createProfile(request, {
      name: "E2E TD Server",
      driver: "postgres",
      host: PG_HOST,
      port: PG_PORT,
      database_name: "",
      username: PG_USER,
      password: PG_PASSWORD,
      ssl_mode: "disable",
    });

    await page.goto("/");
    await page.getByRole("button", { name: /^E2E TD Server/ }).first().click();

    // Open alpha.users → Table Data tab showing ALPHA.
    await openTable(page, DB_ALPHA, "users");
    await expect(
      page.getByRole("gridcell").filter({ hasText: "ALPHA" }).first(),
    ).toBeVisible();

    // Changing the explorer selection must not change the alpha tab's target.
    await databaseButton(page, DB_BETA).click();
    await expect(
      page.getByRole("gridcell").filter({ hasText: "ALPHA" }).first(),
    ).toBeVisible();
    await expect(page.getByRole("gridcell").filter({ hasText: "BETA" })).toHaveCount(0);

    // Refresh keeps the alpha binding.
    await page.getByRole("button", { name: "Refresh table data" }).click();
    await expect(
      page.getByRole("gridcell").filter({ hasText: "ALPHA" }).first(),
    ).toBeVisible();

    // Open beta.users in a second tab; it must show BETA.
    await openTable(page, DB_BETA, "users");
    await expect(
      page.getByRole("gridcell").filter({ hasText: "BETA" }).first(),
    ).toBeVisible();

    // Switch back to the alpha tab; it must still show ALPHA.
    await page
      .getByRole("region", { name: "Query tabs" })
      .getByRole("button", { name: "users", exact: true })
      .click();
    await expect(
      page.getByRole("gridcell").filter({ hasText: "ALPHA" }).first(),
    ).toBeVisible();
    await expect(page.getByRole("gridcell").filter({ hasText: "BETA" })).toHaveCount(0);

    // Pagination is backend-driven: page 1 → Next → page 2.
    await openTable(page, DB_ALPHA, "many");
    await expect(page.getByTestId("table-data-page")).toContainText("Page 1");
    const next = page.getByRole("button", { name: "Next page" });
    await expect(next).toBeEnabled();
    await next.click();
    await expect(page.getByTestId("table-data-page")).toContainText("Page 2");
    await expect(
      page.getByRole("gridcell").filter({ hasText: "row-101" }).first(),
    ).toBeVisible();

    // Cleanup (best-effort).
    await exec(request, admin, `DROP DATABASE IF EXISTS ${DB_ALPHA} WITH (FORCE)`);
    await exec(request, admin, `DROP DATABASE IF EXISTS ${DB_BETA} WITH (FORCE)`);
  });
});
