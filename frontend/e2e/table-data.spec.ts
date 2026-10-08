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

/** Sets a single structured filter on the active Table Data tab and applies it. */
async function applyFilter(
  page: Page,
  column: string,
  operator: string,
  value?: string,
): Promise<void> {
  const toggle = page.getByTestId("table-data-filter-toggle");
  if ((await toggle.getAttribute("aria-expanded")) !== "true") {
    await toggle.click();
  }
  await page.getByLabel("Filter 1 column").selectOption(column);
  await page.getByLabel("Filter 1 operator").selectOption(operator);
  if (value !== undefined) {
    await page.getByLabel("Filter 1 value").fill(value);
  }
  await page.getByRole("button", { name: "Apply" }).click();
}

/** Opens a grid row's ⋯ action menu and clicks one of its items. */
async function clickRowMenuAction(row: Locator, name: string | RegExp): Promise<void> {
  await row.getByRole("button", { name: /^Row actions/ }).click();
  await row.page().getByRole("menuitem", { name }).click();
}

/** Opens a grid row's ⋯ action menu and leaves it open for assertions. */
async function openRowMenu(row: Locator): Promise<void> {
  await row.getByRole("button", { name: /^Row actions/ }).click();
}

/** Reads a bounded table-data page through the API for verification. */
async function apiTable(
  request: APIRequestContext,
  connectionId: string,
  database: string,
  table: string,
  filters?: string,
): Promise<{ rows: unknown[][]; capabilities: Record<string, boolean> }> {
  const query = new URLSearchParams({ database, schema: "public", table, page: "1", page_size: "100" });
  if (filters) query.set("filters", filters);
  const response = await request.get(`/api/v1/connections/${connectionId}/table-data?${query.toString()}`);
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  return { rows: body.data.rows as unknown[][], capabilities: body.data.row_capabilities };
}

