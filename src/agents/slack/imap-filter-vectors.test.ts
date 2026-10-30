import { expect, test } from "bun:test";
import vectors from "../imap/filter-vectors.json";
import { compileFilter } from "../shared/subscriptions.ts";

test("IMAP's shared subset has the same matches as chat filters", () => {
  for (const row of vectors) {
    if (["[a&&b]", "[a||b]", "[~~]", "[--a]"].includes(row.filter)) {
      expect(() => compileFilter(row.filter)).toThrow("class set operations are unsupported");
      continue;
    }
    expect((compileFilter(row.filter) ?? /(?:)/).test(row.text)).toBe(row.match);
  }
});
