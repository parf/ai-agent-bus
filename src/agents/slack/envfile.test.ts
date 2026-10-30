import { describe, expect, test } from "bun:test";
import { parseEnvList } from "../shared/envfile.ts";
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

// Synchronous parsing cannot be interrupted by a timer on its own event loop.
// Measure CPU in a child, and keep its watchdog in the parent test process.
const parserCpuProbe = `
  const { parseEnvList } = await import(${JSON.stringify(new URL("../shared/envfile.ts", import.meta.url).href)});
  const { name, line } = JSON.parse(await Bun.stdin.text());
  const took = n => {
    const src = name === "unclosed line-start quotes" ? "A=" + line.repeat(n) + "y\\n" : line.repeat(n);
    let best = Infinity;
    for (let i = 0; i < 5; i++) {
      const start = process.cpuUsage();
      parseEnvList("bus secret", src);
      const used = process.cpuUsage(start);
      best = Math.min(best, used.user + used.system);
    }
    return best;
  };
  console.log(JSON.stringify({ small: took(4 << 10), large: took(32 << 10) }));
`;

async function parserCpuTimes(name: string, line: string) {
  const proc = Bun.spawn([process.execPath, "--eval", parserCpuProbe], {
    stdin: "pipe", stdout: "pipe", stderr: "pipe",
  });
  const output = new Response(proc.stdout).text();
  const diagnostic = new Response(proc.stderr).text();
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    proc.stdin.write(JSON.stringify({ name, line }));
    proc.stdin.end();
    const result = await Promise.race([
      proc.exited.then(code => ({ code, expired: false })),
      new Promise<{ code: null; expired: true }>(resolve => {
        timer = setTimeout(() => resolve({ code: null, expired: true }), 2_000);
      }),
    ]);
    if (result.expired) {
      proc.kill("SIGKILL");
      await proc.exited;
    }
    expect(result.expired, `${name}: parser must finish within its 2000 ms inner deadline`).toBe(false);
    expect(result.code, `${name}: parser child failed: ${await diagnostic}`).toBe(0);
    return JSON.parse(await output) as { small: number; large: number };
  } finally {
    clearTimeout(timer);
    if (proc.exitCode === null) proc.kill("SIGKILL");
    await proc.exited;
    await Promise.all([output, diagnostic]);
  }
}

// Eight times the input may cost about eight times the CPU time. The old
// unquote regex and repeated-prefix line counting were super-linear.
describe("a large secret", () => {
  for (const [name, line] of Object.entries({
    "unclosed line-start quotes": "'x ",
    "many assignments": "A=b\n",
    "unclosed opening quotes": "A='x\n",
  })) test(`${name} parse in linear time`, async () => {
    const { small, large } = await parserCpuTimes(name, line);
    expect(large).toBeLessThan(24 * small);
  }, 10_000); // Outer harness bound exceeds the independent child watchdog.
});
