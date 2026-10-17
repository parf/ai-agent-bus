import { afterAll, beforeAll, expect, test } from "bun:test";
import { spawn, type Subprocess } from "bun";
import { join } from "node:path";
import { identity, startFixture, type Fixture } from "./daemon.ts";
import { Bus, type CallOutcome } from "../../clients/typescript/src/index.js";

// The S.1–S.3b review findings, each pinned through a real daemon and the
// actual agent against a fake Slack (slack-review-S.1 … S.3b): what reaches a
// caller carries no credential, a malformed answer is answered, a caller's
// deadline bounds every request and retry, a landed post keeps its
// coordinates, JSON calls are answered in JSON, threads keep Slack's order,
// directories refresh on a miss and belong to one workspace.
const t = (name: string, fn: () => Promise<void>) => test(name, fn, 30000);

let fx: Fixture;
let callerToken: string;
let agent: Subprocess;
const name = `#slack-review-${process.pid}@t`;
type Seen = { path: string; auth: string; params: any; at: number };
let requests: Seen[] = [];
type Route = (params: any, n: number, auth: string) => { status?: number; headers?: Record<string, string>; body?: unknown; stream?: "errored" };
let routes: Record<string, Route> = {};
const seen = (method: string) => requests.filter(r => r.path === `/api/${method}`);
let fake: ReturnType<typeof Bun.serve>;
const secretA = "xoxb-REVIEW-SECRET-AAAA";
const secretB = "xoxb-REVIEW-SECRET-BBBB";

async function setSecret(bot: string) {
  await fx.call(fx.ownerToken, "POST", "/secret", { name, secret: `SLACK_BOT_TOKEN=${bot}\nSLACK_APP_TOKEN=xapp-REVIEW-APP-SECRET\nSLACK_API_URL=http://127.0.0.1:${fake.port}/api/\n` });
}

beforeAll(async () => {
  fx = await startFixture();
  callerToken = await identity(fx, `#caller-${process.pid}@t`);
  await fx.call(fx.ownerToken, "POST", "/register", { name, kind: "agent", allow: ["*"] });
  const agentToken = (await fx.call(fx.ownerToken, "POST", "/token", { name })).token;
  fake = Bun.serve({ port: 0, hostname: "127.0.0.1", async fetch(req) {
    const path = new URL(req.url).pathname;
    const raw = await req.text();
    // What the agent sends is a form; a JSON body is read too, as Slack would refuse it.
    const params: any = (req.headers.get("content-type") ?? "").includes("json")
      ? JSON.parse(raw || "{}")
      : Object.fromEntries(new URLSearchParams(raw));
    const auth = req.headers.get("authorization") ?? "";
    requests.push({ path, auth, params, at: Date.now() });
    const method = path.slice("/api/".length);
    const route = routes[method];
    if (!route) return Response.json({ ok: false, error: "no route" });
    const answer = route(params, seen(method).length, auth);
    if (answer.stream === "errored") return new Response(new ReadableStream({ start(c) { c.error(new Error("connection dropped")); } }), { status: 200 });
    return new Response(JSON.stringify(answer.body === undefined ? { ok: true } : answer.body), { status: answer.status ?? 200, headers: { "content-type": "application/json", ...(answer.headers ?? {}) } });
  } });
  await setSecret(secretA);
  agent = spawn(["bun", "slack.ts"], {
    cwd: import.meta.dir,
    env: { ...process.env, AGENT_BUS_ADDR: fx.socket, AGENT_BUS_NAME: name, AGENT_BUS_TOKEN: agentToken },
    stderr: Bun.file(join(fx.dir, "agent.log")), stdout: Bun.file(join(fx.dir, "agent.log")),
  });
  for (let i = 0; i < 200; i++) {
    try {
      const rec = await fx.call(fx.ownerToken, "GET", `/lookup?name=${encodeURIComponent(name)}`);
      if (String(rec?.descr ?? "").startsWith("slack:")) break;
    } catch { /* not yet */ }
    await Bun.sleep(50);
  }
  // Serving: its start-up auth.test is behind it, never in a test's wire.
  await call("help");
}, 30000);

afterAll(async () => {
  agent?.kill("SIGKILL");
  await Bun.sleep(50);
  fake?.stop(true);
  await fx?.stop();
}, 15000);

async function call(body: string, timeout = 12000): Promise<string> {
  const bus = Bus.connect({ addr: fx.socket, token: callerToken });
  const outcome: CallOutcome = await bus.call(name, body, { timeout });
  return outcome.kind === "answer" ? outcome.message.body : "(timeout)";
}

function behave(next: Record<string, Route>): void {
  requests = [];
  routes = next;
}

const leaks = (s: string) => [secretA, secretB, "xapp-REVIEW-APP-SECRET"].filter(x => s.includes(x));

t("a successful answer that reflects the bearer still carries no credential (S.1)", async () => {
  behave({ "auth.test": (_p, _n, auth) => ({ body: { ok: true, user: auth, user_id: "U1", team: "T", url: "https://example.invalid" } }) });
  const text = await call("whoami");
  const json = await call('{"method":"whoami"}');
  expect(seen("auth.test")).toHaveLength(2); // the reflection really happened
  expect(leaks(text)).toEqual([]);
  expect(leaks(json)).toEqual([]);
  expect(text).toContain("[redacted]");
});

t("a valid-JSON answer that is not Slack's shape is answered as malformed, not left unanswered (S.1)", async () => {
  for (const body of [null, 42, [], { user: "x" }, "ok"]) {
    behave({ "auth.test": () => ({ body }) });
    expect(await call("whoami")).toBe("error: malformed answer (200)");
  }
});

