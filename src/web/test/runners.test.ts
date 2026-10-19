// The Runners pages against a disposable daemon and a fake runner that answers
// `status` and `reload` from its own inbox, as agent-bus-runner does
// (Plans/R1.0-Release/runner/web-face.md#acceptance-checklist).
import { afterAll, beforeAll, expect, test } from "bun:test";
import { mkdtempSync, rmSync, existsSync } from "node:fs";
import { join } from "node:path";
import { tmpdir, userInfo } from "node:os";
import { makeHandler } from "../server.ts";
import { waits } from "../pages/runners.tsx";

const ORIGIN = "http://127.0.0.1:6782";
const A = "#runner/alpha@test", B = "#runner/beta@test", C = "#runner/gamma@test", D = "#runner/delta@test";
let dir = "", daemon: ReturnType<typeof Bun.spawn> | undefined;
let handle: (r: Request) => Promise<Response>;
let owner = "", bob = "", carol = "";
let serving = true;
const asked: { runner: string; body: string }[] = [];
/** The host's files the fake runner holds; any other is absent. */
const files: Record<string, string> = { "agent.d/hello/agent.json": '{"command":"./hello.sh"}', "agent.d/hello/env.dist": "GREETING=hi\nNAME=\n", "runner/hello/env": "NAME=<world>\n" };

async function api(path: string, body: unknown, token = owner, method = "POST") {
  const r = await fetch(`http://unix${path}`, { method, body: method === "POST" ? JSON.stringify(body) : undefined, headers: { "X-Agent-Bus-Token": token }, unix: join(dir, "bus.sock") } as RequestInit);
  const t = await r.text();
  if (!r.ok) throw new Error(`${path} ${r.status} ${t}`);
  return t ? JSON.parse(t) : null;
}

/** A runner that answers: its status names one visible and one hidden agent. */
async function fakeRunner(name: string, token: string, delay = 0) {
  while (serving) {
    let e: any;
    try { e = await api("/consume?wait=500ms", undefined, token, "GET"); } catch { await Bun.sleep(50); continue; }
    if (!e || e.receipt) continue;
    asked.push({ runner: name, body: e.body });
    if (delay) await Bun.sleep(delay);
    const verb = String(e.body).split(/\s/)[0];
    const file = String(e.body).split("\n")[0]!.slice(verb.length + 1);
    // As the runner does: what reaches the host's files is for its Owner and Maintainers.
    const managing = ["start", "stop", "restart", "enable", "disable", "read", "write", "create", "delete"].includes(verb);
    const answer = managing && e.from !== "owner@test" ? `error: ${verb} is for the runner's Owner and Maintainers: it reaches the host's files and their secrets`
      : delay ? JSON.stringify({ runner: name, version: "0.8.999", agents: [{ name: "#late@test", enabled: true, state: "idle" }] })
      : verb === "read" ? JSON.stringify(file in files ? { file, exists: true, content: files[file] } : { file, exists: false, content: "" })
      : verb === "write" && String(e.body).includes("bad") ? `error: ${file}: line 1: not KEY=value`
      : e.body === "status" || e.body === "reload" || ["start", "stop", "restart", "enable", "disable", "write", "create", "delete"].includes(verb)
      ? JSON.stringify({ runner: name, version: "0.8.999", agents: [
          { name: "#hello@test", key: "hello", enabled: true, state: "idle", processes: { running: 0, min: 0, max: 1 }, serves: "runner", sandbox: "on", limits: { memory_bytes: 512 << 20, processes: 64, disk_bytes: 1 << 30 } },
          { name: "#secretive@test", key: "secretive", enabled: true, state: "unconfigured", reason: "exited 78: SLACK_BOT_TOKEN is not set", serves: "itself", sandbox: "off", last_exit: { code: 78, at: new Date().toISOString(), reason: "slack: SLACK_BOT_TOKEN is not set" } },
          // As a runner before 0.8.203 answers: no sandbox, limits or lifecycle.
          { name: "#plain@test", enabled: true, state: "idle" },
        ], started_at: new Date(Date.now() - 7200_000).toISOString(), agent_dir: "/var/lib/agent-bus/runners/x/agent.d" })
      : `unknown command ${e.body}; use status or reload`;
    await api("/send", { to: e.from, topic: e.topic, tag: e.tag, body: answer, re: e.id }, token);
  }
}

