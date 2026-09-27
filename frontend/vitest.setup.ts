import "@testing-library/jest-dom/vitest";

import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

// Vitest globals are disabled, so React Testing Library cannot auto-register
// its cleanup hook; register it explicitly to isolate component tests.
afterEach(cleanup);
