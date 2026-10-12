import { fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  executeDatabaseImport,
  exportDatabase,
  uploadDatabaseImport,
} from "@/lib/api/endpoints";
import { downloadBlob } from "@/lib/export/build-export";
import { renderWithProviders } from "@/test/render";
import {
  useDatabaseScriptStore,
  type DatabaseScriptTarget,
} from "@/store/useDatabaseScriptStore";

import { DatabaseScriptDialogs } from "./database-script-dialogs";

vi.mock("@/lib/api/endpoints", () => ({
  exportDatabase: vi.fn(),
  uploadDatabaseImport: vi.fn(),
  executeDatabaseImport: vi.fn(),
}));

vi.mock("@/lib/export/build-export", () => ({
  downloadBlob: vi.fn(),
}));

const target: DatabaseScriptTarget = {
  connectionId: "c1",
  database: "alpha",
  engine: "postgres",
};

describe("DatabaseScriptDialogs (PRF-02/T17)", () => {
  beforeEach(() => {
    useDatabaseScriptStore.setState({ request: null });
    vi.mocked(exportDatabase).mockReset();
    vi.mocked(uploadDatabaseImport).mockReset();
    vi.mocked(executeDatabaseImport).mockReset();
    vi.mocked(downloadBlob).mockReset();
  });

  afterEach(() => {
    useDatabaseScriptStore.setState({ request: null });
  });

  it("renders nothing without a request", () => {
    renderWithProviders(<DatabaseScriptDialogs />);
    expect(screen.queryByTestId("database-export-dialog")).not.toBeInTheDocument();
    expect(screen.queryByTestId("database-restore-dialog")).not.toBeInTheDocument();
  });

  it("exports the captured database with the selected mode", async () => {
    vi.mocked(exportDatabase).mockResolvedValue({
      blob: new Blob(["sql"]),
      filename: "alpha_schema_data.sql",
    });
    useDatabaseScriptStore.getState().open("export", target);

    renderWithProviders(<DatabaseScriptDialogs />);
    expect(screen.getByTestId("database-export-mode-schema_data")).toBeChecked();
    fireEvent.click(screen.getByTestId("database-export-mode-data"));
    fireEvent.click(screen.getByTestId("database-export-submit"));

    await waitFor(() =>
      expect(exportDatabase).toHaveBeenCalledWith("c1", {
        database: "alpha",
        mode: "data",
      }),
    );
    expect(downloadBlob).toHaveBeenCalledWith(expect.any(Blob), "alpha_schema_data.sql");
    await waitFor(() => expect(useDatabaseScriptStore.getState().request).toBeNull());
  });

  it("preflights then executes a restore", async () => {
    vi.mocked(uploadDatabaseImport).mockResolvedValue({
      token: "tok-1",
      engine: "postgres",
      database: "alpha",
      mode: "schema_data",
      tables: 2,
      indexes: 1,
      foreign_keys: 1,
      statements: 5,
      excluded: ["views"],
    });
    vi.mocked(executeDatabaseImport).mockResolvedValue({
      status: "ok",
      database: "alpha",
      statements: 4,
    });
    useDatabaseScriptStore.getState().open("import", target);

    renderWithProviders(<DatabaseScriptDialogs />);
    fireEvent.change(screen.getByTestId("database-restore-file"), {
      target: { files: [new File(["sql"], "alpha.sql")] },
    });
    fireEvent.click(screen.getByTestId("database-restore-upload"));

    await screen.findByTestId("database-restore-preview");
    expect(screen.getByTestId("database-restore-preview")).toHaveTextContent("2");
    expect(uploadDatabaseImport).toHaveBeenCalledWith("c1", {
      database: "alpha",
      file: expect.any(File),
    });
    // Upload alone must not execute anything.
    expect(executeDatabaseImport).not.toHaveBeenCalled();

    fireEvent.click(screen.getByTestId("database-restore-execute"));
    await screen.findByTestId("database-restore-done");
    expect(executeDatabaseImport).toHaveBeenCalledWith("c1", "tok-1");
  });
});
