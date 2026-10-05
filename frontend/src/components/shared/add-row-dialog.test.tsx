import { fireEvent, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { insertRow } from "@/lib/api/endpoints";
import { renderWithProviders } from "@/test/render";
import type { TableColumnInfo } from "@/types/api";

import { AddRowDialog } from "./add-row-dialog";

vi.mock("@/lib/api/endpoints", () => ({
  insertRow: vi.fn(),
}));

const columns: TableColumnInfo[] = [
  { name: "id", database_type: "bigint", primary_key: true, nullable: false, insertable: false, updatable: false },
  { name: "name", database_type: "text", nullable: false, insertable: true, updatable: true },
  { name: "nickname", database_type: "text", nullable: true, insertable: true, updatable: true },
  { name: "amount", database_type: "bigint", nullable: true, insertable: true, updatable: true },
  { name: "total", database_type: "numeric", nullable: true, insertable: false, updatable: false },
];

function renderDialog(overrides: Partial<Parameters<typeof AddRowDialog>[0]> = {}) {
  return renderWithProviders(
    <AddRowDialog
      connectionId="c1"
      database="alpha"
      schema="public"
      table="users"
      columns={columns}
      onClose={vi.fn()}
      onInserted={vi.fn()}
      {...overrides}
    />,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(insertRow).mockResolvedValue({ affected_rows: 1 } as never);
});

describe("AddRowDialog", () => {
  it("is metadata-driven and protects non-insertable columns", () => {
    renderDialog();
    // Non-insertable columns are shown read-only, not editable.
    expect(screen.queryByLabelText("id mode")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("total mode")).not.toBeInTheDocument();
    expect(screen.getByLabelText("name mode")).toBeInTheDocument();
    expect(screen.getByLabelText("amount mode")).toBeInTheDocument();
  });

  it("offers NULL only for nullable columns", () => {
    renderDialog();
    expect(screen.getByLabelText("nickname mode")).toHaveTextContent("null");
    expect(screen.getByLabelText("name mode")).not.toHaveTextContent("null");
  });

  it("distinguishes VALUE / NULL / DEFAULT and omits default columns", async () => {
    renderDialog();
    fireEvent.change(screen.getByLabelText("name value"), { target: { value: "Alfie" } });
    fireEvent.change(screen.getByLabelText("nickname mode"), { target: { value: "null" } });
    fireEvent.change(screen.getByLabelText("amount mode"), { target: { value: "default" } });
    fireEvent.click(screen.getByRole("button", { name: "Insert" }));

    await waitFor(() =>
      expect(insertRow).toHaveBeenCalledWith("c1", {
        database: "alpha",
        schema: "public",
        table: "users",
        values: {
          name: { mode: "value", value: "Alfie" },
          nickname: { mode: "null" },
        },
      }),
    );
  });

  it("keeps BIGINT values as exact strings", async () => {
    renderDialog();
    fireEvent.change(screen.getByLabelText("amount mode"), { target: { value: "value" } });
    fireEvent.change(screen.getByLabelText("amount value"), {
      target: { value: "9223372036854775807" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Insert" }));
    await waitFor(() =>
      expect(insertRow).toHaveBeenCalledWith(
        "c1",
        expect.objectContaining({
          values: expect.objectContaining({
            amount: { mode: "value", value: "9223372036854775807" },
          }),
        }),
      ),
    );
  });

  it("prevents duplicate submission while inserting", async () => {
    let resolve: (value: unknown) => void = () => {};
    vi.mocked(insertRow).mockImplementation(
      () => new Promise((r) => { resolve = r; }) as never,
    );
    renderDialog();
    const insert = screen.getByRole("button", { name: "Insert" });
    fireEvent.click(insert);
    fireEvent.click(insert);
    expect(insertRow).toHaveBeenCalledTimes(1);
    resolve({ affected_rows: 1 });
  });
});
