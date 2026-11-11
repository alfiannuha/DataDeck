import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import {
  SIDEBAR_DEFAULT_WIDTH,
  SIDEBAR_MAX_WIDTH,
  SIDEBAR_MIN_WIDTH,
  SIDEBAR_WIDTH_STORAGE_KEY,
  clampSidebarWidth,
  useResizableSidebar,
} from "./use-resizable-sidebar";

beforeEach(() => {
  window.localStorage.clear();
});

describe("clampSidebarWidth", () => {
  it("clamps to the minimum", () => {
    expect(clampSidebarWidth(100)).toBe(SIDEBAR_MIN_WIDTH);
  });
  it("clamps to the maximum", () => {
    expect(clampSidebarWidth(9999, 4000)).toBe(SIDEBAR_MAX_WIDTH);
  });
  it("keeps valid widths", () => {
    expect(clampSidebarWidth(360)).toBe(360);
  });
  it("respects the viewport (leaves room for the workspace)", () => {
    // 600px viewport - 320px reserved = 280px upper bound.
    expect(clampSidebarWidth(520, 600)).toBe(280);
  });
});

describe("useResizableSidebar", () => {
  it("defaults to 280px", () => {
    const { result } = renderHook(() => useResizableSidebar());
    expect(result.current.width).toBe(SIDEBAR_DEFAULT_WIDTH);
  });

  it("widens and narrows on drag", () => {
    const { result } = renderHook(() => useResizableSidebar());
    act(() => result.current.beginResize(300));
    act(() => result.current.moveResize(400));
    expect(result.current.width).toBe(380);
    act(() => result.current.moveResize(350));
    expect(result.current.width).toBe(330);
    act(() => result.current.endResize());
  });

  it("cannot go below the minimum or above the maximum", () => {
    const { result } = renderHook(() => useResizableSidebar());
    act(() => result.current.beginResize(300));
    act(() => result.current.moveResize(0));
    expect(result.current.width).toBe(SIDEBAR_MIN_WIDTH);
    act(() => result.current.moveResize(5000));
    expect(result.current.width).toBe(SIDEBAR_MAX_WIDTH);
    act(() => result.current.endResize());
  });

  it("persists the width on release and resets to 280 on reset", () => {
    const { result } = renderHook(() => useResizableSidebar());
    act(() => result.current.beginResize(300));
    act(() => result.current.moveResize(420));
    act(() => result.current.endResize());
    expect(window.localStorage.getItem(SIDEBAR_WIDTH_STORAGE_KEY)).toBe("400");

    act(() => result.current.resetWidth());
    expect(result.current.width).toBe(SIDEBAR_DEFAULT_WIDTH);
    expect(window.localStorage.getItem(SIDEBAR_WIDTH_STORAGE_KEY)).toBe("280");
  });

  it("restores the persisted width on load", async () => {
    window.localStorage.setItem(SIDEBAR_WIDTH_STORAGE_KEY, "360");
    const { result } = renderHook(() => useResizableSidebar());
    await waitFor(() => expect(result.current.width).toBe(360));
  });
});
