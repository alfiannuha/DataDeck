import { fireEvent, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useExecutionStore } from "@/store/useExecutionStore";
import { renderWithProviders } from "@/test/render";
import type { QueryResult } from "@/types/api";

import { ResultsPanel } from "./results-panel";

const downloadExport = vi.fn();
vi.mock("@/lib/export/build-export", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@/lib/export/build-export")>();
  return {
    ...actual,
    downloadExport: (...args: unknown[]) => downloadExport(...args),
  };
});

function result(truncated = false): QueryResult {
  return {
    columns: [
      { name: "id", type: "text" },
      { name: "name", type: "text" },
    ],
    rows: [["1", "a,b"]],
    rows_affected: 1,
    execution_time_ms: 1,
    truncated,
  } as QueryResult;
}

beforeEach(() => {
  useExecutionStore.getState().reset();
  downloadExport.mockReset();
});

describe("ResultsPanel CSV export", () => {
  it("disables the action without an exportable result", () => {
    renderWithProviders(<ResultsPanel />);
    expect(screen.getByRole("button", { name: "Export CSV" })).toBeDisabled();
  });

  it("downloads the current result as CSV without re-running SQL", () => {
    useExecutionStore.getState().resolve(result());
    renderWithProviders(<ResultsPanel />);

    fireEvent.click(screen.getByRole("button", { name: "Export CSV" }));

    expect(downloadExport).toHaveBeenCalledTimes(1);
    const artifact = downloadExport.mock.calls[0][0] as {
      filename: string;
      content: string;
      truncated: boolean;
    };
    expect(artifact.filename).toContain(".csv");
    expect(artifact.content).toContain('"a,b"');
    expect(artifact.truncated).toBe(false);
    expect(screen.getByRole("status")).toHaveTextContent("Exported 1 rows");
  });

  it("warns before exporting a truncated result", () => {
    useExecutionStore.getState().resolve(result(true));
    renderWithProviders(<ResultsPanel />);

    fireEvent.click(screen.getByRole("button", { name: "Export CSV" }));
    expect(downloadExport).not.toHaveBeenCalled();
    expect(
      screen.getByText(/truncated at the 50 MB safety limit/),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Export anyway" }));
    expect(downloadExport).toHaveBeenCalledTimes(1);
    expect(downloadExport.mock.calls[0][0]).toMatchObject({ truncated: true });
  });
});

describe("ResultsPanel JSON export", () => {
  it("disables the action without a result", () => {
    renderWithProviders(<ResultsPanel />);
    expect(screen.getByRole("button", { name: "Export JSON" })).toBeDisabled();
  });

  it("downloads row objects as JSON without re-running SQL", () => {
    useExecutionStore.getState().resolve(result());
    renderWithProviders(<ResultsPanel />);

    fireEvent.click(screen.getByRole("button", { name: "Export JSON" }));

    expect(downloadExport).toHaveBeenCalledTimes(1);
    const artifact = downloadExport.mock.calls[0][0] as {
      filename: string;
      content: string;
      truncated: boolean;
    };
    expect(artifact.filename).toContain(".json");
    expect(JSON.parse(artifact.content)).toEqual([{ id: "1", name: "a,b" }]);
    expect(artifact.truncated).toBe(false);
  });

  it("warns before exporting a truncated result as JSON", () => {
    useExecutionStore.getState().resolve(result(true));
    renderWithProviders(<ResultsPanel />);

    fireEvent.click(screen.getByRole("button", { name: "Export JSON" }));
    expect(downloadExport).not.toHaveBeenCalled();
    expect(screen.getByText(/the JSON will contain only the/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Export anyway" }));
    expect(downloadExport).toHaveBeenCalledTimes(1);
    expect(downloadExport.mock.calls[0][0]).toMatchObject({ truncated: true });
  });

  it("reports an export failure without crashing", () => {
    useExecutionStore.getState().resolve(result());
    downloadExport.mockImplementationOnce(() => {
      throw new Error("boom");
    });
    renderWithProviders(<ResultsPanel />);

    fireEvent.click(screen.getByRole("button", { name: "Export CSV" }));

    expect(screen.getByRole("status")).toHaveTextContent("Export failed");
  });
});
