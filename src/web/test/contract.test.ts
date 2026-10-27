// The face against a disposable daemon: every request goes through the real
// handler and the real daemon over its socket. AGENT_BUS_BIN_DIR names built
// Go programs; without it they are built into a temporary directory.
import { generateKeyPairSync } from "node:crypto";
import { afterAll, beforeAll, describe, expect, test } from "bun:test";
import { mkdtempSync, rmSync, existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { tmpdir, userInfo } from "node:os";
import { makeHandler } from "../server.ts";
import { addDays } from "../ui/range.tsx";

const ORIGIN = "http://127.0.0.1:6781";
let dir = "", bin = "", daemon: ReturnType<typeof Bun.spawn> | undefined;
let handle: (r: Request) => Promise<Response>;
let owner = "", bob = "";

async function api(path: string, body: unknown, token = owner) {
  const r = await fetch(`http://unix${path}`, { method: "POST", body: JSON.stringify(body), headers: { "X-Agent-Bus-Token": token }, unix: join(dir, "bus.sock") } as RequestInit);
  const t = await r.text();
  if (!r.ok) throw new Error(`${path} ${r.status} ${t}`);
  return t ? JSON.parse(t) : null;
}

beforeAll(async () => {
  dir = mkdtempSync(join(process.env.AGENT_BUS_TMP ?? tmpdir(), "web-contract-"));
  bin = process.env.AGENT_BUS_BIN_DIR ?? "";
  if (!bin) {
    bin = join(dir, "bin");
    const b = Bun.spawnSync(["go", "build", "-o", bin + "/", "./cmd/agent-busd", "./cmd/agent-bus"], { cwd: join(import.meta.dir, "../.."), stderr: "pipe" });
    if (b.exitCode) throw new Error(b.stderr.toString());
  }
  daemon = Bun.spawn([join(bin, "agent-busd"), "-addr", "127.0.0.1:0", "-socket", join(dir, "bus.sock"), "-owner", "owner@test", "-db", join(dir, "bus.db"), "-create", "-flush-every", "0", "-dashboard", "", "-ssh-keys", join(dir, "ssh", "authorized_keys"), "-ssh-token", "/usr/local/bin/agent-bus-token"], { stdout: "ignore", stderr: "ignore" });
  const user = join(dir, `user-${userInfo().username}.sock`);
  for (let i = 0; i < 100 && !existsSync(user); i++) await Bun.sleep(50);
  owner = Bun.spawnSync([join(bin, "agent-bus"), "token", "owner@test"], { env: { ...process.env, AGENT_BUS_ADDR: user } }).stdout.toString().trim();
  await api("/user", { name: "bob", person_name: "Bob", create: true });
  bob = (await api("/token", { name: "bob" })).token;
  await api("/register", { name: "#helper@test", kind: "agent", descr: "Helper", allow: ["@owner"] });
  await api("/register", { name: "jobs@test", kind: "queue", descr: "Jobs", allow: ["*"] });
  await api("/register", { name: "news@test", kind: "pubsub", subs: ["jobs@test"], allow: ["*"] });
  await api("/register", { name: "db@test", kind: "service", addr: "db:5432", protocol: "postgresql" });
  await api("/group", { Name: "@ops", Members: ["bob"] });
  await api("/group", { Name: "@hidden", Members: ["owner@test"] });
  await api("/register", { name: "mine@test", kind: "service", addr: "mine:1", protocol: "https", personal: true });
  await api("/group", { Name: "@owner@test/team", Members: ["owner@test"] });
  await api("/manage", { name: "@owner@test/team", personal: true });
  await api("/send", { to: "jobs@test", body: "hello", topic: "t", tag: "x" });
  handle = makeHandler({ daemon: join(dir, "bus.sock"), listen: { hostname: "127.0.0.1", port: 6781 }, dev: false });
}, 120_000);

afterAll(async () => {
  daemon?.kill(); await daemon?.exited;
  if (dir) rmSync(dir, { recursive: true, force: true });
});

type Opts = { method?: string; cookie?: string; form?: Record<string, string>; origin?: string | null; headers?: Record<string, string>; body?: string };
function req(path: string, o: Opts = {}) {
  const headers: Record<string, string> = { host: "127.0.0.1:6781", ...(o.headers ?? {}) };
  if (o.cookie) headers.cookie = `agent_bus_session=${o.cookie}`;
  const method = o.method ?? (o.form || o.body ? "POST" : "GET");
  if (method === "POST" && o.origin !== null) headers.origin = o.origin ?? ORIGIN;
  if (o.form) headers["content-type"] = "application/x-www-form-urlencoded";
  return handle(new Request(ORIGIN + path, { method, headers, body: o.form ? new URLSearchParams(o.form).toString() : o.body }));
}

async function signIn(token: string): Promise<string> {
  const r = await req("/signin", { form: { token } });
  expect(r.status).toBe(303);
  const c = r.headers.get("set-cookie")!;
  return /agent_bus_session=([^;]+)/.exec(c)![1]!;
}

describe("signed out", () => {
  test("landing and chart assets need no CDN and fonts are served locally", async () => {
    const response=await req("/");const page=await response.text();
    expect(page).not.toContain("cdn.jsdelivr.net");
    expect(response.headers.get("content-security-policy")).not.toContain("https:");
    const {buildLocal}=await import("../assets.ts");const a=buildLocal();
    for(const asset of [a.uplot,...a.fonts]) {
      const result=await req(asset.path);expect(result.status).toBe(200);
      expect(result.headers.get("content-type")).toBe(asset.type);
      expect(result.headers.get("cache-control")).toContain("immutable");
    }
  });
  test("token-only sign-in navigates away after a plain password POST", async () => {
    const r = await req("/");
    const page = await r.text();
    expect(r.headers.get("cache-control")).toContain("no-store");
    const iconPath = /src="(\/lucide\.[a-f0-9]+\.js)"/.exec(page)?.[1];
    expect(iconPath).toBeDefined();
    const icon = await req(iconPath!);
    expect(icon.status).toBe(200);
    expect(await icon.text()).not.toContain("sourceMappingURL");
    expect(r.headers.get("content-security-policy")).not.toContain("/lucide@");
    expect(page).toMatch(/<form[^>]*method="post"[^>]*action="\/signin"/);
    expect(page).not.toContain('name="username"');
    expect(page).not.toContain("Bus name");
    expect(page).toMatch(/<input[^>]*id="token"[^>]*name="token"[^>]*type="password"[^>]*autocomplete="current-password"/);
    const signed = await req("/signin", {form:{token:owner}});
    expect(signed.status).toBe(303);
    expect(signed.headers.get("location")).toBe("/");
    expect(signed.headers.get("cache-control")).toContain("no-store");
    expect(await signed.text()).not.toContain(owner);
    const session = /agent_bus_session=([^;]+)/.exec(signed.headers.get("set-cookie")!)![1]!;
    const landed = await req(signed.headers.get("location")!, {cookie:session});
    expect(landed.status).toBe(200);
    expect(await landed.text()).not.toContain('class="signin-form"');

    const refused = await req("/signin", {form:{token:"fixture-private-token"}});
    expect(await refused.text()).not.toContain("fixture-private-token");
  });
  test("the homepage is the landing page", async () => {
    const r = await req("/");
    expect(r.status).toBe(200);
    const t = await r.text();
    expect(t).toContain("One bus for agents and bots");
    expect(t).toContain('action="/signin"');
    expect(t).not.toContain('name="return"');
  });
  test("any other page answers 401 with the sign-in form and its return", async () => {
    for (const p of ["/agents?q=x", "/diagnostics", "/users?kind=other"]) {
      const r = await req(p);
      expect(`${p} ${r.status}`).toBe(`${p} 401`);
      const t = await r.text();
      expect(t).toContain("sign in to open this page");
      expect(t).toContain(`name="return" value="${p.replace("&", "&amp;")}"`);
    }
  });
  test("the style guide exists only in development", async () => {
    expect((await req("/_styleguide")).status).toBe(404);
    const dev = makeHandler({ daemon: join(dir, "bus.sock"), listen: { hostname: "127.0.0.1", port: 6781 }, dev: true });
    const r = await dev(new Request(ORIGIN + "/_styleguide", { headers: { host: "127.0.0.1:6781" } }));
    expect(r.status).toBe(200);
    expect(/\sstyle=/.test(await r.text())).toBe(false);
  });
  test("an unknown address is a 404, not a front page", async () => {
    const r = await req("/no-such-page");
    expect(r.status).toBe(404);
    expect(await r.text()).toContain("No such page");
  });
});

describe("requests", () => {
  test("every POST needs this origin", async () => {
    for (const origin of [null, "http://evil.example"]) {
      const r = await req("/signin", { form: { token: owner }, origin });
      expect(r.status).toBe(403);
      expect(await r.text()).toContain("same-origin form required");
    }
    const r = await req("/record", { form: { action: "create" }, headers: { "sec-fetch-site": "cross-site" } });
    expect(r.status).toBe(403);
  });
  test("a body over 1 MiB is refused", async () => {
    const r = await req("/signin", { body: "token=" + "x".repeat((1 << 20) + 10), headers: { "content-type": "application/x-www-form-urlencoded" } });
    expect(r.status).toBe(413);
  });
  test("HEAD answers as GET without a body", async () => {
    const r = await req("/", { method: "HEAD" });
    expect(r.status).toBe(200);
    expect(await r.text()).toBe("");
  });
  test("headers: no-store pages, immutable hashed assets, the CSP", async () => {
    const page = await req("/");
    expect(page.headers.get("cache-control")).toBe("no-store");
    expect(page.headers.get("content-security-policy")).toContain("frame-ancestors 'none'");
    const css = /href="(\/app\.[0-9a-f]+\.css)"/.exec(await page.text())![1]!;
    const a = await req(css);
    expect(a.status).toBe(200);
    expect(a.headers.get("cache-control")).toContain("immutable");
    expect((await req("/favicon.ico")).status).toBe(404);
    expect((await req("/healthz")).status).toBe(200);
    expect((await req("/healthz", { method: "POST", body: "" })).status).toBe(405);
  });
});

describe("sign-in", () => {
  test("a refused token is a 401 with one message", async () => {
    const r = await req("/signin", { form: { token: "wrong" } });
    expect(r.status).toBe(401);
    expect(await r.text()).toContain("that credential was not accepted");
  });
  test("the cookie is the session, never the token, and has no lifetime of its own", async () => {
    const r = await req("/signin", { form: { token: owner, return: "/queues" } });
    expect(r.status).toBe(303);
    expect(r.headers.get("location")).toBe("/queues");
    const c = r.headers.get("set-cookie")!;
    expect(c).toContain("HttpOnly");
    expect(c).toContain("SameSite=Strict");
    expect(c).not.toContain("Max-Age");
    expect(c).not.toContain(owner);
  });
  test("return=//evil lands on /", async () => {
    const r = await req("/signin", { form: { token: owner, return: "//evil.example/x" } });
    expect(r.headers.get("location")).toBe("/");
  });
  test("a malformed cookie or token is refused, never a crash", async () => {
    const r = await handle(new Request(ORIGIN + "/agents", { headers: { host: "127.0.0.1:6781", cookie: "agent_bus_session=%E0%A4%A" } }));
    expect(r.status).toBe(401);
    const t = await req("/signin", { form: { token: "abc\r\ndef" } });
    expect(t.status).toBe(401);
  });
  test("a cookie that is not a session id is no session, not a daemon outage", async () => {
    const r = await handle(new Request(ORIGIN + "/agents", { headers: { host: "127.0.0.1:6781", cookie: "agent_bus_session=abc%0d%0aX-Agent-Bus-Token: def" } }));
    expect(r.status).toBe(401);
    expect(await r.text()).toContain("sign in to open this page");
  });
  test("an ended session asks to sign in again", async () => {
    const r = await req("/agents", { cookie: "deadbeef" });
    expect(r.status).toBe(401);
    expect(await r.text()).toContain("that session has ended");
  });
});

describe("pages as the daemon owner", () => {
  let s = "";
  beforeAll(async () => { s = await signIn(owner); });
  test("agent settings show the record owner in the heading without duplicate identity fields", async () => {
    await api("/register", { name: "#bobs-agent@test", kind: "agent" }, bob);
    const page = await req("/agent/edit?name=%23bobs-agent@test", { cookie: s });
    expect(page.status).toBe(200);
    const html = await page.text();
    const heading = html.match(/<h1\b[^>]*>[\s\S]*?<\/h1>/)?.[0];
    expect(heading).toContain("#bobs-agent@test");
    expect(heading).toContain("(owner: ");
    expect(heading).toContain('>👤</span>');
    expect(heading).toContain('>bob</code>');
    expect(html).not.toContain('id="f-record-name"');
    expect(html).not.toContain('id="f-record-owner"');
    const form = html.match(/<form\b[^>]*id="form-save"[\s\S]*?<\/form>/)?.[0];
    expect(form).toContain('name="name" value="#bobs-agent@test"');
    expect(form).toContain('name="tags"');
    expect(form).not.toContain('type="checkbox" name="personal"');
  });
  test("record filters count matching choices and disable empty ones without a Status column", async () => {
    const names = ["#facet-reading@test", "#facet-queued@test", "#facet-inactive@test"];
    for (const name of names) await api("/register", { name, kind: "agent", allow: ["*"] });
    await api("/manage", { name: names[2], status: "inactive" });
    await api("/send", { to: names[1], body: "waiting" });
    const token = (await api("/token", { name: names[0] })).token;
    const stop = new AbortController();
    const poll = fetch("http://unix/consume?wait=30s", { headers: { "X-Agent-Bus-Token": token }, unix: join(dir, "bus.sock"), signal: stop.signal } as RequestInit).catch(() => undefined);
    const segment = (html: string, label: string) => html.match(new RegExp(`<div class="segmented" role="group" aria-label="${label}">([\\s\\S]*?)</div>`))?.[1] ?? "";
    try {
      let ready = false;
      for (let i = 0; i < 100; i++) {
        const r = await fetch(`http://unix/lookup?name=${encodeURIComponent(names[0]!)}`, { headers: { "X-Agent-Bus-Token": owner }, unix: join(dir, "bus.sock") } as RequestInit);
        if (((await r.json()) as { readers?: number }).readers === 1) { ready = true; break; }
        await Bun.sleep(10);
      }
      expect(ready).toBe(true);
      const active = await (await req("/agents?q=facet-", { cookie: s })).text();
      expect(active).not.toMatch(/<th[^>]*>Status<\/th>/);
      expect(segment(active, "Status")).toContain("Active (2)");
      expect(segment(active, "Status")).toContain("Inactive (1)");
      expect(segment(active, "Status")).not.toContain(">All");
      expect(active).not.toContain("name=%23facet-inactive%40test");
      expect(segment(active, "Readers")).toContain("Reading now (1)");
      expect(segment(active, "Readers")).toContain("No reader now (1)");
      expect(segment(active, "Readers")).toMatch(/<span[^>]*aria-disabled="true"[^>]*>Unavailable \(0\)<\/span>/);
      expect(segment(active, "Queue")).toContain("Holding work (1)");
      const reading = await (await req("/agents?q=facet-&readers=present", { cookie: s })).text();
      expect(segment(reading, "Queue")).toMatch(/<span[^>]*aria-disabled="true"[^>]*>Holding work \(0\)<\/span>/);
      expect(segment(reading, "Readers")).toContain("No reader now (1)");
      expect(segment(reading, "Status")).toMatch(/<span[^>]*aria-disabled="true"[^>]*>Inactive \(0\)<\/span>/);
      const inactive = await (await req("/agents?q=facet-&state=inactive", { cookie: s })).text();
      expect(inactive).toContain("name=%23facet-inactive%40test");
      expect(inactive).not.toContain("name=%23facet-reading%40test");
    } finally {
      stop.abort(); await poll;
    }
  });
  test("record tags preserve comma-only labels and filters count before pagination", async () => {
    for (let i=0;i<32;i++) {
      const suffix=String.fromCharCode(97+Math.floor(i/26))+String.fromCharCode(97+i%26);
      await api("/register",{name:`#tagbrowse-${suffix}@test`,kind:"agent",allow:["bob"],tags:[`BUILD-Notes,extra_${suffix}`]});
    }
    await api("/register",{name:"#tagbrowse-hidden@test",kind:"agent",tags:["invisible"]});
    const bs=await signIn(bob);
    const html=await (await req("/agents?q=tagbrowse",{cookie:bs})).text();
    const top=html.match(/<nav class="tabs" aria-label="Record views">[\s\S]*?<\/nav>/)?.[0]??"";
    expect(top).toContain('build-notes<span class="count">32</span>');
    expect(html).not.toContain("tag=invisible");
    const ownerHidden=await (await req("/agents?q=tagbrowse-hidden",{cookie:s})).text();
    expect(ownerHidden).toContain("tag=invisible");
    expect(html).toContain("more tags");
    const mine=await (await req("/agents?q=tagbrowse&scope=my",{cookie:bs})).text();
    const mineTop=mine.match(/<nav class="tabs" aria-label="Record views">[\s\S]*?<\/nav>/)?.[0]??"";
    expect(mineTop).toContain('All<span class="count">32</span>');
    expect(mineTop).not.toContain("build-notes");
    const selected=await (await req("/agents?q=tagbrowse&tag=extra_bf",{cookie:bs})).text();
    const selectedTop=selected.match(/<nav class="tabs" aria-label="Record views">[\s\S]*?<\/nav>/)?.[0]??"";
    expect(selectedTop).toContain('aria-current="true">extra_bf');
    expect(selected).toContain("name=%23tagbrowse-bf%40test");
    expect(selected).not.toContain("name=%23tagbrowse-aa%40test");
    const saved=await req("/record",{cookie:s,form:{action:"save",name:"#tagbrowse-aa@test",descr:"tagged",tags:"BUILD-Notes,CLAUDE,no-tags,v2"}});
    expect(saved.status).toBe(303);
    const bad = await req("/record",{cookie:s,form:{action:"save",name:"#tagbrowse-aa@test",tags:"bad tag"}});
    expect(bad.status).toBe(400);
    expect(await bad.text()).toContain("bad tag");

    const edit=await (await req("/agent/edit?name=%23tagbrowse-aa%40test",{cookie:s})).text();
    expect(edit).toContain('name="tags" data-tags type="text" value="build-notes,claude,v2"');
    expect(edit.indexOf('name="tags"')).toBeGreaterThan(edit.indexOf('name="descr"'));
    expect(edit.indexOf('name="tags"')).toBeLessThan(edit.indexOf('name="maintainers"'));
  });
  test("record Messages uses actual waiting and lost counts without duplicate or zero rows", async () => {
    const name="#message-card@test";
    await api("/register",{name,kind:"agent",bound:1,overflow:"ring"});
    const page=async()=>await (await req(`/agent?name=${encodeURIComponent(name)}`,{cookie:s})).text();
    expect(await page()).not.toContain('id="counters"');
    for(let i=0;i<3;i++)await api("/send",{to:name,body:`message ${i}`});
    const counter=async()=>/<section[^>]*id="counters">([\s\S]*?)<\/section>/.exec(await page())![1]!;
    let card=await counter();
    expect(card).toContain("<dt>Messages</dt>");
    expect(card).toMatch(/class="badge tone-warn"[^>]*>1 waiting<\/span>/);
    expect(card).toMatch(/class="badge tone-bad"[^>]*>2 lost<\/span>/);
    expect(card).toContain('title="Dropped 2, expired 0"');
    for(const label of ["Held now","Readers","Accepted","Dequeued","Accepted / Dequeued","Dropped / expired"]) expect(card).not.toContain(`<dt>${label}</dt>`);
    const taken=await fetch(`http://unix/consume?inbox=${encodeURIComponent(name)}&wait=0s`,{headers:{"X-Agent-Bus-Token":owner},unix:join(dir,"bus.sock")} as RequestInit);
    expect(taken.status).toBe(200);
    card=await counter();expect(card).not.toContain("waiting");expect(card).toContain("2 lost");expect(card).not.toContain("Oldest held");
    const healthy="#healthy-message-card@test";await api("/register",{name:healthy,kind:"agent"});
    await api("/send",{to:healthy,body:"one"});
    expect((await fetch(`http://unix/consume?inbox=${encodeURIComponent(healthy)}&wait=0s`,{headers:{"X-Agent-Bus-Token":owner},unix:join(dir,"bus.sock")} as RequestInit)).status).toBe(200);
    const html=await (await req(`/agent?name=${encodeURIComponent(healthy)}`,{cookie:s})).text();
    const idle=/<section[^>]*id="counters">([\s\S]*?)<\/section>/.exec(html)![1]!;
    expect(idle.match(/<dt>/g)).toHaveLength(1);expect(idle).toContain("<dt>Messages</dt>");
    expect(idle).not.toContain("badge");expect(idle).not.toMatch(/>0<\//);
    const status=/<section[^>]*id="status">([\s\S]*?)<\/section>/.exec(html)![1]!;
    for(const label of ["Last used","Reached","Queue bound","Retention","When full"])expect(status).not.toContain(`<dt>${label}</dt>`);
  });
  test("record cards hide unused state and keep settings and store reachable", async () => {
    const name = "#layout-empty@test";
    await api("/register", { name, kind:"agent", allow:["@owner"], maintainers:["bob"] });
    const html = await (await req(`/agent?name=${encodeURIComponent(name)}`, {cookie:s})).text();
    for (const id of ["route", "roles", "locks", "configuration"]) expect(html).not.toContain(`id="${id}"`);
    expect(html).not.toContain("No values yet. Add one.");
    expect(html).not.toContain("Stored and durable.");
    expect(html).not.toContain("Add one</a>");
    expect(html).toContain(`href="/kv/record?name=${encodeURIComponent(name)}"`);
    expect(html).toContain(`href="/agent/edit?name=${encodeURIComponent(name)}"`);
    expect(html).not.toContain('id="counters"');
    const positions = ["activity", "status", "kv"].map(id => html.indexOf(`id="${id}"`));
    expect(positions.every(pos => pos >= 0)).toBe(true);
    expect(positions).toEqual([...positions].sort((a,b) => a-b));
    expect(html).not.toContain("<dt>Config</dt>");
    const settings = await (await req(`/agent/edit?name=${encodeURIComponent(name)}`, {cookie:s})).text();
    for (const field of ["subs", "roles", "config"]) expect(settings).toContain(`name="${field}"`);
  });
  test("record cards show declared or assigned roles, held locks and stored Configuration", async () => {
    const name = "#layout-used@test";
    await api("/register", {name, kind:"agent", roles:["operate"], allow:["*"]});
    await api("/configure", {Name:name, Config:{mode:"used-layout-config"}});
    await api("/send",{to:name,body:"kept message"});
    await api("/lock", {record:name, name:"layout-held", ttl:"1m"});
    try {
      const html = await (await req(`/agent?name=${encodeURIComponent(name)}`, {cookie:s})).text();
      const positions = ["activity", "counters", "status", "configuration", "locks", "kv", "roles"].map(id => html.indexOf(`id="${id}"`));
      expect(positions.every(pos => pos >= 0)).toBe(true);
      expect(positions).toEqual([...positions].sort((a,b) => a-b));
      expect(html).toContain("<dt>Config</dt>");
      for (const text of ["Understands: operate", "layout-held", "used-layout-config", 'action="/release-lock"']) expect(html).toContain(text);
      await api("/manage", {name, roles:[], allow:["*(audit)"]});
      const assigned = await (await req(`/agent?name=${encodeURIComponent(name)}`, {cookie:s})).text();
      expect(assigned).toContain('id="roles"'); expect(assigned).toContain("*: audit");
      await api("/manage", {name, allow:["*"]});
      const access = await (await req(`/agent?name=${encodeURIComponent(name)}`, {cookie:s})).text();
      expect(access).toContain('id="roles"'); expect(access).toContain("*: access without an assigned role");
    } finally { await api("/release", {record:name, name:"layout-held"}); }
  });
  test("agent Roles shows automatic authority and explicit allow-list grants", async () => {
    await api("/register", {name:"#role-display@test", kind:"agent", roles:["deploy","commit"], maintainers:["bob"], allow:["@owner","@ops(deploy)","*(audit)","ann"]});
    const html = await (await req("/agent?name=%23role-display%40test", {cookie:s})).text();
    const section = html.slice(html.indexOf('id="roles"'));
    expect(section).toContain("Roles");
    expect(section).toContain("Understands: deploy, commit");
    expect(section).toContain("owner@test: owner (automatic)");
    expect(section).toContain("bob: maintainer (automatic)");
    expect(section).toContain("@ops: deploy");
    expect(section).toContain("*: audit");
    expect(section).toContain("ann: access without an assigned role");
  });
  test("assigned roles round trip through agent forms and edit tooltips explain usage", async () => {
    const created = await req("/record", { cookie: s, form: { action:"create", kind:"agent", name:"#assigned@test", descr:"Role test", allow:"@ops(deploy, read_only)\n*(guest)" } });
    expect(created.status).toBe(303);
    const page = await req("/agent/edit?name=%23assigned@test", {cookie:s});
    const html = await page.text();
    expect(html).toContain("@ops(deploy,read_only)");
    expect(html).toContain("Roles and usage");
    for (const label of ["About roles", "About the allow list"]) {
      const button = html.match(/<button\b[^>]*>/g)?.find(tag => tag.includes(`aria-label="${label}"`));
      expect(button).toBeDefined();
      const tip = /data-tooltip="([^"]*)"/.exec(button!)?.[1];
      expect(tip).toContain("@dev(deploy, read_only)");
      if (label === "About the allow list") {
        expect(tip).toContain("Who may reach this record, one term per line.");
        expect(tip).toContain("Role names use");
      }
    }
    expect(html).toContain("AB_ROLE_DEPLOY=1");
    expect(html).toContain("Assign roles in the allow list");
    expect(html).toContain("owner and maintainer roles are generated automatically");
    expect(html).not.toContain("assigning them here rejects the entire save");
    expect(html).not.toContain("assigning either rejects the save");
    expect(html).not.toContain("assigning either in an ACL rejects the whole save");
    const saved = await req("/record", {cookie:s,form:{action:"save",name:"#assigned@test",descr:"Role test",edit_allow:"1",allow:"@ops(deploy, audit)\n*(guest)"}});
    expect(saved.status).toBe(303);
    const sent = await api("/send",{to:"#assigned@test",body:"role check"},bob);
    const token = (await api("/token",{name:"#assigned@test"})).token;
    const got = await fetch("http://unix/consume?wait=1ms", { headers:{"X-Agent-Bus-Token":token}, unix:join(dir,"bus.sock") } as RequestInit);
    const message = await got.json() as any;
    expect(message.message_id).toBe(sent.message_id);
    expect(message.roles).toEqual(["deploy","audit","guest"]);
    const refused = await req("/record",{cookie:s,form:{action:"save",name:"#assigned@test",descr:"must not save",edit_allow:"1",allow:"@ops(deploy, maintainer)"}});
    expect(refused.status).toBe(400);
    const unchanged = await req("/agent/edit?name=%23assigned@test",{cookie:s});
    const after = await unchanged.text();
    expect(after).toContain("@ops(deploy,audit)");
    expect(after).not.toContain("must not save");
  });
  const pages = ["/", "/agents", "/services", "/queues", "/pubsub", "/agents?personal=1", "/queues?personal=1", "/services?personal=1", "/groups?personal=1", "/agents/new", "/services/new", "/queues/new", "/pubsub/new",
    "/agent?name=%23helper@test", "/queue?name=jobs@test", "/pubsub/channel?name=news@test", "/service?name=db@test", "/agent/edit?name=%23helper@test",
    "/record-deactivate?name=jobs@test", "/record-danger?name=jobs@test", "/activity", "/activity?name=jobs@test", "/diagnostics",
    "/users", "/users/new", "/user?name=bob", "/user/edit?name=bob", "/user-danger?name=bob", "/user-deactivate?name=bob", "/groups", "/groups/new", "/group?name=@ops", "/group/edit?name=@ops", "/account"];
  test("every page renders, with no inline style", async () => {
    for (const p of pages) {
      const r = await req(p, { cookie: s });
      const t = await r.text();
      expect(`${p} ${r.status}`).toBe(`${p} 200`);
      expect(t).toContain('class="documentation-links" aria-label="Documentation"');
      expect(t).toContain("https://github.com/parf/ai-agent-bus/blob/main/docs/");
      expect(`${p} ${/\sstyle=/.test(t)}`).toBe(`${p} false`);
      expect(`${p} ${/<script(?![^>]*\bsrc=)/.test(t)}`).toBe(`${p} false`);
    }
  });
  test("record forms give Roles the same full width as Tags and keep useful guidance", async () => {
    for(const path of ["/agents/new?personal=1","/agent/edit?name=%23helper@test"]){
      const html=await(await req(path,{cookie:s})).text();
      const fields=html.split(/<div class="field(?= |")/);
      for(const name of ["roles","tags"]){
        const field=fields.find(part=>part.includes(`name="${name}"`));
        expect(field?.startsWith(' wide">')).toBe(true);
      }
      expect(html).not.toContain("Only the Owner changes the personal tag.");
      expect(html).toContain("Roles callers can be given, e.g. deploy, read_only. Owner and Maintainer are automatic.");
      expect(html).toContain("An empty list admits only the Owner and Maintainers.");
      expect(html).toContain("Keep secrets in Secret and config in Configuration.");
    }
  });
  test("agent add and edit help links to the fields documentation",async()=>{
    for(const path of ["/agents/new","/agent/edit?name=%23helper@test"]){
      const html=await (await req(path,{cookie:s})).text();
      for(const target of ["03-records.md#tags","02-access.md#acl","03-records-agent.md#roles","03-records-service.md#secrets"]){
        expect(html).toContain(`href="https://github.com/parf/ai-agent-bus/blob/main/docs/${target}"`);
      }
      const popovers=[...html.matchAll(/<div popover[^>]*>[\s\S]*?<\/div>/g)].map(m=>m[0]);
      expect(popovers.some(p=>p.includes('docs-link')&&p.includes('03-records-service.md#secrets'))).toBe(true);
    }
  });
  test("record status reuses listing times for Updated and Last used in its first row", async () => {
    const name="#status-times@test";
    await api("/register",{name,kind:"agent"});
    const path=`/agent?name=${encodeURIComponent(name)}`;
    const before=await (await req(path,{cookie:s})).text();
    expect(before).not.toContain("<dt>Last used</dt>");
    const token=(await api("/token",{name})).token;
    const used=await fetch("http://unix/status",{headers:{"X-Agent-Bus-Token":token},unix:join(dir,"bus.sock")} as RequestInit);
    expect(used.status).toBe(200);
    const page=await (await req(path,{cookie:s})).text();
    const status=/<section[^>]*id="status">([\s\S]*?)<\/section>/.exec(page)![1]!;
    const listing=await (await req(`/agents?q=${encodeURIComponent(name)}`,{cookie:s})).text();
    const row=[...listing.matchAll(/<tr\b[^>]*>[\s\S]*?<\/tr>/g)].map(m=>m[0]).find(row=>row.includes(name))!;
    for(const label of ["Updated","Last used"]) {
      const time=new RegExp(`<dt>${label}</dt><dd>(<time[^>]*>[^<]+</time>)</dd>`).exec(status)?.[1];
      expect(time).toBeDefined();expect(row).toContain(time!);
      expect(time).toMatch(/title="[^"]+">now<\/time>/);
    }
    expect(status).toMatch(/class="status-times">[\s\S]*?<dt>Updated<\/dt>[\s\S]*?<dt>Last used<\/dt>[\s\S]*?<\/dl>/);
  });
  test("Status and Policy share a card with the state badge in its header", async () => {
    await api("/register", {name:"layout-policy@test",kind:"queue",bound:33,ttl:"7m",overflow:"ring"});
    await api("/register", {name:"layout-resource@test",kind:"resource",resource:{uri:"https://example.com/facts"}});
    for (const [path, hasPolicy] of [["/queue?name=layout-policy@test",true], ["/agent?name=%23helper@test",false],
      ["/pubsub/channel?name=news@test",false], ["/resource?name=layout-resource@test",false], ["/service?name=db@test",false]] as const) {
      const html=await (await req(path,{cookie:s})).text();
      const card=/<section[^>]*id="status">([\s\S]*?)<\/section>/.exec(html)?.[1];
      expect(card).toBeDefined();
      expect(card).toMatch(/<header[^>]*>[\s\S]*?<h2>[\s\S]*?Status &amp; Policy[\s\S]*?<span class="pill tone-ok">[\s\S]*?Active<\/span>[\s\S]*?<\/h2>[\s\S]*?<\/header>/);
      expect(card).toContain("<dt>Updated</dt>");
      expect(card).not.toContain("<dt>Reached</dt>");
      for (const label of ["Queue bound", "Retention", "When full"]) expect(card!.includes(`<dt>${label}</dt>`)).toBe(hasPolicy);
      if(hasPolicy) expect(card!.indexOf("<dt>Updated</dt>")).toBeLessThan(card!.indexOf("<dt>Queue bound</dt>"));
      expect(html).not.toMatch(/<h2>(?:<[^>]+><\/[^>]+>)?Policy<\/h2>/);
      if(path.includes("layout-policy")) for (const value of ["33", "7m", "drop the oldest"]) expect(card).toContain(value);
    }
    const settings = await (await req("/pubsub/channel/edit?name=news@test", {cookie:s})).text();
    expect(settings).toContain("Deliver-To list");
  });
  test("a route's page says whether the configured route is allowed now", async () => {
    // Fresh names: the destination's ACL flips below, and no other page reads them.
    await api("/register", { name: "routed@test", kind: "queue", allow: ["relay@test"] });
    await api("/register", { name: "relay@test", kind: "queue", subs: ["routed@test"], allow: ["*"] });
    const allowed = "Route allowed now.</span>", refused = '<p class="warn small">Configured, but <code>routed@test</code> does not allow <code>relay@test</code>';
    const before = await (await req("/queue/edit?name=relay@test", { cookie: s })).text();
    expect(`${before.includes(allowed)} ${before.includes(refused)}`).toBe("true false");
    // The route stays configured; only the destination's allow list changes.
    await api("/manage", { name: "routed@test", allow: ["bob"] });
    const after = await (await req("/queue/edit?name=relay@test", { cookie: s })).text();
    expect(`${after.includes(allowed)} ${after.includes(refused)}`).toBe("false true");
    expect(after).toContain('name="subs" type="text" value="routed@test"');
    expect(after.indexOf(refused)).toBeGreaterThan(after.indexOf('name="subs"'));
  });
  test("a user's ssh-ed25519 key is pasted when the user is added, or later on their page", async () => {
    const edKey = () => {
      const x = Buffer.from(generateKeyPairSync("ed25519").publicKey.export({ format: "jwk" }).x!, "base64url");
      const str = (b: Buffer) => Buffer.concat([Buffer.from([0, 0, 0, b.length]), b]);
      return "ssh-ed25519 " + Buffer.concat([str(Buffer.from("ssh-ed25519")), str(x)]).toString("base64");
    };
    const keys = () => { try { return readFileSync(join(dir, "ssh", "authorized_keys"), "utf8"); } catch { return ""; } };
    const page = await (await req("/users/new", { cookie: s })).text();
    expect(page).toMatch(/<form id="form-create"[^]*<textarea id="f-ssh_key" name="ssh_key"[^]*<\/form>/);
    const first = edKey();
    const made = await req("/user", { cookie: s, form: { action: "create", name: "keyed@test", ssh_key: first + " keyed@laptop", return: "/users" } });
    expect(made.status).toBe(303);
    expect(keys()).toContain(`restrict,command="/usr/local/bin/agent-bus-token keyed@test" ${first}\n`);
    expect(keys()).not.toContain("keyed@laptop");
    // On the user's page, whoever may edit them adds or replaces it.
    const profile = await (await req("/user?name=keyed@test", { cookie: s })).text();
    expect(profile).toContain('<form id="form-ssh-key" method="post" action="/user">');
    const bad = await req("/user", { cookie: s, form: { action: "ssh-key", name: "keyed@test", ssh_key: "ssh-rsa AAAAB3NzaC1yc2E=" } });
    expect(bad.status).toBe(400);
    const badText = await bad.text();
    expect(badText).toContain("not an ssh-ed25519 public key");
    expect(badText).toMatch(/<textarea id="f-ssh_key" name="ssh_key"[^>]*>\nssh-rsa AAAAB3NzaC1yc2E=<\/textarea>/);
    const second = edKey();
    expect((await req("/user", { cookie: s, form: { action: "ssh-key", name: "keyed@test", ssh_key: second } })).status).toBe(303);
    expect(keys()).toContain(second);
    expect(keys()).not.toContain(first);
    // An ordinary user edits no profile, their own included, so gets no key form.
    const own = await (await req("/user?name=bob", { cookie: await signIn(bob) })).text();
    expect(own).toContain("<h1>");
    expect(own).not.toContain('id="form-ssh-key"');
  });
  test("signed in, /users?kind=other goes to the leftovers", async () => {
    const r = await req("/users?kind=other", { cookie: s });
    expect(r.status).toBe(303);
    expect(r.headers.get("location")).toBe("/diagnostics#leftovers");
  });
  test("a bad return on a user action cannot fail it afterwards", async () => {
    const r = await req("/user", { cookie: s, form: { action: "active", name: "bob", return: "/user?a\r\nb" } });
    expect(r.status).toBe(303);
  });
  test("an address from before 0.8.108 still answers: a moved page 301s to its new one, an old form still posts", async () => {
    for (const [from, to] of [["/pubsub/topic?name=news@test", "/pubsub/channel?name=news@test"], ["/pubsub/topic/edit?name=news@test", "/pubsub/channel/edit?name=news@test"],
                              ["/service-danger?name=jobs@test", "/record-danger?name=jobs@test"], ["/service-deactivate?name=jobs@test", "/record-deactivate?name=jobs@test"]]) {
      const r = await req(from, { cookie: s });
      expect(r.status).toBe(301);
      expect(r.headers.get("location")).toBe(to);
    }
    const old = await req("/service", { cookie: s, form: { action: "save", name: "jobs@test", descr: "Jobs", bound: "abc" } });
    expect(old.status).toBe(400); // answered by the record form, as /record answers it
    expect(await old.text()).toContain('id="form-save"');
    const look = async (name: string) => (await fetch(`http://unix/lookup?name=${encodeURIComponent(name)}`, { headers: { "X-Agent-Bus-Token": owner }, unix: join(dir, "bus.sock") } as RequestInit)).json();
    await api("/register", { name: "bridged@test", kind: "queue", descr: "before", allow: ["*"] });
    const saved = await req("/service", { cookie: s, form: { action: "save", name: "bridged@test", descr: "after" } });
    expect(saved.status).toBe(303);
    expect((await look("bridged@test")).descr).toBe("after");
    const confirm = await req("/service-confirm", { cookie: s, form: { action: "delete", name: "bridged@test" } });
    expect(confirm.status).toBe(200);
    expect(await confirm.text()).toContain('action="/record"');
  });
  test("only a non-manager recipient gets the pubsub unsubscribe notice", async () => {
    const name="layout-fanout@test", path=`/pubsub/channel?name=${name}`, recipient="#layout-recipient@test";
    await api("/register", {name:recipient,kind:"agent"});
    const token=(await api("/token",{name:recipient})).token;
    await api("/register", {name,kind:"pubsub",subs:[recipient],allow:["*"]});
    const b=await signIn(token);
    const page=await (await req(path,{cookie:b})).text();
    expect(page).not.toContain('id="deliver-to"');
    expect(page).toContain("Take my inbox off this list");
    expect(page).toContain('name="action" value="unsubscribe"');
    expect(page).not.toContain("Edit settings");
    const manager=await (await req(path,{cookie:s})).text();
    expect(manager).not.toContain("Take my inbox off this list");
    expect(manager).not.toContain('value="remove-recipient"');
    const settings=await (await req(`/pubsub/channel/edit?name=${name}`,{cookie:s})).text();
    expect(settings).toContain('name="subs"'); expect(settings).toContain(recipient);
    await api("/manage",{name,maintainers:[recipient]});
    expect(await (await req(path,{cookie:b})).text()).not.toContain("Take my inbox off this list");
    await api("/manage",{name,maintainers:[]});
    const off=await req("/record",{cookie:b,form:{action:"unsubscribe",name}});
    expect(off.status).toBe(303);
    expect(await (await req(path,{cookie:b})).text()).not.toContain("Take my inbox off this list");
  });
  test("a refused save never re-emits a foreign return", async () => {
    const r = await req("/record", { cookie: s, form: { action: "save", name: "jobs@test", descr: "Jobs", bound: "abc", return: "https://evil.example/x" } });
    expect(r.status).toBe(400);
    expect(await r.text()).not.toContain("evil.example");
  });
  test("a taken name marks the name even when a list line reads like the message", async () => {
    const r = await req("/record", { cookie: s, form: { action: "create", kind: "queue", name: "jobs@test", allow: "registered\n@owner" } });
    expect(r.status).toBe(412);
    const t = await r.text();
    expect(t).toMatch(/id="create-name"[^>]*aria-invalid="true"/);
    expect(t).toMatch(/id="f-allow"[^>]*aria-invalid="false"/);
  });
  test("a daemon refusal naming a member line marks that line", async () => {
    const r = await req("/groups", { cookie: s, form: { action: "save", name: "@ops", members: "bob\nnosuchuser" } });
    expect(r.status).toBe(404);
    const t = await r.text();
    expect(t).toMatch(/id="f-members"[^>]*data-bad-line="2"[^>]*aria-invalid="true"/);
    expect(t).toContain("Line 2:");
  });
  test("a user inbox offers no Danger Zone", async () => {
    expect(await (await req("/queue?name=owner@test", { cookie: s })).text()).not.toContain("/record-danger");
    expect((await req("/record-danger?name=owner@test", { cookie: s })).status).toBe(403);
  });
  test("deactivation starts from the Danger Zone for each active registry kind", async () => {
    for (const [detail, name] of [["/agent", "#helper@test"], ["/service", "db@test"], ["/queue", "jobs@test"], ["/pubsub/channel", "news@test"]]) {
      const q = `name=${encodeURIComponent(name)}`;
      const page = await (await req(`${detail}?${q}`, { cookie: s })).text();
      expect(page).toContain(`/record-danger?${q}`);
      expect(page).not.toContain('action="/record-deactivate"');
      const danger = await (await req(`/record-danger?${q}`, { cookie: s })).text();
      expect(danger).toContain(`href="/record-deactivate?${q}"`);
    }
    const group = await (await req("/record-danger?name=%40ops", { cookie: s })).text();
    expect(group).not.toContain("/record-deactivate");
  });
  test("user deactivation starts from the user's Danger Zone", async () => {
    const detail = await (await req("/user?name=bob", { cookie: s })).text();
    expect(detail).toContain("/user-danger?name=bob");
    expect(detail).not.toContain('action="/user-deactivate"');
    const danger = await (await req("/user-danger?name=bob", { cookie: s })).text();
    expect(danger).toContain("/user-deactivate?name=bob");
    expect((await req("/user-danger?name=owner%40test", { cookie: s })).status).toBe(403);
  });
  test("every list has exactly one Register action, empty or not", async () => {
    for (const p of ["/agents", "/services", "/pubsub?personal=1", "/pubsub", "/groups", "/groups?personal=1"]) {
      const t = await (await req(p, { cookie: s })).text();
      expect(`${p} ${(t.match(/href="\/[a-z]+\/new[^"]*"/g) ?? []).length}`).toBe(`${p} 1`);
    }
  });
  test("Personal is a filter on each kind's list, kept by the sidebar", async () => {
    const t = await (await req("/services?personal=1", { cookie: s })).text();
    expect(t).toContain("mine@test");
    expect(t).not.toContain("db@test");
    expect(t).not.toContain('aria-label="Kind"');
    expect(t).toMatch(/class="tab personal-view" aria-current="true"/);
    // The sidebar keeps the filter on every kind, and never elsewhere.
    for (const p of ["/agents", "/queues", "/pubsub", "/groups"]) expect(t).toContain(`href="${p}?personal=1" class="side-link personal"`);
    expect(t).toMatch(/<a href="\/services\?personal=1" class="side-link personal" aria-current="page"/);
    expect(t).toContain('href="/users" class="side-link "');
    // Only the columns that tell rows apart: no Type, and no Owner on your own list.
    expect(t).not.toMatch(/<th[^>]*>Type<\/th>/);
    expect(t).not.toMatch(/<th[^>]*>Owner<\/th>/);
    expect(t).not.toMatch(/<th[^>]*>Status<\/th>/);
    expect(t.match(/href="\/[a-z]+\/new[^"]*"/g)).toEqual(['href="/services/new?personal=1"']);
    // Filters and paging keep the filter too.
    expect(t).toContain('tag=personal');
    const plain = await (await req("/services", { cookie: s })).text();
    expect(plain).not.toContain("?personal=1\" class=\"side-link");
    expect(plain).toMatch(/<th[^>]*>Owner<\/th>/);
    const form = await (await req("/services/new?personal=1", { cookie: s })).text();
    expect(form).toMatch(/name="personal" value="on" checked/);
    expect(form).toContain('href="/services?personal=1"');
    // A Personal record's page leads back to its Personal list and keeps the sidebar's filter.
    const detail = await (await req("/service?name=mine@test", { cookie: s })).text();
    expect(detail).toContain('class="back-link" href="/services?personal=1"');
    expect(detail).toContain('href="/agents?personal=1" class="side-link personal"');
    const groups = await (await req("/groups?personal=1", { cookie: s })).text();
    expect(groups).toContain("@owner@test/team");
    expect(groups).not.toContain(">@ops<");
    for (const gone of ["/personal", "/personal?kind=service"]) expect((await req(gone, { cookie: s })).status).toBe(404);
  });
  test("Week and Month ranges render, with Prev and Next where they lead somewhere", async () => {
    const week = await (await req("/activity?range=week", { cookie: s })).text();
    expect(week).toContain('aria-current="true">Week</a>');
    expect(week).toMatch(/rel="prev"/);
    expect(week).not.toMatch(/rel="next"/);
    expect((week.match(/class="week-row"/g) ?? []).length).toBe(7);
    expect(week).toMatch(/<option value="jobs@test">jobs@test \(1\)<\/option>/);
    const month = await (await req("/queue?name=jobs@test&range=month", { cookie: s })).text();
    expect((month.match(/class="cal-cell/g) ?? []).length).toBe(30);
    expect(month).toContain('href="/activity?name=jobs%40test&amp;range=month"');
    // Yesterday on the daemon's clock: bun test runs in UTC, the daemon does not.
    const st = await (await fetch("http://unix/status", { headers: { "X-Agent-Bus-Token": owner }, unix: join(dir, "bus.sock") } as RequestInit)).json() as { today: number };
    const y = addDays(st.today, -1);
    const past = await (await req(`/activity?at=${y}`, { cookie: s })).text();
    expect(past).toMatch(/rel="next"/);
    expect(past).toContain(">Today</a>");
    // The dates are the title, in bold; the page does not talk about slots.
    const titled = await (await req(`/activity?range=week&at=${y}`, { cookie: s })).text();
    expect(titled).toMatch(/<span class="title-date">[A-Z][a-z]{2} \d+ – (?:[A-Z][a-z]{2} )?\d+<\/span>/);
    expect(titled).toMatch(/<title>Activity · [A-Z][a-z]{2} \d+ – /);
    expect(titled).not.toMatch(/ten-minute slots|, hourly|, daily/);
    const future = await (await req("/activity?range=week&at=991231", { cookie: s })).text();
    expect(future).not.toMatch(/rel="next"/);
    expect(future).not.toContain(">Today</a>");
  });
  test("the Activity chooser shows each record's hits in the last day", async () => {
    const t = await (await req("/activity", { cookie: s })).text();
    expect(t).toMatch(/<option value="jobs@test">jobs@test \(1\)<\/option>/);
    expect(t).not.toContain('<option value="db@test">');
    const chosen = await (await req("/activity?name=db@test", { cookie: s })).text();
    expect(chosen).toMatch(/<option value="db@test" selected>db@test \(0\)<\/option>/);
    expect(t).toMatch(/<option value="">All visible \(\d+\)<\/option>/);
  });
  test("kinds are drawn with the CLI's own glyphs, never a drawing of their own", async () => {
    const go = readFileSync(join(import.meta.dir, "../../internal/display/entity.go"), "utf8");
    for (const [p, glyph] of [["/agents", "👾"], ["/queue?name=jobs@test", "📮"], ["/groups", "👥"], ["/users", "👤"]]) {
      const t = await (await req(p, { cookie: s })).text();
      expect(go).toContain(`"${glyph}`);
      expect(`${p} ${t.includes(`>${glyph}</span>`)}`).toBe(`${p} true`);
      for (const drawn of ["bot", "inbox", "megaphone", "satellite-dish", "user-round", "users", "crown"]) expect(`${p} ${t.includes(`data-lucide="${drawn}"`)}`).toBe(`${p} false`);
    }
  });
  test("an absent name is 404 No such name, with no Try again", async () => {
    const r = await req("/agent?name=%23nobody@test", { cookie: s });
    expect(r.status).toBe(404);
    const t = await r.text();
    expect(t).toContain("No such name");
    expect(t).not.toContain("Try again");
  });
  test("a detail address redirects to its kind's own", async () => {
    const r = await req("/agent?name=jobs@test", { cookie: s });
    expect(r.status).toBe(302);
    expect(r.headers.get("location")).toBe("/queue?name=jobs@test");
  });
  test("a refused registration returns the form, keeps its values, never the secret", async () => {
    const r = await req("/record", { cookie: s, form: { action: "create", kind: "agent", name: "nohash@test", descr: "kept-descr", secret: "TOPSECRET=1", allow: "@owner" } });
    expect(r.status).toBe(400);
    const t = await r.text();
    expect(t).toContain("Check this form");
    expect(t).toContain('value="kept-descr"');
    expect(t).not.toContain("TOPSECRET");
  });
  test("a taken name is 412 and marks the name field", async () => {
    const r = await req("/record", { cookie: s, form: { action: "create", kind: "queue", name: "jobs@test" } });
    expect(r.status).toBe(412);
    expect(await r.text()).toMatch(/id="create-name"[^>]*aria-invalid="true"/);
  });
  test("record and group registration shares Maintainers with editing and stores the grant", async () => {
    for (const [kind, path, name] of [
      ["agent", "/agents/new", "#maint-agent@test"], ["service", "/services/new", "maint-service@test"],
      ["queue", "/queues/new", "maint-queue@test"], ["pubsub", "/pubsub/new", "maint-topic@test"],
      ["resource", "/resources/new", "maint-resource@test"], ["group", "/groups/new", "@maint-form"],
    ]) {
      const page = await (await req(path, { cookie: s })).text();
      expect(`${kind} ${page.includes('name="maintainers"')}`).toBe(`${kind} true`);
      const form: Record<string, string> = kind === "group"
        ? { action: "save", new: "1", name, members: "owner@test", maintainers: "bob" }
        : { action: "create", kind, name, addr: "db:5432", protocol: "postgresql", uri: "https://example.com/a.txt", maintainers: "bob" };
      const created = await req(kind === "group" ? "/groups" : "/record", { cookie: s, form });
      expect(`${kind} ${created.status}`).toBe(`${kind} 303`);
      const lookup = await fetch(`http://unix/lookup?name=${encodeURIComponent(name)}`, { headers: { "X-Agent-Bus-Token": owner }, unix: join(dir, "bus.sock") } as RequestInit);
      const record = await lookup.json() as { maintainers?: string[] };
      expect(record.maintainers).toEqual(["bob"]);
      const editPath = kind === "group" ? "/group/edit" : kind === "pubsub" ? "/pubsub/channel/edit" : `/${kind}/edit`;
      const edit = await (await req(`${editPath}?name=${encodeURIComponent(name)}`, { cookie: s })).text();
      expect(edit).toContain('name="maintainers"');
      expect(edit).toContain('>bob</textarea>');
      await api("/manage", { name, descr: "edited by the assigned Maintainer" }, bob);
    }
  });
  test("an invalid Maintainer keeps the registration form and creates no record", async () => {
    for (const group of [false, true]) {
      const name = group ? "@bad-maint-form" : "bad-maint-form@test";
      const form: Record<string, string> = group ? { action: "save", new: "1", name, members: "owner@test", maintainers: "ghost@test" }
        : { action: "create", kind: "queue", name, maintainers: "ghost@test" };
      const rejected = await req(group ? "/groups" : "/record", { cookie: s, form });
      expect(rejected.status).toBe(404);
      const html = await rejected.text();
      expect(html).toContain('>ghost@test</textarea>');
      expect(html).toMatch(/id="f-maintainers"[^>]*aria-invalid="true"/);
      const lookup = await fetch(`http://unix/lookup?name=${encodeURIComponent(name)}`, { headers: { "X-Agent-Bus-Token": owner }, unix: join(dir, "bus.sock") } as RequestInit);
      expect(lookup.status).toBe(404);
    }
  });
  test("resource registration help explains names and template formats", async () => {
    const html = await (await req("/resources/new", { cookie: s })).text();
    expect(html).toContain("It cannot be changed afterwards.</p>");
    expect(html).toContain('aria-label="About resource templates"');
    expect(html).toContain("md://notes/{name}");
    expect(html).toContain("docs%2Fintro.md");
    expect(html).toContain("md://notes/{+path}");
    expect(html).toContain("md://notes/docs/intro.md");
    expect(html).toContain("not a glob or regular expression");
    expect(html).toMatch(/<div class="field-heading"><label for="f-uri">[\s\S]*?name="template"[\s\S]*?<input id="f-uri"/);
    expect(html).toContain("read file under the root (path jail applies)");
    expect(html).toContain("mysql://realmo/{table}/schema");
    expect(html).toContain("SHOW CREATE TABLE");
    expect(html).toContain("log://{source}/tail");
    expect(html).toContain("snapshot the ring buffer");
    expect(html).toContain('href="https://datatracker.ietf.org/doc/html/rfc6570">URI Template syntax</a>');
    expect(html).toContain('href="https://modelcontextprotocol.io/specification/latest/server/resources"');
  });
  test("Personal uses a creation checkbox and an edit tag", async () => {
    for (const path of ["/agents/new", "/services/new", "/queues/new", "/pubsub/new", "/resources/new", "/groups/new", "/agent/edit?name=%23helper%40test", "/group/edit?name=%40ops"]) {
      const html = await (await req(path, { cookie: s })).text();
      if (path.includes("/edit")) {
        expect(html).toContain('name="tags"');
        expect(html).not.toContain('type="checkbox" name="personal"');
      } else {
        expect(html).toContain('type="checkbox" name="personal"');
        expect(html).toMatch(/<div class="field-heading"><label for="(?:create-name|f-name)">Name[\s\S]*?name="personal"/);
        expect(html).toContain('aria-label="About Personal records" data-tooltip=');
      }
      expect(html).not.toContain("<legend>Classification</legend>");
      expect(html).not.toMatch(/Personal<span class="hint">/);
    }
  });
  test("queue settings share one header and keep their submitted controls", async () => {
    const page=await(await req("/agent/edit?name=%23helper@test",{cookie:s})).text();
    const group=page.match(/<fieldset class="queue-fields wide">([\s\S]*?)<\/fieldset>/)?.[1] ?? "";
    expect(group).toContain("Queue");
    for(const name of ["ttl","bound","overflow"]) expect(group).toContain(`name="${name}"`);
    expect(group).toMatch(/name="bound"[^>]*value=""[^>]*placeholder="default"/);
    expect(group).not.toContain('class="hint"');
    const saved=await req("/record",{cookie:s,form:{action:"save",name:"#helper@test",ttl:"1h",bound:"12",overflow:"ring"}});
    expect(saved.status).toBe(303);
    const rec=await(await fetch(`http://unix/lookup?name=%23helper%40test`,{headers:{"X-Agent-Bus-Token":owner},unix:join(dir,"bus.sock")} as RequestInit)).json();
    expect(rec.ttl).toBe("1h"); expect(rec.bound).toBe(12); expect(rec.overflow).toBe("ring");
    await req("/record",{cookie:s,form:{action:"save",name:"#helper@test",ttl:"",bound:"",overflow:"strict"}});
  });
  test("settings link to Locks and Store and render compact empty textareas", async () => {
    const page=await(await req("/agent/edit?name=%23helper@test",{cookie:s})).text();
    expect(page).toContain('href="/locks?q=%23helper%40test"');
    expect(page).toContain('href="/kv/record?name=%23helper%40test"');
    expect(page).toMatch(/<textarea[^>]*name="secret"[^>]*rows="1"[^>]*><\/textarea>/);
    expect(page).toMatch(/<textarea[^>]*name="config"[^>]*rows="1"/);
  });
  test("Configuration is shown and edited while Secret stays hidden", async () => {
    const cases = [["#config-edit@test", "agent", "/agent/edit"], ["config-edit@test", "service", "/service/edit"], ["@config-edit", "group", "/group/edit"]];
    for (const [name, kind, path] of cases) {
      await api("/register", { name, kind, descr: "before", ...(kind === "service" ? { addr: "db:1", protocol: "https" } : {}) });
      const empty = await (await req(`${path}?name=${encodeURIComponent(name!)}`, { cookie: s })).text();
      expect(empty).toContain("No secret is stored.");
      expect(empty).toContain("No configuration is stored.");
      await api("/configure", { Name: name, Config: { token: "PRIVATE-CONFIG" } });
      await api("/secret", { name, secret: "ENV_PRIVATE=value\n" });
      const page = await (await req(`${path}?name=${encodeURIComponent(name!)}`, { cookie: s })).text();
      expect(page.indexOf('name="config"')).toBeGreaterThan(page.indexOf('name="secret"'));
      expect(page).toMatch(/<textarea[^>]*name="config"[^>]*rows="3"/);
      expect(page).toContain('aria-label="About the configuration" data-tooltip=');
      expect(page).toContain("PRIVATE-CONFIG");
      expect(page).not.toContain("ENV_PRIVATE");
      expect(page).toMatch(/<textarea[^>]*name="secret"[^>]*><\/textarea>/);
      expect(page).toMatch(/<textarea[^>]*name="config"[^>]*>\n\{\n  &quot;token&quot;: &quot;PRIVATE-CONFIG&quot;\n\}<\/textarea>/);
      expect(page).toContain("A secret is stored and not shown.");
      expect(page).toContain("Edit the stored configuration.");
      expect(page).toContain("Secret (ENV)");
      expect(page).toMatch(/<textarea[^>]*name="secret"[^>]*rows="1"/);
      const danger = await (await req(`/record-danger?name=${encodeURIComponent(name!)}`, { cookie: s })).text();
      expect(danger).not.toContain('name="config"');
      const post = kind === "group" ? "/groups" : "/record";
      const form = { action: "save", name: name!, descr: "before", ...(kind === "group" ? { members: "" } : {}) };
      const config = async () => {
        const r = await fetch(`http://unix/config?name=${encodeURIComponent(name!)}`, { headers: { "X-Agent-Bus-Token": owner }, unix: join(dir, "bus.sock") } as RequestInit);
        return r.json();
      };
      const secret = async () => (await fetch(`http://unix/secret?name=${encodeURIComponent(name!)}`, { headers: { "X-Agent-Bus-Token": owner }, unix: join(dir, "bus.sock") } as RequestInit)).text();
      // A form left open while the values change elsewhere submits them empty, which keeps what is stored now.
      await api("/configure", { Name: name, Config: { token: "PRIVATE-CONFIG" } });
      await api("/secret", { name, secret: "ENV_PRIVATE=rotated\n" });
      expect((await req(post, { cookie: s, form: { ...form, config: "", secret: "" } })).status).toBe(303);
      expect(await config()).toEqual({ token: "PRIVATE-CONFIG" });
      expect(await secret()).toBe("ENV_PRIVATE=rotated\n");
      expect((await req(post, { cookie: s, form: { ...form, config: '{"value":2}' } })).status).toBe(303);
      expect(await config()).toEqual({ value: 2 });
      const exact = '{"huge":9007199254740993}';
      expect((await req(post, { cookie: s, form: { ...form, config: exact } })).status).toBe(303);
      const raw = await fetch(`http://unix/config?name=${encodeURIComponent(name!)}`, { headers: { "X-Agent-Bus-Token": owner }, unix: join(dir, "bus.sock") } as RequestInit);
      expect(await raw.text()).toContain("9007199254740993");
      const exactPage = await (await req(`${path}?name=${encodeURIComponent(name!)}`, {cookie:s})).text();
      expect(exactPage).toContain("9007199254740993");
      const bad = await req(post, { cookie: s, form: { ...form, descr: "must-not-save", config: "INVALID-PRIVATE-CONFIG" } });
      expect(bad.status).toBe(400);
      const refused = await bad.text();
      expect(refused).toContain("Configuration must be valid JSON");
      expect(refused).toMatch(/<textarea[^>]*name="config"[^>]*>\nINVALID-PRIVATE-CONFIG<\/textarea>/);
      expect(refused).not.toContain("The submitted configuration is not shown again");
      expect(refused).not.toContain("ENV_PRIVATE");
      expect(await config()).toEqual({ huge: 9007199254740993 });
      const record = await fetch(`http://unix/lookup?name=${encodeURIComponent(name!)}`, { headers: { "X-Agent-Bus-Token": owner }, unix: join(dir, "bus.sock") } as RequestInit);
      expect(((await record.json()) as { descr: string }).descr).toBe("before");
    }
  });
  test("configuration is visible to Owner and Maintainer only, never the Secret", async () => {
    const name = "#rt16@test";
    await api("/register", {name, kind:"agent", allow:["*"], maintainers:["bob"]});
    const cli = Bun.spawnSync([join(bin, "agent-bus"), "agent-template", name, '{"channel":"RT16-CONFIG","nested":{"n":2}}'], {
      env: {...Object.fromEntries(Object.entries(process.env).filter(([k]) => !k.startsWith("AGENT_BUS_"))), AGENT_BUS_ADDR:join(dir,"bus.sock"), AGENT_BUS_TOKEN:owner},
      stdout:"pipe", stderr:"pipe",
    });
    expect(cli.exitCode).toBe(0);
    await api("/secret", {name, secret:"RT16_SECRET=never-show"});
    const b = await signIn(bob);
    for (const cookie of [s,b]) {
      const page = await (await req("/agent?name="+encodeURIComponent(name),{cookie})).text();
      expect(page).toContain('id="configuration"');
      expect(page).toContain("RT16-CONFIG");
      expect(page).toContain('\n  &quot;channel&quot;:');
      expect(page).not.toContain("never-show");
      expect(page).not.toContain("RT16_SECRET");
      const edit = await (await req("/agent/edit?name="+encodeURIComponent(name),{cookie})).text();
      expect(edit).toContain("RT16-CONFIG");
      expect(edit).not.toContain("never-show");
    }
    const getRecord = async () => (await fetch(`http://unix/lookup?name=${encodeURIComponent(name)}`, {headers:{"X-Agent-Bus-Token":owner},unix:join(dir,"bus.sock")} as RequestInit)).json() as Promise<{config_sha:string}>;
    const before = (await getRecord()).config_sha;
    const pretty = '{\n  "channel": "RT16-CONFIG",\n  "nested": {"n": 2}\n}';
    expect((await req("/record", {cookie:b, form:{action:"save",name,descr:"",config:pretty}})).status).toBe(303);
    expect((await getRecord()).config_sha).toBe(before);
    await api("/manage", {name, maintainers:[]});
    const ordinary = await req("/agent?name="+encodeURIComponent(name),{cookie:b});
    expect(ordinary.status).toBe(200);
    const hidden = await ordinary.text();
    expect(hidden).not.toContain("RT16-CONFIG");
    expect(hidden).not.toContain(before);
    expect(hidden).not.toContain('id="configuration"');
    expect((await req("/record", {cookie:b, form:{action:"save",name,descr:"",config:pretty}})).status).toBe(403);
  });
  test("a registration succeeds and the next page says so once", async () => {
    const r = await req("/record", { cookie: s, form: { action: "create", kind: "queue", name: "fresh@test", descr: "Fresh" } });
    expect(r.status).toBe(303);
    expect(r.headers.get("location")).toBe("/queue?name=fresh%40test");
    const flash = /ab_flash=([^;]+)/.exec(r.headers.get("set-cookie")!)![1]!;
    const next = await handle(new Request(ORIGIN + "/queue?name=fresh@test", { headers: { host: "127.0.0.1:6781", cookie: `agent_bus_session=${s}; ab_flash=${flash}` } }));
    expect(await next.text()).toContain("Registered.");
    expect(next.headers.get("set-cookie")).toContain("ab_flash=; ");
  });
  test("transfer and delete happen only through their confirmation", async () => {
    for (const action of ["transfer", "delete"]) {
      const r = await req("/record", { cookie: s, form: { action, name: "fresh@test", owner: "bob", expected_owner: "owner@test" } });
      expect(`${action} ${r.status}`).toBe(`${action} 400`);
    }
    const stale = await req("/record", { cookie: s, form: { action: "delete", name: "fresh@test", expected_owner: "owner@test", expected_queued: "7", expected_readers: "0", confirmed: "1" } });
    expect(stale.status).toBe(409);
    expect(await stale.text()).toContain("The conditions changed");
    const ok = await req("/record", { cookie: s, form: { action: "delete", name: "fresh@test", expected_owner: "owner@test", expected_queued: "0", expected_readers: "0", confirmed: "1" } });
    expect(ok.status).toBe(303);
  });
  test("a Personal group must carry its owner's prefix, refused before any call", async () => {
    const r = await req("/groups", { cookie: s, form: { action: "save", new: "1", name: "@team", members: "", personal: "on" } });
    expect(r.status).toBe(400);
    expect(await r.text()).toContain("call it @owner@test/&lt;name&gt;");
  });
  test("registering an existing group name is refused, not a silent replace", async () => {
    const r = await req("/groups", { cookie: s, form: { action: "save", new: "1", name: "@ops", members: "" } });
    expect(r.status).toBe(409);
  });
  test("the palette lists what this visitor may see", async () => {
    const r = await req("/palette.json", { cookie: s, headers: { "sec-fetch-site": "same-origin" } });
    const j = await r.json() as { entries: { title: string }[] };
    expect(j.entries.map(e => e.title)).toEqual(expect.arrayContaining(["jobs@test", "#helper@test", "bob", "@ops"]));
  });
});

describe("pages as an ordinary user", () => {
  let s = "";
  beforeAll(async () => { s = await signIn(bob); });
  test("the directory holds only their own row and no Register", async () => {
    const t = await (await req("/users", { cookie: s })).text();
    expect(t).toContain("/user?name=bob");
    expect(t).not.toContain("/user?name=owner%40test");
    expect(t).not.toContain('href="/users/new"');
  });
  test("their palette omits what they may not see", async () => {
    const j = await (await req("/palette.json", { cookie: s, headers: { "sec-fetch-site": "same-origin" } })).json() as { entries: { title: string }[] };
    const titles = j.entries.map(e => e.title);
    expect(titles).toContain("jobs@test");
    expect(titles).not.toContain("#helper@test");
    expect(titles).not.toContain("db@test");
  });
  test("a group whose record they cannot see says so, not a blank owner", async () => {
    const t = await (await req("/groups", { cookie: s })).text();
    const row = t.slice(t.indexOf("@hidden"), t.indexOf("</tr>", t.indexOf("@hidden")));
    expect(row).toContain("Not visible to you");
  });
  test("a record they may not see is No such name", async () => {
    const r = await req("/agent?name=%23helper@test", { cookie: s });
    expect(r.status).toBe(404);
  });
  test("a record they see but do not manage offers no controls", async () => {
    const t = await (await req("/queue?name=jobs@test", { cookie: s })).text();
    expect(t).not.toContain("Deactivate…");
    expect(t).not.toContain("/record-danger");
    expect((await req("/queue/edit?name=jobs@test", { cookie: s })).status).toBe(403);
  });
  test("palette refuses a cross-site fetch and a signed-out one", async () => {
    expect((await req("/palette.json", { cookie: s, headers: { "sec-fetch-site": "cross-site" } })).status).toBe(403);
    expect((await req("/palette.json", { headers: { "sec-fetch-site": "same-origin" } })).status).toBe(401);
    expect((await req("/palette.json", { cookie: s })).status).toBe(403);
  });
});

describe("locks, resources and the nav", () => {
  let s = "";
  async function as(path: string, token: string, body: unknown) {
    const r = await fetch(`http://unix${path}`, { method: "POST", body: JSON.stringify(body), headers: { "X-Agent-Bus-Token": token }, unix: join(dir, "bus.sock") } as RequestInit);
    return { status: r.status, text: await r.text() };
  }
  beforeAll(async () => {
    s = await signIn(owner);
    await api("/register", { name: "pair@test", kind: "queue", allow: ["*"] });
    await api("/manage", { name: "pair@test", maintainers: ["bob"] });
    await api("/register", { name: "notes@test", kind: "resource", descr: "Team notes", allow: ["*"], resource: { uri: "md://notes/{+path}", template: true, source: "#helper@test", mimeType: "text/markdown" } });
    await api("/register", { name: "readme@test", kind: "resource", resource: { uri: "https://example.com/README.md" } });
  });

  test("the nav puts Users and Groups right after Overview, and Resources after PubSub", async () => {
    const t = await (await req("/", { cookie: s })).text();
    const at = (href: string) => t.indexOf(`href="${href}"`);
    expect([at("/"), at("/users"), at("/groups"), at("/agents"), at("/pubsub"), at("/resources"), at("/activity")].every((v, i, a) => v >= 0 && (i === 0 || v > a[i - 1]!))).toBe(true);
  });

  test("a record's page shows its locks to its Owner, with time left and force release behind a confirmation", async () => {
    expect((await as("/lock", bob, { record: "pair@test", name: "deploy", ttl: "5m" })).status).toBe(200);
    const t = await (await req("/queue?name=pair@test", { cookie: s })).text();
    expect(t).toMatch(/<td data-label="Name"><code>deploy<\/code><\/td>/);
    expect(t).toMatch(/<td data-label="Holder"><code>bob<\/code><\/td>/);
    expect(t).toMatch(/<td data-label="Time left">[45]m<\/td>/);
    expect(t).toMatch(/<details class="inline-confirm"><summary[^>]*>Force release<\/summary>/);
  });

  test("someone who does not manage the record sees no Locks card", async () => {
    const b = await signIn(bob);
    expect(await (await req("/queue?name=jobs@test", { cookie: b })).text()).not.toMatch(/<h2>(?:<[^>]+><\/[^>]+>)?Locks<\/h2>/);
    expect(await (await req("/queue?name=pair@test", { cookie: b })).text()).toMatch(/<h2>(?:<[^>]+><\/[^>]+>)?Locks<\/h2>/);
  });

  test("a plain release of someone else's lock is the daemon's refusal, not a success", async () => {
    const r = await req("/release-lock", { cookie: s, form: { record: "pair@test", name: "deploy" } });
    expect(r.status).toBe(409);
    expect((await as("/lock", owner, { record: "pair@test", name: "deploy", ttl: "1m" })).status).toBe(409);
  });

  test("a force release releases it, and says so", async () => {
    const r = await req("/release-lock", { cookie: s, form: { record: "pair@test", name: "deploy", force: "1" } });
    expect(r.status).toBe(303);
    expect(r.headers.get("set-cookie")).toContain("ab_flash=lock-force-released");
    expect(r.headers.get("location")).toBe("/queue?name=pair%40test#locks");
    expect((await as("/lock", owner, { record: "pair@test", name: "deploy", ttl: "1m" })).status).toBe(200);
  });

  test("the Personal filter keeps Resources in the nav", async () => {
    const t = await (await req("/agents?personal=1", { cookie: s })).text();
    expect(t).toContain('href="/resources?personal=1"');
  });

  test("the Overview strip counts resources", async () => {
    expect(await (await req("/", { cookie: s })).text()).toMatch(/<a[^>]*href="\/resources"[^>]*class="[^"]*tile|class="[^"]*tile[^"]*"[^>]*href="\/resources"/);
  });

  test("a resource's page says nothing about delivery", async () => {
    expect(await (await req("/resource?name=notes@test", { cookie: s })).text()).not.toMatch(/<span class="pill">(?:<[^>]+>[^<]*<\/[^>]+>)?Delivery:/);
    expect(await (await req("/queue?name=jobs@test", { cookie: s })).text()).toMatch(/<span class="pill">(?:<[^>]+>[^<]*<\/[^>]+>)?Delivery:/);
  });

  test("a queue filter in a Resources link does not empty the list", async () => {
    const t = await (await req("/resources?readers=none&work=held&sort=queued", { cookie: s })).text();
    expect(t).toMatch(/<td[^>]*data-label="URI"><code>md:\/\/notes\/\{\+path\}<\/code><\/td>/);
  });

  test("the holder sees their own lock with Release, and releasing says so", async () => {
    const b = await signIn(bob);
    expect((await as("/lock", bob, { record: "pair@test", name: "mine", ttl: "5m" })).status).toBe(200);
    const t = await (await req("/queue?name=pair@test", { cookie: b })).text();
    expect(t).toMatch(/<td data-label="Holder"><code>bob<\/code> <span class="pill tone-accent">you<\/span><\/td>/);
    expect(t).toMatch(/<input type="hidden" name="name" value="mine"><button class="btn btn-sm">Release<\/button>/);
    const r = await req("/release-lock", { cookie: b, form: { record: "pair@test", name: "mine" } });
    expect(r.status).toBe(303);
    expect(r.headers.get("set-cookie")).toContain("ab_flash=lock-released");
    expect(r.headers.get("location")).toBe("/queue?name=pair%40test#locks");
  });

  test("the Locks page lists every lock on records the viewer may use, with kind and search filters", async () => {
    expect((await as("/lock", owner, { record: "pair@test", name: "page-a", ttl: "5m" })).status).toBe(200);
    expect((await as("/lock", owner, { record: "db@test", name: "page-b", ttl: "5m" })).status).toBe(200);
    const t = await (await req("/locks", { cookie: s })).text();
    expect(t).toMatch(/<td data-label="Record"><div class="rec-cell">[\s\S]*?<a href="\/queue\?name=pair%40test#locks"><code>pair@test<\/code><\/a><\/div><\/td><td data-label="Lock"><code>page-a<\/code><\/td>/);
    expect(t).toMatch(/<td data-label="Lock"><code>page-b<\/code><\/td>/);
    // The page's own form carries the way back, not only the test's.
    expect(t).toMatch(/<form method="post" action="\/release-lock" class="inline-form"><input type="hidden" name="record" value="pair@test"><input type="hidden" name="name" value="page-a"><input type="hidden" name="return" value="\/locks">/);
    const byRecord = await (await req("/locks?q=pair", { cookie: s })).text();
    expect(byRecord).toMatch(/<td data-label="Lock"><code>page-a<\/code><\/td>/);
    expect(byRecord).not.toMatch(/<td data-label="Lock"><code>page-b<\/code><\/td>/);
    expect(await (await req("/locks?q=owner@test", { cookie: s })).text()).toMatch(/<td data-label="Lock"><code>page-b<\/code><\/td>/);
    const queues = await (await req("/locks?kind=queue", { cookie: s })).text();
    expect(queues).toMatch(/<td data-label="Lock"><code>page-a<\/code><\/td>/);
    expect(queues).not.toMatch(/<td data-label="Lock"><code>page-b<\/code><\/td>/);
    const found = await (await req("/locks?q=page-b", { cookie: s })).text();
    expect(found).toMatch(/<td data-label="Lock"><code>page-b<\/code><\/td>/);
    expect(found).not.toMatch(/<td data-label="Lock"><code>page-a<\/code><\/td>/);
    // bob maintains pair@test and nothing else here.
    const b = await signIn(bob);
    const theirs = await (await req("/locks", { cookie: b })).text();
    expect(theirs).toMatch(/<td data-label="Lock"><code>page-a<\/code><\/td>/);
    expect(theirs).not.toMatch(/<td data-label="Lock"><code>page-b<\/code><\/td>/);
  });

  test("a release from the Locks page comes back to it", async () => {
    const r = await req("/release-lock", { cookie: s, form: { record: "db@test", name: "page-b", return: "/locks" } });
    expect(r.status).toBe(303);
    expect(r.headers.get("location")).toBe("/locks");
    for (const ret of ["https://evil.example/", "/locks?x"]) {
      expect((await as("/lock", owner, { record: "pair@test", name: "guard", ttl: "1m" })).status).toBe(200);
      const g = await req("/release-lock", { cookie: s, form: { record: "pair@test", name: "guard", return: ret } });
      expect(`${ret} ${g.headers.get("location")}`).toBe(`${ret} /queue?name=pair%40test#locks`);
    }
  });

  test("the nav carries Locks after Resources", async () => {
    const t = await (await req("/", { cookie: s })).text();
    expect(t.indexOf('href="/locks"')).toBeGreaterThan(t.indexOf('href="/resources"'));
    expect(t.indexOf('href="/resources"')).toBeGreaterThan(0);
  });

  test("the Resources list draws 📚 and 🧩 and shows each card's URI and source", async () => {
    const t = await (await req("/resources", { cookie: s })).text();
    expect(t).toMatch(/aria-label="Resource Template">🧩<\/span>/);
    expect(t).toMatch(/aria-label="Resource">📚<\/span>/);
    expect(t).toMatch(/<td[^>]*data-label="URI"><code>md:\/\/notes\/\{\+path\}<\/code><\/td>/);
    expect(t).toMatch(/<td[^>]*data-label="Source"><code>#helper@test<\/code><\/td>/);
    expect(t).not.toMatch(/<th[^>]*>Readers<\/th>/);
  });

  test("a resource's page shows its card and no queue", async () => {
    const t = await (await req("/resource?name=notes@test", { cookie: s })).text();
    expect(t).toMatch(/<h2>(?:<[^>]+>[^<]*<\/[^>]+>)?Resource template<\/h2>/);
    expect(t).toContain("md://notes/{+path}");
    expect(t).not.toMatch(/<h2>(?:<[^>]+><\/[^>]+>)?Queue &amp; counters<\/h2>/);
  });

  test("a resource is registered from its form, and a card without a source is the daemon's refusal", async () => {
    const ok = await req("/record", { cookie: s, form: { action: "create", kind: "resource", name: "web@test", uri: "https://example.com/a.txt", descr: "A" } });
    expect(ok.status).toBe(303);
    expect(ok.headers.get("location")).toContain("/resource?name=web%40test");
    const bad = await req("/record", { cookie: s, form: { action: "create", kind: "resource", name: "nosrc@test", uri: "md://x/a.md" } });
    expect(bad.status).toBe(400);
    expect(await bad.text()).toContain("names the agent or mcp service that answers it");
  });
});

describe("key-value stores", () => {
  let s = "";
  beforeAll(async () => {
    s = await signIn(owner);
    // bob maintains kvq@test, and is only on kvother@test's allow list.
    await api("/register", { name: "kvq@test", kind: "queue", allow: ["*"] });
    await api("/manage", { name: "kvq@test", maintainers: ["bob"] });
    await api("/register", { name: "kvother@test", kind: "queue", allow: ["*"] });
    await api("/kv/set", { record: "kvq@test", name: "stage", value: "built" });
    await api("/kv/set", { record: "kvother@test", name: "secretish", value: "owner only" });
  });

  test("a record's page shows its store to its Owner and a Maintainer, and to nobody on its allow list", async () => {
    const own = await (await req("/queue?name=kvq@test", { cookie: s })).text();
    expect(own).toMatch(/<section class="card  " id="kv">[^]*<code>stage<\/code>[^]*<code class="kv-preview">built<\/code>/);
    expect(own).toContain('href="/kv/record?name=kvq%40test"');
    const b = await signIn(bob);
    expect(await (await req("/queue?name=kvq@test", { cookie: b })).text()).toContain('id="kv"');
    const other = await (await req("/queue?name=kvother@test", { cookie: b })).text();
    expect(other).toContain("<h1>");
    expect(other).not.toContain('id="kv"');
    expect(other).not.toContain("owner only");
    expect((await req("/kv/record?name=kvother@test", { cookie: b })).status).toBe(403);
  });

  test("the KV page lists the stores the viewer may use, with search and a kind filter", async () => {
    const t = await (await req("/kv", { cookie: s })).text();
    expect(t).toMatch(/<a class="btn btn-sm" href="\/kv\/record\?name=kvq%40test">Open<\/a>/);
    expect(t).toMatch(/<a class="btn btn-sm" href="\/kv\/record\?name=kvother%40test">Open<\/a>/);
    const found = await (await req("/kv?q=kvother", { cookie: s })).text();
    expect(found).not.toContain('href="/kv/record?name=kvq%40test"');
    expect(found).toContain('href="/kv/record?name=kvother%40test"');
    const b = await signIn(bob);
    const theirs = await (await req("/kv", { cookie: b })).text();
    expect(theirs).toContain('href="/kv/record?name=kvq%40test"');
    expect(theirs).not.toContain('href="/kv/record?name=kvother%40test"');
  });

  test("a value is added, edited, counted and deleted through the forms, an integer past 2^53 kept exact", async () => {
    const big = "4611686018427387903";
    const add = await req("/kv-set", { cookie: s, form: { record: "kvq@test", kind: "int", name: "runs", value: big, how: "add", return: "store" } });
    expect(`${add.status} ${add.headers.get("location")}`).toBe("303 /kv/record?name=kvq%40test");
    const store = await (await req("/kv/record?name=kvq@test", { cookie: s })).text();
    expect(store).toContain(`<code class="kv-preview">${big}</code>`);
    // A kind with no values has no card.
    expect(store).toContain('id="kv-int"');
    expect(store).not.toContain('id="kv-json"');
    expect(store).not.toContain("None.");
    const dec = await req("/kv-inc", { cookie: s, form: { record: "kvq@test", kind: "int", name: "runs", n: "-1" } });
    expect(dec.status).toBe(303);
    const value = await (await req("/kv/value?record=kvq@test&kind=int&name=runs", { cookie: s })).text();
    expect(value).toContain('value="4611686018427387902"');
    // A second add is refused and says so, the typed value kept.
    const again = await req("/kv-set", { cookie: s, form: { record: "kvq@test", kind: "int", name: "runs", value: "7", how: "add", return: "store" } });
    expect(again.status).toBe(412);
    expect(await again.text()).toContain("already stored");
    // JSON edited whole; one that is not an object is refused with the text kept.
    expect((await req("/kv-set", { cookie: s, form: { record: "kvq@test", kind: "json", name: "doc", value: '{"a":1}', return: "store" } })).status).toBe(303);
    const bad = await req("/kv-set", { cookie: s, form: { record: "kvq@test", kind: "json", name: "doc", value: "[1, 2]", how: "replace", return: "value" } });
    expect(bad.status).toBe(400);
    expect(await bad.text()).toMatch(/<textarea id="f-value" name="value"[^>]*>\n\[1, 2\]<\/textarea>/);
    // A save on the value page is done, and goes back to the store.
    const saved = await req("/kv-set", { cookie: s, form: { record: "kvq@test", kind: "json", name: "doc", value: '{"a":2}', how: "replace", return: "value" } });
    expect(`${saved.status} ${saved.headers.get("location")}`).toBe("303 /kv/record?name=kvq%40test");
    const del = await req("/kv-delete", { cookie: s, form: { record: "kvq@test", kind: "json", name: "doc" } });
    expect(del.status).toBe(303);
    expect(await (await req("/kv/record?name=kvq@test", { cookie: s })).text()).not.toContain("<code>doc</code>");
  });

  test("an unedited Save on the value page stores the value unchanged", async () => {
    // What a browser submits from the page: the parser drops one newline after
    // <textarea>, and a form sends every line break as CRLF.
    const resubmit = async (kind: string, name: string) => {
      const page = await (await req(`/kv/value?record=kvq@test&kind=${kind}&name=${name}`, { cookie: s })).text();
      const text = /<textarea id="f-value"[^>]*>([^]*?)<\/textarea>/.exec(page)![1]!.replace(/^\n/, "")
        .replace(/&quot;/g, '"').replace(/&#39;/g, "'").replace(/&lt;/g, "<").replace(/&gt;/g, ">").replace(/&amp;/g, "&").replace(/\r?\n/g, "\r\n");
      const r = await req("/kv-set", { cookie: s, form: { record: "kvq@test", kind, name, value: text, how: "replace", return: "value" } });
      expect(r.status).toBe(303);
    };
    const raw = async (kind: string, name: string) => {
      const r = await fetch(`http://unix/kv?record=kvq@test&kind=${kind}&name=${name}`, { headers: { "X-Agent-Bus-Token": owner }, unix: join(dir, "bus.sock") } as RequestInit);
      return /"value":(.*)}$/.exec((await r.text()).trim())![1];
    };
    const doc = '{"id":12345678901234567890,"n":[1e400,2]}';
    await fetch("http://unix/kv/set", { method: "POST", body: `{"record":"kvq@test","kind":"json","name":"ids","value":${doc}}`, headers: { "X-Agent-Bus-Token": owner }, unix: join(dir, "bus.sock") } as RequestInit);
    expect(await raw("json", "ids")).toBe(doc);
    await resubmit("json", "ids");
    expect(await raw("json", "ids")).toBe(doc);
    // A typed integer past 2^53 is stored as typed.
    const typed = await req("/kv-set", { cookie: s, form: { record: "kvq@test", kind: "json", name: "ids", value: '{"id": 98765432109876543210}', how: "replace", return: "value" } });
    expect(typed.status).toBe(303);
    expect(await raw("json", "ids")).toBe('{"id":98765432109876543210}');
    await api("/kv/set", { record: "kvq@test", name: "script", value: "\nfirst\nsecond\n" });
    await resubmit("string", "script");
    expect(await raw("string", "script")).toBe(JSON.stringify("\nfirst\nsecond\n"));
  });

  test("an agent's settings say the roles it understands, and its page shows them", async () => {
    await api("/register", { name: "#roled@test", kind: "agent", allow: ["@owner"] });
    const edit = await (await req("/agent/edit?name=%23roled@test", { cookie: s })).text();
    expect(edit).toMatch(/<input id="f-roles" name="roles"/);
    const saved = await req("/record", { cookie: s, form: { action: "save", name: "#roled@test", descr: "roled", roles: "deploy, read_only" } });
    expect(saved.status).toBe(303);
    const page = await (await req("/agent?name=%23roled@test", { cookie: s })).text();
    expect(page).toMatch(/<section class="card  " id="roles">[^]*Understands: deploy, read_only/);
    // A name the daemon refuses comes back with the text kept.
    const bad = await req("/record", { cookie: s, form: { action: "save", name: "#roled@test", descr: "roled", roles: "Read-Only" } });
    expect(bad.status).toBe(400);
    expect(await bad.text()).toContain('value="Read-Only"');
  });
  test("the nav carries KV after Locks", async () => {
    const t = await (await req("/", { cookie: s })).text();
    expect(t.indexOf('href="/kv"')).toBeGreaterThan(t.indexOf('href="/locks"'));
    expect(t.indexOf('href="/locks"')).toBeGreaterThan(0);
  });
});

describe("the daemon unreachable", () => {
  test("502 The bus is not answering, and the socket path never shows", async () => {
    const h = makeHandler({ daemon: join(dir, "missing.sock"), listen: { hostname: "127.0.0.1", port: 6781 }, dev: false });
    const r = await h(new Request(ORIGIN + "/agents", { headers: { host: "127.0.0.1:6781", cookie: "agent_bus_session=abcdef0123456789" } }));
    expect(r.status).toBe(502);
    const t = await r.text();
    expect(t).toContain("The bus is not answering");
    expect(t).not.toContain("missing.sock");
  });
});

test("agent metadata pages show every declaration but never Secret values", async () => {
  const name = "#metadata@test", sentinel = "rt9-secret-value-must-not-render";
  await api("/register", { name, kind: "agent" });
  await api("/secret", { Name: name, Secret: `TOKEN=${sentinel}` });
  const token = (await api("/token", { name })).token;
  let serving = true;
  const asks: string[] = [];
  let bare = false;
  const loop = (async () => {
    while (serving) {
      const response = await fetch("http://unix/consume?wait=100ms", { headers: { "X-Agent-Bus-Token": token }, unix: join(dir, "bus.sock") } as RequestInit);
      const raw = await response.text();
      const e = raw ? JSON.parse(raw) : null;
      if (!e) continue;
      asks.push(e.ask);
      const body = JSON.stringify({ body_formats: ["text", "json"], description: "Metadata fixture purpose", version: "1.2.3", license: "MIT", author: "Fixture author", homepage: "https://example.com/agent", methods: { help: { help: "Fixture help entry", args: [] }, destroy: { help: "Delete fixture", args: [{ name: "mode", help: "Choose format", type: ["text", "json"], optional: true, rest: true }], returns: "Deletion result", role: ["operate", "admin"], destructive: true } }, config: { mode: { type: ["text", "json"], help: "Formatting mode", example: "text", default: "json" } }, secrets: [{ name: "TOKEN", help: "API token purpose", required: true, value: sentinel }] });
      await api("/send", { to: e.from, topic: e.topic, tag: e.tag, re: e.id, body: e.ask === "status" ? "unsupported: status" : bare ? (e.ask === "help" ? "Bare fixture help" : "unknown ask") : body }, token);
    }
  })();
  try {
    const c = await signIn(owner);
    // The outstanding consume publishes the fixture's readiness.
    for (let i = 0; i < 100; i++) {
      const r = await fetch(`http://unix/lookup?name=${encodeURIComponent(name)}`, { headers: { "X-Agent-Bus-Token": owner }, unix: join(dir, "bus.sock") } as RequestInit);
      if ((await r.json() as any).readers) break;
      await Bun.sleep(5);
    }
    const page = await (await req(`/agent?name=${encodeURIComponent(name)}`, { cookie: c })).text();
    for (const item of ["About this agent", "Metadata fixture purpose", "1.2.3", "MIT", "Fixture author", "https://example.com/agent", "Methods", "destroy", "Choose format", "text | json", "optional", "Deletion result", "operate or admin", "⚠ destructive", "API token purpose", "required"]) expect(page).toContain(item);
    expect(page).not.toContain(sentinel);
    expect(page).toContain('Methods (1)');
    expect(page).toContain('<details class="method-row" id="method-destroy">');
    expect(page).not.toContain('Fixture help entry');
    expect(page).toContain('id="configuration"');
    expect(page).toContain("Formatting mode");
    expect(page).toContain("No configuration is stored.");
    await api("/configure", { Name: name, Config: { mode: "text", extra: true } });
    const filtered = await (await req(`/agent?name=${encodeURIComponent(name)}&m=not-matching`, { cookie: c })).text();
    expect(filtered).toContain('0 of 1 methods match.');
    expect(filtered).not.toContain('id="method-destroy"');
    expect(filtered).toContain('name="m" value="not-matching"');
    const edit = await (await req(`/agent/edit?name=${encodeURIComponent(name)}`, { cookie: c })).text();
    for (const item of ["Formatting mode", "Example:", "Default:", "text | json", "Undocumented Configuration keys: extra", "Expected Secret keys", "TOKEN", "API token purpose"]) expect(edit).toContain(item);
    expect(edit).not.toContain(sentinel);
    expect(asks).toEqual(["help-json", "help-json"]);
    expect(await (await req(`/agent-status?name=${encodeURIComponent(name)}`, {cookie:c})).text()).toBe("");
    expect(asks.at(-1)).toBe("status");
    bare = true;
    await api("/register", { name, kind: "agent", descr: "changed metadata" });
    const fallback = await (await req(`/agent?name=${encodeURIComponent(name)}`, { cookie: c })).text();
    expect(fallback).toContain("Bare fixture help");
    expect(fallback).not.toContain("<th>Method</th>");
    expect(fallback).not.toContain("Expected Secret keys");
    expect(asks.slice(-2)).toEqual(["help-json", "help"]);
  } finally { serving = false; await loop; }
}, 15000);

test("shared tag badges and self owners hide empty tags on every record view", async () => {
  const c = await signIn(owner);
  const name = "#badge-fixture@test";
  await api("/register", {name, kind:"agent", tags:["messaging"]});
  let page = await (await req("/agents?q=badge-fixture", {cookie:c})).text();
  expect(page).toMatch(/<a class="record-tag[^\"]*" href="[^\"]*tag=messaging[^\"]*">messaging<\/a>/);
  expect(page).toMatch(/<td[^>]*data-label="Owner"><span class="pill tone-accent">you<\/span><\/td>/);
  page = await (await req("/agents?scope=my&q=badge-fixture", {cookie:c})).text();
  expect(page).not.toMatch(/<th[^>]*>Owner<\/th>/);
  await api("/register", {name:"#badge-foreign@test",kind:"agent"}, bob);
  const foreign=await (await req("/agents?q=badge-foreign",{cookie:c})).text();
  expect(foreign).toMatch(/<td[^>]*data-label="Owner"><code>bob<\/code><\/td>/);
  await api("/manage", {name, personal:true});
  page = await (await req(`/agent?name=${encodeURIComponent(name)}`, {cookie:c})).text();
  expect(page).toMatch(/<a class="record-tag tag-personal"[^>]*>personal<\/a>/);
  await api("/manage", {name, personal:false, tags:[]});
  for (const path of ["/agents?q=badge-fixture", `/agent?name=${encodeURIComponent(name)}`, `/agent/edit?name=${encodeURIComponent(name)}`]) {
    const html = await (await req(path, {cookie:c})).text();
    // Exclude the inline application bundle and stylesheet: assert rendered elements.
    const content = html.replace(/<script\b[^>]*>[\s\S]*?<\/script>/g, "").replace(/<style\b[^>]*>[\s\S]*?<\/style>/g, "");
    expect(content).not.toContain("no-tags");
    expect(content).not.toContain('aria-label="Tags"');
  }
});

test("bus status is fetched on demand and the offline page has no status request", async () => {
 const c=await signIn(owner);
 const status=await req("/agent-status?name=%23bus", {cookie:c});
 expect(status.status).toBe(200);
 expect(status.headers.get("cache-control")).toBe("no-store");
 expect(await status.text()).toMatch(/<span class="pill tone-ok">ok<\/span>.*records/);
 const page=await (await req("/agent?name=%23bus", {cookie:c})).text();
 expect(page).toContain('data-agent-status="/agent-status?name=%23bus"');
 const offline=await (await req("/agent?name=%23helper%40test", {cookie:c})).text();
 expect(offline).not.toContain('data-agent-status=');
});

test("record settings is primary while its store stays secondary", async () => {
 const c=await signIn(owner);
 const page=await (await req("/agent?name=%23helper%40test",{cookie:c})).text();
 const links=page.match(/<a\b[^>]*>[\s\S]*?<\/a>/g)!;
 const edit=links.find(link=>link.endsWith('Edit settings</a>'))!;
 expect(edit).toContain('class="btn btn-primary"');
 const store=links.find(link=>link.endsWith('Open store</a>'))!;
 expect(store).toContain('btn-secondary');expect(store).not.toContain('btn-primary');
 expect(page).toMatch(/<a[^>]*class="side-link [^"]*"[^>]*aria-current="page"/);
});
