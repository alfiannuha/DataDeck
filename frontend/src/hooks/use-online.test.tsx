import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { useOnline } from "./use-online";

function setOnline(online: boolean) {
  Object.defineProperty(window.navigator, "onLine", {
    configurable: true,
    value: online,
  });
  act(() => {
    window.dispatchEvent(new Event(online ? "online" : "offline"));
  });
}

afterEach(() => {
  setOnline(true);
});

describe("useOnline", () => {
  it("reports browser connectivity and reacts to events", () => {
    const { result } = renderHook(() => useOnline());
    expect(result.current).toBe(true);

    setOnline(false);
    expect(result.current).toBe(false);

    setOnline(true);
    expect(result.current).toBe(true);
  });
});
