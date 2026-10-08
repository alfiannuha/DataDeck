import { fireEvent, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { browseTableData, updateRow } from "@/lib/api/endpoints";
import { ApiClientError } from "@/lib/api-client";
import { renderWithProviders } from "@/test/render";
import type { TableColumnInfo } from "@/types/api";

import { EditRowDialog } from "./edit-row-dialog";

vi.mock("@/lib/api/endpoints", () => ({
  updateRow: vi.fn(),
  browseTableData: vi.fn(),
}));

const columns: TableColumnInfo[] = [
  { name: "id", database_type: "bigint", primary_key: true, nullable: false, insertable: false, updatable: false },
  { name: "name", database_type: "text", nullable: false, insertable: true, updatable: true },
  { name: "nickname", database_type: "text", nullable: true, insertable: true, updatable: true },
  { name: "total", database_type: "numeric", nullable: true, insertable: false, updatable: false },
  { name: "amount", database_type: "bigint", nullable: true, insertable: true, updatable: true },
];

const row = ["42", "Alice", null, "10", "9223372036854775807"];

function renderDialog(overrides: Partial<Parameters<typeof EditRowDialog>[0]> = {}) {
  return renderWithProviders(
    <EditRowDialog
      connectionId="c1"
      database="alpha"
      schema="public"
      table="users"
      columns={columns}
      row={row}
      onClose={vi.fn()}
      onUpdated={vi.fn()}
      {...overrides}
    />,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(updateRow).mockResolvedValue({ affected_rows: 1 } as never);
  vi.mocked(browseTableData).mockResolvedValue({ rows: [] } as never);
});

describe("EditRowDialog", () => {
  it("renders PK/generated columns read-only and others editable", () => {
    renderDialog();
    expect(screen.queryByLabelText("id value")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("total value")).not.toBeInTheDocument();
    expect(screen.getByLabelText("name value")).toBeInTheDocument();
    expect(screen.getByLabelText("amount value")).toHaveValue("9223372036854775807");
  });

  it("disables Save until a field changes", () => {
    renderDialog();
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    fireEvent.change(screen.getByLabelText("name value"), { target: { value: "Alice Smith" } });
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();
  });

  it("submits only changed fields with identity and expected snapshot", async () => {
    const onUpdated = vi.fn();
    renderDialog({ onUpdated });
    fireEvent.change(screen.getByLabelText("name value"), { target: { value: "Alice Smith" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(updateRow).toHaveBeenCalledWith("c1", {
        database: "alpha",
        schema: "public",
        table: "users",
        identity: { id: "42" },
        expected: { name: "Alice", nickname: null, total: "10", amount: "9223372036854775807" },
        changes: { name: "Alice Smith" },
      }),
    );
    await waitFor(() => expect(onUpdated).toHaveBeenCalled());
  });

  it("supports explicit NULL and keeps BIGINT exact", async () => {
    renderDialog({ row: ["42", "Alice", "Al", "10", "9223372036854775807"] });
    fireEvent.change(screen.getByLabelText("nickname mode"), { target: { value: "null" } });
    fireEvent.change(screen.getByLabelText("amount value"), {
      target: { value: "9223372036854775806" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(updateRow).toHaveBeenCalledWith(
        "c1",
        expect.objectContaining({
          changes: { nickname: null, amount: "9223372036854775806" },
        }),
      ),
    );
  });

  it("keeps the dialog usable and surfaces ROW_CONFLICT with a Reload action", async () => {
    vi.mocked(updateRow).mockRejectedValueOnce(
      new ApiClientError("the row changed since it was loaded", "ROW_CONFLICT", 409),
    );
    renderDialog();
    fireEvent.change(screen.getByLabelText("name value"), { target: { value: "X" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(await screen.findByTestId("edit-row-dialog")).toBeInTheDocument();
    expect(await screen.findByRole("alert")).toHaveTextContent("ROW_CONFLICT");
    expect(screen.getByTestId("edit-row-reload")).toBeInTheDocument();
  });

  it("Reload refetches the row by identity and resets the draft", async () => {
    vi.mocked(updateRow).mockRejectedValueOnce(
      new ApiClientError("the row changed since it was loaded", "ROW_CONFLICT", 409),
    );
    vi.mocked(browseTableData).mockResolvedValue({
      rows: [["42", "Bob", null, "10", "9223372036854775807"]],
    } as never);
    renderDialog();
    fireEvent.change(screen.getByLabelText("name value"), { target: { value: "X" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    fireEvent.click(await screen.findByTestId("edit-row-reload"));

    await waitFor(() =>
      expect(browseTableData).toHaveBeenCalledWith(
        "c1",
        expect.objectContaining({
          table: "users",
          filters: [{ column: "id", operator: "equals", value: "42" }],
        }),
      ),
    );
    // The draft now mirrors the fresh server row; Save is idle again.
    await waitFor(() =>
      expect(screen.getByLabelText("name value")).toHaveValue("Bob"),
    );
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("blocks invalid values client-side without calling the API", async () => {
    renderDialog();
    fireEvent.change(screen.getByLabelText("amount value"), { target: { value: "abc" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(await screen.findByTestId("row-field-error")).toHaveTextContent(
      "amount: must be a whole number.",
    );
    expect(screen.getByLabelText("amount value")).toHaveAttribute("aria-invalid", "true");
    expect(updateRow).not.toHaveBeenCalled();
  });

  it("blocks malformed JSON before sending", async () => {
    renderDialog({
      columns: [
        { name: "id", database_type: "bigint", primary_key: true, nullable: false, updatable: false },
        { name: "meta", database_type: "jsonb", nullable: true, updatable: true },
      ],
      row: ["1", null],
    });
    fireEvent.change(screen.getByLabelText("meta mode"), { target: { value: "value" } });
    fireEvent.change(screen.getByLabelText("meta value"), { target: { value: "{" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(await screen.findByTestId("row-field-error")).toHaveTextContent(
      "meta: must be valid JSON.",
    );
    expect(updateRow).not.toHaveBeenCalled();
  });

  it("offers NULL only for nullable columns", () => {
    renderDialog();
    expect(screen.getByLabelText("nickname mode").querySelectorAll("option")).toHaveLength(2);
    expect(screen.getByLabelText("name mode").querySelectorAll("option")).toHaveLength(1);
  });

  it("prevents duplicate Save while submitting", async () => {
    let resolve: (value: unknown) => void = () => {};
    vi.mocked(updateRow).mockImplementation(() => new Promise((r) => { resolve = r; }) as never);
    renderDialog();
    fireEvent.change(screen.getByLabelText("name value"), { target: { value: "X" } });
    const save = screen.getByRole("button", { name: "Save" });
    fireEvent.click(save);
    fireEvent.click(save);
    expect(updateRow).toHaveBeenCalledTimes(1);
    resolve({ affected_rows: 1 });
  });
});
