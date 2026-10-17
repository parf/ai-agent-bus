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
