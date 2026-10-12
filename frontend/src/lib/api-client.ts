import type { ApiEnvelope } from "@/types/api";

/**
 * Backend base URL.
 *
 * - Development: point at the Go daemon, e.g.
 *   NEXT_PUBLIC_API_URL=http://127.0.0.1:8080
 * - Single-binary / same-origin deployment: set NEXT_PUBLIC_API_URL="" so
 *   requests are relative and served by the embedded backend.
 */
export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://127.0.0.1:8080";

/** All backend routes live under this prefix. */
export const API_PREFIX = "/api/v1";

/** Build an absolute (or same-origin) URL for an API path. */
export function apiUrl(path: string): string {
  return `${API_BASE_URL}${API_PREFIX}${path}`;
}

/**
 * Normalized API failure derived from the standard error envelope. Components
 * and hooks branch on `code`, never on raw Response objects.
 */
export class ApiClientError extends Error {
  readonly code: string;
  readonly status: number;
  readonly position?: number;

  constructor(
    message: string,
    code: string,
    status: number,
    position?: number,
  ) {
    super(message);
    this.name = "ApiClientError";
    this.code = code;
    this.status = status;
    this.position = position;
  }
}

/** Pagination metadata carried in the envelope `meta` field. */
export interface PaginationMeta {
  page: number;
  page_size: number;
  total: number;
  total_pages: number;
}

/** A bounded list page. */
export interface Paginated<T> {
  items: T[];
  meta: PaginationMeta;
}

function isAbortError(error: unknown): boolean {
  return (
    typeof error === "object" &&
    error !== null &&
    "name" in error &&
    (error as { name?: unknown }).name === "AbortError"
  );
}

async function requestEnvelope<T>(
  path: string,
  init?: RequestInit,
): Promise<ApiEnvelope<T>> {
  let response: Response;
  try {
    response = await fetch(apiUrl(path), {
      ...init,
      headers: {
        "Content-Type": "application/json",
        ...init?.headers,
      },
    });
  } catch (cause) {
    if (isAbortError(cause)) {
      throw cause;
    }
    throw new ApiClientError(
      cause instanceof Error ? cause.message : "Network request failed",
      "CONNECTION_ERROR",
      0,
    );
  }

  // Even a malformed body must surface as a normalized error.
  const envelope = (await response
    .json()
    .catch(() => null)) as ApiEnvelope<T> | null;

  if (!envelope || typeof envelope.success !== "boolean") {
    throw new ApiClientError(
      "Malformed API response",
      "INTERNAL_ERROR",
      response.status,
    );
  }

  if (!envelope.success || envelope.error) {
    throw new ApiClientError(
      envelope.error?.message ?? "Request failed",
      envelope.error?.code ?? "INTERNAL_ERROR",
      response.status,
      envelope.error?.position,
    );
  }

  return envelope;
}

/**
 * Single entry point for backend HTTP calls. All requests go through here so
 * envelope handling, error normalization and headers stay in one place.
 *
 * Cancellation is supported through `init.signal`; abort errors propagate
 * unchanged so TanStack Query can discard cancelled work.
 */
export async function apiFetch<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  const envelope = await requestEnvelope<T>(path, init);
  return envelope.data;
}

/** Like apiFetch but also returns the envelope `meta` (e.g. pagination). */
export async function apiFetchMeta<T>(
  path: string,
  init?: RequestInit,
): Promise<Paginated<T>> {
  const envelope = await requestEnvelope<T[]>(path, init);
  return {
    items: envelope.data,
    meta: envelope.meta as unknown as PaginationMeta,
  };
}

/**
 * Multipart/form-data POST whose body is a FormData instance (file uploads).
 * The JSON Content-Type must not be set — the browser adds the multipart
 * boundary itself. Error normalization matches apiFetch: JSON envelope only.
 */
async function requestEnvelopeForm<T>(
  path: string,
  init?: RequestInit,
): Promise<ApiEnvelope<T>> {
  let response: Response;
  try {
    response = await fetch(apiUrl(path), {
      method: "POST",
      body: init?.body,
      signal: init?.signal,
      headers: init?.headers,
    });
  } catch (cause) {
    if (isAbortError(cause)) {
      throw cause;
    }
    throw new ApiClientError(
      cause instanceof Error ? cause.message : "Network request failed",
      "CONNECTION_ERROR",
      0,
    );
  }

  const envelope = (await response
    .json()
    .catch(() => null)) as ApiEnvelope<T> | null;

  if (!envelope || typeof envelope.success !== "boolean") {
    throw new ApiClientError(
      "Malformed API response",
      "INTERNAL_ERROR",
      response.status,
    );
  }

  if (!envelope.success || envelope.error) {
    throw new ApiClientError(
      envelope.error?.message ?? "Request failed",
      envelope.error?.code ?? "INTERNAL_ERROR",
      response.status,
      envelope.error?.position,
    );
  }

  return envelope;
}

/** Like apiFetch but for multipart/form-data bodies (file uploads). */
export async function apiFetchForm<T>(
  path: string,
  form: FormData,
  signal?: AbortSignal,
): Promise<T> {
  const envelope = await requestEnvelopeForm<T>(path, { body: form, signal });
  return envelope.data;
}

/** A binary attachment returned by a download endpoint. */
export interface DownloadResult {
  blob: Blob;
  /** Server-suggested filename from Content-Disposition, when present. */
  filename: string | null;
}

/**
 * Extract the filename from a Content-Disposition header, handling both the
 * quoted (`filename="a.csv"`) and, for simplicity, the first token form. Returns
 * null when the header is missing or has no filename.
 */
export function parseContentDispositionFilename(
  header: string | null,
): string | null {
  if (!header) return null;
  const match = /filename\*?=(?:UTF-8''|")?([^";]+)"?/i.exec(header);
  const name = match?.[1]?.trim();
  return name ? name : null;
}

/**
 * POST/GET binary download. A successful response (non-JSON) is returned as a
 * Blob with its suggested filename; a JSON error envelope (any status) is
 * normalized into an ApiClientError exactly like apiFetch. This keeps the
 * "errors are envelopes, success is a file" contract in one place.
 */
export async function apiFetchDownload(
  path: string,
  init?: RequestInit,
): Promise<DownloadResult> {
  let response: Response;
  try {
    response = await fetch(apiUrl(path), {
      ...init,
      headers: {
        "Content-Type": "application/json",
        ...init?.headers,
      },
    });
  } catch (cause) {
    if (isAbortError(cause)) {
      throw cause;
    }
    throw new ApiClientError(
      cause instanceof Error ? cause.message : "Network request failed",
      "CONNECTION_ERROR",
      0,
    );
  }

  const contentType = response.headers.get("Content-Type") ?? "";
  // Errors are always the JSON envelope; a file is never application/json.
  if (!response.ok || contentType.includes("application/json")) {
    const envelope = (await response
      .json()
      .catch(() => null)) as ApiEnvelope<unknown> | null;
    if (envelope?.error) {
      throw new ApiClientError(
        envelope.error.message,
        envelope.error.code,
        response.status,
        envelope.error.position,
      );
    }
    throw new ApiClientError(
      envelope ? "Request failed" : "Malformed API response",
      "INTERNAL_ERROR",
      response.status,
    );
  }

  const blob = await response.blob();
  return {
    blob,
    filename: parseContentDispositionFilename(
      response.headers.get("Content-Disposition"),
    ),
  };
}
