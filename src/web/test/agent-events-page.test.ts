// Real page handlers and Ctx.call transport against a private #bus fixture.
import { afterAll, beforeAll, expect, test } from "bun:test";
import { makeHandler } from "../server.ts";
import type { EventEntry, EventLog } from "../agent-events.tsx";

const A = "#one@fixture", B = "#two@fixture";
const owner = "aaaaaaaa", maintainer = "bbbbbbbb", daemonOwner = "cccccccc", reader = "dddddddd";
const now = "2026-11-02T10:00:00.000Z";
const logs = new Map<string, EventLog>();
for (const name of [A, B]) {
  const entries = [1, 2, 3, 4, 5].map(id => ({ id, level: id < 3 ? "alert" : id === 3 ? "warning" : "notice", text: `${name}-private-${id}`, at: now, last_at: now, count: 1 })) as EventEntry[];
  logs.set(name, { log: name, rings: { alert: entries.filter(e => e.level === "alert").reverse(), warning: entries.filter(e => e.level === "warning"), notice: entries.filter(e => e.level === "notice").reverse() }, counts: { alert: 2, warning: 1, notice: 2 } });
}
let allowed = true;
let wrongLog = false;
let sizeRefusal = false;
let inactive = false;
let absent = false;
let listHttpStatus = 0;
const calls: { cred: string; body: string; to: string }[] = [];
const pending = new Map<string, unknown>();
let daemon: ReturnType<typeof Bun.serve>;
let handle: ReturnType<typeof makeHandler>;
beforeAll(() => {
  daemon = Bun.serve({ hostname: "127.0.0.1", port: 0, async fetch(req) {
    const url = new URL(req.url), cred = req.headers.get("X-Agent-Bus-Token") ?? "";
    const canRead = cred === daemonOwner || (allowed && (cred === owner || cred === maintainer));
    const rec = (name: string) => ({ name, kind: "agent", owner: "owner@fixture", can_manage: cred === owner || cred === maintainer, readers: 0 });
    if (url.pathname === "/send") {
      const e = await req.json() as { to: string; body: string; tag: string };
      calls.push({ cred, body: e.body, to: e.to });
      if (e.body === "list" && listHttpStatus) return Response.json({error:"fixture"}, {status:listHttpStatus});
      expect(e.to).toBe("#bus");
      expect(e.body === "list" || e.body === `read ${A}` || e.body === `read ${B}`).toBe(true);
      const answer = !canRead ? { error: { code: "forbidden", text: "private" } }
        : absent ? { error: { code: "not_found", text: "no log" } }
        : sizeRefusal ? { error: { code: "internal_error", text: "size" } }
        : e.body === "list" ? { now, logs: [...logs.values()].map(l => ({ log: l.log, counts: l.counts, latest_alert_at: now })) }
        : logs.get(wrongLog ? B : e.body.slice(5));
      pending.set(e.tag, answer);
      return Response.json({ id: "fixture" });
    }
    if (url.pathname === "/consume") {
      const tag = url.searchParams.get("tag")!;
      const answer = pending.get(tag); pending.delete(tag);
      return Response.json({ body: JSON.stringify(answer) });
    }
    const routes: Record<string, unknown> = {
      "/identity": { version: "fixture", owner: "daemon@fixture" },
      "/status": { you: cred === daemonOwner ? "daemon@fixture" : cred === maintainer ? "maintainer@fixture" : cred === reader ? "reader@fixture" : "owner@fixture", daemon_owner: cred === daemonOwner },
      "/lookup": rec(url.searchParams.get("name")!),
      "/ls": [rec(A), rec(B)], "/inactive": inactive ? [rec(A)] : [],
      "/users": [], "/names": [], "/groups": {}, "/activity": [], "/activity/days": [],
      "/holders": { locks: {} }, "/config": {},
      "/kv/list": { record: A, kind: "agent", values: { string: [], int: [], json: [] } },
    };
    return url.pathname in routes ? Response.json(routes[url.pathname]) : Response.json({ error: "no fixture route" }, { status: 404 });
  } });
  handle = makeHandler({ daemon: daemon.url.toString(), listen: { hostname: "127.0.0.1", port: 0 }, dev: false });
});
afterAll(() => daemon?.stop(true));
async function page(name = A, expanded = false, cred = owner) {
  const url = new URL("/agent", "http://web.fixture"); url.searchParams.set("name", name);
  if (expanded) url.searchParams.set("events", "all");
  const response = await handle(new Request(url, { headers: { cookie: `agent_bus_session=${cred}` } }));
  expect(response.status).toBe(200);
  return response.text();
}
const ids = (html: string) => [...html.matchAll(/data-event-id="(\d+)"/g)].map(m => Number(m[1]));

