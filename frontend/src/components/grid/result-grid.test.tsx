import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { QueryResult } from "@/types/api";

import { ResultGrid } from "./result-grid";

function result(columns: string[], rows: unknown[][]): QueryResult {
  return {
    columns: columns.map((name) => ({ name, type: "text" })),
    rows,
    rows_affected: rows.length,
    execution_time_ms: 1,
    truncated: false,
  } as QueryResult;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("ResultGrid", () => {
  it("renders column headers", () => {
    render(<ResultGrid result={result(["id", "email"], [["1", "a@b.c"]])} />);
    expect(screen.getByRole("columnheader", { name: "id" })).toBeInTheDocument();
    expect(
      screen.getByRole("columnheader", { name: "email" }),
    ).toBeInTheDocument();
  });

  it("shows a message when there are zero rows", () => {
    render(<ResultGrid result={result(["id"], [])} />);
    expect(screen.getByText("No rows returned.")).toBeInTheDocument();
  });

  it("renders NULL intentionally", () => {
    render(<ResultGrid result={result(["value"], [[null]])} />);
    expect(screen.getByText("NULL")).toBeInTheDocument();
  });

  it("preserves BIGINT strings", () => {
    render(<ResultGrid result={result(["big"], [["9007199254740993"]])} />);
    expect(screen.getByText("9007199254740993")).toBeInTheDocument();
  });

  it("renders JSON compactly", () => {
    render(<ResultGrid result={result(["meta"], [[{ role: "admin" }]])} />);
    expect(screen.getByText('{"role":"admin"}')).toBeInTheDocument();
  });

  it("copies the raw cell value on click", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    render(<ResultGrid result={result(["big"], [["9007199254740993"]])} />);

    fireEvent.click(screen.getByRole("gridcell"));

    await vi.waitFor(() =>
      expect(writeText).toHaveBeenCalledWith("9007199254740993"),
    );
  });

  it("exposes resize handles and horizontal width for each column", () => {
    const { container } = render(
      <ResultGrid result={result(["a", "b", "c"], [["1", "2", "3"]])} />,
    );
    expect(container.querySelectorAll(".cursor-col-resize")).toHaveLength(3);
    const header = container.querySelector('[role="row"]') as HTMLElement;
    expect(header.style.gridTemplateColumns.split(" ")).toHaveLength(3);
  });

  it("resizes a column by dragging its handle", () => {
    const { container } = render(
      <ResultGrid result={result(["a", "b"], [["1", "2"]])} />,
    );
    const header = container.querySelector('[role="row"]') as HTMLElement;
    const before = header.style.gridTemplateColumns;

    const handle = container.querySelector(".cursor-col-resize")!;
    fireEvent.mouseDown(handle, { clientX: 180 });
    fireEvent.mouseMove(document, { clientX: 280 });
    fireEvent.mouseUp(document);

    expect(header.style.gridTemplateColumns).not.toBe(before);
  });

  it("virtualizes rows: DOM rows stay bounded for 1,000 rows", () => {
    const rows = Array.from({ length: 1000 }, (_, i) => [String(i)]);
    render(<ResultGrid result={result(["n"], rows)} />);

    const domRows = screen.getAllByRole("row").length - 1; // minus header row
    expect(domRows).toBeGreaterThan(0);
    expect(domRows).toBeLessThan(60);
  });
});
