import { ApiClientError } from "@/lib/api-client";

/**
 * Human-readable mapping for backend row-mutation errors (PRF-02/T11 §15).
 * The backend stays authoritative; this only improves presentation. Codes are
 * kept visible for support, and no raw SQL/DSN ever appears here.
 */
const MESSAGES: Record<string, string> = {
  ROW_NOT_FOUND: "This row no longer exists.",
  ROW_CONFLICT: "This row changed since it was loaded. Refresh and try again.",
  ROW_NOT_MUTABLE: "This row cannot be modified.",
  ROW_IDENTITY_REQUIRED:
    "A primary key is required to edit or delete this row.",
  ROW_IDENTITY_INVALID: "The row identity does not match this table.",
  COLUMN_READ_ONLY: "A database-managed column cannot be changed.",
  INVALID_COLUMN_VALUE: "One or more values are not valid for their column types.",
  CONSTRAINT_VIOLATION: "The change violates a database constraint.",
  MUTATION_AFFECTED_MULTIPLE_ROWS:
    "Safety failure: the change matched more than one row and was stopped. No data was modified.",
  EXPORT_FORMAT_UNSUPPORTED:
    "This object cannot be exported in that format. SQL scripts require a base table.",
  EXPORT_LIMIT_EXCEEDED:
    "The export is too large. Apply filters to narrow it down.",
};

/** True when the failure represents a mutation-safety violation, not a validation error. */
export function isMutationSafetyError(code: string): boolean {
  return code === "MUTATION_AFFECTED_MULTIPLE_ROWS";
}

/**
 * Renders a mutation error for dialog presentation: the friendly message
 * followed by the stable backend code, e.g.
 * "This row changed since it was loaded. Refresh and try again. (ROW_CONFLICT)".
 */
export function mutationErrorMessage(cause: unknown, fallback: string): string {
  if (cause instanceof ApiClientError) {
    const message = MESSAGES[cause.code] ?? cause.message;
    return `${message} (${cause.code})`;
  }
  return fallback;
}
