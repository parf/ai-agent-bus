import { expect, test } from "bun:test";
import vectors from "../imap/filter-vectors.json";
import { filterProblem, toRegExp } from "./subs.js";

test("IMAP's shared subset has the same matches as chat filters", () => {
  for (const row of vectors) {
    expect(filterProblem(row.filter)).toBeUndefined();
    expect(toRegExp(row.filter).test(row.text)).toBe(row.match);
  }
});
