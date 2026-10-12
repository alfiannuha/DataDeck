import {
  expect,
  test,
  type APIRequestContext,
  type Locator,
  type Page,
} from "@playwright/test";

/**
 * PRF-02/T17 database export/restore against PostgreSQL.
 *
 * Requires E2E_PG_HOST (and optional E2E_PG_PORT/USER/PASSWORD). Seeds its own
 * databases through the DataDeck API; skipped without PostgreSQL. Verifies the
 * export is bound to the explicit database (no alpha/beta crossover), the
 * restore only accepts a DataDeck artifact into an empty target, and a non-empty
 * target is refused by preflight.
 */
const PG_HOST = process.env.E2E_PG_HOST;
const PG_PORT = Number(process.env.E2E_PG_PORT ?? "5432");
const PG_USER = process.env.E2E_PG_USER ?? "postgres";
const PG_PASSWORD = process.env.E2E_PG_PASSWORD ?? "";
const DB_PREFIX = "datadeck_e2e_ds";
const DB_ALPHA = `${DB_PREFIX}_alpha`;
const DB_BETA = `${DB_PREFIX}_beta`;

async function createProfile(
  request: APIRequestContext,
  data: Record<string, unknown>,
): Promise<string> {
  const response = await request.post("/api/v1/connections", { data });
  expect(response.ok()).toBeTruthy();
  return (await response.json()).data.id as string;
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

async function openDatabaseMenu(page: Page, database: string): Promise<Locator> {
  await page
    .getByRole("list", { name: "Databases" })
    .getByRole("button", { name: `More actions for ${database}` })
    .click();
  const menu = page.getByRole("menu", { name: `More actions for ${database}` });
  await expect(menu).toBeVisible();
  return menu;
}

test.describe("database export/restore (PostgreSQL)", () => {
  test.skip(!PG_HOST, "E2E_PG_HOST not set; skipping PostgreSQL database-script E2E");

  test("export is database-scoped and restore only accepts a DataDeck artifact into an empty target", async ({
    page,
    request,
  }) => {
    const admin = await createProfile(request, {
      name: "E2E DS Admin",
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
        name: `E2E DS ${database}`,
        driver: "postgres",
        host: PG_HOST,
        port: PG_PORT,
        database_name: database,
        username: PG_USER,
        password: PG_PASSWORD,
        ssl_mode: "disable",
      });
      await exec(request, connection, "CREATE TABLE public.users (id int PRIMARY KEY, marker text)");
      await exec(request, connection, `INSERT INTO public.users VALUES (1, '${marker}')`);
      return connection;
    };
    const alpha = await seed(DB_ALPHA, "ALPHA");
    const beta = await seed(DB_BETA, "BETA");

    const server = await createProfile(request, {
      name: "E2E DS Server",
      driver: "postgres",
      host: PG_HOST,
      port: PG_PORT,
      database_name: "",
      username: PG_USER,
      password: PG_PASSWORD,
      ssl_mode: "disable",
    });

    // ――― Export alpha via the API and prove isolation ―――
    const exportResponse = await request.post(
      `/api/v1/connections/${server}/database-export`,
      { data: { database: DB_ALPHA, mode: "schema_data" } },
    );
    expect(exportResponse.ok()).toBeTruthy();
    const artifact = await exportResponse.text();
    expect(artifact).toContain("-- datadeck:export");
    expect(artifact).toContain("-- database: " + DB_ALPHA);
    expect(artifact).toContain("'ALPHA'");
    expect(artifact).not.toContain("BETA");

    // Restoring beta's own artifact into non-empty beta is refused (preflight).
    const betaExport = await request.post(
      `/api/v1/connections/${server}/database-export`,
      { data: { database: DB_BETA, mode: "schema_data" } },
    );
    const betaArtifact = await betaExport.text();
    const conflict = await request.post(`/api/v1/connections/${beta}/database-import`, {
      multipart: {
        database: DB_BETA,
        file: { name: "beta.sql", mimeType: "text/plain", buffer: Buffer.from(betaArtifact) },
      },
    });
    expect(conflict.status()).toBe(409);

    // ――― Empty alpha, then restore the artifact through the UI ―――
    await exec(request, admin, `DROP DATABASE ${DB_ALPHA} WITH (FORCE)`);
    await exec(request, admin, `CREATE DATABASE ${DB_ALPHA}`);

    await page.goto("/");
    await page.getByRole("button", { name: /^E2E DS Server/ }).first().click();
    await expect(databaseButton(page, DB_BETA)).toBeVisible();
    await page.reload();
    await page.getByRole("button", { name: /^E2E DS Server/ }).first().click();

    await openDatabaseMenu(page, DB_ALPHA);
    await page.getByRole("menuitem", { name: "Import SQL (Restore)" }).click();
    const dialog = page.getByTestId("database-restore-dialog");
    await expect(dialog).toBeVisible();
    await dialog.getByTestId("database-restore-file").setInputFiles({
      name: "alpha.sql",
      mimeType: "text/plain",
      buffer: Buffer.from(artifact),
    });
    await dialog.getByTestId("database-restore-upload").click();
    await expect(dialog.getByTestId("database-restore-preview")).toBeVisible();
    await dialog.getByTestId("database-restore-execute").click();
    await expect(dialog.getByTestId("database-restore-done")).toBeVisible();
    await dialog.getByTestId("database-restore-dismiss").click();

    // Alpha restored; beta untouched.
    const restored = await request.post("/api/v1/query/execute", {
      data: { connection_id: alpha, sql: "SELECT marker FROM public.users" },
    });
    const rows = (await restored.json()).data.rows as string[][];
    expect(rows).toEqual([["ALPHA"]]);
    const betaRows = await request.post("/api/v1/query/execute", {
      data: { connection_id: beta, sql: "SELECT marker FROM public.users" },
    });
    expect((await betaRows.json()).data.rows).toEqual([["BETA"]]);

    // Cleanup (best-effort).
    await exec(request, admin, `DROP DATABASE IF EXISTS ${DB_ALPHA} WITH (FORCE)`);
    await exec(request, admin, `DROP DATABASE IF EXISTS ${DB_BETA} WITH (FORCE)`);
  });
});
