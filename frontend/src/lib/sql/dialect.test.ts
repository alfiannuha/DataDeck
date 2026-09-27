import { describe, expect, it } from "vitest";

import { languageForDialect } from "./dialect";

describe("languageForDialect", () => {
  it.each(["postgres", "mysql", "sqlite"] as const)(
    "returns language support for %s",
    (dialect) => {
      expect(languageForDialect(dialect)).toBeTruthy();
    },
  );
});