beforeAll(async () => {
  dir = mkdtempSync(join(process.env.AGENT_BUS_TMP ?? tmpdir(), "web-runners-"));
  let bin = process.env.AGENT_BUS_BIN_DIR ?? "";
  if (!bin) {
    bin = join(dir, "bin");
    const b = Bun.spawnSync(["go", "build", "-o", bin + "/", "./cmd/agent-busd", "./cmd/agent-bus-token"], { cwd: join(import.meta.dir, "../.."), stderr: "pipe" });
    if (b.exitCode) throw new Error(b.stderr.toString());
  }
  daemon = Bun.spawn([join(bin, "agent-busd"), "-addr", "127.0.0.1:0", "-socket", join(dir, "bus.sock"), "-owner", "owner@test", "-db", join(dir, "bus.db"), "-create", "-flush-every", "0", "-dashboard", "", "-ssh-keys", join(dir, "ssh", "authorized_keys"), "-ssh-token", "/usr/local/bin/agent-bus-token"], { stdout: "ignore", stderr: "ignore" });
  const user = join(dir, `user-${userInfo().username}.sock`);
  for (let i = 0; i < 100 && !existsSync(user); i++) await Bun.sleep(50);
  owner = Bun.spawnSync([join(bin, "agent-bus-token"), "owner@test"], { env: { ...process.env, AGENT_BUS_ADDR: user } }).stdout.toString().trim();
  for (const u of ["bob", "carol"]) await api("/user", { name: u, person_name: u, create: true });
  bob = (await api("/token", { name: "bob" })).token;
  carol = (await api("/token", { name: "carol" })).token;
  // alpha answers and admits bob; beta answers nobody; gamma is hidden from bob.
  await api("/register", { name: A, kind: "agent", descr: "alpha's runner", allow: ["bob"] });
  await api("/register", { name: B, kind: "agent", descr: "silent" });
  await api("/register", { name: C, kind: "agent" });
  await api("/register", { name: D, kind: "agent", descr: "answers late" });
  // Not runners: a runner-tagged task, and a look-alike name.
  await api("/register", { name: "#tasker@test", kind: "agent", tags: ["runner"], allow: ["bob", "carol"] });
  await api("/register", { name: "#runner-x@test", kind: "agent", allow: ["carol"] });
  await api("/register", { name: "#hello@test", kind: "agent", descr: "Hello agent" });
  await api("/register", { name: "#secretive@test", kind: "agent" });
  void fakeRunner(A, (await api("/token", { name: A })).token);
  void fakeRunner(C, (await api("/token", { name: C })).token);
  void fakeRunner(D, (await api("/token", { name: D })).token, 1500);
  waits.status = 2000; waits.chooser = 1000; waits.reload = 2000; waits.control = 2000; waits.file = 2000;
  handle = makeHandler({ daemon: join(dir, "bus.sock"), listen: { hostname: "127.0.0.1", port: 6782 }, dev: false });
}, 120_000);

afterAll(async () => {
  serving = false;
  await Bun.sleep(600);
  daemon?.kill(); await daemon?.exited;
  if (dir) rmSync(dir, { recursive: true, force: true });
});

function req(path: string, cookie: string, form?: Record<string, string>) {
  const headers: Record<string, string> = { host: "127.0.0.1:6782", cookie: `agent_bus_session=${cookie}` };
  if (form) { headers.origin = ORIGIN; headers["content-type"] = "application/x-www-form-urlencoded"; }
  return handle(new Request(ORIGIN + path, { method: form ? "POST" : "GET", headers, body: form ? new URLSearchParams(form).toString() : undefined }));
}