test.describe("table data browsing (PostgreSQL)", () => {
  test.skip(!PG_HOST, "E2E_PG_HOST not set; skipping PostgreSQL Table Data E2E");
  // §30 copy flows read the clipboard back through the page.
  test.use({ permissions: ["clipboard-read", "clipboard-write"] });

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
      await exec(request, connection, "CREATE TABLE public.logs (message text)");
      await exec(
        request,
        connection,
        "CREATE TABLE public.memberships (tenant_id text NOT NULL, user_id bigint NOT NULL, role text, PRIMARY KEY (tenant_id, user_id))",
      );
      await exec(
        request,
        connection,
        "INSERT INTO public.memberships VALUES ('tenant-a', 9223372036854775807, 'viewer'), ('tenant-b', 9223372036854775807, 'viewer')",
      );
      // Unique-only table and a view: capability-matrix targets (T11 §28).
      await exec(
        request,
        connection,
        "CREATE TABLE public.uniq (email text UNIQUE NOT NULL, name text)",
      );
      await exec(
        request,
        connection,
        "CREATE VIEW public.v_users AS SELECT id, marker FROM public.users",
      );
      // Same table name with different values for cross-database sorting.
      const names: Record<string, string[]> = {
        ALPHA: ["ZETA", "OMEGA", "THETA"],
        BETA: ["ALPHA", "BETA", "GAMMA"],
      };
      await exec(
        request,
        connection,
        "CREATE TABLE public.namers (id int PRIMARY KEY, name text)",
      );
      for (const [index, name] of names[marker].entries()) {
        await exec(
          request,
          connection,
          `INSERT INTO public.namers VALUES (${index + 1}, '${name}')`,
        );
      }
      // >pageSize rows in deliberately mixed order to prove global sorting.
      await exec(
        request,
        connection,
        "CREATE TABLE public.scores AS SELECT g AS id, (g * 37) % 250 AS score FROM generate_series(1, 250) g",
      );
      await exec(request, connection, "ALTER TABLE public.scores ADD PRIMARY KEY (id)");
    };
    await seed(DB_ALPHA, "ALPHA");
    await seed(DB_BETA, "BETA");

    const serverId = await createProfile(request, {
      name: "E2E TD Server",
      driver: "postgres",
      host: PG_HOST,
      port: PG_PORT,
      database_name: "",
      username: PG_USER,
      password: PG_PASSWORD,
      ssl_mode: "disable",
    });
    // Direct per-database profile for external verification mutations.
    const alphaDirect = await createProfile(request, {
      name: "E2E TD direct alpha",
      driver: "postgres",
      host: PG_HOST,
      port: PG_PORT,
      database_name: DB_ALPHA,
      username: PG_USER,
      password: PG_PASSWORD,
      ssl_mode: "disable",
    });
    const betaDirect = await createProfile(request, {
      name: "E2E TD direct beta",
      driver: "postgres",
      host: PG_HOST,
      port: PG_PORT,
      database_name: DB_BETA,
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

    // Server-side sorting: alpha.namers ASC → OMEGA, THETA, ZETA.
    await openTable(page, DB_ALPHA, "namers");
    await page.getByRole("button", { name: "Sort by name" }).click();
    await expect(
      page.getByRole("gridcell").filter({ hasText: "OMEGA" }).first(),
    ).toBeVisible();

    // Explorer change + Refresh must not retarget the sorted alpha tab.
    await databaseButton(page, DB_BETA).click();
    await page.getByRole("button", { name: "Refresh table data" }).click();
    await expect(
      page.getByRole("gridcell").filter({ hasText: "OMEGA" }).first(),
    ).toBeVisible();
    await expect(page.getByRole("gridcell").filter({ hasText: "ALPHA" })).toHaveCount(0);

    // Beta sorts its own data independently (ALPHA, BETA, GAMMA).
    await openTable(page, DB_BETA, "namers");
    await page.getByRole("button", { name: "Sort by name" }).click();
    await expect(
      page.getByRole("gridcell").filter({ hasText: "GAMMA" }).first(),
    ).toBeVisible();
    await expect(page.getByRole("gridcell").filter({ hasText: "OMEGA" })).toHaveCount(0);

    // Global sorting before pagination: page 1 has the smallest scores; page 2
    // continues the globally sorted dataset (a current-page sort would fail).
    await openTable(page, DB_ALPHA, "scores");
    await page.getByRole("button", { name: "Sort by score" }).click();
    await expect(page.getByRole("gridcell").nth(2)).toHaveText("0");
    const next = page.getByRole("button", { name: "Next page" });
    await expect(next).toBeEnabled();
    await next.click();
    await expect(page.getByTestId("table-data-page")).toContainText("Page 2");
    await expect(page.getByRole("gridcell").nth(2)).toHaveText("100");

    // Server-side filtering: alpha.users marker = ALPHA (never beta). Switch to
    // the alpha users tab and filter it.
    await page
      .getByRole("region", { name: "Query tabs" })
      .getByRole("button", { name: "users", exact: true })
      .click();
    await applyFilter(page, "marker", "equals", "ALPHA");
    await expect(
      page.getByRole("gridcell").filter({ hasText: "ALPHA" }).first(),
    ).toBeVisible();
    await expect(page.getByRole("gridcell").filter({ hasText: "BETA" })).toHaveCount(0);

    // Explorer change + Refresh must not retarget the filtered alpha tab.
    await databaseButton(page, DB_BETA).click();
    await page.getByRole("button", { name: "Refresh table data" }).click();
    await expect(page.getByRole("gridcell").filter({ hasText: "BETA" })).toHaveCount(0);
    await expect(
      page.getByRole("gridcell").filter({ hasText: "ALPHA" }).first(),
    ).toBeVisible();

    // beta.users filters independently (marker = BETA).
    await openTable(page, DB_BETA, "users");
    await applyFilter(page, "marker", "equals", "BETA");
    await expect(
      page.getByRole("gridcell").filter({ hasText: "BETA" }).first(),
    ).toBeVisible();
    await expect(page.getByRole("gridcell").filter({ hasText: "ALPHA" })).toHaveCount(0);

    // Global filtering before pagination: score >= 150 with page_size 50 →
    // page 1 is 150..199 and page 2 continues at 200 (a page-local filter fails).
    await page
      .getByRole("region", { name: "Query tabs" })
      .getByRole("button", { name: "scores", exact: true })
      .click();
    await page.getByLabel("Page size").selectOption("50");
    await applyFilter(page, "score", "greater_or_equal", "150");
    await expect(page.getByRole("gridcell").nth(2)).toHaveText("150");
    const filteredNext = page.getByRole("button", { name: "Next page" });
    await expect(filteredNext).toBeEnabled();
    await filteredNext.click();
    await expect(page.getByTestId("table-data-page")).toContainText("Page 2");
    await expect(page.getByRole("gridcell").nth(2)).toHaveText("200");

    // Cross-database insert (BLOCKER gate): the dialog snapshots its target
    // when it opens (§21). Switching the explorer AND the active tab while the
    // dialog is open must not retarget the insert (T11 §27). dispatchEvent:
    // the modal overlay blocks real clicks.
    await page
      .getByRole("region", { name: "Query tabs" })
      .getByRole("button", { name: "users", exact: true })
      .click();
    await databaseButton(page, DB_BETA).click();
    await page.getByTestId("table-data-add-row").click();
    await page.getByLabel("id value").fill("100");
    await page.getByLabel("marker mode").selectOption("value");
    await page.getByLabel("marker value").fill("INSERTED_ALPHA");
    // Radix modal dialogs put aria-hidden on the rest of the app, so role
    // queries cannot see these controls; CSS locators bypass the a11y filter.
    const alphaNode = page
      .locator("button")
      .filter({ hasText: new RegExp(`^${DB_ALPHA}( database)?$`) })
      .first();
    await alphaNode.dispatchEvent("click");
    const scoresTab = page
      .locator('ul[aria-label="Open query tabs"] button')
      .filter({ hasText: /^scores$/ })
      .first();
    await scoresTab.dispatchEvent("click");
    await expect(page.getByTestId("add-row-dialog")).toBeVisible();
    await expect(page.getByTestId("add-row-dialog")).toContainText("users");
    await page.getByRole("button", { name: "Insert" }).click();
    await expect(page.getByTestId("add-row-dialog")).toHaveCount(0);

    const insertedAlpha = await apiTable(
      request, serverId, DB_ALPHA, "users",
      `[{"column":"marker","operator":"equals","value":"INSERTED_ALPHA"}]`,
    );
    expect(insertedAlpha.rows.length).toBe(1);
    const insertedBeta = await apiTable(
      request, serverId, DB_BETA, "users",
      `[{"column":"marker","operator":"equals","value":"INSERTED_ALPHA"}]`,
    );
    expect(insertedBeta.rows.length).toBe(0);

    // The §27 tab switch left the scores tab active; return to the users tab.
    await page
      .getByRole("region", { name: "Query tabs" })
      .getByRole("button", { name: "users", exact: true })
      .click();

    // Explicit NULL mode.
    await page.getByTestId("table-data-add-row").click();
    await page.getByLabel("id value").fill("101");
    await page.getByLabel("marker mode").selectOption("value");
    await page.getByLabel("marker value").fill("INSERTED_NULL");
    await page.getByLabel("note mode").selectOption("null");
    await page.getByRole("button", { name: "Insert" }).click();
    await expect(page.getByTestId("add-row-dialog")).toHaveCount(0);
    const nullRow = await apiTable(
      request, serverId, DB_ALPHA, "users",
      `[{"column":"marker","operator":"equals","value":"INSERTED_NULL"}]`,
    );
    expect(nullRow.rows.length).toBe(1);
    expect(nullRow.rows[0][2]).toBeNull();

    // No-PK base table: Add Row enabled, Update/Delete unavailable via menu.
    await openTable(page, DB_ALPHA, "logs");
    const logsCaps = await apiTable(request, serverId, DB_ALPHA, "logs");
    expect(logsCaps.capabilities.insert).toBe(true);
    expect(logsCaps.capabilities.update).toBe(false);
    expect(logsCaps.capabilities.delete).toBe(false);
    await page.getByTestId("table-data-add-row").click();
    await page.getByLabel("message mode").selectOption("value");
    await page.getByLabel("message value").fill("e2e-log");
    await page.getByRole("button", { name: "Insert" }).click();
    await expect(page.getByTestId("add-row-dialog")).toHaveCount(0);
    const logsRows = await apiTable(
      request, serverId, DB_ALPHA, "logs",
      `[{"column":"message","operator":"equals","value":"e2e-log"}]`,
    );
    expect(logsRows.rows.length).toBe(1);

    // §28 matrix — logs (no PK): Edit/Delete disabled with explanations,
    // Duplicate stays available (it is a plain insert), Copy always works.
    const logRow = page.getByRole("row").filter({ hasText: "e2e-log" }).first();
    await openRowMenu(logRow);
    await expect(page.getByTestId("row-menu-edit")).toHaveAttribute("aria-disabled", "true");
    await expect(page.getByTestId("row-menu-edit")).toContainText(
      "No primary key — row editing is unavailable.",
    );
    await expect(page.getByTestId("row-menu-delete")).toHaveAttribute("aria-disabled", "true");
    await expect(page.getByTestId("row-menu-delete")).toContainText(
      "No primary key — row deletion is unavailable.",
    );
    await expect(page.getByTestId("row-menu-duplicate")).not.toHaveAttribute("aria-disabled");
    await expect(page.getByTestId("row-menu-copy")).toBeVisible();
    await page.keyboard.press("Escape");

    // Unique-only table (no PK): same disabled explanations, Add Row enabled.
    await exec(request, alphaDirect, "INSERT INTO public.uniq VALUES ('u@x', 'U')");
    await openTable(page, DB_ALPHA, "uniq");
    const uniqRow = page.getByRole("row").filter({ hasText: "u@x" }).first();
    await openRowMenu(uniqRow);
    await expect(page.getByTestId("row-menu-edit")).toHaveAttribute("aria-disabled", "true");
    await expect(page.getByTestId("row-menu-edit")).toContainText("No primary key");
    await expect(page.getByTestId("row-menu-delete")).toHaveAttribute("aria-disabled", "true");
    await page.keyboard.press("Escape");
    await expect(page.getByTestId("table-data-add-row")).toBeEnabled();

    // View: every row-mutation action is disabled with the read-only reason.
    await openTable(page, DB_ALPHA, "v_users");
    const viewRow = page.getByRole("row").filter({ hasText: "ALPHA" }).first();
    await openRowMenu(viewRow);
    for (const menuId of ["row-menu-edit", "row-menu-duplicate", "row-menu-delete"]) {
      await expect(page.getByTestId(menuId)).toHaveAttribute("aria-disabled", "true");
      await expect(page.getByTestId(menuId)).toContainText("This object is read-only.");
    }
    await expect(page.getByTestId("row-menu-copy")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.getByTestId("table-data-add-row")).toBeDisabled();
    await expect(page.getByTestId("table-data-add-row")).toHaveAttribute(
      "title",
      "This object is read-only.",
    );

    // Wrong-database update (BLOCKER gate): explorer on beta, edit a row from
    // the alpha tab, and only alpha may change.
    await openTable(page, DB_ALPHA, "users");
    await databaseButton(page, DB_BETA).click();
    await exec(request, alphaDirect, "UPDATE public.users SET note = 'ONE' WHERE id = 1");
    await page.getByRole("button", { name: "Refresh table data" }).click();
    const firstAlphaRow = page.getByRole("row").filter({ hasText: "ONE" }).first();
    await clickRowMenuAction(firstAlphaRow, "Edit Row");
    await page.getByLabel("note value").fill("ALPHA_EDIT");
    await page.getByRole("button", { name: "Save" }).click();
    await expect(page.getByTestId("edit-row-dialog")).toHaveCount(0);

    const alphaEdited = await apiTable(
      request, serverId, DB_ALPHA, "users",
      `[{"column":"note","operator":"equals","value":"ALPHA_EDIT"}]`,
    );
    expect(alphaEdited.rows.length).toBe(1);
    const betaEdited = await apiTable(
      request, serverId, DB_BETA, "users",
      `[{"column":"note","operator":"equals","value":"ALPHA_EDIT"}]`,
    );
    expect(betaEdited.rows.length).toBe(0);

    // Optimistic concurrency: load the row, change it externally, then save.
    await page.getByRole("button", { name: "Refresh table data" }).click();
    const staleRow = page.getByRole("row").filter({ hasText: "x" }).first();
    await clickRowMenuAction(staleRow, "Edit Row");
    await exec(request, alphaDirect, "UPDATE public.users SET note = 'EXT' WHERE id = 2 AND note = 'x'");
    await page.getByLabel("note value").fill("MINE");
    await page.getByRole("button", { name: "Save" }).click();
    await expect(page.getByRole("alert")).toContainText("ROW_CONFLICT");
    // §21: Reload refetches the row and resets the draft to the fresh values.
    await page.getByTestId("edit-row-reload").click();
    await expect(page.getByRole("alert")).toHaveCount(0);
    await expect(page.getByLabel("note value")).toHaveValue("EXT");
    await expect(page.getByLabel("marker value")).toHaveValue("ALPHA");
    await expect(page.getByRole("button", { name: "Save" })).toBeDisabled();
    await page.getByRole("button", { name: "Cancel" }).click();

    // Row deleted during edit → ROW_NOT_FOUND.
    await exec(request, alphaDirect, "INSERT INTO public.users VALUES (50, 'ALPHA', 'ZED')");
    await page.getByRole("button", { name: "Refresh table data" }).click();
    const doomedRow = page.getByRole("row").filter({ hasText: "ZED" }).first();
    await clickRowMenuAction(doomedRow, "Edit Row");
    await exec(request, alphaDirect, "DELETE FROM public.users WHERE id = 50");
    await page.getByLabel("note value").fill("GONE");
    await page.getByRole("button", { name: "Save" }).click();
    await expect(page.getByRole("alert")).toContainText("ROW_NOT_FOUND");
    await page.getByRole("button", { name: "Cancel" }).click();

    // Composite PK update changes exactly one row.
    await openTable(page, DB_ALPHA, "memberships");
    const tenantARow = page.getByRole("row").filter({ hasText: "tenant-a" }).first();
    await clickRowMenuAction(tenantARow, "Edit Row");
    await page.getByLabel("role value").fill("admin");
    await page.getByRole("button", { name: "Save" }).click();
    await expect(page.getByTestId("edit-row-dialog")).toHaveCount(0);
    const memberships = await apiTable(request, serverId, DB_ALPHA, "memberships");
    const roles = Object.fromEntries(memberships.rows.map((r) => [r[0], r[2]]));
    expect(roles["tenant-a"]).toBe("admin");
    expect(roles["tenant-b"]).toBe("viewer");

    // Duplicate: PK is never copied; value columns are prefilled from the row.
    await openTable(page, DB_ALPHA, "users");
    await databaseButton(page, DB_BETA).click();
    await applyFilter(page, "id", "equals", "1");
    const duplicateSource = page.getByRole("row").filter({ hasText: "ALPHA_EDIT" }).first();
    await clickRowMenuAction(duplicateSource, "Duplicate Row");
    const duplicateDialog = page.getByTestId("add-row-dialog");
    await expect(duplicateDialog).toContainText("Duplicate Row — users");
    await expect(page.getByLabel("marker value")).toHaveValue("ALPHA");
    await page.getByLabel("id value").fill("200");
    await page.getByRole("button", { name: "Insert" }).click();
    await expect(page.getByTestId("add-row-dialog")).toHaveCount(0);
    const duplicated = await apiTable(
      request, serverId, DB_ALPHA, "users",
      `[{"column":"id","operator":"equals","value":"200"}]`,
    );
    expect(duplicated.rows.length).toBe(1);
    expect(duplicated.rows[0][1]).toBe("ALPHA");
    // Duplicate targets only the tab's database.
    const duplicatedBeta = await apiTable(
      request, serverId, DB_BETA, "users",
      `[{"column":"id","operator":"equals","value":"200"}]`,
    );
    expect(duplicatedBeta.rows.length).toBe(0);

    // §30: Copy / Copy as JSON put the row on the clipboard and never mutate.
    // The filter (id = 1) still shows exactly the source row.
    const copiedRow = page.getByRole("row").filter({ hasText: "ALPHA_EDIT" }).first();
    await openRowMenu(copiedRow);
    await page.getByRole("menuitem", { name: "Copy", exact: true }).click();
    const tsv = await page.evaluate(() => navigator.clipboard.readText());
    expect(tsv.split("\t")[0]).toBe("1");
    expect(tsv).toContain("ALPHA_EDIT");
    await openRowMenu(copiedRow);
    await page.getByRole("menuitem", { name: "Copy as JSON" }).click();
    const copiedJson = JSON.parse(
      await page.evaluate(() => navigator.clipboard.readText()),
    ) as Record<string, unknown>;
    expect(Object.keys(copiedJson)).toEqual(["id", "marker", "note"]);
    expect(copiedJson.marker).toBe("ALPHA");
    expect(copiedJson.note).toBe("ALPHA_EDIT");
    // Right-click opens the same row menu (T11 §2 / ADR-010 menu family).
    await copiedRow.click({ button: "right" });
    await expect(page.getByTestId("row-menu-edit")).toBeVisible();
    await page.keyboard.press("Escape");
    // Zero mutations: no dialog opened and the row is still there.
    await expect(page.getByTestId("add-row-dialog")).toHaveCount(0);
    await expect(page.getByTestId("edit-row-dialog")).toHaveCount(0);
    await expect(page.getByTestId("delete-row-dialog")).toHaveCount(0);
    await expect(
      page.getByRole("row").filter({ hasText: "ALPHA_EDIT" }).first(),
    ).toBeVisible();

    // Same id in both databases: deleting it from the alpha tab must only
    // delete alpha's row (cross-database delete BLOCKER gate).
    await exec(request, alphaDirect, "INSERT INTO public.users VALUES (77, 'DELETE_ME', NULL)");
    await exec(request, betaDirect, "INSERT INTO public.users VALUES (77, 'DELETE_ME', NULL)");
    await applyFilter(page, "id", "equals", "77");
    const deleteTarget = page.getByRole("row").filter({ hasText: "DELETE_ME" }).first();
    await clickRowMenuAction(deleteTarget, "Delete Row");
    const deleteDialog = page.getByTestId("delete-row-dialog");
    await expect(deleteDialog).toContainText("id = 77");
    await expect(deleteDialog).toContainText(`${DB_ALPHA} / public.users`);
    // Cancel leaves the row untouched.
    await page.getByRole("button", { name: "Cancel" }).click();
    await expect(page.getByTestId("delete-row-dialog")).toHaveCount(0);
    expect((await apiTable(request, serverId, DB_ALPHA, "users", `[{"column":"id","operator":"equals","value":"77"}]`)).rows.length).toBe(1);

    await clickRowMenuAction(deleteTarget, "Delete Row");
    await page.getByRole("button", { name: "Delete", exact: true }).click();
    await expect(page.getByTestId("delete-row-dialog")).toHaveCount(0);
    const alphaAfterDelete = await apiTable(
      request, serverId, DB_ALPHA, "users",
      `[{"column":"id","operator":"equals","value":"77"}]`,
    );
    expect(alphaAfterDelete.rows.length).toBe(0);
    const betaAfterDelete = await apiTable(
      request, serverId, DB_BETA, "users",
      `[{"column":"id","operator":"equals","value":"77"}]`,
    );
    expect(betaAfterDelete.rows.length).toBe(1);

    // Optimistic concurrency on delete: the row changes after it was loaded.
    await applyFilter(page, "id", "equals", "1");
    const conflictDeleteRow = page.getByRole("row").filter({ hasText: "ALPHA_EDIT" }).first();
    await clickRowMenuAction(conflictDeleteRow, "Delete Row");
    await exec(request, alphaDirect, "UPDATE public.users SET marker = 'CHANGED' WHERE id = 1");
    await page.getByRole("button", { name: "Delete", exact: true }).click();
    await expect(page.getByRole("alert")).toContainText("ROW_CONFLICT");
    await page.getByRole("button", { name: "Cancel" }).click();
    expect((await apiTable(request, serverId, DB_ALPHA, "users", `[{"column":"id","operator":"equals","value":"1"}]`)).rows.length).toBe(1);

    // Row deleted externally → ROW_NOT_FOUND, dialog stays open.
    await exec(request, alphaDirect, "INSERT INTO public.users VALUES (88, 'DOOMED', NULL)");
    await applyFilter(page, "id", "equals", "88");
    const notFoundDeleteRow = page.getByRole("row").filter({ hasText: "DOOMED" }).first();
    await clickRowMenuAction(notFoundDeleteRow, "Delete Row");
    await exec(request, alphaDirect, "DELETE FROM public.users WHERE id = 88");
    await page.getByRole("button", { name: "Delete", exact: true }).click();
    await expect(page.getByRole("alert")).toContainText("ROW_NOT_FOUND");
    await expect(page.getByTestId("delete-row-dialog")).toBeVisible();
    await page.getByRole("button", { name: "Cancel" }).click();

    // Cleanup (best-effort).
    await exec(request, admin, `DROP DATABASE IF EXISTS ${DB_ALPHA} WITH (FORCE)`);
    await exec(request, admin, `DROP DATABASE IF EXISTS ${DB_BETA} WITH (FORCE)`);
  });
});
