import { afterAll, beforeAll, expect, test } from "bun:test";
import { makeHandler } from "../server.ts";
import { PAGE_SIZE } from "../pagination.ts";
import type { Rec } from "../ctx.ts";

// The real page handlers and HTTP daemon client, over a read-only directory
// fixture. Five thousand owned records reproduce the measured growing pages.
const you = "owner@fixture";
const records: Rec[] = Array.from({ length: 5000 }, (_, i) => ({
  name: `#worker${String(i).padStart(4, "0")}@fixture`, kind: "agent", owner: you,
  descr: `Worker ${i}`,
}));
const foreign: Rec = { name: "#foreign@fixture", kind: "agent", owner: "other@fixture" };
let daemon: ReturnType<typeof Bun.serve>;
let handle: (req: Request) => Promise<Response>;
const calls: string[] = [];

beforeAll(() => {
  daemon = Bun.serve({ hostname: "127.0.0.1", port: 0, fetch(req) {
    const path = new URL(req.url).pathname;
    calls.push(`${req.method} ${path}`);
    if (req.method !== "GET") return new Response("writes forbidden", { status: 405 });
    const body: Record<string, unknown> = {
      "/identity": { version: "fixture", host: "fixture" },
      "/status": { you, administrator: true, daemon_owner: true },
      "/users": [{ name: you, kind: "user", daemon_owner: true, status: "active", records: records.map(r => r.name) }],
      "/ls": [...records.slice(0, -1), foreign],
      "/inactive": [records.at(-1)],
      "/names": [],
      "/kv/list": { record: you, kind: "user", values: { string: [], int: [], json: [] } },
    };
    return path in body ? Response.json(body[path]) : new Response("unexpected path", { status: 404 });
  } });
  handle = makeHandler({ daemon: daemon.url.toString(), listen: { hostname: "127.0.0.1", port: 0 }, dev: false });
});
afterAll(() => daemon?.stop(true));

const paths = ["/account", `/user?name=${encodeURIComponent(you)}&return=%2Fusers%3Fq%3Downer`];
async function page(path: string, query: Record<string, string> = {}) {
  calls.length = 0;
  const url = new URL(path, "http://web.fixture");
  for (const [key, value] of Object.entries(query)) url.searchParams.set(key, value);
  const response = await handle(new Request(url, { headers: { cookie: "agent_bus_session=deadbeef" } }));
  expect(response.status).toBe(200);
  const html = await response.text();
  expect(calls.every(call => call.startsWith("GET "))).toBe(true);
  expect(calls.filter(call => call === "GET /ls")).toHaveLength(1);
  return html;
}
const rows = (html: string) => /<ul class="subs-list">([\s\S]*?)<\/ul>/.exec(html)?.[1].match(/<li>/g)?.length ?? 0;

for (const path of paths) {
  test(`${path.split("?")[0]} stays below 100 KB with 5,000 owned records`, async () => {
    const html = await page(path);
    expect(new TextEncoder().encode(html).byteLength).toBeLessThan(100_000);
    expect(rows(html)).toBe(PAGE_SIZE);
    expect(html).toContain("Page 1 of 50");
    expect(html).toContain('rel="next"');
  });
}

test("owned-record search reaches a record beyond page one and preserves User navigation", async () => {
  const name = records[PAGE_SIZE + 5]!.name;
  for (const path of paths) {
    const second = await page(path, { page: "2" });
    expect(second).toContain(`<code>${name}</code>`);
    expect(rows(second)).toBe(PAGE_SIZE);
    expect(second).toContain('rel="prev"');
    const found = await page(path, { q: name.toUpperCase() });
    expect(found).toContain(`<code>${name}</code>`);
    expect(rows(found)).toBe(1);
    expect(found).toContain('name="q"');
    if (path.startsWith("/user")) {
      expect(found).toContain(`name="name" value="${you}"`);
      expect(found).toContain('name="return" value="/users?q=owner"');
      expect(second).toContain("name=owner%40fixture&amp;return=%2Fusers%3Fq%3Downer&amp;page=3");
    }
    const inactive = await page(path, { q: records.at(-1)!.name });
    expect(rows(inactive)).toBe(1);
    expect(inactive).toContain("INACTIVE");
    const missing = await page(path, { q: foreign.name });
    expect(rows(missing)).toBe(0);
    expect(missing).toContain("No owned record matches this search");
    const filteredPage = await page(path, { q: "worker", page: "2" });
    expect(filteredPage).toContain("q=worker&amp;page=3");
    const outOfRange = await page(path, { q: name, page: "500" });
    expect(outOfRange).toContain("Page 1 of 1");
    expect(rows(outOfRange)).toBe(1);
  }
});
