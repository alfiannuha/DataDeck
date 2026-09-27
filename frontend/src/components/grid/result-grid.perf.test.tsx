import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { QueryResult } from "@/types/api";

import { ResultGrid } from "./result-grid";

const SIZES = [1_000, 10_000, 50_000, 100_000];

function makeResult(count: number, columnCount = 5): QueryResult {
  const columns = Array.from({ length: columnCount }, (_, c) => ({
    name: `c${c}`,
    type: "text",
  }));
  const rows = Array.from({ length: count }, (_, r) =>
    Array.from({ length: columnCount }, (_, c) => `${r}:${c}`),
  );
  return {
    columns,
    rows,
    rows_affected: count,
    execution_time_ms: 0,
    truncated: false,
  } as QueryResult;
}

describe("ResultGrid virtualization across dataset sizes", () => {
  it("keeps DOM rows bounded regardless of dataset size", () => {
    for (const size of SIZES) {
      const started = performance.now();
      const { unmount } = render(<ResultGrid result={makeResult(size)} />);
      const domRows = screen.getAllByRole("row").length - 1; // minus header
      const elapsed = Math.round(performance.now() - started);

      // Observable evidence for the review report.
      process.stdout.write(
        `\n[perf] size=${size} domRows=${domRows} mountMs=${elapsed}`,
      );

      expect(domRows).toBeGreaterThan(0);
      expect(domRows).toBeLessThan(60);
      unmount();
    }
  });
});
