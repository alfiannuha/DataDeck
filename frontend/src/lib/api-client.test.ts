import { afterEach, describe, expect, it, vi } from "vitest";

import {
  ApiClientError,
  apiFetch,
  apiFetchDownload,
  apiUrl,
  parseContentDispositionFilename,
} from "./api-client";

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

/** A binary-capable response mock: headers map + blob(). */
function downloadResponse({
  status = 200,
  ok = true,
  contentType,
  contentDisposition,
  json,
  blob,
}: {
  status?: number;
  ok?: boolean;
  contentType?: string;
  contentDisposition?: string;
  json?: () => Promise<unknown>;
  blob?: () => Promise<Blob>;
}): Response {
  const headers = new Map<string, string>();
  if (contentType) headers.set("Content-Type", contentType);
  if (contentDisposition) headers.set("Content-Disposition", contentDisposition);
  return {
    status,
    ok,
    headers: { get: (name: string) => headers.get(name) ?? null },
    json: json ?? (async () => {
      throw new Error("not json");
    }),
    blob: blob ?? (async () => new Blob(["data"])),
  } as unknown as Response;
}

describe("parseContentDispositionFilename", () => {
  it("extracts a quoted filename", () => {
    expect(
      parseContentDispositionFilename(
        'attachment; filename="datadeck_app_public_users_20260101.csv"',
      ),
    ).toBe("datadeck_app_public_users_20260101.csv");
  });

  it("returns null when missing or without a filename", () => {
    expect(parseContentDispositionFilename(null)).toBeNull();
    expect(parseContentDispositionFilename("attachment")).toBeNull();
  });
});

describe("apiFetchDownload", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns the blob and server-suggested filename on success", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        downloadResponse({
          contentType: "text/csv",
          contentDisposition: 'attachment; filename="users.csv"',
          blob: async () => new Blob(["a,b\n1,2\n"], { type: "text/csv" }),
        }),
      ),
    );

    const result = await apiFetchDownload("/connections/c1/table-data/export");
    expect(result.filename).toBe("users.csv");
    expect(result.blob).toBeInstanceOf(Blob);
    expect(result.blob.size).toBeGreaterThan(0);
  });

  it("normalizes a JSON error envelope into ApiClientError", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        downloadResponse({
          status: 400,
          ok: false,
          contentType: "application/json",
          json: async () => ({
            success: false,
            data: null,
            error: {
              code: "EXPORT_FORMAT_UNSUPPORTED",
              message: "sql export requires a base table",
            },
            meta: {},
          }),
        }),
      ),
    );

    const error = await captureError(
      apiFetchDownload("/connections/c1/table-data/export"),
    );
    expect(error).toBeInstanceOf(ApiClientError);
    expect(error.code).toBe("EXPORT_FORMAT_UNSUPPORTED");
    expect(error.status).toBe(400);
  });
});
