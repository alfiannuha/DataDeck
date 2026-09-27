export type ExportFormat = "csv" | "json";

/**
 * Sanitize a string for use as a filename part: keep only filesystem-safe
 * characters, collapse runs, cap length, and fall back when empty.
 */
export function sanitizeFilenamePart(
  value: string | null | undefined,
  fallback = "connection",
): string {
  const cleaned = (value ?? "")
    .replace(/[^A-Za-z0-9._-]+/g, "-")
    .replace(/^[-.]+|[-.]+$/g, "")
    .slice(0, 64);
  return cleaned || fallback;
}

function timestampPart(timestamp: Date): string {
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${timestamp.getFullYear()}${pad(timestamp.getMonth() + 1)}${pad(
    timestamp.getDate(),
  )}-${pad(timestamp.getHours())}${pad(timestamp.getMinutes())}${pad(
    timestamp.getSeconds(),
  )}`;
}

/**
 * Build a safe export filename such as `datadeck_My-Conn_20260927-143200.csv`.
 * Connection names are sanitized; passwords/DSNs are never included.
 */
export function buildExportFilename({
  connectionName,
  format,
  timestamp = new Date(),
}: {
  connectionName?: string | null;
  format: ExportFormat;
  timestamp?: Date;
}): string {
  const connection = sanitizeFilenamePart(connectionName, "datadeck");
  return `datadeck_${connection}_${timestampPart(timestamp)}.${format}`;
}
