import { afterAll, beforeAll, expect, test } from "bun:test";
import { spawn, type Subprocess } from "bun";
import { join } from "node:path";
import { identity, startFixture, type Fixture } from "./daemon.ts";
import { Bus, type CallOutcome } from "../../clients/typescript/src/index.js";

// S.3: post and read against a fake Slack, under the pinned HTTP policy
// (Plans/R1.2/slack/TODO.md S.3). The fake records every request; each test
// scripts its behavior and asserts on the wire, not just the answers.
const slow = { timeout: 25000 } as const;
const t = (name: string, fn: () => Promise<void>) => test(name, fn, slow.timeout);

let fx: Fixture;
let callerToken: string;
let agent: Subprocess;
const name = `#slack-${process.pid}@t`;
let requests: { path: string; auth: string; params: any }[] = [];
type Route = (params: any, n: number) => { status?: number; headers?: Record<string, string>; body?: unknown; raw?: string };
let routes: Record<string, Route> = {};
const seen = (method: string) => requests.filter(r => r.path === `/api/${method}`);
let fakeServer: ReturnType<typeof Bun.serve>;
let fakePort = 0;

beforeAll(async () => {
  fx = await startFixture();
  callerToken = await identity(fx, `#caller-${process.pid}@t`);
  await fx.call(fx.ownerToken, "POST", "/register", { name, kind: "agent", allow: ["*"] });
  const agentToken = (await fx.call(fx.ownerToken, "POST", "/token", { name })).token;
  await fx.call(fx.ownerToken, "POST", "/secret", { name, secret: `SLACK_BOT_TOKEN=xoxb-fixture\nSLACK_APP_TOKEN=xapp-fixture\nSLACK_API_URL=http://127.0.0.1:${() => fakePort}/api/\n`.replace("${port}", "") });
  fakeServer = Bun.serve({ port: 0, hostname: "127.0.0.1", async fetch(req) {
    const path = new URL(req.url).pathname;
    const params = await req.json().catch(() => ({}));
    requests.push({ path, auth: req.headers.get("authorization") ?? "", params });
    const route = routes[path.slice("/api/".length)];
    if (!route) return Response.json({ ok: false, error: "no route" });
    const answer = route(params, seen(path.slice("/api/".length)).length);
    if (answer.status === -1) return new Response(new ReadableStream({ start(c) { c.enqueue(new TextEncoder().encode("0123456789")); c.close(); } }), { status: 200, headers: { "content-length": "1000" } }); // the body is cut off mid-flight
    if ("raw" in answer) return new Response(String(answer.raw), { status: answer.status ?? 200, headers: { "content-type": "text/html" } });
    return new Response(JSON.stringify(answer.body ?? { ok: true }), { status: answer.status ?? 200, headers: { "content-type": "application/json", ...(answer.headers ?? {}) } });
  } });
  fakePort = fakeServer.port;
  await fx.call(fx.ownerToken, "POST", "/secret", { name, secret: `SLACK_BOT_TOKEN=xoxb-fixture\nSLACK_APP_TOKEN=xapp-fixture\nSLACK_API_URL=http://127.0.0.1:${fakePort}/api/\n` });
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
}, 30000);

afterAll(async () => {
  agent?.kill("SIGKILL");
  await Bun.sleep(50);
  fakeServer?.stop(true);
  await fx?.stop();
}, 15000);

async function call(body: string): Promise<string> {
  const bus = Bus.connect({ addr: fx.socket, token: callerToken });
  const outcome: CallOutcome = await bus.call(name, body, { timeout: 12000 });
  return outcome.kind === "answer" ? outcome.message.body : "(timeout)";
}

/** Reset the wire and install this test's routes. */
function behave(next: Record<string, Route>): void {
  requests = [];
  routes = next;
}

t("post sends the right message and answers the permalink", async () => {
  behave({
    "chat.postMessage": p => ({ body: { ok: true, ts: "1234.0056", channel: p.channel } }),
    "chat.getPermalink": () => ({ body: { ok: true, permalink: "https://team.slack.com/archives/C1/p12340056" } }),
  });
  expect(await call("post C123 deploy done")).toBe("https://team.slack.com/archives/C1/p12340056");
  expect(seen("chat.postMessage")).toHaveLength(1);
  expect(seen("chat.postMessage")[0]!.params).toEqual({ channel: "C123", text: "deploy done" });
});

