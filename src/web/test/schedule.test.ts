import { afterAll, beforeAll, beforeEach, expect, test } from "bun:test";
import { makeHandler } from "../server.ts";

// The Crons and Ats pages, a schedule's own page, its forms and the record
// card, against a fake daemon; the KV page's tag filter beside them.
const you = "owner@fixture", A = "#nightly@fixture", Q = "jobs@fixture", P = "news@fixture";
const at = (h: number) => `2026-11-12T${String(h).padStart(2, "0")}:00:00Z`;
const base = { target: Q, body: "report", creator: you, runs: 0, sent: 0, dropped: 0, created: at(1), changed: at(1) };
const schedules: { id: string; holder: string; enabled: boolean; [field: string]: unknown }[] = [
  { ...base, id: "c-ok", holder: A, holder_kind: "agent", kind: "recurring", expr: "0 9 * * 1-5", enabled: true, next: at(9), last: at(8), last_outcome: "sent", runs: 3, sent: 3 },
  { ...base, id: "c-failed", holder: Q, holder_kind: "queue", kind: "recurring", expr: "*/5 * * * *", enabled: false, disabled: "failure", last: at(7), last_outcome: "admission_refused", runs: 5, dropped: 5, refused_in_a_row: 5 },
  { ...base, id: "c-hand", holder: P, holder_kind: "pubsub", kind: "recurring", expr: "0 0 * * *", enabled: false, disabled: "hand", until: "2026-12-31T00:00:00Z", max_runs: 4, max_late: "5m0s" },
  { ...base, id: "c-gone", holder: A, holder_kind: "agent", kind: "recurring", expr: "0 12 * * *", enabled: true, next: at(12), last: at(6), last_outcome: "target_gone", runs: 1, dropped: 1 },
  { ...base, id: "o-1", holder: A, holder_kind: "agent", kind: "once", at: at(15), enabled: true, next: at(15) },
];
const upcoming = [9, 10, 11, 12, 13].map(at);
const outcomes: Record<string, { at: string; slot: string; outcome: string; detail?: string }[]> = {
  "c-ok": [{ at: at(8), slot: at(8), outcome: "sent" }],
  "c-failed": [{ at: at(7), slot: at(7), outcome: "admission_refused", detail: "not admitted" }],
  "c-gone": [{ at: at(6), slot: at(6), outcome: "target_gone" }],
};
const records = [
  { name: A, kind: "agent", owner: you, tags: ["ops"], can_manage: true },
  { name: Q, kind: "queue", owner: you, tags: ["ops", "billing"], can_manage: true },
  { name: P, kind: "pubsub", owner: you, can_manage: true },
];
let daemon: ReturnType<typeof Bun.serve>;
let handle: (req: Request) => Promise<Response>;
let refuseAdd: { status: number; error: string } | undefined;
let warning = "";
const calls: { method: string; path: string; query: string; body: string }[] = [];

