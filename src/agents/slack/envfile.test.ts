import { describe, expect, test } from "bun:test";
import { parseEnvList } from "./envfile.ts";
import { endpoint, scrub } from "./slackapi.ts";

import cases from "../../internal/envfile/testdata/common.json";

describe("the shared dotenv conformance table", () => {
  for (const c of cases) test(c.name, () => {
    if ("line" in c) {
      try { parseEnvList("bus secret", c.source); throw new Error("accepted invalid input"); }
      catch (err) {
        expect(String(err)).toContain(`line ${c.line}`);
        expect(String(err)).not.toContain("PRIVATE_SENTINEL");
      }
    } else {
      expect(parseEnvList("bus secret", c.source)).toEqual((c.want ?? []).map(v => ({key:v.Key, value:v.Value})));
    }
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

// The same bound as the Go parser's: eight times the input may cost about
// eight times the time. The upstream unquote regex backtracked from every
// line-start quote, and the line count re-split the text before each match.
describe("a large secret", () => {
  for (const [name, line] of Object.entries({
    "unclosed line-start quotes": "'x ",
    "many assignments": "A=b\n",
    "unclosed opening quotes": "A='x\n",
  })) test(`${name} parse in linear time`, () => {
    const took = (n: number) => {
      const src = name === "unclosed line-start quotes" ? `A=${line.repeat(n)}y\n` : line.repeat(n);
      let best = Infinity;
      for (let i = 0; i < 3; i++) {
        const start = performance.now();
        parseEnvList("bus secret", src);
        best = Math.min(best, performance.now() - start);
      }
      return best;
    };
    const small = took(2 << 10), large = took(16 << 10);
    expect(large).toBeLessThan(16 * small);
  });
});
