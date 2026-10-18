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
const A = "#runner/alpha@test", B = "#runner/beta@test", C = "#runner/gamma@test";
let dir = "", daemon: ReturnType<typeof Bun.spawn> | undefined;
let handle: (r: Request) => Promise<Response>;
let owner = "", bob = "", carol = "";
let serving = true;
const asked: { runner: string; body: string }[] = [];

async function api(path: string, body: unknown, token = owner, method = "POST") {
  const r = await fetch(`http://unix${path}`, { method, body: method === "POST" ? JSON.stringify(body) : undefined, headers: { "X-Agent-Bus-Token": token }, unix: join(dir, "bus.sock") } as RequestInit);
  const t = await r.text();
  if (!r.ok) throw new Error(`${path} ${r.status} ${t}`);
  return t ? JSON.parse(t) : null;
}

/** A runner that answers: its status names one visible and one hidden agent. */
async function fakeRunner(name: string, token: string) {
  while (serving) {
    let e: any;
    try { e = await api("/consume?wait=500ms", undefined, token, "GET"); } catch { await Bun.sleep(50); continue; }
    if (!e || e.receipt) continue;
    asked.push({ runner: name, body: e.body });
    const answer = e.body === "status" || e.body === "reload"
      ? JSON.stringify({ runner: name, version: "0.8.999", agents: [
          { name: "#hello@test", enabled: true, state: "idle", processes: { running: 0, min: 0, max: 1 } },
          { name: "#secretive@test", enabled: true, state: "unconfigured", reason: "exited 78: SLACK_BOT_TOKEN is not set" },
        ] })
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
  // Not runners: a runner-tagged task, and a look-alike name.
  await api("/register", { name: "#tasker@test", kind: "agent", tags: ["runner"], allow: ["bob", "carol"] });
  await api("/register", { name: "#runner-x@test", kind: "agent", allow: ["carol"] });
  await api("/register", { name: "#hello@test", kind: "agent", descr: "Hello agent" });
  await api("/register", { name: "#secretive@test", kind: "agent" });
  void fakeRunner(A, (await api("/token", { name: A })).token);
  void fakeRunner(C, (await api("/token", { name: C })).token);
  waits.status = 2000; waits.chooser = 1000; waits.reload = 2000;
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
  expect(rows(t, "runner")).toEqual([A, B, C]);
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
  expect(rows(t, "agent")).toEqual(["#hello@test", "#secretive@test"]);
  expect(t).toContain("exited 78: SLACK_BOT_TOKEN is not set");
  // bob sees neither record: both rows say so, and neither is a link.
  expect(t).not.toContain(`href="/agent?name=${encodeURIComponent("#hello@test")}`);
  expect(t.match(/its record is not visible to you/g)?.length).toBe(2);
  const own = await (await req(`/runner?name=${encodeURIComponent(A)}`, await signIn(owner))).text();
  const link = `/agent?name=${encodeURIComponent("#hello@test")}&amp;return=${encodeURIComponent(`/runner?name=${encodeURIComponent(A)}`)}`;
  expect(own).toContain(`href="${link}"`);
  expect(own).toContain("Hello agent");
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
  expect(rows(ok, "agent")).toEqual(["#hello@test", "#secretive@test"]);
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