beforeAll(() => {
  daemon = Bun.serve({ hostname: "127.0.0.1", port: 0, async fetch(req) {
    const url = new URL(req.url), path = url.pathname;
    const body = await req.text();
    calls.push({ method: req.method, path, query: url.search, body });
    const record = url.searchParams.get("record");
    switch (path) {
      case "/identity": return Response.json({ version: "fixture", host: "fixture" });
      case "/status": return Response.json({ you });
      case "/ls": return Response.json(records);
      case "/inactive": return Response.json([]);
      case "/users": return Response.json([]);
      case "/holders": return Response.json({ record, locks: {} });
      case "/config": return Response.json(null);
      case "/activity": case "/activity/days": return Response.json([]);
      case "/lookup": return Response.json(records.find(r => r.name === url.searchParams.get("name")));
      case "/kv/list": return record ? Response.json({ record, kind: "queue", values: { string: [], int: [], json: [] } })
        : Response.json([{ record: A, kind: "agent", string: 1, int: 0, json: 0 }, { record: Q, kind: "queue", string: 2, int: 0, json: 0 }, { record: P, kind: "pubsub", string: 0, int: 1, json: 0 }]);
      case "/schedule/list":
        if (record === P) return Response.json({ error: "not found" }, { status: 404 });
        return Response.json({ schedules: record ? schedules.filter(s => s.holder === record) : schedules });
      case "/schedule": {
        const s = schedules.find(x => x.id === url.searchParams.get("id"));
        return s ? Response.json({ ...s, outcomes: outcomes[s.id], upcoming: s.enabled ? upcoming : undefined }) : Response.json({ error: "not found" }, { status: 404 });
      }
      case "/schedule/add":
        if (refuseAdd) return Response.json({ error: refuseAdd.error }, { status: refuseAdd.status });
        return Response.json({ schedule: { ...schedules[0], id: "new-1" }, warning });
      case "/schedule/edit":
        if (refuseAdd) return Response.json({ error: refuseAdd.error }, { status: refuseAdd.status });
        return Response.json({ schedule: schedules[0] });
      case "/schedule/enable": case "/schedule/disable": return Response.json({ schedule: schedules[0] });
      case "/schedule/run": return Response.json({ outcome: { at: at(10), slot: at(10), outcome: "send_refused" } });
      case "/schedule/delete": return Response.json({ id: JSON.parse(body).id, deleted: true });
      default: return Response.json({ error: "unexpected path" }, { status: 404 });
    }
  } });
  handle = makeHandler({ daemon: daemon.url.toString(), listen: { hostname: "127.0.0.1", port: 0 }, dev: false });
});
afterAll(() => daemon?.stop(true));
beforeEach(() => { calls.length = 0; refuseAdd = undefined; warning = ""; });

const origin = "http://web.fixture";
const cookie = { cookie: "agent_bus_session=deadbeef" };
const get = async (path: string, status = 200) => {
  const r = await handle(new Request(origin + path, { headers: cookie }));
  expect(r.status).toBe(status);
  return r.text();
};
const post = (path: string, form: Record<string, string>) => handle(new Request(origin + path, { method: "POST", redirect: "manual",
  headers: { ...cookie, origin, host: "web.fixture", "content-type": "application/x-www-form-urlencoded" }, body: new URLSearchParams(form).toString() }));
const rows = (html: string) => [...html.matchAll(/<tbody>([\s\S]*?)<\/tbody>/g)].flatMap(m => [...m[1]!.matchAll(/<tr\b[^>]*>[\s\S]*?<\/tr>/g)].map(r => r[0]));
const ids = (html: string) => rows(html).map(r => /href="\/schedule\?id=([^"]+)"/.exec(r)?.[1]).filter(Boolean).sort();
const lastPost = (path: string) => { const c = calls.filter(c => c.path === path && c.method === "POST").at(-1); return c ? JSON.parse(c.body) : undefined; };

test("the nav carries Crons and Ats after KV", async () => {
  const t = await get("/crons");
  const kv = t.indexOf('href="/kv"'), crons = t.indexOf('href="/crons"'), ats = t.indexOf('href="/ats"');
  expect(kv).toBeGreaterThan(0);
  expect(crons).toBeGreaterThan(kv);
  expect(ats).toBeGreaterThan(crons);
  expect(t).toMatch(/href="\/crons" class="side-link [^"]*" aria-current="page"/);
});

test("Crons lists only recurring schedules, each row linked to its holder and its own page", async () => {
  const t = await get("/crons");
  expect(ids(t)).toEqual(["c-failed", "c-gone", "c-hand", "c-ok"]);
  const ok = rows(t).find(r => r.includes("c-ok"))!;
  expect(ok).toContain(`href="/agent?name=${encodeURIComponent(A)}"`);
  expect(ok).toContain("<code>0 9 * * 1-5</code>");
  expect(ok).toContain(`<code>${Q}</code>`);
  expect(ok).toContain(`<time datetime="${at(9)}" title="${at(9)}" data-local>2026-11-12 09:00 UTC</time>`);
  expect(ok).toMatch(/<span class="pill tone-ok">sent<\/span>/);
  expect(ok).toContain('<a class="btn btn-sm" href="/schedule?id=c-ok">Open</a>');
  const queue = rows(t).find(r => r.includes("c-failed"))!;
  expect(queue).toContain(`href="/queue?name=${encodeURIComponent(Q)}"`);
  expect(ids(await get("/ats"))).toEqual(["o-1"]);
  // The list carries each last outcome and holder kind: no read per schedule.
  expect(calls.filter(c => c.path === "/schedule")).toEqual([]);
});

