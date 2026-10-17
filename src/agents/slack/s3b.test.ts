import { afterAll, beforeAll, expect, test } from "bun:test";
import { spawn, type Subprocess } from "bun";
import { join } from "node:path";
import { identity, startFixture, type Fixture } from "./daemon.ts";
import { Bus, type CallOutcome } from "../../clients/typescript/src/index.js";

// S.3b: the directory and message tools, each sending its one API method with
// the right arguments, #name and @name resolving through the cached lists
// (refreshed once on a miss) and pagination followed (TODO S.3b).
const slow = { timeout: 25000 } as const;
const t = (name: string, fn: () => Promise<void>) => test(name, fn, slow.timeout);

let fx: Fixture;
let callerToken: string;
let agent: Subprocess;
const name = `#slack-${process.pid}@t`;
let requests: { path: string; params: any }[] = [];
type Route = (params: any, n: number) => { status?: number; body?: unknown };
let routes: Record<string, Route> = {};
let fakeServer: ReturnType<typeof Bun.serve>;

/** The fake's world: two pages of channels, one page of users, a known thread. */
const channelsPage1 = [{ id: "C1", name: "dev", num_members: 12, is_member: true, topic: { value: "Build chat" } }, { id: "C2", name: "ops", num_members: 3, is_member: false }];
const channelsPage2 = [{ id: "C3", name: "dev-legacy", num_members: 1 }];
const members = [
  { id: "U1", name: "ada", real_name: "Ada Lovelace", profile: { email: "ada@example.com", title: "Engineer" }, tz: "UTC" },
  { id: "U2", name: "bob", real_name: "Bob", profile: {}, is_bot: true },
  { id: "U3", name: "gone", deleted: true, profile: {} },
];
let joinedChannel: string | null = null;

