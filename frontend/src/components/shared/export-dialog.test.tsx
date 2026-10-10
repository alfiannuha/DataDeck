import { fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiClientError } from "@/lib/api-client";
import { downloadBlob } from "@/lib/export/build-export";
import { exportTableData } from "@/lib/api/endpoints";
import { renderWithProviders } from "@/test/render";
import { useExportStore, type ExportTarget } from "@/store/useExportStore";

import { ExportDialog } from "./export-dialog";

vi.mock("@/lib/api/endpoints", () => ({
  exportTableData: vi.fn(),
}));

vi.mock("@/lib/export/build-export", () => ({
  downloadBlob: vi.fn(),
}));

const baseTarget: ExportTarget = {
  connectionId: "c1",
  database: "app",
  schema: "public",
  table: "users",
  objectType: "BASE TABLE",
};

describe("ExportDialog (PRF-02/T14)", () => {
  beforeEach(() => {
    useExportStore.setState({ target: null });
    vi.mocked(exportTableData).mockReset();
    vi.mocked(downloadBlob).mockReset();
  });

  afterEach(() => {
    useExportStore.setState({ target: null });
  });

  it("renders nothing without a target", () => {
    renderWithProviders(<ExportDialog />);
    expect(screen.queryByTestId("export-dialog")).not.toBeInTheDocument();
  });

  it("exports the explicit binding as CSV and saves the server filename", async () => {
    vi.mocked(exportTableData).mockResolvedValue({
      blob: new Blob(["x"]),
      filename: "users.csv",
    });
    useExportStore.setState({ target: baseTarget });

    renderWithProviders(<ExportDialog />);

    expect(screen.getByTestId("export-format-csv")).toBeChecked();
    fireEvent.click(screen.getByTestId("export-submit"));

    await waitFor(() =>
      expect(exportTableData).toHaveBeenCalledWith("c1", {
        database: "app",
        schema: "public",
        table: "users",
        format: "csv",
        sort: null,
        filters: undefined,
        includeHeaders: true,
        transaction: undefined,
      }),
    );
    expect(downloadBlob).toHaveBeenCalledWith(expect.any(Blob), "users.csv");
    // A successful export closes the dialog.
    await waitFor(() => expect(useExportStore.getState().target).toBeNull());
  });

  it("disables SQL for views and offers the transaction toggle for base tables", () => {
    useExportStore.setState({
      target: { ...baseTarget, objectType: "VIEW" },
    });
    renderWithProviders(<ExportDialog />);
    expect(screen.getByTestId("export-format-sql")).toBeDisabled();
    expect(screen.getByTestId("export-format-csv")).not.toBeDisabled();
  });

  it("passes the transaction toggle for SQL exports", async () => {
    vi.mocked(exportTableData).mockResolvedValue({
      blob: new Blob(["x"]),
      filename: null,
    });
    useExportStore.setState({ target: baseTarget });

    renderWithProviders(<ExportDialog />);
    fireEvent.click(screen.getByTestId("export-format-sql"));
    fireEvent.click(screen.getByTestId("export-transaction")); // turn off
    fireEvent.click(screen.getByTestId("export-submit"));

    await waitFor(() =>
      expect(exportTableData).toHaveBeenCalledWith(
        "c1",
        expect.objectContaining({ format: "sql", transaction: false }),
      ),
    );
    // No server filename → fall back to a client-built name.
    expect(downloadBlob).toHaveBeenCalledWith(expect.any(Blob), "users.sql");
  });

  it("surfaces a backend export error and keeps the dialog open", async () => {
    vi.mocked(exportTableData).mockRejectedValue(
      new ApiClientError("sql export requires a base table", "EXPORT_FORMAT_UNSUPPORTED", 400),
    );
    useExportStore.setState({ target: baseTarget });

    renderWithProviders(<ExportDialog />);
    fireEvent.click(screen.getByTestId("export-submit"));

    const error = await screen.findByTestId("export-error");
    expect(error).toHaveTextContent("EXPORT_FORMAT_UNSUPPORTED");
    expect(useExportStore.getState().target).not.toBeNull();
    expect(downloadBlob).not.toHaveBeenCalled();
  });
});
