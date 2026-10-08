import { fireEvent, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { deleteRow } from "@/lib/api/endpoints";
import { ApiClientError } from "@/lib/api-client";
import { renderWithProviders } from "@/test/render";

import { DeleteRowDialog } from "./delete-row-dialog";

vi.mock("@/lib/api/endpoints", () => ({
  deleteRow: vi.fn(),
}));

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(deleteRow).mockResolvedValue({ affected_rows: 1 } as never);
});

function renderDialog(overrides: Partial<Parameters<typeof DeleteRowDialog>[0]> = {}) {
  return renderWithProviders(
    <DeleteRowDialog
      connectionId="c1"
      database="alpha"
      schema="public"
      table="users"
      identity={{ id: "7" }}
      expected={{ id: "7", name: "Alice" }}
      onClose={vi.fn()}
      onDeleted={vi.fn()}
      {...overrides}
    />,
  );
}

describe("DeleteRowDialog", () => {
  it("shows the target and identity before confirming", async () => {
    renderDialog();
    const dialog = await screen.findByTestId("delete-row-dialog");
    expect(dialog).toHaveTextContent("alpha / public.users");
    expect(dialog).toHaveTextContent("id = 7");
    expect(deleteRow).not.toHaveBeenCalled();
  });

  it("deletes with identity and expected, then reports success", async () => {
    const onDeleted = vi.fn();
    renderDialog({ onDeleted });
    fireEvent.click(await screen.findByRole("button", { name: "Delete" }));

    await waitFor(() =>
      expect(deleteRow).toHaveBeenCalledWith("c1", {
        database: "alpha",
        schema: "public",
        table: "users",
        identity: { id: "7" },
        expected: { id: "7", name: "Alice" },
      }),
    );
    await waitFor(() => expect(onDeleted).toHaveBeenCalled());
  });

  it("surfaces the error code and stays open on failure", async () => {
    vi.mocked(deleteRow).mockRejectedValueOnce(
      new ApiClientError("the row changed since it was loaded", "ROW_CONFLICT", 409),
    );
    const onDeleted = vi.fn();
    renderDialog({ onDeleted });
    fireEvent.click(await screen.findByRole("button", { name: "Delete" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("ROW_CONFLICT");
    expect(screen.getByTestId("delete-row-dialog")).toBeInTheDocument();
    expect(onDeleted).not.toHaveBeenCalled();
  });

  it("Cancel closes without deleting", async () => {
    const onClose = vi.fn();
    renderDialog({ onClose });
    fireEvent.click(await screen.findByRole("button", { name: "Cancel" }));
    expect(onClose).toHaveBeenCalled();
    expect(deleteRow).not.toHaveBeenCalled();
  });

  it("blocks double submission while deleting", async () => {
    let resolve: (value: unknown) => void = () => {};
    vi.mocked(deleteRow).mockImplementation(
      () => new Promise((r) => { resolve = r; }) as never,
    );
    renderDialog();
    const button = await screen.findByRole("button", { name: "Delete" });
    fireEvent.click(button);
    fireEvent.click(button);
    expect(deleteRow).toHaveBeenCalledTimes(1);
    resolve({ affected_rows: 1 });
  });
});
