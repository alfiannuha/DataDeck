import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { resetInstallState } from "@/lib/pwa/install";

import { usePwaInstall } from "./use-pwa-install";

interface DeferredPrompt extends Event {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: "accepted" | "dismissed" }>;
}

function fireBeforeInstallPrompt(outcome: "accepted" | "dismissed" = "accepted") {
  const event = new Event("beforeinstallprompt") as DeferredPrompt;
  event.prompt = vi.fn().mockResolvedValue(undefined);
  event.userChoice = Promise.resolve({ outcome });
  act(() => {
    window.dispatchEvent(event);
  });
  return event;
}

beforeEach(() => {
  resetInstallState();
  vi.stubGlobal("matchMedia", () => ({ matches: false }));
});

afterEach(() => {
  vi.unstubAllGlobals();
  resetInstallState();
});

describe("usePwaInstall", () => {
  it("reports no install affordance when unsupported", () => {
    const { result } = renderHook(() => usePwaInstall());
    expect(result.current.canInstall).toBe(false);
    expect(result.current.standalone).toBe(false);
  });

  it("becomes installable when the browser fires the install prompt", async () => {
    const { result } = renderHook(() => usePwaInstall());

    const event = fireBeforeInstallPrompt("accepted");
    expect(result.current.canInstall).toBe(true);

    let outcome: string | undefined;
    await act(async () => {
      outcome = await result.current.install();
    });

    expect(event.prompt).toHaveBeenCalledTimes(1);
    expect(outcome).toBe("accepted");
    expect(result.current.canInstall).toBe(false);
  });

  it("reports standalone mode and hides the install affordance", () => {
    vi.stubGlobal("matchMedia", (query: string) => ({
      matches: query === "(display-mode: standalone)",
    }));
    const { result } = renderHook(() => usePwaInstall());
    expect(result.current.standalone).toBe(true);
    expect(result.current.canInstall).toBe(false);
  });

  it("returns unavailable when install is requested without a prompt", async () => {
    const { result } = renderHook(() => usePwaInstall());
    await expect(result.current.install()).resolves.toBe("unavailable");
  });
});
