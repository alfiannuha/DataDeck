import { describe, expect, it } from "vitest";

import { buildExportFilename, sanitizeFilenamePart } from "./filename";

describe("sanitizeFilenamePart", () => {
  it("replaces filesystem-hostile characters", () => {
    expect(sanitizeFilenamePart('my/conn:*?"<>|')).toBe("my-conn");
  });

  it("prevents path traversal", () => {
    expect(sanitizeFilenamePart("../../etc/passwd")).toBe("etc-passwd");
  });

  it("falls back when empty", () => {
    expect(sanitizeFilenamePart("   ")).toBe("connection");
    expect(sanitizeFilenamePart(null)).toBe("connection");
  });

  it("caps length", () => {
    expect(sanitizeFilenamePart("a".repeat(200)).length).toBe(64);
  });
});

describe("buildExportFilename", () => {
  it("includes sanitized connection, timestamp and format", () => {
    const filename = buildExportFilename({
      connectionName: "My Conn/PG",
      format: "csv",
      timestamp: new Date(2026, 8, 27, 14, 32, 0),
    });
    expect(filename).toBe("datadeck_My-Conn-PG_20260927-143200.csv");
    expect(filename).not.toMatch(/[/\\]/);
  });

  it("works without a connection name", () => {
    expect(
      buildExportFilename({
        format: "json",
        timestamp: new Date(2026, 0, 1, 0, 0, 0),
      }),
    ).toBe("datadeck_datadeck_20260101-000000.json");
  });
});
