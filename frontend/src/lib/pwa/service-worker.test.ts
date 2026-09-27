import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { usePwaStore } from "@/store/usePwaStore";

import {
  activateUpdate,
  registerServiceWorker,
  resetServiceWorkerState,
} from "./service-worker";

function fakeRegistration(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    waiting: null,
    installing: null,
    addEventListener: vi.fn(),
    update: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  };
}

let controllerListener: (() => void) | null = null;

function stubNavigator(register: ReturnType<typeof vi.fn>, controller: unknown) {
  vi.stubGlobal("navigator", {
    serviceWorker: {
      controller,
      register,
      addEventListener: (_type: string, listener: () => void) => {
        controllerListener = listener;
      },
    },
  });
}

beforeEach(() => {
  resetServiceWorkerState();
  controllerListener = null;
  usePwaStore.setState({ updateReady: false });
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("service worker update lifecycle", () => {
  it("detects a waiting update and activates it on request", async () => {
    const postMessage = vi.fn();
    const registration = fakeRegistration({
      waiting: { postMessage },
    });
    stubNavigator(vi.fn().mockResolvedValue(registration), {});

    registerServiceWorker();
    await vi.waitFor(() =>
      expect(usePwaStore.getState().updateReady).toBe(true),
    );

    expect(activateUpdate()).toBe(true);
    expect(postMessage).toHaveBeenCalledWith({ type: "SKIP_WAITING" });
  });

  it("does not report an update for the first install", async () => {
    const registration = fakeRegistration();
    stubNavigator(vi.fn().mockResolvedValue(registration), null);

    registerServiceWorker();
    await Promise.resolve();

    expect(usePwaStore.getState().updateReady).toBe(false);
  });

  it("reloads only after the user activates an update", async () => {
    const postMessage = vi.fn();
    const registration = fakeRegistration({ waiting: { postMessage } });
    stubNavigator(vi.fn().mockResolvedValue(registration), {});

    registerServiceWorker();
    await vi.waitFor(() =>
      expect(usePwaStore.getState().updateReady).toBe(true),
    );

    const reload = vi.fn();
    Object.defineProperty(window, "location", {
      configurable: true,
      value: { reload },
    });

    // A spontaneous controllerchange (no user update) must not reload.
    controllerListener?.();
    expect(reload).not.toHaveBeenCalled();

    activateUpdate();
    controllerListener?.();
    expect(reload).toHaveBeenCalledTimes(1);
  });

  it("degrades gracefully when service workers are unsupported", () => {
    vi.stubGlobal("navigator", {});
    expect(() => registerServiceWorker()).not.toThrow();
  });
});
