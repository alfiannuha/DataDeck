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
