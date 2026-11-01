import { afterAll, beforeAll, expect, test } from "bun:test";
import { makeHandler } from "../server.ts";

// A record's big keys on its store page, and their delete form, against a fake daemon.
const you = "owner@fixture", record = "jobs@fixture";
const hex = "ab".repeat(32);
let daemon: ReturnType<typeof Bun.serve>;
let handle: (req: Request) => Promise<Response>;
let listing: () => Response = () => Response.json({ record, keys: [] });
const calls: { method: string; path: string; body: string }[] = [];

beforeAll(() => {
  daemon = Bun.serve({ hostname: "127.0.0.1", port: 0, async fetch(req) {
    const path = new URL(req.url).pathname;
    calls.push({ method: req.method, path, body: await req.text() });
    switch (path) {
      case "/identity": return Response.json({ version: "fixture", host: "fixture" });
      case "/status": return Response.json({ you });
      case "/kv/list": return Response.json({ record, kind: "queue", values: { string: [], int: [], json: [] } });
      case "/big/list": return listing();
      case "/big/delete": return Response.json({ record, name: "report", deleted: true });
      default: return new Response("unexpected path", { status: 404 });
    }
  } });
  handle = makeHandler({ daemon: daemon.url.toString(), listen: { hostname: "127.0.0.1", port: 0 }, dev: false });
});
afterAll(() => daemon?.stop(true));

const cookie = { cookie: "agent_bus_session=deadbeef" };
const store = async () => {
  const r = await handle(new Request(`http://web.fixture/kv/record?name=${encodeURIComponent(record)}`, { headers: cookie }));
  expect(r.status).toBe(200);
  return r.text();
};
const card = (html: string) => /<section[^>]*id="kv-big"[\s\S]*?<\/section>/.exec(html)?.[0] ?? "";

test("the store page lists a record's big keys without their contents", async () => {
  listing = () => Response.json({ record, keys: [
    { name: "report", size: 3 << 20, sha256: hex, held: "memory", expires: new Date(Date.now() + 90 * 60_000).toISOString() },
    { name: "tiny", size: 12, sha256: "cd".repeat(32), held: "file", expires: new Date(Date.now() + 3 * 86400_000).toISOString() }] });
  const c = card(await store());
  expect(c).toContain("Big keys");
  expect(c).toContain("<code>report</code>");
  expect(c).toContain("3.0 MiB");
  expect(c).toContain(`title="${hex}"`);
  expect(c).toContain(">memory<");
  expect(c).toMatch(/>1h 2\dm<|>1h 3\dm<|>1h 29m</);
  expect(c).toContain("<code>tiny</code>");
  expect(c).toContain("12 B");
  expect(c).toMatch(/>2d 2\dh<|>3d 0h<|>2d 23h</);
  expect(c).toContain('action="/big-delete"');
  expect(c).toContain(">abababababab…<");
  expect(c).not.toContain(`>${hex}`);
});

test("a record with no big keys, or one the visitor may not list, shows no card", async () => {
  listing = () => Response.json({ record, keys: [] });
  expect(card(await store())).toBe("");
  listing = () => new Response('acl: not admitted', { status: 403 });
  expect(card(await store())).toBe("");
});

test("Delete posts the record and name to the daemon and returns to the store with a notice", async () => {
  calls.length = 0;
  const origin = "http://web.fixture";
  const r = await handle(new Request(origin + "/big-delete", { method: "POST", redirect: "manual",
    headers: { ...cookie, origin, host: "web.fixture", "content-type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({ record, name: "report" }).toString() }));
  expect(r.status).toBe(303);
  expect(r.headers.get("location")).toBe(`/kv/record?name=${encodeURIComponent(record)}`);
  expect(r.headers.get("set-cookie")).toContain("ab_flash=big-deleted");
  const del = calls.find(c => c.path === "/big/delete");
  expect(del?.method).toBe("POST");
  expect(JSON.parse(del!.body)).toEqual({ record, name: "report" });
});

test("a daemon whose big-key store is unavailable shows its cause instead of hiding the card", async () => {
  listing = () => Response.json({ error: "the big-key store directory cannot be opened" }, { status: 503 });
  const c = card(await store());
  expect(c).toContain("Big keys");
  expect(c).toContain('<p class="warn">the big-key store directory cannot be opened</p>');
});
