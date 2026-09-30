// The face against a disposable daemon: every request goes through the real
// handler and the real daemon over its socket. AGENT_BUS_BIN_DIR names built
// Go programs; without it they are built into a temporary directory.
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
    const b = Bun.spawnSync(["go", "build", "-o", bin + "/", "./cmd/agent-busd", "./cmd/agent-bus-token"], { cwd: join(import.meta.dir, "../.."), stderr: "pipe" });
    if (b.exitCode) throw new Error(b.stderr.toString());
  }
  daemon = Bun.spawn([join(bin, "agent-busd"), "-addr", "127.0.0.1:0", "-socket", join(dir, "bus.sock"), "-owner", "owner@test", "-db", join(dir, "bus.db"), "-create", "-flush-every", "0", "-dashboard", ""], { stdout: "ignore", stderr: "ignore" });
  const user = join(dir, `user-${userInfo().username}.sock`);
  for (let i = 0; i < 100 && !existsSync(user); i++) await Bun.sleep(50);
  owner = Bun.spawnSync([join(bin, "agent-bus-token"), "owner@test"], { env: { ...process.env, AGENT_BUS_ADDR: user } }).stdout.toString().trim();
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
  test("the homepage is the landing page", async () => {
    const r = await req("/");
    expect(r.status).toBe(200);
    const t = await r.text();
    expect(t).toContain("One bus for agents, bots and services");
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
    const r = await req("/service", { form: { action: "create" }, headers: { "sec-fetch-site": "cross-site" } });
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
  const pages = ["/", "/agents", "/services", "/queues", "/pubsub", "/agents?personal=1", "/queues?personal=1", "/services?personal=1", "/groups?personal=1", "/agents/new", "/services/new", "/queues/new", "/pubsub/new",
    "/agent?name=%23helper@test", "/queue?name=jobs@test", "/pubsub/topic?name=news@test", "/service?name=db@test", "/agent/edit?name=%23helper@test",
    "/service-deactivate?name=jobs@test", "/service-danger?name=jobs@test", "/activity", "/activity?name=jobs@test", "/diagnostics",
    "/users", "/users/new", "/user?name=bob", "/user/edit?name=bob", "/user-danger?name=bob", "/user-deactivate?name=bob", "/groups", "/groups/new", "/group?name=@ops", "/group/edit?name=@ops", "/account"];
  test("every page renders, with no inline style", async () => {
    for (const p of pages) {
      const r = await req(p, { cookie: s });
      const t = await r.text();
      expect(`${p} ${r.status}`).toBe(`${p} 200`);
      expect(`${p} ${/\sstyle=/.test(t)}`).toBe(`${p} false`);
      expect(`${p} ${/<script(?![^>]*\bsrc=)/.test(t)}`).toBe(`${p} false`);
    }
  });
  test("a topic's page has no queue policy, which a queue's page has", async () => {
    const policy = /<h2>(?:<[^>]+><\/[^>]+>)?Policy<\/h2>/;
    const queue = await (await req("/queue?name=jobs@test", { cookie: s })).text();
    expect(policy.test(queue)).toBe(true);
    const topic = await (await req("/pubsub/topic?name=news@test", { cookie: s })).text();
    expect(policy.test(topic)).toBe(false);
    expect(topic).toContain("Deliver-To");
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
  test("a refused save never re-emits a foreign return", async () => {
    const r = await req("/service", { cookie: s, form: { action: "save", name: "jobs@test", descr: "Jobs", bound: "abc", return: "https://evil.example/x" } });
    expect(r.status).toBe(400);
    expect(await r.text()).not.toContain("evil.example");
  });
  test("a taken name marks the name even when a list line reads like the message", async () => {
    const r = await req("/service", { cookie: s, form: { action: "create", kind: "queue", name: "jobs@test", allow: "registered\n@owner" } });
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
    expect(await (await req("/queue?name=owner@test", { cookie: s })).text()).not.toContain("/service-danger");
    expect((await req("/service-danger?name=owner@test", { cookie: s })).status).toBe(403);
  });
  test("deactivation starts from the Danger Zone for each active registry kind", async () => {
    for (const [detail, name] of [["/agent", "#helper@test"], ["/service", "db@test"], ["/queue", "jobs@test"], ["/pubsub/topic", "news@test"]]) {
      const q = `name=${encodeURIComponent(name)}`;
      const page = await (await req(`${detail}?${q}`, { cookie: s })).text();
      expect(page).toContain(`/service-danger?${q}`);
      expect(page).not.toContain('action="/service-deactivate"');
      const danger = await (await req(`/service-danger?${q}`, { cookie: s })).text();
      expect(danger).toContain(`href="/service-deactivate?${q}"`);
    }
    const group = await (await req("/service-danger?name=%40ops", { cookie: s })).text();
    expect(group).not.toContain("/service-deactivate");
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
    expect(t).toMatch(/<th[^>]*>Status<\/th>/);
    expect(t.match(/href="\/[a-z]+\/new[^"]*"/g)).toEqual(['href="/services/new?personal=1"']);
    // Filters and paging keep the filter too.
    expect(t).toContain('href="/services?personal=1&amp;state=active"');
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
    const r = await req("/service", { cookie: s, form: { action: "create", kind: "agent", name: "nohash@test", descr: "kept-descr", secret: "TOPSECRET=1", allow: "@owner" } });
    expect(r.status).toBe(400);
    const t = await r.text();
    expect(t).toContain("Check this form");
    expect(t).toContain('value="kept-descr"');
    expect(t).not.toContain("TOPSECRET");
  });
  test("a taken name is 412 and marks the name field", async () => {
    const r = await req("/service", { cookie: s, form: { action: "create", kind: "queue", name: "jobs@test" } });
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
      const created = await req(kind === "group" ? "/groups" : "/service", { cookie: s, form });
      expect(`${kind} ${created.status}`).toBe(`${kind} 303`);
      const lookup = await fetch(`http://unix/lookup?name=${encodeURIComponent(name)}`, { headers: { "X-Agent-Bus-Token": owner }, unix: join(dir, "bus.sock") } as RequestInit);
      const record = await lookup.json() as { maintainers?: string[] };
      expect(record.maintainers).toEqual(["bob"]);
      const editPath = kind === "group" ? "/group/edit" : kind === "pubsub" ? "/pubsub/topic/edit" : `/${kind}/edit`;
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
      const rejected = await req(group ? "/groups" : "/service", { cookie: s, form });
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
  test("Personal is a simple checkbox with its explanation in hover help on create and edit", async () => {
    for (const path of ["/agents/new", "/services/new", "/queues/new", "/pubsub/new", "/resources/new", "/groups/new", "/agent/edit?name=%23helper%40test", "/group/edit?name=%40ops"]) {
      const html = await (await req(path, { cookie: s })).text();
      expect(html).toContain('type="checkbox" name="personal"');
      expect(html).toMatch(/<div class="field-heading"><label for="(?:create-name|f-name|f-record-name)">Name[\s\S]*?name="personal"/);
      expect(html).toContain('aria-label="About Personal records" data-tooltip=');
      expect(html).not.toContain("<legend>Classification</legend>");
      expect(html).not.toMatch(/Personal<span class="hint">/);
    }
  });
  test("a registration succeeds and the next page says so once", async () => {
    const r = await req("/service", { cookie: s, form: { action: "create", kind: "queue", name: "fresh@test", descr: "Fresh" } });
    expect(r.status).toBe(303);
    expect(r.headers.get("location")).toBe("/queue?name=fresh%40test");
    const flash = /ab_flash=([^;]+)/.exec(r.headers.get("set-cookie")!)![1]!;
    const next = await handle(new Request(ORIGIN + "/queue?name=fresh@test", { headers: { host: "127.0.0.1:6781", cookie: `agent_bus_session=${s}; ab_flash=${flash}` } }));
    expect(await next.text()).toContain("Registered.");
    expect(next.headers.get("set-cookie")).toContain("ab_flash=; ");
  });
  test("transfer and delete happen only through their confirmation", async () => {
    for (const action of ["transfer", "delete"]) {
      const r = await req("/service", { cookie: s, form: { action, name: "fresh@test", owner: "bob", expected_owner: "owner@test" } });
      expect(`${action} ${r.status}`).toBe(`${action} 400`);
    }
    const stale = await req("/service", { cookie: s, form: { action: "delete", name: "fresh@test", expected_owner: "owner@test", expected_queued: "7", expected_readers: "0", confirmed: "1" } });
    expect(stale.status).toBe(409);
    expect(await stale.text()).toContain("The conditions changed");
    const ok = await req("/service", { cookie: s, form: { action: "delete", name: "fresh@test", expected_owner: "owner@test", expected_queued: "0", expected_readers: "0", confirmed: "1" } });
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
    expect(t).not.toContain("/service-danger");
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
    const ok = await req("/service", { cookie: s, form: { action: "create", kind: "resource", name: "web@test", uri: "https://example.com/a.txt", descr: "A" } });
    expect(ok.status).toBe(303);
    expect(ok.headers.get("location")).toContain("/resource?name=web%40test");
    const bad = await req("/service", { cookie: s, form: { action: "create", kind: "resource", name: "nosrc@test", uri: "md://x/a.md" } });
    expect(bad.status).toBe(400);
    expect(await bad.text()).toContain("names the agent or mcp service that answers it");
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