test("a holder is linked by the kind the daemon names, even one the visitor's listing lacks", async () => {
  schedules.push({ ...base, id: "c-unlisted", holder: "unlisted@fixture", holder_kind: "queue", kind: "recurring", expr: "0 1 * * *", enabled: true });
  try {
    const row = rows(await get("/crons?q=unlisted")).find(r => r.includes("c-unlisted"))!;
    expect(row).toContain('href="/queue?name=unlisted%40fixture"');
  } finally { schedules.pop(); }
});

test("disabled by failure is red, disabled by hand grey", async () => {
  const t = await get("/crons");
  const failed = rows(t).find(r => r.includes("c-failed"))!, hand = rows(t).find(r => r.includes("c-hand"))!;
  expect(failed).toMatch(/^<tr class="schedule-failed">/);
  expect(failed).toContain('<span class="pill tone-bad"><span class="dot" aria-hidden="true"></span>Disabled by failure</span>');
  expect(hand).toMatch(/^<tr class="inactive-record">/);
  expect(hand).toContain('<span class="pill tone-muted"><span class="dot" aria-hidden="true"></span>Disabled by hand</span>');
});

test("the filters narrow by holder, kind, state, problems and tag, with counts per tag", async () => {
  expect(ids(await get(`/crons?q=jobs`))).toEqual(["c-failed"]);
  expect(ids(await get(`/crons?kind=pubsub`))).toEqual(["c-hand"]);
  expect(ids(await get(`/crons?state=failure`))).toEqual(["c-failed"]);
  expect(ids(await get(`/crons?state=hand`))).toEqual(["c-hand"]);
  // Failure, refusals in a row, or a last outcome that was not a send.
  expect(ids(await get(`/crons?problems=1`))).toEqual(["c-failed", "c-gone"]);
  expect(ids(await get(`/crons?tag=billing`))).toEqual(["c-failed"]);
  expect(ids(await get(`/crons?tag=ops`))).toEqual(["c-failed", "c-gone", "c-ok"]);
  const t = await get("/crons");
  const tabs = /<nav class="tabs" aria-label="Holder tags">[\s\S]*?<\/nav>/.exec(t)![0];
  expect(tabs).toContain('href="/crons?tag=ops" class="tab ">ops<span class="count">3</span>');
  expect(tabs).toContain('href="/crons?tag=billing" class="tab ">billing<span class="count">1</span>');
  expect(tabs).not.toContain("no-tags");
  // Tag counts follow the other filters.
  const narrowed = /<nav class="tabs" aria-label="Holder tags">[\s\S]*?<\/nav>/.exec(await get("/crons?problems=1"))![0];
  expect(narrowed).toContain('href="/crons?problems=1&amp;tag=ops" class="tab ">ops<span class="count">2</span>');
});

test("an empty list and a list with no match say so", async () => {
  const none = await get("/crons?q=nobody");
  expect(none).toContain("No crons match these filters");
  expect(none).toContain('<a class="btn" href="/crons">Clear filters</a>');
});