test("agent page preview and expand read the selected agent anew as the visitor, never another agent", async () => {
  calls.length = 0;
  const preview = await page();
  expect(ids(preview)).toEqual([5, 4, 3]);
  expect(preview).toContain("Showing 3 of 5 retained entries.");
  expect(preview).not.toContain(`${B}-private`);
  const expanded = await page(A, true);
  expect(ids(expanded)).toEqual([2, 1, 3, 5, 4]);
  expect(expanded).toContain("Showing 5 of 5 retained entries.");
  expect(expanded).toContain("Full retained log");
  expect(calls).toEqual([{ cred: owner, body: `read ${A}`, to: "#bus" }, { cred: owner, body: `read ${A}`, to: "#bus" }]);
  const other = await page(B);
  expect(other).toContain(`${B}-private-5`); expect(other).not.toContain(`${A}-private`);
  wrongLog = true;
  try {
    const crossed = await page(A, true);
    expect(crossed).toContain('<p class="warn" data-events-unavailable>');
    expect(crossed).not.toContain(`${B}-private`);
  } finally { wrongLog = false; }
});
test("direct preview and expand recheck access, including revocation and daemon Owner", async () => {
  expect(ids(await page(A, true, maintainer))).toHaveLength(5);
  for (const expanded of [false, true]) {
    const refused = await page(A, expanded, reader);
    expect(refused).toContain('<p data-events-not-allowed>Not allowed');
    expect(refused).not.toContain(`${A}-private`); expect(refused).not.toContain("No retained events");
  }
  allowed = false;
  try {
    const revoked = await page(A, true, owner);
    expect(revoked).toContain('<p data-events-not-allowed>'); expect(ids(revoked)).toHaveLength(0);
    expect(ids(await page(A, true, daemonOwner))).toHaveLength(5);
  } finally { allowed = true; }
});
test("inactive agent events also recheck access and size refusal is not an empty log", async () => {
  inactive = true;
  try {
    expect(ids(await page(A, true))).toHaveLength(5);
    expect(await page(A, true, reader)).toContain('<p data-events-not-allowed>');
  } finally { inactive = false; }
  sizeRefusal = true;
  try {
    const refused = await page(A, true);
    expect(refused).toContain('data-events-unavailable'); expect(refused).not.toContain("No retained events");
  } finally { sizeRefusal = false; }
});
test("agent list makes one summary-only call with no entry bodies or unauthorized marks", async () => {
  calls.length = 0;
  const list = async (cred: string) => (await handle(new Request("http://web.fixture/agents", { headers: { cookie: `agent_bus_session=${cred}` } }))).text();
  const html = await list(owner);
  expect(html.match(/data-event-summary/g)).toHaveLength(2);
  expect(html).toContain('tone-bad">alert in last day'); expect(html).toContain('tone-warn">1 retained warning');
  expect(html).not.toContain("-private-"); expect(calls).toEqual([{ cred: owner, body: "list", to: "#bus" }]);
  const denied = await list(reader);
  expect(denied).not.toContain('data-event-summary'); expect(denied).not.toContain("-private-");
  expect(denied).not.toContain('data-event-summaries-unavailable');
});


test("a missing log is not disclosed before read authority, on preview or direct expand", async () => {
  absent = true;
  try {
    for (const expanded of [false, true]) {
      const authorized = await page(A, expanded, owner);
      expect(authorized).toContain('<p data-events-not-found>No events yet for this agent.</p>');
      const refused = await page(A, expanded, reader);
      expect(refused).toContain('<p data-events-not-allowed>');
      expect(refused).not.toContain('data-events-not-found'); expect(refused).not.toContain('data-events-empty');
    }
  } finally { absent = false; }
});


test("forbidden summaries are quiet while actual list failures have an unavailable element", async () => {
  const list = async () => {
    const url = new URL("http://web.fixture/agents"); url.searchParams.set("q", A);
    const response = await handle(new Request(url, { headers: { cookie: `agent_bus_session=${reader}` } }));
    expect(response.status).toBe(200);
    return response.text();
  };
  for (const status of [403, 503]) {
    listHttpStatus = status;
    try {
      const html = await list();
      // The listing's own row, never the signed-in name or an inline style.
      const body = html.match(/<tbody>([\s\S]*?)<\/tbody>/)?.[1] ?? "";
      expect(body).toMatch(/<tr\b[^>]*>[\s\S]*?<a href="[^"]*"><strong><code>#one@fixture<\/code><\/strong><\/a>/);
      expect(html).not.toContain('data-event-summary');
      if (status === 403) expect(html).not.toContain('data-event-summaries-unavailable');
      else expect(html).toContain('<p class="muted small" data-event-summaries-unavailable>Event summaries unavailable.</p>');
    } finally { listHttpStatus = 0; }
  }
});
