import { describe, expect, test } from "bun:test";
import { parseEnvList } from "./envfile.ts";

// The secret's dotenv rules: no expansion anywhere, quotes as written, and
// errors that name their line (Plans/R1.2/slack/README.md#file-formats).
describe("the dotenv parser", () => {
  test("plain values, comments and export", () => {
    const out = parseEnvList("s", "# a comment\nA=1\nexport B=two words\n\nC=value # trailing\n");
    expect(out).toEqual([{ key: "A", value: "1" }, { key: "B", value: "two words" }, { key: "C", value: "value" }]);
  });

  test("single quotes are literal, double quotes escape", () => {
    const out = parseEnvList("s", `A='$x # not a comment'\nB="a\\nb\\t\\"c\\\\d"\nC=""\n`);
    expect(out).toEqual([
      { key: "A", value: "$x # not a comment" },
      { key: "B", value: "a\nb\t\"c\\d" },
      { key: "C", value: "" },
    ]);
  });

  test("errors name the line and the problem", () => {
    expect(() => parseEnvList("the bus secret", "A=1\nno equals here\n")).toThrow("the bus secret:2");
    expect(() => parseEnvList("s", "A-B=1\n")).toThrow("is not a variable name");
    expect(() => parseEnvList("s", "A='unclosed\n")).toThrow("the ' is never closed");
    expect(() => parseEnvList("s", "A=\"unclosed\n")).toThrow('the " is never closed');
  });
});
