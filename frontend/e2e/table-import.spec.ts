import {
  expect,
  test,
  type APIRequestContext,
  type Locator,
  type Page,
} from "@playwright/test";

/**
 * PRF-02/T15 Table import (CSV) against PostgreSQL.
 *
 * Requires E2E_PG_HOST (and optional E2E_PG_PORT/USER/PASSWORD). Seeds its own
 * databases through the DataDeck API. Skipped when no PostgreSQL is configured.
 * Verifies the import stays bound to the table the menu was opened on (α/β
 * isolation, never the active tab or Explorer selection), that an unquoted empty
 * field imports as NULL, and that read-only objects cannot be imported into.
 */
const PG_HOST = process.env.E2E_PG_HOST;
const PG_PORT = Number(process.env.E2E_PG_PORT ?? "5432");
const PG_USER = process.env.E2E_PG_USER ?? "postgres";
const PG_PASSWORD = process.env.E2E_PG_PASSWORD ?? "";
const DB_PREFIX = "datadeck_e2e_im";
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

/** Uploads `csv` through the import dialog and executes it. */
async function importCsvViaMenu(
  page: Page,
  database: string,
  table: string,
  filename: string,
  csv: string,
): Promise<void> {
  await openTableMenu(page, database, table);
  await page.getByRole("menuitem", { name: "Import" }).click();
  const dialog = page.getByTestId("import-dialog");
  await expect(dialog).toBeVisible();
  await dialog.getByTestId("import-file").setInputFiles({
    name: filename,
    mimeType: "text/csv",
    buffer: Buffer.from(csv, "utf-8"),
  });
  await dialog.getByTestId("import-upload").click();
  await expect(dialog.getByTestId("import-validation-ok")).toBeVisible();
  await dialog.getByTestId("import-execute").click();
  await expect(dialog.getByTestId("import-done-message")).toBeVisible();
  await dialog.getByTestId("import-dismiss").click();
  await expect(page.getByTestId("import-dialog")).toHaveCount(0);
}

test.describe("table import (PostgreSQL)", () => {
  test.skip(!PG_HOST, "E2E_PG_HOST not set; skipping PostgreSQL import E2E");

  test("imports CSV bound to the menu target and rejects read-only objects", async ({
    page,
    request,
  }) => {
    const admin = await createProfile(request, {
      name: "E2E IM Admin",
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
        name: `E2E IM ${database}`,
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
        `INSERT INTO public.users VALUES (1, '${marker}', 'seed')`,
      );
      await exec(
        request,
        connection,
        "CREATE VIEW public.v_users AS SELECT id, marker FROM public.users",
      );
      return connection;
    };
    const alphaDirect = await seed(DB_ALPHA, "ALPHA");
    await seed(DB_BETA, "BETA");

    await createProfile(request, {
      name: "E2E IM Server",
      driver: "postgres",
      host: PG_HOST,
      port: PG_PORT,
      database_name: "",
      username: PG_USER,
      password: PG_PASSWORD,
      ssl_mode: "disable",
    });

    await page.goto("/");
    await page.getByRole("button", { name: /^E2E IM Server/ }).first().click();

    // Open an unrelated beta tab first: it must never retarget the alpha import.
    await openTableMenu(page, DB_BETA, "users");
    await page.getByRole("menuitem", { name: "View Data" }).click();
    await expect(
      page.getByRole("gridcell").filter({ hasText: "BETA" }).first(),
    ).toBeVisible();

    // ――― CSV import into alpha.users; a trailing empty field becomes NULL ―――
    await importCsvViaMenu(
      page,
      DB_ALPHA,
      "users",
      "users.csv",
      "id,marker,note\n2,IMPORTED,\n",
    );

    const rows = await request
      .post("/api/v1/query/execute", {
        data: {
          connection_id: alphaDirect,
          sql: "SELECT id, marker, note FROM public.users ORDER BY id",
        },
      })
      .then((response) => response.json());
    const values = rows.data.rows as (string | number | null)[][];
    // The id is a 32-bit int (serialized as a JSON number); normalize for compare.
    expect(values.map((row) => [String(row[0]), row[1], row[2]])).toEqual([
      ["1", "ALPHA", "seed"],
      ["2", "IMPORTED", null],
    ]);

    // ――― Read-only object: the Import menu item is disabled with an explanation ―――
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
