import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { OfflineBanner } from "./offline-banner";

function setOnline(online: boolean) {
  Object.defineProperty(window.navigator, "onLine", {
    configurable: true,
    value: online,
  });
  fireEvent(window, new Event(online ? "online" : "offline"));
}

afterEach(() => setOnline(true));

describe("OfflineBanner", () => {
  it("is hidden while online", () => {
    setOnline(true);
    render(<OfflineBanner />);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("explains that only the cached shell is available offline", () => {
    setOnline(false);
    render(<OfflineBanner />);
    const banner = screen.getByRole("status");
    expect(banner).toHaveTextContent("Backend unavailable");
    expect(banner).toHaveTextContent(/cached app shell/);
    expect(banner).toHaveTextContent(/editor content is preserved/);
  });

  it("disappears when connectivity returns", () => {
    setOnline(false);
    render(<OfflineBanner />);
    expect(screen.getByRole("status")).toBeInTheDocument();

    setOnline(true);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });
});
