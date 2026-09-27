import { render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ServiceWorkerRegistration } from "./service-worker-registration";

function fakeRegistration() {
  return {
    waiting: null,
    installing: null,
    addEventListener: vi.fn(),
    update: vi.fn().mockResolvedValue(undefined),
  };
}

afterEach(() => {
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
});

describe("ServiceWorkerRegistration", () => {
  it("registers the service worker in production", async () => {
    vi.stubEnv("NODE_ENV", "production");
    const register = vi.fn().mockResolvedValue(fakeRegistration());
    vi.stubGlobal("navigator", {
      serviceWorker: { register, controller: {}, addEventListener: vi.fn() },
    });

    render(<ServiceWorkerRegistration />);

    await vi.waitFor(() => expect(register).toHaveBeenCalledWith("/sw.js"));
  });

  it("does not register outside production", () => {
    vi.stubEnv("NODE_ENV", "test");
    const register = vi.fn().mockResolvedValue(fakeRegistration());
    vi.stubGlobal("navigator", {
      serviceWorker: { register, controller: {}, addEventListener: vi.fn() },
    });

    render(<ServiceWorkerRegistration />);

    expect(register).not.toHaveBeenCalled();
  });

  it("degrades gracefully when the browser has no service worker support", () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubGlobal("navigator", {});

    expect(() => render(<ServiceWorkerRegistration />)).not.toThrow();
  });
});
