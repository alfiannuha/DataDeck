import { fireEvent, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { applyStructure } from "@/lib/api/endpoints";
import { ApiClientError } from "@/lib/api-client";
import { renderWithProviders } from "@/test/render";

import { DeleteTableDialog } from "./delete-table-dialog";

vi.mock("@/lib/api/endpoints", () => ({
  applyStructure: vi.fn(),
}));

const target = {
  connectionId: "c1",
  connectionName: "CCM",
  database: "alpha",
  schema: "public",
  table: "users",
};

describe("DeleteTableDialog (PRF-05/E)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("requires two steps and an exact table name before dropping", async () => {
    vi.mocked(applyStructure).mockResolvedValue({ operation: "drop_table", statements: 1 } as never);
    renderWithProviders(<DeleteTableDialog target={target} onClose={() => {}} />);

    // Step 1: warning shows the full target; no delete button yet.
    expect(screen.getByTestId("delete-table-dialog")).toHaveTextContent("CCM");
    expect(screen.getByTestId("delete-table-dialog")).toHaveTextContent("public");
    expect(screen.queryByTestId("delete-table-submit")).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId("delete-table-continue"));

    const submit = screen.getByTestId("delete-table-submit");
    expect(submit).toBeDisabled();

    // Case-sensitive exact match, whitespace trimmed.
    fireEvent.change(screen.getByTestId("delete-table-typed"), { target: { value: "Users" } });
    expect(submit).toBeDisabled();
    fireEvent.change(screen.getByTestId("delete-table-typed"), { target: { value: "  users  " } });
    await waitFor(() => expect(submit).not.toBeDisabled());

    fireEvent.click(submit);
    await waitFor(() =>
      expect(applyStructure).toHaveBeenCalledWith("c1", {
        operation: "drop_table",
        database: "alpha",
        schema: "public",
        table: "users",
      }),
    );
    expect(await screen.findByTestId("delete-table-done")).toBeInTheDocument();
  });

  it("keeps the dialog open with an error when the drop fails", async () => {
    vi.mocked(applyStructure).mockRejectedValue(
      new ApiClientError("cannot drop table: dependent objects exist", "SQL_ERROR", 400),
    );
    renderWithProviders(<DeleteTableDialog target={target} onClose={() => {}} />);
    fireEvent.click(screen.getByTestId("delete-table-continue"));
    fireEvent.change(screen.getByTestId("delete-table-typed"), { target: { value: "users" } });
    fireEvent.click(screen.getByTestId("delete-table-submit"));

    expect(await screen.findByTestId("delete-table-error")).toHaveTextContent(
      "dependent objects",
    );
    expect(screen.queryByTestId("delete-table-done")).not.toBeInTheDocument();
  });
});