test("a schedule's page shows its message, creator, next fire and outcomes, with its controls", async () => {
  const t = await get("/schedule?id=c-failed");
  expect(t).toContain('<pre class="kv-value"><code>report</code></pre>');
  expect(t).toContain(`<dt>Creator</dt><dd><code>${you}</code></dd>`);
  expect(t).toContain("<dt>Refused in a row</dt>");
  expect(t).not.toContain("<dt>Upcoming</dt>");
  const out = /<section[^>]*id="schedule-outcomes"[\s\S]*?<\/section>/.exec(t)![0];
  expect(out).toContain("admission refused");
  expect(out).toContain("not admitted");
  expect(t).toContain('<button type="submit" class="btn btn-secondary" name="on" value="1">');
  expect(t).toContain('formaction="/schedule-run"');
  expect(t).toMatch(/<details class="inline-confirm"><summary class="btn btn-sm btn-danger">Delete<\/summary>\s*<form method="post" action="\/schedule-delete"/);
  const ok = await get("/schedule?id=c-ok");
  expect(ok).toContain('name="on" value="0"');
  const next = /<dt>Upcoming<\/dt><dd><ol class="schedule-upcoming">([\s\S]*?)<\/ol>/.exec(ok)![1]!;
  expect([...next.matchAll(/<time datetime="([^"]+)" title="\1" data-local>/g)].map(m => m[1])).toEqual(upcoming);
  await get("/schedule?id=missing", 404);
});

test("enable, disable, run now and delete post the id and come back with the daemon's answer", async () => {
  let r = await post("/schedule-switch", { id: "c-hand", kind: "recurring", on: "1" });
  expect(`${r.status} ${r.headers.get("location")}`).toBe("303 /schedule?id=c-hand");
  expect(r.headers.get("set-cookie")).toContain("ab_flash=schedule-enabled");
  expect(lastPost("/schedule/enable")).toEqual({ id: "c-hand" });
  r = await post("/schedule-switch", { id: "c-ok", kind: "recurring", on: "0" });
  expect(lastPost("/schedule/disable")).toEqual({ id: "c-ok" });
  expect(r.headers.get("set-cookie")).toContain("ab_flash=schedule-disabled");
  r = await post("/schedule-run", { id: "c-ok", kind: "recurring" });
  expect(lastPost("/schedule/run")).toEqual({ id: "c-ok" });
  expect(r.headers.get("set-cookie")).toContain("ab_flash=schedule-run-send_refused");
  r = await post("/schedule-delete", { id: "o-1", kind: "once" });
  expect(`${r.status} ${r.headers.get("location")}`).toBe("303 /ats");
  expect(lastPost("/schedule/delete")).toEqual({ id: "o-1" });
  expect(r.headers.get("set-cookie")).toContain("ab_flash=schedule-deleted");
});

test("Add sends the record, the time form, target and body, and shows the daemon's refusal on the form", async () => {
  let r = await post("/schedule-add", { return: "recurring", how: "expr", record: A, when: "0 9 * * *", target: Q, body: "a\r\nb" });
  expect(`${r.status} ${r.headers.get("location")}`).toBe("303 /schedule?id=new-1");
  expect(r.headers.get("set-cookie")).toContain("ab_flash=schedule-added;");
  expect(lastPost("/schedule/add")).toEqual({ record: A, expr: "0 9 * * *", target: Q, body: "a\nb" });
  r = await post("/schedule-add", { return: "recurring", how: "expr", record: A, when: "0 9 * * *", target: Q, body: "b", until: "2026-12-31T00:00:00Z", max_runs: "3", max_late: "5m", disabled: "on" });
  expect(lastPost("/schedule/add")).toEqual({ record: A, expr: "0 9 * * *", target: Q, body: "b", until: "2026-12-31T00:00:00Z", max_runs: 3, max_late: "5m", disabled: true });
  calls.length = 0;
  r = await post("/schedule-add", { return: "recurring", how: "expr", record: A, when: "0 9 * * *", target: Q, body: "b", max_runs: "two" });
  expect(r.status).toBe(400);
  expect(await r.text()).toContain("Max runs is a whole number above 0.");
  expect(lastPost("/schedule/add")).toBeUndefined();
  warning = "near the cap";
  r = await post("/schedule-add", { return: "once", how: "at", record: A, when: "14:30", target: Q, body: "x" });
  expect(lastPost("/schedule/add")).toEqual({ record: A, at: "14:30", target: Q, body: "x" });
  expect(r.headers.get("set-cookie")).toContain("ab_flash=schedule-added-near-cap");
  refuseAdd = { status: 400, error: "the expression has six fields but no seconds" };
  r = await post("/schedule-add", { return: "recurring", how: "expr", record: A, when: "bad", target: Q, body: "keep me" });
  expect(r.status).toBe(400);
  const t = await r.text();
  expect(t).toContain("the expression has six fields but no seconds");
  expect(t).toContain("keep me</textarea>");
  expect(t).toContain('value="bad"');
});

test("the Schedules card on a record page counts crons, ats and problems; a refused record has none", async () => {
  const agent = await get(`/agent?name=${encodeURIComponent(A)}`);
  const card = /<section[^>]*id="schedules"[\s\S]*?<\/section>/.exec(agent)?.[0] ?? "";
  expect(card).toContain("<strong>2</strong> crons");
  expect(card).toContain("<strong>1</strong> at");
  expect(card).toContain(`href="/crons?record=${encodeURIComponent(A)}#schedule-add"`);
  // c-gone's last outcome was not a send.
  expect(card).toMatch(/<span class="count tone-bad"[^>]*>1 with problems<\/span>/);
  expect(card).not.toContain("disabled by failure");
  const queue = /<section[^>]*id="schedules"[\s\S]*?<\/section>/.exec(await get(`/queue?name=${encodeURIComponent(Q)}`))?.[0] ?? "";
  expect(queue).toMatch(/<span class="count tone-bad"[^>]*>1 with problems<\/span>/);
  expect(queue).toContain('<p class="warn">1 disabled by failure.</p>');
  expect(await get(`/pubsub/channel?name=${encodeURIComponent(P)}`)).not.toContain('id="schedules"');
});

test("the KV page filters by the record's tags, with counts per tag", async () => {
  const stores = (html: string) => rows(html).map(r => /href="\/kv\/record\?name=([^"]+)"/.exec(r)?.[1]).filter(Boolean).map(s => decodeURIComponent(s!)).sort();
  const t = await get("/kv");
  expect(stores(t)).toEqual([A, P, Q].sort());
  const tabs = /<nav class="tabs" aria-label="Record tags">[\s\S]*?<\/nav>/.exec(t)![0];
  expect(tabs).toContain('href="/kv?tag=ops" class="tab ">ops<span class="count">2</span>');
  expect(tabs).toContain('href="/kv?tag=billing" class="tab ">billing<span class="count">1</span>');
  expect(stores(await get("/kv?tag=billing"))).toEqual([Q]);
  expect(stores(await get("/kv?tag=ops"))).toEqual([A, Q].sort());
  const narrowed = /<nav class="tabs" aria-label="Record tags">[\s\S]*?<\/nav>/.exec(await get("/kv?kind=agent"))![0];
  expect(narrowed).toContain('href="/kv?kind=agent&amp;tag=ops" class="tab ">ops<span class="count">1</span>');
});

