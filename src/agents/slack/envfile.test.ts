import { describe, expect, test } from "bun:test";
import { parseEnvList } from "./envfile.ts";
import { endpoint, scrub } from "./slackapi.ts";

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

  test("quoted values may carry a trailing comment, and junk after the quote is refused", () => {
    expect(parseEnvList("s", `A="token" # comment\n`)).toEqual([{ key: "A", value: "token" }]);
    expect(parseEnvList("s", `B='lit'   # c\n`)).toEqual([{ key: "B", value: "lit" }]);
    expect(() => parseEnvList("s", `A="abc"junk"\n`)).toThrow("junk after the closing quote");
    expect(() => parseEnvList("s", `A='abc' junk\n`)).toThrow("junk after the closing quote");
  });

  test("unknown escapes keep their bytes; an escaped quote does not close; a missing close is refused", () => {
    // The file holds A="abc\\q" (one escaped backslash, then q): \\ is one
    // backslash, q survives, so the value is abc\q — no byte silently dropped.
    expect(parseEnvList("s", `A="abc\\\\q"\n`)).toEqual([{ key: "A", value: "abc\\q" }]);
    // The file holds A="abc\q": an unknown escape keeps both bytes as written.
    expect(parseEnvList("s", `A="abc\\q"\n`)).toEqual([{ key: "A", value: "abc\\q" }]);
    expect(parseEnvList("s", `B="say \\"hi\\" now"\n`)).toEqual([{ key: "B", value: 'say "hi" now' }]);
    // The file holds A="abc\" — the quote is escaped, so nothing closes it.
    expect(() => parseEnvList("s", `A="abc\\"`)).toThrow('the " is never closed');
  });  test("errors name the line and the problem", () => {
    expect(() => parseEnvList("the bus secret", "A=1\nno equals here\n")).toThrow("the bus secret:2");
    expect(() => parseEnvList("s", "A-B=1\n")).toThrow("is not a variable name");
    expect(() => parseEnvList("s", "A='unclosed\n")).toThrow("the ' is never closed");
    expect(() => parseEnvList("s", "A=\"unclosed\n")).toThrow('the " is never closed');
  });
});

describe("the endpoint and scrubbing", () => {
  test("the endpoint is the secret's URL or Slack's own, never the environment", () => {
    expect(endpoint(undefined)).toBe("https://slack.com/api");
    expect(endpoint("http://127.0.0.1:9/api/")).toBe("http://127.0.0.1:9/api");
  });

  test("scrub removes the agent's own credential bytes", () => {
    expect(scrub("diagnostic Bearer xoxb-abcdef123456 end", "xoxb-abcdef123456")).toBe("diagnostic Bearer [redacted] end");
    expect(scrub("no secrets here", "xoxb-abcdef123456")).toBe("no secrets here");
  });
});
