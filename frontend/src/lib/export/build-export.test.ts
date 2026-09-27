import { describe, expect, it } from "vitest";

import type { QueryResult } from "@/types/api";

import { buildExport } from "./build-export";

function result(
  columns: { name: string; type?: string }[],
  rows: unknown[][],
  truncated = false,
): QueryResult {
  return {
    columns: columns.map((c) => ({ name: c.name, type: c.type ?? "text" })),
    rows,
    rows_affected: rows.length,
    execution_time_ms: 1,
    truncated,
  } as QueryResult;
}

const opts = { timestamp: new Date(2026, 8, 27, 14, 32, 0) };

describe("buildExport CSV", () => {
  it("writes a header and escaped rows", () => {
    const artifact = buildExport(
      result(
        [{ name: "id" }, { name: "name" }, { name: "meta" }],
        [
          ["9007199254740993", "a,b", { role: "admin" }],
          [null, 'say "hi"', null],
        ],
      ),
      { format: "csv", connectionName: "PG", ...opts },
    );

    expect(artifact.content).toBe(
      'id,name,meta\n' +
        '9007199254740993,"a,b","{""role"":""admin""}"\n' +
        ',"say ""hi""",\n',
    );
  });

  it("preserves BIGINT exactly and NULL as empty", () => {
    const artifact = buildExport(
      result([{ name: "big" }, { name: "n" }], [["9007199254740993", null]]),
      { format: "csv", ...opts },
    );
    expect(artifact.content).toContain("9007199254740993");
    expect(artifact.content.split("\n")[1]).toBe("9007199254740993,");
  });

  it("has csv mime and filename", () => {
    const artifact = buildExport(result([{ name: "id" }], [["1"]]), {
      format: "csv",
      connectionName: "PG",
      ...opts,
    });
    expect(artifact.mime).toContain("text/csv");
    expect(artifact.filename).toBe("datadeck_PG_20260927-143200.csv");
  });
});

describe("buildExport JSON", () => {
  it("emits an array of objects, preserving exact values", () => {
    const artifact = buildExport(
      result(
        [{ name: "big" }, { name: "n" }, { name: "meta" }],
        [["9007199254740993", null, { role: "admin" }]],
      ),
      { format: "json", ...opts },
    );
    const parsed = JSON.parse(artifact.content) as Record<string, unknown>[];
    expect(parsed).toEqual([
      { big: "9007199254740993", n: null, meta: { role: "admin" } },
    ]);
    expect(artifact.mime).toContain("application/json");
  });

  it("disambiguates duplicate column names", () => {
    const artifact = buildExport(
      result([{ name: "id" }, { name: "id" }], [["1", "2"]]),
      { format: "json", ...opts },
    );
    expect(JSON.parse(artifact.content)).toEqual([{ id: "1", id_2: "2" }]);
  });
});

describe("buildExport truncation", () => {
  it("carries the truncated flag and subset", () => {
    const artifact = buildExport(
      result([{ name: "n" }], [["1"], ["2"]], true),
      { format: "csv", ...opts },
    );
    expect(artifact.truncated).toBe(true);
    expect(artifact.rowCount).toBe(2);
  });

  it("reports non-truncated results", () => {
    const artifact = buildExport(result([{ name: "n" }], [["1"]]), {
      format: "json",
      ...opts,
    });
    expect(artifact.truncated).toBe(false);
  });
});

describe("buildExport CSV formula safety", () => {
  it("prefixes formula-sensitive cells", () => {
    const artifact = buildExport(
      result([{ name: "v" }], [["=1+1"], ["-5"]]),
      { format: "csv", ...opts },
    );
    const lines = artifact.content.split("\n");
    expect(lines[1]).toBe("'=1+1");
    expect(lines[2]).toBe("-5");
  });
});

describe("buildExport performance", () => {
  it("serializes a large result in one pass", () => {
    const rows = Array.from({ length: 50_000 }, (_, i) => [String(i), `v,${i}`]);
    const artifact = buildExport(
      result([{ name: "id" }, { name: "v" }], rows),
      { format: "csv", ...opts },
    );
    expect(artifact.rowCount).toBe(50_000);
    expect(artifact.content.split("\n")[50_000]).toBe('49999,"v,49999"');
  });
});

describe("buildExport JSON shape", () => {
  it("uses column names as object keys", () => {
    const artifact = buildExport(
      result(
        [{ name: "id" }, { name: "name" }],
        [["9007199254740993", "Example"]],
      ),
      { format: "json", ...opts },
    );
    expect(JSON.parse(artifact.content)).toEqual([
      { id: "9007199254740993", name: "Example" },
    ]);
  });

  it("preserves nested JSON and binary representation", () => {
    const artifact = buildExport(
      result(
        [{ name: "meta" }, { name: "blob" }],
        [[{ tags: ["a", "b"], n: 2 }, "AAECAwQ="]],
      ),
      { format: "json", ...opts },
    );
    expect(JSON.parse(artifact.content)).toEqual([
      { meta: { tags: ["a", "b"], n: 2 }, blob: "AAECAwQ=" },
    ]);
  });

  it("emits an empty array for zero rows", () => {
    const artifact = buildExport(result([{ name: "id" }], []), {
      format: "json",
      ...opts,
    });
    expect(artifact.content).toBe("[]");
    expect(artifact.rowCount).toBe(0);
  });

  it("uses the shared json filename and mime", () => {
    const artifact = buildExport(result([{ name: "id" }], [["1"]]), {
      format: "json",
      connectionName: "PG",
      ...opts,
    });
    expect(artifact.filename).toBe("datadeck_PG_20260927-143200.json");
    expect(artifact.mime).toContain("application/json");
  });
});
