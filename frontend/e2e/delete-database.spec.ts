import {
  expect,
  test,
  type APIRequestContext,
  type Locator,
  type Page,
} from "@playwright/test";

/**
 * PRF-02/T18 destructive database management against PostgreSQL.
 *
 * Requires E2E_PG_HOST (and optional E2E_PG_PORT/USER/PASSWORD). Uses disposable
 * databases only; skipped without PostgreSQL. Verifies the delete is bound to
 * the captured database (beta is never touched), protected system databases are
 * refused, and a confirmation token cannot be used for another target.
 */
const PG_HOST = process.env.E2E_PG_HOST;
const PG_PORT = Number(process.env.E2E_PG_PORT ?? "5432");
const PG_USER = process.env.E2E_PG_USER ?? "postgres";
const PG_PASSWORD = process.env.E2E_PG_PASSWORD ?? "";
// Run-unique names: the delete test recreates databases and must never collide
// with a pool left over from a previous run against a long-lived server.
const RUN = Math.random().toString(36).slice(2, 8);
const DB_PREFIX = `datadeck_e2e_del_${RUN}`;
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
  database?: string,
): Promise<void> {
  const response = await request.post("/api/v1/query/execute", {
    data: database ? { connection_id: connectionId, sql, database } : { connection_id: connectionId, sql },
  });
  expect(response.ok(), `SQL failed: ${sql}`).toBeTruthy();
}

async function query(
  request: APIRequestContext,
  connectionId: string,
  sql: string,
  database: string,
): Promise<unknown> {
  const response = await request.post("/api/v1/query/execute", {
    data: { connection_id: connectionId, sql, database },
  });
  expect(response.ok()).toBeTruthy();
  return (await response.json()).data.rows;
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

test.describe("delete database (PostgreSQL)", () => {
  test.skip(!PG_HOST, "E2E_PG_HOST not set; skipping PostgreSQL delete-database E2E");

  test("deletes only the captured database and rejects protected/token-substitution", async ({
    page,
    request,
  }) => {
    const admin = await createProfile(request, {
      name: "E2E DEL Admin",
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

    // A single server-level profile drives every per-database operation, so the
    // only pool for each database is (server, database) — the one the delete
    // handler retires. (The production flow never keeps extra profiles open.)
    const server = await createProfile(request, {
      name: "E2E DEL Server",
      driver: "postgres",
      host: PG_HOST,
      port: PG_PORT,
      database_name: "",
      username: PG_USER,
      password: PG_PASSWORD,
      ssl_mode: "disable",
    });
    for (const [database, marker] of [
      [DB_ALPHA, "ALPHA"],
      [DB_BETA, "BETA"],
    ] as const) {
      await exec(request, server, "CREATE TABLE public.t (id int PRIMARY KEY, marker text)", database);
      await exec(request, server, `INSERT INTO public.t VALUES (1, '${marker}')`, database);
    }

    // Protected system database is refused by preflight.
    const protectedResp = await request.post(
      `/api/v1/connections/${server}/databases/postgres/delete-preflight`,
    );
    expect(protectedResp.status()).toBe(400);

    // A token for alpha cannot be used to delete beta.
    const preflight = await request.post(
      `/api/v1/connections/${server}/databases/${DB_ALPHA}/delete-preflight`,
    );
    expect(preflight.ok()).toBeTruthy();
    const token = (await preflight.json()).data.token as string;
    const substitution = await request.post(
      `/api/v1/connections/${server}/databases/${DB_BETA}/delete`,
      {
        data: {
          confirmation_token: token,
          typed_database_name: DB_BETA,
          acknowledged: true,
        },
      },
    );
    expect(substitution.status()).toBe(400);
    // Beta is still present after the rejected attempt.
    expect(await query(request, server, "SELECT marker FROM public.t", DB_BETA)).toEqual([["BETA"]]);

    // ――― Delete alpha through the UI ―――
    await page.goto("/");
    await page.getByRole("button", { name: /^E2E DEL Server/ }).first().click();
    await expect(databaseButton(page, DB_ALPHA)).toBeVisible();

    await openDatabaseMenu(page, DB_ALPHA);
    await page.getByRole("menuitem", { name: "Delete Database" }).click();
    const dialog = page.getByTestId("delete-database-dialog");
    await expect(dialog).toBeVisible();
    await dialog.getByTestId("delete-database-typed").fill(DB_ALPHA);
    await dialog.getByTestId("delete-database-ack").check();
    await expect(dialog.getByTestId("delete-database-submit")).toBeEnabled();
    await dialog.getByTestId("delete-database-submit").click();
    await expect(dialog.getByTestId("delete-database-done")).toBeVisible();
    await dialog.getByTestId("delete-database-cancel").click();

    // Alpha is gone; beta remains with its data.
    await expect(databaseButton(page, DB_ALPHA)).toHaveCount(0);
    await expect(databaseButton(page, DB_BETA)).toBeVisible();
    expect(await query(request, server, "SELECT marker FROM public.t", DB_BETA)).toEqual([["BETA"]]);

    // Cleanup (best-effort).
    await exec(request, admin, `DROP DATABASE IF EXISTS ${DB_BETA} WITH (FORCE)`);
  });
});
