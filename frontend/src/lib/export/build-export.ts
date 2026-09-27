import type { QueryResult } from "@/types/api";

import { buildExportFilename, type ExportFormat } from "./filename";
import { csvCellValue, csvEscape, disambiguateColumns, guardFormulaExport, jsonCellValue } from "./values";

export type { ExportFormat };

export interface ExportArtifact {
  filename: string;
  mime: string;
  content: string;
  /** True when the source result was itself truncated by the backend. */
  truncated: boolean;
  rowCount: number;
  columnCount: number;
}

const MIME: Record<ExportFormat, string> = {
  csv: "text/csv;charset=utf-8",
  json: "application/json;charset=utf-8",
};

function toCsv(columns: string[], rows: unknown[][]): string {
  const lines: string[] = [columns.map(csvEscape).join(",")];
  for (const row of rows) {
    lines.push(
      columns
        .map((_, index) =>
          csvEscape(guardFormulaExport(csvCellValue(row[index]))),
        )
        .join(","),
    );
  }
  return `${lines.join("\n")}\n`;
}

function toJson(columns: string[], rows: unknown[][]): string {
  const objects = rows.map((row) => {
    const record: Record<string, unknown> = {};
    columns.forEach((name, index) => {
      record[name] = jsonCellValue(row[index]);
    });
    return record;
  });
  return JSON.stringify(objects);
}

/**
 * Build an export artifact from the current result set. This is a pure function
 * over `QueryResult`: it never re-executes SQL and never scrapes the DOM.
 *
 * A result flagged `truncated` produces a partial export; the flag is carried
 * through so the UI can warn the user.
 */
export function buildExport(
  result: QueryResult,
  {
    format,
    connectionName,
    timestamp = new Date(),
  }: {
    format: ExportFormat;
    connectionName?: string | null;
    timestamp?: Date;
  },
): ExportArtifact {
  const columns = disambiguateColumns(result.columns);
  const rows = (result.rows ?? []) as unknown[][];
  const content = format === "csv" ? toCsv(columns, rows) : toJson(columns, rows);

  return {
    filename: buildExportFilename({ connectionName, format, timestamp }),
    mime: MIME[format],
    content,
    truncated: Boolean(result.truncated),
    rowCount: rows.length,
    columnCount: columns.length,
  };
}

/**
 * Trigger a browser download for an artifact. Kept separate from the serializers
 * so the export layer stays testable without a DOM.
 */
export function downloadExport(artifact: ExportArtifact): void {
  if (typeof document === "undefined") return;
  const blob = new Blob([artifact.content], { type: artifact.mime });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = artifact.filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}
