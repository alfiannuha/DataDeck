import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiClientError, apiFetch, apiUrl } from "./api-client";

function jsonResponse(status: number, body: unknown): Response {
  return {
    status,
    json: async () => body,
  } as unknown as Response;
}

type CapturedError = Error & {
  code?: string;
  status?: number;
  position?: number;
};

async function captureError(promise: Promise<unknown>): Promise<CapturedError> {
  try {
    await promise;
  } catch (error) {
    return error as CapturedError;
  }
  throw new Error("expected the promise to reject");
}

describe("apiUrl", () => {
  it("prefixes every path with /api/v1", () => {
    expect(apiUrl("/health")).toMatch(/\/api\/v1\/health$/);
  });
});

describe("apiFetch", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns data on a success envelope", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse(200, {
          success: true,
          data: { status: "healthy" },
          error: null,
          meta: {},
        }),
      ),
    );

    await expect(apiFetch("/health")).resolves.toEqual({ status: "healthy" });
  });

  it("normalizes an error envelope into ApiClientError", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse(400, {
          success: false,
          data: null,
          error: {
            code: "SQL_SYNTAX_ERROR",
            message: 'syntax error at or near "WHERRE"',
            position: 62,
          },
          meta: {},
        }),
      ),
    );

    const error = await captureError(apiFetch("/query/execute"));
    expect(error).toBeInstanceOf(ApiClientError);
    expect(error.code).toBe("SQL_SYNTAX_ERROR");
    expect(error.status).toBe(400);
    expect(error.position).toBe(62);
    expect(error.message).toContain("syntax error");
  });

  it("rejects a malformed (non-JSON) response", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        status: 502,
        json: async () => {
          throw new Error("not json");
        },
      } as unknown as Response),
    );

    const error = await captureError(apiFetch("/health"));
    expect(error).toBeInstanceOf(ApiClientError);
    expect(error.code).toBe("INTERNAL_ERROR");
    expect(error.status).toBe(502);
  });

  it("rejects a JSON body that is not an envelope", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(200, 42)));

    const error = await captureError(apiFetch("/health"));
    expect(error).toBeInstanceOf(ApiClientError);
    expect(error.code).toBe("INTERNAL_ERROR");
  });

  it("normalizes network failures into CONNECTION_ERROR", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("failed")));

    const error = await captureError(apiFetch("/health"));
    expect(error).toBeInstanceOf(ApiClientError);
    expect(error.code).toBe("CONNECTION_ERROR");
    expect(error.status).toBe(0);
  });

  it("propagates abort errors unchanged for cancellation", async () => {
    const abortError = new Error("aborted");
    abortError.name = "AbortError";
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(abortError));

    const error = await captureError(apiFetch("/health"));
    expect(error).toBe(abortError);
    expect(error).not.toBeInstanceOf(ApiClientError);
    expect(error.name).toBe("AbortError");
  });
});
