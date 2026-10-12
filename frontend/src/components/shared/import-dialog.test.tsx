import { fireEvent, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  executeTableImport,
  uploadTableImport,
  validateTableImport,
} from "@/lib/api/endpoints";
import { renderWithProviders } from "@/test/render";
import { useImportStore, type ImportTarget } from "@/store/useImportStore";
import type { TableImportSession, TableImportValidation } from "@/types/api";

import { ImportDialog } from "./import-dialog";

vi.mock("@/lib/api/endpoints", () => ({
  uploadTableImport: vi.fn(),
  validateTableImport: vi.fn(),
  executeTableImport: vi.fn(),
}));

const baseTarget: ImportTarget = {
  connectionId: "c1",
  database: "app",
  schema: "public",
  table: "users",
  objectType: "BASE TABLE",
};

const session: TableImportSession = {
  import_id: "sess-1",
  filename: "users.csv",
  format: "csv",
  columns: ["name", "age"],
  row_count: 2,
  preview: [
    [{ value: "ada", is_null: false }, { value: "36", is_null: false }],
    [{ is_null: true }, { value: "1", is_null: false }],
  ],
};

const validation: TableImportValidation = {
  valid: true,
  row_count: 2,
  mapping: [{ source: 0, target: "name" }, { source: 1, target: "age" }],
  errors: [],
};

describe("ImportDialog (PRF-02/T15)", () => {
  beforeEach(() => {
    useImportStore.setState({ target: null });
    vi.mocked(uploadTableImport).mockReset();
    vi.mocked(validateTableImport).mockReset();
    vi.mocked(executeTableImport).mockReset();
  });

  afterEach(() => {
    useImportStore.setState({ target: null });
  });

  it("renders nothing without a target", () => {
    renderWithProviders(<ImportDialog />);
    expect(screen.queryByTestId("import-dialog")).not.toBeInTheDocument();
  });

  it("uploads, reviews the resolved mapping, and executes against the binding", async () => {
    vi.mocked(uploadTableImport).mockResolvedValue(session);
    vi.mocked(validateTableImport).mockResolvedValue(validation);
    vi.mocked(executeTableImport).mockResolvedValue({ inserted: 2 });
    useImportStore.setState({ target: baseTarget });

    renderWithProviders(<ImportDialog />);

    fireEvent.change(screen.getByTestId("import-file"), {
      target: { files: [new File(["name,age\nada,36"], "users.csv")] },
    });
    fireEvent.click(screen.getByTestId("import-upload"));

    await screen.findByTestId("import-validation-ok");
    expect(uploadTableImport).toHaveBeenCalledWith("c1", {
      database: "app",
      schema: "public",
      table: "users",
      format: "csv",
      csvHeader: true,
      file: expect.any(File),
    });
    expect(validateTableImport).toHaveBeenCalledWith("c1", "sess-1", {
      database: "app",
      schema: "public",
      table: "users",
    });
    expect(screen.getByTestId("import-preview")).toBeInTheDocument();
    expect(screen.getByTestId("import-mapping")).toHaveTextContent("name → name");

    fireEvent.click(screen.getByTestId("import-execute"));
    await screen.findByTestId("import-done-message");
    expect(executeTableImport).toHaveBeenCalledWith("c1", "sess-1", {
      database: "app",
      schema: "public",
      table: "users",
    });
  });

  it("blocks execute when validation reports errors", async () => {
    vi.mocked(uploadTableImport).mockResolvedValue(session);
    vi.mocked(validateTableImport).mockResolvedValue({
      ...validation,
      valid: false,
      errors: [{ row: 2, column: "age", message: "value is not numeric" }],
    });
    useImportStore.setState({ target: baseTarget });

    renderWithProviders(<ImportDialog />);
    fireEvent.change(screen.getByTestId("import-file"), {
      target: { files: [new File(["x"], "users.csv")] },
    });
    fireEvent.click(screen.getByTestId("import-upload"));

    await screen.findByTestId("import-validation-errors");
    expect(screen.getByTestId("import-execute")).toBeDisabled();
    expect(executeTableImport).not.toHaveBeenCalled();
  });

  it("never uploads into a read-only object", () => {
    useImportStore.setState({
      target: { ...baseTarget, objectType: "VIEW" },
    });
    renderWithProviders(<ImportDialog />);
    expect(screen.getByTestId("import-file")).toBeDisabled();
    expect(screen.getByTestId("import-format-csv")).toBeDisabled();
    fireEvent.change(screen.getByTestId("import-file"), {
      target: { files: [new File(["x"], "users.csv")] },
    });
    expect(screen.getByTestId("import-upload")).toBeDisabled();
  });
});
