import { fireEvent, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SIDEBAR_WIDTH_STORAGE_KEY } from "@/hooks/use-resizable-sidebar";
import { renderWithProviders } from "@/test/render";

import { Sidebar } from "./sidebar";

vi.mock("@/components/sidebar/connection-list", () => ({
  ConnectionList: () => <div data-testid="connections" />,
}));
vi.mock("@/components/sidebar/schema-explorer", () => ({
  SchemaExplorer: () => <div data-testid="schema" />,
}));

beforeEach(() => {
  window.localStorage.clear();
});

function aside(): HTMLElement {
  return screen.getByLabelText("Connections and schema");
}

describe("Sidebar (resizable)", () => {
  it("renders at the default 280px with a resize handle", () => {
    renderWithProviders(<Sidebar />);
    expect(aside().style.width).toBe("280px");
    expect(screen.getByTestId("sidebar-resize-handle")).toBeInTheDocument();
  });

  it("widens on pointer drag, persists, and resets on double-click", () => {
    renderWithProviders(<Sidebar />);
    const handle = screen.getByTestId("sidebar-resize-handle");

    fireEvent.pointerDown(handle, { clientX: 300, pointerId: 1 });
    fireEvent.pointerMove(handle, { clientX: 420, pointerId: 1 });
    expect(aside().style.width).toBe("400px");

    fireEvent.pointerUp(handle, { clientX: 420, pointerId: 1 });
    expect(window.localStorage.getItem(SIDEBAR_WIDTH_STORAGE_KEY)).toBe("400");

    fireEvent.doubleClick(handle);
    expect(aside().style.width).toBe("280px");
    expect(window.localStorage.getItem(SIDEBAR_WIDTH_STORAGE_KEY)).toBe("280");
  });
});