t("a 429 waits its Retry-After before the retry (S.3)", async () => {
  behave({
    "chat.postMessage": (_p, n) => (n === 1 ? { status: 429, headers: { "retry-after": "1" }, body: { ok: false, error: "ratelimited" } } : { body: { ok: true, channel: "C1", ts: "5.5" } }),
    "chat.getPermalink": () => ({ body: { ok: true, permalink: "p" } }),
  });
  expect(await call("post C1 later")).toBe("p");
  const [first, second] = seen("chat.postMessage");
  expect(second!.at - first!.at).toBeGreaterThanOrEqual(900);
});

t("no write is begun after the caller's deadline, even when a 429 asked for a retry (S.3 P1)", async () => {
  behave({
    "chat.postMessage": () => ({ status: 429, headers: { "retry-after": "1" }, body: { ok: false, error: "ratelimited" } }),
    "chat.getPermalink": () => ({ body: { ok: true, permalink: "p" } }),
  });
  // Answered at once when the wait cannot fit, or not at all: never a retry.
  expect(["error: ratelimited", "(timeout)"]).toContain(await call("post C1 too-late", 300));
  await Bun.sleep(3000); // well past the 1 s Retry-After the agent was told, even under load
  expect(seen("chat.postMessage")).toHaveLength(1);
  expect(seen("chat.getPermalink")).toHaveLength(0);
});

t("a post that landed keeps its coordinates when the permalink request drops, and is not posted again (S.3)", async () => {
  behave({
    "chat.postMessage": () => ({ body: { ok: true, channel: "DREAL", ts: "7.7" } }),
    "chat.getPermalink": () => ({ stream: "errored" }),
  });
  expect(await call("post U77 once")).toBe("DREAL ts=7.7");
  expect(JSON.parse(await call('{"method":"post","channel":"U77","text":"once"}'))).toEqual({ ok: true, channel: "DREAL", ts: "7.7" });
  expect(seen("chat.postMessage")).toHaveLength(2); // one per call, never a repeat
});

t("a transport failure of a read is an answer, not a silence (S.3)", async () => {
  behave({ "conversations.history": () => ({ stream: "errored" }) });
  expect(await call("read C1")).toStartWith("error:");
  expect(JSON.parse(await call('{"method":"read","channel":"C1"}')).ok).toBe(false);
});

t("a JSON read is answered in JSON, oldest first, timestamps as strings (S.3)", async () => {
  behave({ "conversations.history": () => ({ body: { ok: true, messages: [{ ts: "2.10", user: "U2", text: "b" }, { ts: "1.10", user: "U1", text: "a" }] } }) });
  expect(JSON.parse(await call('{"method":"read","channel":"C1","limit":2}'))).toEqual({ ok: true, messages: [{ ts: "1.10", user: "U1", text: "a" }, { ts: "2.10", user: "U2", text: "b" }] });
});

t("an @name that misses refreshes the users once; an id goes straight to users.info (S.3b)", async () => {
  let late = false;
  behave({
    "users.list": () => ({ body: { ok: true, members: [{ id: "U1", name: "ada" }, ...(late ? [{ id: "U9", name: "late" }] : [])] } }),
    "users.info": p => ({ body: { ok: true, user: { id: p.user, name: p.user === "U9" ? "late" : "ada", tz: "UTC" } } }),
  });
  expect(await call("user @ada")).toStartWith("U1 ada");
  late = true;
  expect(await call("user @late")).toStartWith("U9 late");
  expect(await call("user @nobody")).toBe("error: no user @nobody");
  const lists = seen("users.list").length;
  expect(await call("user U42")).toStartWith("U42");
  expect(seen("users.list")).toHaveLength(lists); // an id needs no list
  expect(seen("users.info").at(-1)!.params).toEqual({ user: "U42" });
});

t("the directories belong to one workspace: a changed token lists again (S.3b)", async () => {
  behave({
    "conversations.list": (_p, _n, auth) => ({ body: { ok: true, channels: [{ id: auth.includes(secretA) ? "CA" : "CB", name: "dev" }] } }),
    "chat.postMessage": p => ({ body: { ok: true, channel: p.channel, ts: "1.1" } }),
    "chat.getPermalink": () => ({ body: { ok: true, permalink: "p" } }),
  });
  await call("post #dev a");
  expect(seen("chat.postMessage").at(-1)!.params.channel).toBe("CA");
  await setSecret(secretB);
  try {
    await call("post #dev b");
    expect(seen("chat.postMessage").at(-1)!.params.channel).toBe("CB");
    expect(seen("chat.postMessage").at(-1)!.auth).toBe(`Bearer ${secretB}`);
  } finally {
    await setSecret(secretA);
  }
});

t("channel asks Slack for its member count, and an absent count is not zero (S.3b)", async () => {
  behave({ "conversations.info": p => ({ body: { ok: true, channel: { id: p.channel, name: "dev", created: 1, ...(p.include_num_members ? { num_members: 12 } : {}) } } }) });
  expect(await call("channel C1")).toContain("members=12");
  expect(seen("conversations.info")[0]!.params).toEqual({ channel: "C1", include_num_members: "true" });
});

t("each message tool sends its one method exactly once (S.3b)", async () => {
  const methods = ["reactions.add", "chat.update", "chat.delete", "conversations.join"];
  behave(Object.fromEntries(methods.map(m => [m, () => ({ body: { ok: true } })])));
  for (const body of ["react C1 1.0 tada", "edit C1 1.0 new text", "delete C1 1.0", "join C1"]) await call(body);
  for (const m of methods) expect(seen(m)).toHaveLength(1);
  expect(requests).toHaveLength(4); // and nothing else
});