t("an email resolves to its user id, with no DM opened", async () => {
  behave({
    "users.lookupByEmail": p => ({ body: { ok: true, user: { id: "U77", name: "someone" } } }),
    // Slack answers with the DM it opened, not the user id it was given.
    "chat.postMessage": () => ({ body: { ok: true, ts: "1.2", channel: "DREAL" } }),
    "chat.getPermalink": () => ({ body: { ok: true, permalink: "https://team.slack.com/archives/C1/p12" } }),
  });
  expect(JSON.parse(await call('{"method":"post","channel":"someone@example.com","text":"hi"}')))
    .toEqual({ ok: true, channel: "DREAL", ts: "1.2", permalink: "https://team.slack.com/archives/C1/p12" });
  expect(seen("chat.getPermalink")[0]!.params).toEqual({ channel: "DREAL", message_ts: "1.2" });
  expect(seen("users.lookupByEmail")[0]!.params).toEqual({ email: "someone@example.com" });
  expect(seen("chat.postMessage")[0]!.params.channel).toBe("U77");
  expect(seen("conversations.open")).toHaveLength(0);
});

t("username and icon map to Slack's customize fields", async () => {
  behave({
    "chat.postMessage": () => ({ body: { ok: true, ts: "1.2", channel: "C1" } }),
    "chat.getPermalink": () => ({ body: { ok: true, permalink: "p" } }),
  });
  await call('{"method":"post","channel":"C1","text":"t","username":"robot","icon":":fire:"}');
  expect(seen("chat.postMessage")[0]!.params.icon_emoji).toBe(":fire:");
  await call('{"method":"post","channel":"C1","text":"t","icon":"https://example.com/a.png"}');
  expect(seen("chat.postMessage")[1]!.params.icon_url).toBe("https://example.com/a.png");
  expect(seen("chat.postMessage")[1]!.params.icon_emoji).toBeUndefined();
});

t("a failed permalink after a post answers channel ts, without posting again", async () => {
  behave({
    "chat.postMessage": () => ({ body: { ok: true, ts: "99.1", channel: "C9" } }),
    "chat.getPermalink": () => ({ body: { ok: false, error: "message_not_found" } }),
  });
  expect(await call("post C9 once")).toBe("C9 ts=99.1");
  expect(seen("chat.postMessage")).toHaveLength(1);
});

t("read pages through cursors to N, oldest first, timestamps as strings", async () => {
  behave({
    "conversations.history": (p, n) => n === 1
      ? { body: { ok: true, messages: [{ ts: "5.0", user: "U5", text: "fifth" }, { ts: "4.0", user: "U4", text: "fourth" }], response_metadata: { next_cursor: "older" } } }
      : { body: { ok: true, messages: [{ ts: "3.0", user: "U3", text: "third" }, { ts: "2.0", user: "U2", text: "second" }, { ts: "1.0", user: "U1", text: "first" }] } },
  });
  expect(await call("read C1 5")).toBe("1.0 U1 first\n2.0 U2 second\n3.0 U3 third\n4.0 U4 fourth\n5.0 U5 fifth");
  expect(seen("conversations.history")).toHaveLength(2);
});

t("ok:false becomes an error answer", async () => {
  behave({ "chat.postMessage": () => ({ body: { ok: false, error: "channel_not_found" } }) });
  expect(await call("post CNOPE hi")).toBe("error: channel_not_found");
});

t("a 429 waits its Retry-After and retries once; a second is ratelimited", async () => {
  behave({ "chat.postMessage": (_p, n) => n === 1
    ? { status: 429, headers: { "retry-after": "0" }, body: { ok: false, error: "ratelimited" } }
    : { body: { ok: true, ts: "1.1", channel: "C1" } },
    "chat.getPermalink": () => ({ body: { ok: true, permalink: "p" } }) });
  expect(await call("post C1 again")).toBe("p");
  expect(seen("chat.postMessage")).toHaveLength(2);
  behave({ "chat.postMessage": () => ({ status: 429, headers: { "retry-after": "0" }, body: { ok: false, error: "ratelimited" } }) });
  expect(await call("post C1 twice")).toBe("error: ratelimited");
  expect(seen("chat.postMessage")).toHaveLength(2); // one try, one retry, no third
});

t("a write is not retried on 5xx, nor on a lost response", async () => {
  behave({ "chat.postMessage": () => ({ status: 500, body: { ok: false, error: "internal_error" } }) });
  expect(await call("post C1 no")).toContain("error:");
  expect(seen("chat.postMessage")).toHaveLength(1);
  behave({ "chat.postMessage": () => ({ status: -1, body: {} }) });
  const answer = await call("post C1 lost");
  expect(answer).toContain("error:");
  expect(seen("chat.postMessage")).toHaveLength(1);
});

t("a read retries once on 5xx and then answers; a malformed answer is an error", async () => {
  behave({ "conversations.history": (_p, n) => n === 1
    ? { status: 503, body: { ok: false, error: "fatal" } }
    : { body: { ok: true, messages: [{ ts: "1.0", user: "U1", text: "only" }] } } });
  expect(await call("read C1")).toBe("1.0 U1 only");
  expect(seen("conversations.history")).toHaveLength(2);
  behave({ "conversations.history": () => ({ status: 502, raw: "<html>not json</html>" }) });
  expect(await call("read C1")).toBe("error: malformed answer (502)");
});
