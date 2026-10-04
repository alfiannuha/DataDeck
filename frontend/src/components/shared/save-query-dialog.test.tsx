import { fireEvent, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  createSavedQuery,
  executeQuery,
  updateSavedQuery,
} from "@/lib/api/endpoints";
import { ApiClientError } from "@/lib/api-client";
import { renderWithProviders } from "@/test/render";

import { SaveQueryDialog } from "./save-query-dialog";

vi.mock("@/lib/api/endpoints", () => ({
  createSavedQuery: vi.fn(),
  updateSavedQuery: vi.fn(),
  listSavedQueries: vi.fn(),
  deleteSavedQuery: vi.fn(),
  executeQuery: vi.fn(),
}));

beforeEach(() => {
  vi.clearAllMocks();
});

describe("SaveQueryDialog", () => {
  it("creates a saved query with the SQL and no execution", async () => {
    vi.mocked(createSavedQuery).mockResolvedValue({
      id: "s1",
      title: "My query",
      tags: null,
    } as never);
    const onSaved = vi.fn();
    renderWithProviders(
      <SaveQueryDialog
        open
        onOpenChange={vi.fn()}
        sql="SELECT 1"
        connectionId="c1"
        onSaved={onSaved}
      />,
    );

    fireEvent.change(screen.getByLabelText("Title"), {
      target: { value: "My query" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(createSavedQuery).toHaveBeenCalledWith(
        expect.objectContaining({
          title: "My query",
          sql_text: "SELECT 1",
          connection_id: "c1",
        }),
      ),
    );
    expect(onSaved).toHaveBeenCalledWith(expect.objectContaining({ id: "s1" }));
    expect(executeQuery).not.toHaveBeenCalled();
  });

  it("validates the title", async () => {
    renderWithProviders(
      <SaveQueryDialog
        open
        onOpenChange={vi.fn()}
        sql="SELECT 1"
        connectionId={null}
        onSaved={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("Title is required.")).toBeInTheDocument();
    expect(createSavedQuery).not.toHaveBeenCalled();
  });

  it("updates an existing saved query", async () => {
    vi.mocked(updateSavedQuery).mockResolvedValue({
      id: "s1",
      title: "Renamed",
      tags: null,
    } as never);
    const onSaved = vi.fn();
    renderWithProviders(
      <SaveQueryDialog
        open
        onOpenChange={vi.fn()}
        sql="SELECT 2"
        connectionId={null}
        existing={{ id: "s1", title: "Old", tags: null }}
        onSaved={onSaved}
      />,
    );

    const title = screen.getByLabelText("Title");
    expect(title).toHaveValue("Old");
    fireEvent.change(title, { target: { value: "Renamed" } });
    fireEvent.click(screen.getByRole("button", { name: "Update" }));

    await waitFor(() =>
      expect(updateSavedQuery).toHaveBeenCalledWith("s1", expect.objectContaining({ title: "Renamed" })),
    );
    expect(createSavedQuery).not.toHaveBeenCalled();
  });

  it("keeps the dialog open and shows the error when saving fails", async () => {
    vi.mocked(createSavedQuery).mockRejectedValue(
      new ApiClientError("conflict", "CONFLICT", 409),
    );
    const onSaved = vi.fn();
    renderWithProviders(
      <SaveQueryDialog
        open
        onOpenChange={vi.fn()}
        sql="SELECT 1"
        connectionId="c1"
        onSaved={onSaved}
      />,
    );

    fireEvent.change(screen.getByLabelText("Title"), {
      target: { value: "My query" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(await screen.findByText(/CONFLICT/)).toBeInTheDocument();
    expect(onSaved).not.toHaveBeenCalled();
    // The dialog is still open with the entered title preserved.
    expect(screen.getByLabelText("Title")).toHaveValue("My query");
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();
  });

  it("persists the database context when provided", async () => {
    vi.mocked(createSavedQuery).mockResolvedValue({ id: "s1", title: "DB", tags: null } as never);
    renderWithProviders(
      <SaveQueryDialog
        open
        onOpenChange={vi.fn()}
        sql="SELECT 1"
        connectionId="c1"
        databaseName="CCM"
        onSaved={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByLabelText("Title"), { target: { value: "DB" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(createSavedQuery).toHaveBeenCalledWith(
        expect.objectContaining({ database_name: "CCM" }),
      ),
    );
  });

  it("omits database_name when no context is provided", async () => {
    vi.mocked(createSavedQuery).mockResolvedValue({ id: "s1", title: "NoDB", tags: null } as never);
    renderWithProviders(
      <SaveQueryDialog
        open
        onOpenChange={vi.fn()}
        sql="SELECT 1"
        connectionId="c1"
        onSaved={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByLabelText("Title"), { target: { value: "NoDB" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(createSavedQuery).toHaveBeenCalled());
    const body = vi.mocked(createSavedQuery).mock.calls[0][0] as Record<string, unknown>;
    expect(body.database_name).toBeUndefined();
  });
});