beforeAll(async () => {
  fx = await startFixture();
  callerToken = await identity(fx, `#caller-${process.pid}@t`);
  await fx.call(fx.ownerToken, "POST", "/register", { name, kind: "agent", allow: ["*"] });
  const agentToken = (await fx.call(fx.ownerToken, "POST", "/token", { name })).token;
  fakeServer = Bun.serve({ port: 0, hostname: "127.0.0.1", async fetch(req) {
    const path = new URL(req.url).pathname;
    const params = await req.json().catch(() => ({}));
    requests.push({ path, params });
    const key = path.slice("/api/".length);
    const route = routes[key] ?? defaultRoutes[key];
    if (!route) return Response.json({ ok: false, error: `no route ${key}` });
    return Response.json(route(params, requests.filter(r => r.path === path).length).body ?? { ok: true });
  } });
  await fx.call(fx.ownerToken, "POST", "/secret", { name, secret: `SLACK_BOT_TOKEN=xoxb-fixture\nSLACK_APP_TOKEN=xapp-fixture\nSLACK_API_URL=http://127.0.0.1:${fakeServer.port}/api/\n` });
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

const defaultRoutes: Record<string, Route> = {
  "conversations.list": (_p, n) => n === 1
    ? { body: { ok: true, channels: channelsPage1, response_metadata: { next_cursor: "more" } } }
    : { body: { ok: true, channels: channelsPage2 } },
  "users.list": () => ({ body: { ok: true, members } }),
  "users.info": p => ({ body: { ok: true, user: { id: p.user, name: "ada", real_name: "Ada Lovelace", profile: { email: "ada@example.com", title: "Engineer" }, tz: "UTC" } } }),
  "users.lookupByEmail": () => ({ body: { ok: true, user: { id: "U1" } } }),
  "conversations.info": p => ({ body: { ok: true, channel: { id: p.channel, name: "dev", num_members: 12, topic: { value: "Build chat" }, purpose: { value: "Everything builds" }, created: 1700000000 } } }),
  // Slack answers a thread oldest first, the parent first, page after page.
  "conversations.replies": (_p, n) => n === 1
    ? { body: { ok: true, messages: [{ ts: "1.0", user: "U1", text: "first" }], response_metadata: { next_cursor: "later" } } }
    : { body: { ok: true, messages: [{ ts: "2.0", user: "U1", text: "second" }] } },
  "reactions.add": () => ({ body: { ok: true } }),
  "chat.update": () => ({ body: { ok: true } }),
  "chat.delete": () => ({ body: { ok: true } }),
  "conversations.join": p => { joinedChannel = p.channel; return { body: { ok: true, channel: { id: p.channel } } }; },
  "chat.postMessage": () => ({ body: { ok: true, ts: "9.9", channel: "C1" } }),
  "chat.getPermalink": () => ({ body: { ok: true, permalink: "p" } }),
  "conversations.history": () => ({ body: { ok: true, messages: [] } }),
  "auth.test": () => ({ body: { ok: true, user: "agentbot", team: "T", url: "https://t.slack.com" } }),
};

async function call(body: string): Promise<string> {
  const bus = Bus.connect({ addr: fx.socket, token: callerToken });
  const outcome: CallOutcome = await bus.call(name, body, { timeout: 12000 });
  return outcome.kind === "answer" ? outcome.message.body : "(timeout)";
}
const seen = (method: string) => requests.filter(r => r.path === `/api/${method}`);
const reset = () => { requests = []; routes = {}; };

t("channels pages through the cursor and answers the documented fields", async () => {
  reset();
  const answer = await call("channels");
  expect(answer).toContain("C1 #dev members=12 member topic=Build chat");
  expect(answer).toContain("C3 #dev-legacy members=1");
  expect(seen("conversations.list")).toHaveLength(2);
  expect(await call("channels dev")).not.toContain("ops");
});

t("users lists people, not bots or the deleted, and filters anywhere", async () => {
  reset();
  const answer = await call("users");
  expect(answer).toContain("U1 ada (Ada Lovelace) <ada@example.com>");
  expect(answer).not.toContain("bob");
  expect(answer).not.toContain("gone");
  expect(await call("users lovelace")).toContain("U1");
});

t("user answers one person by id, email and @name", async () => {
  reset();
  expect(await call("user U1")).toContain("U1 ada");
  expect(await call("user ada@example.com")).toContain("U1 ada");
  expect(await call("user @ada")).toContain("U1 ada");
  expect(seen("users.lookupByEmail")).toHaveLength(1);
});

t("channel answers one channel's facts", async () => {
  reset();
  const answer = await call("channel #dev");
  expect(answer).toBe("C1 #dev members=12 topic=Build chat purpose=Everything builds created=1700000000");
});

t("thread pages to N oldest first", async () => {
  reset();
  expect(await call("thread C1 123.0")).toBe("1.0 U1 first\n2.0 U1 second");
  expect(seen("conversations.replies")[0]!.params).toEqual({ channel: "C1", ts: "123.0", limit: 20 });
});

t("react, edit, delete and join each send their one method with the right arguments", async () => {
  reset();
  expect(await call("react #dev 5.5 :white_check_mark:")).toBe("reacted");
  expect(seen("reactions.add")[0]!.params).toEqual({ channel: "C1", timestamp: "5.5", name: "white_check_mark" });
  expect(await call("edit #dev 5.5 fixed text")).toBe("edited");
  expect(seen("chat.update")[0]!.params).toEqual({ channel: "C1", ts: "5.5", text: "fixed text" });
  expect(await call("delete #dev 5.5")).toBe("deleted");
  expect(seen("chat.delete")[0]!.params).toEqual({ channel: "C1", ts: "5.5" });
  expect(await call("join #ops")).toBe("joined #ops");
  expect(seen("conversations.join")[0]!.params).toEqual({ channel: "C2" });
});

t("a #name miss refreshes the cache once, then answers no channel", async () => {
  reset();
  // C1..C3 are the cached world; make the refresh serve a new channel.
  routes["conversations.list"] = (_p, n) => n === 1
    ? { body: { ok: true, channels: [...channelsPage1, { id: "C9", name: "late", num_members: 2 }], response_metadata: { next_cursor: "more" } } }
    : { body: { ok: true, channels: channelsPage2 } };
  expect(await call("join #late")).toBe("joined #late");
  expect(seen("conversations.list")).toHaveLength(2); // the refresh paged again
  // Now one that truly is not there: one refresh, then the refusal.
  reset();
  expect(await call("join #nope")).toBe("error: no channel #nope");
  expect(seen("conversations.list")).toHaveLength(2);
});