async function signIn(token: string): Promise<string> {
  const r = await handle(new Request(ORIGIN + "/signin", { method: "POST", headers: { host: "127.0.0.1:6782", origin: ORIGIN, "content-type": "application/x-www-form-urlencoded" }, body: new URLSearchParams({ token }).toString() }));
  expect(r.status).toBe(303);
  return /agent_bus_session=([^;]+)/.exec(r.headers.get("set-cookie")!)![1]!;
}

const rows = (t: string, attr: string) => [...t.matchAll(new RegExp(`<tr data-${attr}="([^"]+)"`, "g"))].map(m => m[1]!.replace(/&#x2F;|&#47;/g, "/"));

test("several runners: a chooser of exactly the visitor's runners, each asked, none by tag or look-alike name", async () => {
  const r = await req("/runners", await signIn(owner));
  expect(r.status).toBe(200);
  const t = await r.text();
  expect(rows(t, "runner")).toEqual([A, B, D, C]);
  expect(t).toContain("answering · v0.8.999");
  expect(t).toContain("no answer"); // beta is silent, and is not shown as answering
});

test("exactly one runner: a 301 to it, not stored, though others exist", async () => {
  const r = await req("/runners", await signIn(bob));
  expect(r.status).toBe(301);
  expect(r.headers.get("location")).toBe(`/runner?name=${encodeURIComponent(A)}`);
  expect(r.headers.get("cache-control")).toBe("no-store");
});

test("no runner: the empty state, and a runner tag or a look-alike name does not make one", async () => {
  const r = await req("/runners", await signIn(carol));
  expect(r.status).toBe(200);
  const t = await r.text();
  expect(t).toContain("No runners you can see");
  expect(rows(t, "runner")).toEqual([]);
});

test("a runner's page lists its agents as it answered, linked back to it, a hidden one unlinked", async () => {
  const before = asked.filter(a => a.runner === A).length;
  const t = await (await req(`/runner?name=${encodeURIComponent(A)}`, await signIn(bob))).text();
  expect(asked.filter(a => a.runner === A && a.body === "status").length).toBe(before + 1);
  expect(rows(t, "agent")).toEqual(["#hello@test", "#secretive@test", "#plain@test"]);
  expect(t).toContain("exited 78: SLACK_BOT_TOKEN is not set");
  // bob sees neither record: both rows say so, and neither is a link.
  expect(t).not.toContain(`href="/agent?name=${encodeURIComponent("#hello@test")}`);
  expect(t.match(/its record is not visible to you/g)?.length).toBe(3);
  const own = await (await req(`/runner?name=${encodeURIComponent(A)}`, await signIn(owner))).text();
  const link = `/agent?name=${encodeURIComponent("#hello@test")}&amp;return=${encodeURIComponent(`/runner?name=${encodeURIComponent(A)}`)}`;
  expect(own).toContain(`<div class="rec-title"><a href="${link}"><strong>Hello agent</strong><code>#hello@test</code></a></div>`); // as the agents list draws it
  expect(own).toContain("Hello agent");
  // The summary, the inbox from the visitor's own records, and a runner
  // older than the node named as such.
  expect(own).toContain("<span data-counts>1 unconfigured · 2 idle</span>");
  expect(own).toMatch(/<td data-label="Inbox" data-inbox>0 reading<\/td>/); // nothing queued: not said
  expect(own).toContain("newer than this node&#39;s"); // the fake answers 0.8.999
  // What the runner reports of itself (RD99), each field as it said.
  expect(own).toMatch(/<td data-label="Protection" data-protection><span title="• 512 MiB memory\n• 64 processes\n• 1 GiB disk" data-limits><span class="pill tone-ok">sandbox<\/span><\/span><\/td>/);
  expect(own).toMatch(/<td data-label="Protection" data-protection><span class="pill tone-warn">sandbox off<\/span><\/td>/);
  expect(own).not.toContain('data-label="Reason and last exit"'); // on the agent's own page
  expect(own).toContain("<th>Last used</th><th>Manage</th></tr></thead>"); // Manage last, no reason column
  expect(own).toContain("serves itself");
  expect(own).toMatch(/<tr data-agent="#plain@test">.*?<td data-label="Protection" data-protection><span class="muted">—<\/span><\/td>/);
  expect(own).toContain("a script per message");
  expect(own).toContain("<dt>Up since</dt><dd><span title=");
  expect(own).toContain('<dl class="pairs" data-paths><dt>Agent code</dt><dd><code>/var/lib/agent-bus/runners/x/agent.d</code></dd></dl>');
  expect(own).not.toContain("<dt>Asked</dt>"); // the moment the page asked: no news
  expect(own).toContain("2h ago");
  expect(t).toMatch(/<td data-label="Inbox" data-inbox><span class="muted">—<\/span><\/td>/); // bob sees no record, so no inbox
});

test("a runner that does not answer is identified, not shown with no agents", async () => {
  const t = await (await req(`/runner?name=${encodeURIComponent(B)}`, await signIn(owner))).text();
  expect(t).toContain('data-runner-problem>No answer within 2 s');
  expect(t).not.toContain("The runner serves no agents");
  expect(rows(t, "agent")).toEqual([]);
});

test("a runner hidden from the visitor, or a name that is no runner, is not found", async () => {
  const bobs = await signIn(bob);
  expect((await req(`/runner?name=${encodeURIComponent(C)}`, bobs)).status).toBe(404);
  expect((await req(`/runner?name=${encodeURIComponent("#tasker@test")}`, bobs)).status).toBe(404);
  expect((await req("/runner-reload", bobs, { name: C })).status).toBe(404);
  expect(asked.filter(a => a.runner === C && a.body === "reload").length).toBe(0);
});

test("reload is the runner's answer; a silent runner's reload is uncertain, not success", async () => {
  const s = await signIn(owner);
  const before = asked.filter(a => a.runner === A && a.body === "reload").length;
  const ok = await (await req("/runner-reload", s, { name: A })).text();
  expect(asked.filter(a => a.runner === A && a.body === "reload").length).toBe(before + 1);
  expect(ok).toContain("Reloaded: the runner answered");
  expect(ok).toContain("<title>Host settings"); // reload answers on the host's settings page
  const back = await (await req("/runner-reload", s, { name: A, key: "hello" })).text();
  expect(back).toContain("<title>Manage hello"); // reload from an agent's page returns to it
  expect(back).toContain("Reloaded: the runner answered");
  const silent = await (await req("/runner-reload", s, { name: B })).text();
  expect(silent).toContain("data-runner-warning>The reload was sent and its outcome is not known");
  expect(silent).not.toContain("Reloaded:");
});

test("an agent's page opened from a runner returns to it; a forged return does not", async () => {
  const s = await signIn(owner);
  const back = `/runner?name=${encodeURIComponent(A)}`;
  const t = await (await req(`/agent?name=${encodeURIComponent("#hello@test")}&return=${encodeURIComponent(back)}`, s)).text();
  expect(t).toContain(`class="back-link" href="${back.replace("&", "&amp;")}"`);
  const forged = await (await req(`/agent?name=${encodeURIComponent("#hello@test")}&return=${encodeURIComponent("//evil.example/runner")}`, s)).text();
  expect(forged).not.toContain("evil.example");
});

test("a reload sent and then unanswerable is uncertain, never 'not carried out'", async () => {
  // In front of the daemon: every /consume fails, as a daemon going away
  // after it accepted the send would.
  const path = join(dir, "proxy.sock");
  const proxy = Bun.serve({ unix: path, async fetch(r) {
    const u = new URL(r.url);
    if (u.pathname === "/consume") return new Response("going away", { status: 503 });
    return fetch(`http://unix${u.pathname}${u.search}`, { method: r.method, headers: r.headers, body: r.body, unix: join(dir, "bus.sock") } as RequestInit);
  } });
  try {
    const via = makeHandler({ daemon: path, listen: { hostname: "127.0.0.1", port: 6782 }, dev: false });
    const s = await signIn(owner);
    const before = asked.filter(a => a.runner === A && a.body === "reload").length;
    const r = await via(new Request(ORIGIN + "/runner-reload", { method: "POST", headers: { host: "127.0.0.1:6782", origin: ORIGIN, cookie: `agent_bus_session=${s}`, "content-type": "application/x-www-form-urlencoded" }, body: new URLSearchParams({ name: A }).toString() }));
    const t = await r.text();
    for (let i = 0; i < 40 && asked.filter(a => a.runner === A && a.body === "reload").length === before; i++) await Bun.sleep(50);
    expect(asked.filter(a => a.runner === A && a.body === "reload").length).toBe(before + 1); // it was sent
    expect(t).toContain("data-runner-warning>The reload was sent and its outcome is not known");
    expect(t).not.toContain("The reload was not carried out");
  } finally {
    proxy.stop(true);
  }
});

test("a late answer stays with the call that asked: the next call never takes it", async () => {
  // delta answers after the chooser stopped waiting; its answer arrives in
  // the visitor's inbox while alpha's page is being asked. Only a tag of each
  // call's own keeps it out of alpha's.
  const s = await signIn(owner);
  await (await req("/runners", s)).text();
  const t = await (await req(`/runner?name=${encodeURIComponent(A)}`, s)).text();
  await Bun.sleep(1200);
  const again = await (await req(`/runner?name=${encodeURIComponent(A)}`, s)).text();
  for (const page of [t, again]) expect(rows(page, "agent")).toEqual(["#hello@test", "#secretive@test", "#plain@test"]);
});

const askedOf = (body: string) => asked.filter(a => a.runner === A && a.body === body).length;

test("the runner's page only informs and alerts; each agent's Manage, last in its row, holds its controls, stop and disable in the danger zone", async () => {
  const s = await signIn(owner);
  const info = await (await req(`/runner?name=${encodeURIComponent(A)}`, s)).text();
  expect(info).not.toContain('action="/runner-control"');
  expect(info).not.toContain('action="/runner-reload"');
  expect(info).toContain(`href="/runner-settings?name=${encodeURIComponent(A)}" data-host-settings`);
  expect(info).toMatch(new RegExp(`<tr data-agent="#hello@test">.*<td data-label="Manage"><a href="/runner-settings\\?name=${encodeURIComponent(A)}&amp;key=hello" data-manage>Manage</a></td></tr>`, "s")); // the limits' tooltip spans lines
  expect(info).toMatch(/<tr data-agent="#plain@test">.*?<td data-label="Manage"><span class="muted">—<\/span><\/td><\/tr>/s); // no key: nothing to manage by
  expect(info).toMatch(/<ul class="alerts" data-alerts>.*<code>#secretive@test<\/code> is unconfigured: exited 78: SLACK_BOT_TOKEN is not set/);
  expect(info).toMatch(/<ul class="alerts" data-alerts>.*<code>#secretive@test<\/code> has its sandbox off/);
  expect((await req(`/runner-manage?name=${encodeURIComponent(A)}`, s)).status).toBe(404); // no page managing them all
  const host = await (await req(`/runner-settings?name=${encodeURIComponent(A)}`, s)).text();
  expect(host).toContain('action="/runner-reload"');
  expect(host).not.toContain('action="/runner-control"');
  const m = await (await req(`/runner-settings?name=${encodeURIComponent(A)}&key=hello`, s)).text();
  expect(m).toContain("<title>Manage hello");
  // Headed as the agent's Edit page: back to the runner, kind, name, Owner; no access sentence.
  expect(m).toContain(`Back to ${A}</a>`);
  expect(m).toMatch(/<span class="title-text">Manage Agent <[^>]+>#hello@test<\/[a-z]+><span class="muted small"> \(owner: (?:(?!<\/h1>).)*owner@test(?:(?!<\/h1>).)*<\/h1>/s);
  expect(m).not.toContain("Only its Owner and Maintainers");
  expect(m).toContain('<p class="muted small">Restart the agent to apply changes.</p>');
  // One line: its state, then Restart and Reload; no sentence about how long each lasts.
  expect(m).toMatch(/<div class="agent-status" data-agent="#hello@test"><span class="muted">Status<\/span><span class="pill tone-ok">idle<\/span><span class="sep" aria-hidden="true"><\/span><span data-control><form[^]*?value="restart"[^]*?<\/form><\/span><form method="post" action="\/runner-reload" class="inline-form" data-reload><input type="hidden" name="name" value="#runner\/alpha@test"><input type="hidden" name="key" value="hello">/);
  expect(m).not.toContain("Start and Restart last");
  // Stop and Disable look different: stop is a pause of hours, disable lasts.
  expect(m).toMatch(/<button type="submit" class="btn btn-warn" name="verb" value="stop"><i class="ic " data-lucide="square"/);
  expect(m).toMatch(/<button type="submit" class="btn btn-danger" name="verb" value="disable"><i class="ic " data-lucide="power-off"/);
  // In the status line, after Reload.
  expect(m).toMatch(/<div class="agent-status" data-agent="#hello@test">(?:(?!<\/div>).)*data-reload>(?:(?!<\/div>).)*<\/form><details class="danger-inline" data-danger-zone>/s);
  expect(m).toMatch(/<section class="card  " id="control"><div class="card-body">/); // no heading: the page title names the agent
  const zone = m.indexOf('<details class="danger-inline" data-danger-zone><summary>');
  expect(zone).toBeGreaterThan(0); // one closed expand
  expect(m).toMatch(/data-danger-zone><summary>[^]{0,600}?<\/(?:svg|span|i)> Danger zone \(Stop, Disable\)<\/summary>/); // naming what it holds
  const before = m.slice(0, zone), after = m.slice(zone);
  expect(before).toMatch(/<span data-control><form method="post" action="\/runner-control" class="inline-form">.*?<input type="hidden" name="agent" value="hello">.*?value="restart"/);
  for (const v of ["stop", "disable"]) {
    expect(before).not.toContain(`value="${v}"`);
    // Each its own row, holding only its own verb.
    const one = after.match(new RegExp(`<span class="danger-action" data-danger="${v}" title="[^"]+">(.*?)</form></span>`, "s"));
    expect(one?.[1]).toContain('<input type="hidden" name="agent" value="hello">');
    expect([...one![1]!.matchAll(/name="verb" value="(\w+)"/g)].map(x => x[1])).toEqual([v]);
  }
  expect(m).not.toContain("data-exit"); // nothing to say: nothing said
  const sec = await (await req(`/runner-settings?name=${encodeURIComponent(A)}&key=secretive`, s)).text();
  expect(sec).toContain('<dt>Reason</dt><dd data-reason>exited 78: SLACK_BOT_TOKEN is not set</dd>');
  expect(sec).toMatch(/<dt>Last exit<\/dt><dd data-last-exit><span title="[0-9TZ:.-]+">exit 78, now<\/span>: <code>slack: SLACK_BOT_TOKEN is not set<\/code><\/dd>/);
  const unknown = await (await req(`/runner-settings?name=${encodeURIComponent(A)}&key=nobody`, s)).text();
  expect(unknown).toContain("The runner names no agent nobody");
  expect(unknown).not.toContain('action="/runner-control"');
});

test("a control is the runner's answer; an unknown verb is never sent", async () => {
  const s = await signIn(owner);
  const before = askedOf("stop hello");
  const t = await (await req("/runner-control", s, { name: A, verb: "stop", agent: "hello" })).text();
  expect(askedOf("stop hello")).toBe(before + 1);
  expect(t).toContain("Stop hello: the runner answered");
  expect(t).toContain("<title>Manage hello"); // back on the agent's own page
  const n = asked.length;
  expect((await req("/runner-control", s, { name: A, verb: "frobnicate", agent: "hello" })).status).toBe(400);
  expect((await req("/runner-control", s, { name: A, verb: "stop", agent: "hello world" })).status).toBe(400);
  expect(asked.length).toBe(n);
  const silent = await (await req("/runner-control", s, { name: B, verb: "stop", agent: "hello" })).text();
  expect(silent).toContain("data-runner-warning>Stop hello was sent and its outcome is not known");
}, 15_000);

test("settings show the author's files read only and the host's in forms, absent said as absent", async () => {
  const t = await (await req(`/runner-settings?name=${encodeURIComponent(A)}&key=hello`, await signIn(owner))).text();
  expect(t).toContain('<div class="runner-file" data-file="agent.d/hello/env.dist"><h3 class="file-head"><code>agent.d/hello/env.dist</code> <span class="muted">— read only, from the agent&#39;s code</span></h3><pre class="file-view">GREETING=hi\nNAME=\n</pre></div>');
  expect(t).not.toMatch(/<form[^>]*data-file="agent\.d/);
  expect(t).toMatch(/<form method="post" action="\/runner-file" class="runner-file" data-file="runner\/hello\/env">.*?<textarea name="content"[^>]*>\nNAME=&lt;world&gt;\n<\/textarea>/s);
  expect(t).toMatch(/<form method="post" action="\/runner-file" class="runner-file" data-file="runner\/hello\/agent.json"><h3 class="file-head"><code>runner\/hello\/agent.json<\/code> <span class="muted">— overrides processes, deadline, idle, depends · absent<\/span><span data-sample>/); // one line: file, what, absent, sample
  const none = await (await req(`/runner-settings?name=${encodeURIComponent(A)}&key=plain`, await signIn(owner))).text();
  expect(none).toContain('<div class="runner-file" data-file="agent.d/plain/env.dist"><h3 class="file-head"><code>agent.d/plain/env.dist</code> <span class="muted">— absent</span></h3></div>');
  const inv = await (await req(`/runner-settings?name=${encodeURIComponent(A)}`, await signIn(owner))).text();
  expect(inv).toContain('data-file="runner/agents.json"');
  expect(inv).toMatch(/data-file="runner\/agents.json">.*?<span data-sample><a class="docs-link" href="https:\/\/github.com\/parf\/ai-agent-bus\/blob\/main\/docs\/runner\/agents.json.dist"/s);
  expect(t).toMatch(/data-file="runner\/hello\/agent.json">.*?<span data-sample><a class="docs-link" href="https:\/\/github.com\/parf\/ai-agent-bus\/blob\/main\/docs\/runner\/agent.json.dist"/s);
  expect(t).not.toMatch(/data-file="runner\/hello\/env">(?:(?!<\/form>).)*data-sample/s); // an env has no sample
  expect(t).toContain('<h3 class="file-head"><code>runner/hello/env</code> <span class="muted">— KEY=value per line; holds secrets</span></h3>'); // present: no absent
  expect(t).not.toContain("Shipped with the agent");
  expect(inv).not.toContain("agent.d/");
});

test("a visitor who may see the runner but not manage it is told so once, and offered nothing", async () => {
  const s = await signIn(bob);
  for (const page of [`/runner-settings?name=${encodeURIComponent(A)}&key=hello`, `/runner-settings?name=${encodeURIComponent(A)}`]) {
    const t = await (await req(page, s)).text();
    expect(t).toContain('<p class="warn" role="alert" data-denied>Access denied: only the runner&#39;s Owner and Maintainers manage it.</p>');
    for (const offered of ['action="/runner-control"', 'action="/runner-reload"', 'action="/runner-file"', 'action="/runner-instance"', "data-danger-zone", "<textarea"]) expect(t).not.toContain(offered);
    expect(t).not.toContain("NAME=&lt;world&gt;"); // the host's env stays the runner's
  }
  const own = await (await req(`/runner-settings?name=${encodeURIComponent(A)}&key=hello`, await signIn(owner))).text();
  expect(own).not.toContain("data-denied"); // the Owner is not told it
});

test("a save sends the file whole; a refusal keeps what was typed; the author's files are not a target", async () => {
  const s = await signIn(owner);
  const ok = await (await req("/runner-file", s, { name: A, key: "hello", file: "runner/hello/env", content: "\r\n\r\nNAME=you\r\n" })).text();
  // A first blank line, as the browser sends it after dropping the opening newline, stays.
  expect(askedOf("write runner/hello/env\n\n\nNAME=you\n")).toBe(1);
  expect(ok).toContain("Saved runner/hello/env: the runner checked and applied it.");
  const bad = await (await req("/runner-file", s, { name: A, key: "hello", file: "runner/hello/env", content: "bad line" })).text();
  expect(bad).toContain("data-runner-warning>Saving runner/hello/env was not carried out. The runner answered: error: runner/hello/env: line 1");
  expect(bad).toMatch(/data-file="runner\/hello\/env">.*?<textarea name="content"[^>]*>\nbad line<\/textarea>/s);
  const n = asked.length;
  expect((await req("/runner-file", s, { name: A, key: "hello", file: "agent.d/hello/env.dist", content: "X=1" })).status).toBe(400);
  expect((await req("/runner-file", s, { name: A, key: "hello", file: "runner/tokens/x", content: "X=1" })).status).toBe(400);
  expect(asked.length).toBe(n);
});

test("an instance is created and deleted by the runner's own calls", async () => {
  const s = await signIn(owner);
  const made = await (await req("/runner-instance", s, { name: A, verb: "create", key: "hello/work" })).text();
  expect(askedOf("create hello/work")).toBe(1);
  expect(made).toContain("Created hello/work: configure its env below.");
  expect(made).toContain('data-file="runner/hello/work/env"');
  const confirm = await (await req(`/runner-instance-delete?name=${encodeURIComponent(A)}&key=hello/work`, s)).text();
  expect(confirm).toContain("<code>runner/hello/work/env</code>, with the secrets in it");
  expect(confirm).toContain("its record on the bus, with its inbox, bus secret and configuration");
  expect(made).toMatch(new RegExp(`Danger zone \\(Delete\\)</summary>.*?<span class="danger-action" data-danger="delete" title="[^"]+"><a class="btn btn-danger" href="/runner-instance-delete\\?name=${encodeURIComponent(A)}&amp;key=hello%2Fwork" data-delete`, "s"));
  const unconfirmed = await (await req("/runner-instance", s, { name: A, verb: "delete", key: "hello/work", confirm: "hello/wor" })).text();
  expect(unconfirmed).toContain("data-runner-warning>Type hello/work to delete it; nothing was deleted.");
  expect(askedOf("delete hello/work")).toBe(0);
  const gone = await (await req("/runner-instance", s, { name: A, verb: "delete", key: "hello/work", confirm: "hello/work" })).text();
  expect(askedOf("delete hello/work")).toBe(1);
  expect(gone).toContain("Deleting hello/work: the runner answered");
  expect((await req("/runner-instance", s, { name: A, verb: "delete", key: "hello" })).status).toBe(400);
});
