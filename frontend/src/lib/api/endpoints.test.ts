import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createConnection,
  deleteConnection,
  getQueryHistory,
  getSchemas,
} from "./endpoints";

function ok(body: unknown): Response {
  return {
    status: 200,
    json: async () => ({ success: true, data: body, error: null, meta: {} }),
  } as unknown as Response;
}

describe("endpoints", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("encodes connection ids in the path", async () => {
    const fetchMock = vi.fn().mockResolvedValue(ok({ id: "a/b" }));
    vi.stubGlobal("fetch", fetchMock);

    await deleteConnection("a/b");

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toMatch(/\/api\/v1\/connections\/a%2Fb$/);
    expect(init.method).toBe("DELETE");
  });

  it("adds the connection filter to the history query", async () => {
    const fetchMock = vi.fn().mockResolvedValue(ok([]));
    vi.stubGlobal("fetch", fetchMock);

    await getQueryHistory({ connectionId: "c1" });

    const [url] = fetchMock.mock.calls[0] as [string];
    expect(url).toMatch(/\/api\/v1\/query\/history\?connection_id=c1$/);
  });

  it("sends create bodies as JSON and never adds credential headers", async () => {
    const fetchMock = vi.fn().mockResolvedValue(ok({ id: "c1" }));
    vi.stubGlobal("fetch", fetchMock);

    await createConnection({
      name: "PG",
      driver: "postgres",
      host: "127.0.0.1",
      port: 5432,
      database_name: "app",
      username: "u",
      password: "s3cret",
      ssl_mode: "disable",
    });

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    const headers = init.headers as Record<string, string>;
    expect(headers["Content-Type"]).toBe("application/json");
    expect(headers).not.toHaveProperty("Authorization");
    expect(JSON.parse(init.body as string)).toMatchObject({ password: "s3cret" });
  });

  it("requests the schema tree for a connection", async () => {
    const fetchMock = vi.fn().mockResolvedValue(ok([]));
    vi.stubGlobal("fetch", fetchMock);

    await getSchemas("c1");

    const [url] = fetchMock.mock.calls[0] as [string];
    expect(url).toMatch(/\/api\/v1\/connections\/c1\/schemas$/);
  });
});
