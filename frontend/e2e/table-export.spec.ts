import { readFile } from "node:fs/promises";

import {
  expect,
  test,
  type APIRequestContext,
  type Download,
  type Locator,
  type Page,
} from "@playwright/test";

/**
 * PRF-02/T14 Table export against PostgreSQL.
 *
 * Requires E2E_PG_HOST (and optional E2E_PG_PORT/USER/PASSWORD). Seeds its own
 * databases through the DataDeck API. Skipped when no PostgreSQL is configured.
 * Verifies the download is a real attachment, exports the whole table (ignoring
 * the page-size window), and — critically — stays bound to the table the menu was
 * opened on (α/β isolation), never the active tab or Explorer selection.
 */
const PG_HOST = process.env.E2E_PG_HOST;
const PG_PORT = Number(process.env.E2E_PG_PORT ?? "5432");
const PG_USER = process.env.E2E_PG_USER ?? "postgres";
const PG_PASSWORD = process.env.E2E_PG_PASSWORD ?? "";
const DB_PREFIX = "datadeck_e2e_ex";
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

/** Runs the dialog export and returns the captured download + its content. */
async function exportViaMenu(
  page: Page,
  database: string,
  table: string,
  format: "csv" | "xlsx" | "sql" = "csv",
): Promise<{ download: Download; content: string }> {
  await openTableMenu(page, database, table);
  await page.getByRole("menuitem", { name: "Export" }).click();
  const dialog = page.getByTestId("export-dialog");
  await expect(dialog).toBeVisible();
  await dialog.getByTestId(`export-format-${format}`).click();
  const [download] = await Promise.all([
    page.waitForEvent("download"),
    dialog.getByTestId("export-submit").click(),
  ]);
  const filePath = await download.path();
  const content = filePath ? await readFile(filePath, "utf-8") : "";
  return { download, content };
}

test.describe("table export (PostgreSQL)", () => {
  test.skip(!PG_HOST, "E2E_PG_HOST not set; skipping PostgreSQL export E2E");

  test("downloads the whole table bound to the menu target, not the active tab", async ({
    page,
    request,
  }) => {
    const admin = await createProfile(request, {
      name: "E2E EX Admin",
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
        name: `E2E EX ${database}`,
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
    await seed(DB_BETA, "BETA");
    // 1000 rows: the export must include every row, not just the 100-row page.
    await exec(request, alphaDirect, "CREATE TABLE public.bulk (id int PRIMARY KEY)");
    await exec(
      request,
      alphaDirect,
      "INSERT INTO public.bulk SELECT generate_series(1, 1000)",
    );
    await exec(
      request,
      alphaDirect,
      "CREATE VIEW public.v_users AS SELECT id, marker FROM public.users",
    );

    await createProfile(request, {
      name: "E2E EX Server",
      driver: "postgres",
      host: PG_HOST,
      port: PG_PORT,
      database_name: "",
      username: PG_USER,
      password: PG_PASSWORD,
      ssl_mode: "disable",
    });

    await page.goto("/");
    await page.getByRole("button", { name: /^E2E EX Server/ }).first().click();

    // Open an unrelated beta tab first: it must never retarget the alpha export.
    await openTableMenu(page, DB_BETA, "users");
    await page.getByRole("menuitem", { name: "View Data" }).click();
    await expect(
      page.getByRole("gridcell").filter({ hasText: "BETA" }).first(),
    ).toBeVisible();

    // ――― CSV export of alpha.users downloads the ALPHA rows only ―――
    const csv = await exportViaMenu(page, DB_ALPHA, "users");
    expect(csv.download.suggestedFilename()).toMatch(/\.csv$/);
    expect(csv.download.suggestedFilename()).toContain("users");
    expect(csv.content).toContain("ALPHA");
    expect(csv.content).not.toContain("BETA");
    // The dialog closes after a successful export.
    await expect(page.getByTestId("export-dialog")).toHaveCount(0);

    // ――― Full-table scope: 1000 rows even though the page size is 100 ―――
    const bulk = await exportViaMenu(page, DB_ALPHA, "bulk");
    const lines = bulk.content.trimEnd().split("\n");
    expect(lines).toHaveLength(1001); // header + 1000 rows

    // ――― SQL is unavailable for a view (backend is authoritative) ―――
    await openTableMenu(page, DB_ALPHA, "v_users");
    await page.getByRole("menuitem", { name: "Export" }).click();
    await expect(page.getByTestId("export-dialog")).toBeVisible();
    await expect(page.getByTestId("export-format-sql")).toBeDisabled();
    await page.getByRole("button", { name: "Cancel" }).click();
    await expect(page.getByTestId("export-dialog")).toHaveCount(0);

    // Cleanup (best-effort).
    await exec(request, admin, `DROP DATABASE IF EXISTS ${DB_ALPHA} WITH (FORCE)`);
    await exec(request, admin, `DROP DATABASE IF EXISTS ${DB_BETA} WITH (FORCE)`);
  });
});
