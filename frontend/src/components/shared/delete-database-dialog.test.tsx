import { fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiClientError } from "@/lib/api-client";
import {
  deleteDatabase,
  deleteDatabasePreflight,
} from "@/lib/api/endpoints";
import { renderWithProviders } from "@/test/render";
import {
  useDeleteDatabaseStore,
  type DeleteDatabaseTarget,
} from "@/store/useDeleteDatabaseStore";

import { DeleteDatabaseDialog } from "./delete-database-dialog";

vi.mock("@/lib/api/endpoints", () => ({
  deleteDatabasePreflight: vi.fn(),
  deleteDatabase: vi.fn(),
}));

const target: DeleteDatabaseTarget = {
  connectionId: "c1",
  database: "alpha",
  engine: "postgres",
};

describe("DeleteDatabaseDialog (PRF-02/T18)", () => {
  beforeEach(() => {
    useDeleteDatabaseStore.setState({ target: null });
    vi.mocked(deleteDatabasePreflight).mockReset();
    vi.mocked(deleteDatabase).mockReset();
  });

  afterEach(() => {
    useDeleteDatabaseStore.setState({ target: null });
  });

  it("renders nothing without a target", () => {
    renderWithProviders(<DeleteDatabaseDialog />);
    expect(screen.queryByTestId("delete-database-dialog")).not.toBeInTheDocument();
  });

  it("requires an exact typed name and acknowledgment before enabling delete", async () => {
    vi.mocked(deleteDatabasePreflight).mockResolvedValue({
      token: "tok-1",
      connection_id: "c1",
      database: "alpha",
      engine: "postgres",
      expires_at: "2026-01-01T00:00:00Z",
    });
    useDeleteDatabaseStore.setState({ target });

    renderWithProviders(<DeleteDatabaseDialog />);
    const submit = await screen.findByTestId("delete-database-submit");
    expect(submit).toBeDisabled();

    fireEvent.change(screen.getByTestId("delete-database-typed"), {
      target: { value: "ALPHA" }, // case-sensitive
    });
    fireEvent.click(screen.getByTestId("delete-database-ack"));
    expect(submit).toBeDisabled();

    fireEvent.change(screen.getByTestId("delete-database-typed"), {
      target: { value: "alpha" },
    });
    await waitFor(() => expect(submit).not.toBeDisabled());

    vi.mocked(deleteDatabase).mockResolvedValue({ status: "deleted", database: "alpha" });
    fireEvent.click(submit);
    await screen.findByTestId("delete-database-done");
    expect(deleteDatabase).toHaveBeenCalledWith("c1", "alpha", {
      confirmationToken: "tok-1",
      typedDatabaseName: "alpha",
      acknowledged: true,
    });
  });

  it("blocks when preflight rejects (e.g. protected or in use)", async () => {
    vi.mocked(deleteDatabasePreflight).mockRejectedValue(
      new ApiClientError("this database is protected", "DATABASE_DELETE_PROTECTED", 400),
    );
    useDeleteDatabaseStore.setState({ target });

    renderWithProviders(<DeleteDatabaseDialog />);
    const blocked = await screen.findByTestId("delete-database-blocked");
    expect(blocked).toHaveTextContent("DATABASE_DELETE_PROTECTED");
    expect(screen.queryByTestId("delete-database-submit")).not.toBeInTheDocument();
  });
});
