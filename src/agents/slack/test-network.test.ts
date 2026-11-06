import { expect, test } from "bun:test";
import { endpoint as slackEndpoint } from "./slackapi.ts";
import { endpoint as telegramEndpoint } from "../telegram/telegramapi.ts";
import { assertTestLoopback } from "../shared/test-network.ts";

test("the fixture guard rejects every non-loopback API and socket destination", () => {
  const previous = process.env.AGENT_TEST_LOOPBACK_ONLY;
  process.env.AGENT_TEST_LOOPBACK_ONLY = "1";
  try {
    for (const endpoint of [slackEndpoint, telegramEndpoint]) {
      expect(() => endpoint(undefined)).toThrow("test endpoint must be loopback");
      expect(() => endpoint("https://provider.invalid/api")).toThrow("test endpoint must be loopback");
      expect(endpoint("http://127.0.0.1:9/api/")).toBe("http://127.0.0.1:9/api");
    }
    for (const address of ["wss://fixture-team.slack.com", "http://127.0.0.1.evil.invalid", "http://localhost.evil.invalid"])
      expect(() => assertTestLoopback(address)).toThrow("test endpoint must be loopback");
    for (const address of ["ws://127.0.0.1:9", "http://[::1]:9", "http://localhost:9"])
      expect(() => assertTestLoopback(address)).not.toThrow();
    delete process.env.AGENT_TEST_LOOPBACK_ONLY;
    expect(slackEndpoint(undefined)).toBe("https://slack.com/api");
    expect(telegramEndpoint(undefined)).toBe("https://api.telegram.org");
    expect(() => assertTestLoopback("wss://provider.invalid")).not.toThrow();
  } finally {
    if (previous === undefined) delete process.env.AGENT_TEST_LOOPBACK_ONLY;
    else process.env.AGENT_TEST_LOOPBACK_ONLY = previous;
  }
});
