/**
 * Centralized TanStack Query keys. Keeping them here prevents accidental key
 * drift between queries and mutations.
 */
export const queryKeys = {
  health: ["health"] as const,
  connections: ["connections"] as const,
  /** Selectable databases for a server-level connection (PRF-01). */
  databases: (connectionId: string) =>
    ["connections", connectionId, "databases"] as const,
  /**
   * Schema tree for a specific connection and (for server-level PostgreSQL
   * connections) database. The database is part of the key so metadata never
   * leaks between databases or connections (PRF-01).
   */
  /** Prefix covering every schema query (all databases) for a connection. */
  schemaRoot: (connectionId: string) =>
    ["connections", connectionId, "schema"] as const,
  schema: (connectionId: string, database = "") =>
    ["connections", connectionId, "schema", database] as const,
  /** Prefix for all history queries (used for invalidation). */
  queryHistoryRoot: ["query-history"] as const,
  queryHistory: (connectionId?: string, page = 1) =>
    ["query-history", connectionId ?? "all", page] as const,
  /** Prefix for all saved-query queries (used for invalidation). */
  savedQueriesRoot: ["saved-queries"] as const,
  savedQueries: (connectionId?: string, page = 1) =>
    ["saved-queries", connectionId ?? "all", page] as const,
} as const;
