/**
 * Centralized TanStack Query keys. Keeping them here prevents accidental key
 * drift between queries and mutations.
 */
export const queryKeys = {
  health: ["health"] as const,
  connections: ["connections"] as const,
  /** Schema tree for a specific connection. */
  schema: (connectionId: string) =>
    ["connections", connectionId, "schema"] as const,
  /** Prefix for all history queries (used for invalidation). */
  queryHistoryRoot: ["query-history"] as const,
  queryHistory: (connectionId?: string, page = 1) =>
    ["query-history", connectionId ?? "all", page] as const,
  /** Prefix for all saved-query queries (used for invalidation). */
  savedQueriesRoot: ["saved-queries"] as const,
  savedQueries: (connectionId?: string, page = 1) =>
    ["saved-queries", connectionId ?? "all", page] as const,
} as const;
