// The face against a disposable daemon: every request goes through the real
// handler and the real daemon over its socket. AGENT_BUS_BIN_DIR names built
// Go programs; without it they are built into a temporary directory.
import { afterAll, beforeAll, describe, expect, test } from "bun:test";
import { mkdtempSync, rmSync, existsSync } from "node:fs";
import { join } from "node:path";
import { tmpdir, userInfo } from "node:os";
import { makeHandler } from "../server.ts";

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
  const pages = ["/", "/agents", "/services", "/queues", "/pubsub", "/personal", "/personal?kind=queue", "/agents/new", "/services/new", "/queues/new", "/pubsub/new",
    "/agent?name=%23helper@test", "/queue?name=jobs@test", "/pubsub/topic?name=news@test", "/service?name=db@test", "/agent/edit?name=%23helper@test",
    "/service-deactivate?name=jobs@test", "/service-danger?name=jobs@test", "/activity", "/activity?name=jobs@test", "/diagnostics",
    "/users", "/users/new", "/user?name=bob", "/user/edit?name=bob", "/user-deactivate?name=bob", "/groups", "/groups/new", "/group?name=@ops", "/group/edit?name=@ops", "/account"];
  test("every page renders, with no inline style", async () => {
    for (const p of pages) {
      const r = await req(p, { cookie: s });
      const t = await r.text();
      expect(`${p} ${r.status}`).toBe(`${p} 200`);
      expect(`${p} ${/\sstyle=/.test(t)}`).toBe(`${p} false`);
      expect(`${p} ${/<script(?![^>]*\bsrc=)/.test(t)}`).toBe(`${p} false`);
    }
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
  test("the Activity chooser shows each record's hits in the last day", async () => {
    const t = await (await req("/activity", { cookie: s })).text();
    expect(t).toMatch(/<option value="jobs@test">jobs@test — 1 hit<\/option>/);
    expect(t).toMatch(/<option value="db@test">db@test — 0 hits<\/option>/);
    expect(t).toMatch(/<option value="">All visible — \d+ hits?<\/option>/);
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
