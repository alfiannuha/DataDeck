import { describe, expect, it } from "vitest";

import type { QueryResult } from "@/types/api";

import { buildExport } from "./build-export";

const SIZES = [1_000, 10_000, 50_000, 100_000];

function makeResult(count: number): QueryResult {
  const columns = [
    { name: "id", type: "bigint" },
    { name: "name", type: "text" },
    { name: "meta", type: "jsonb" },
  ];
  const rows = Array.from({ length: count }, (_, i) => [
    String(i),
    `name-${i}`,
    { i, label: `row-${i}` },
  ]);
  return {
    columns,
    rows,
    rows_affected: count,
    execution_time_ms: 0,
    truncated: false,
  } as QueryResult;
}

describe("export serialization benchmark", () => {
  it("serializes representative result sizes in a single pass", () => {
    for (const size of SIZES) {
      const result = makeResult(size);

      const csvStart = performance.now();
      const csv = buildExport(result, {
        format: "csv",
        connectionName: "bench",
        timestamp: new Date(0),
      });
      const csvMs = performance.now() - csvStart;

      const jsonStart = performance.now();
      const json = buildExport(result, {
        format: "json",
        connectionName: "bench",
        timestamp: new Date(0),
      });
      const jsonMs = performance.now() - jsonStart;

      process.stdout.write(
        `\n[export] rows=${size} csvMs=${csvMs.toFixed(1)} jsonMs=${jsonMs.toFixed(1)} ` +
          `csvBytes=${csv.content.length} jsonBytes=${json.content.length}`,
      );

      expect(csv.rowCount).toBe(size);
      expect(csv.content.length).toBeGreaterThan(0);
      expect(json.content.length).toBeGreaterThan(0);
    }
  });
});