test("Edit starts from the schedule as it stands and posts every field to the daemon, the refusal kept on the form", async () => {
  const t = await get("/schedule?id=c-hand");
  const form = /<form id="form-schedule-edit"[\s\S]*?<\/form>/.exec(t)![0];
  expect(form).toContain('value="0 0 * * *"');
  expect(form).toContain('value="2026-12-31T00:00:00Z"');
  expect(form).toContain('value="4"');
  expect(form).toContain('value="5m0s"');
  expect(form).toMatch(/name="disabled" value="on" checked/);
  expect(form).toMatch(/<option value="expr" selected>/);
  let r = await post("/schedule-edit", { id: "c-hand", kind: "recurring", how: "in", when: "2h", target: Q, body: "x\r\ny", until: "", max_runs: "", max_late: "" });
  expect(`${r.status} ${r.headers.get("location")}`).toBe("303 /schedule?id=c-hand");
  expect(r.headers.get("set-cookie")).toContain("ab_flash=schedule-saved");
  expect(lastPost("/schedule/edit")).toEqual({ id: "c-hand", in: "2h", target: Q, body: "x\ny" });
  refuseAdd = { status: 400, error: "owner@fixture may not send to elsewhere@fixture" };
  r = await post("/schedule-edit", { id: "c-hand", kind: "recurring", how: "expr", when: "0 1 * * *", target: "elsewhere@fixture", body: "kept" });
  expect(r.status).toBe(400);
  const page = await r.text();
  expect(page).toContain("owner@fixture may not send to elsewhere@fixture");
  expect(/<form id="form-schedule-edit"[\s\S]*?<\/form>/.exec(page)![0]).toContain('value="elsewhere@fixture"');
});
