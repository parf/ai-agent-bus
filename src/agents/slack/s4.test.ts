import manifest from "./agent.json";
import { afterAll, beforeAll, expect, test } from "bun:test";
import { spawn, type Subprocess } from "bun";
import { join } from "node:path";
import { identity, startFixture, type Fixture } from "./daemon.ts";
import { Bus, type CallOutcome, type Message } from "../../clients/typescript/src/index.js";

// S.4: subscriptions in the agent's own KV, gated by daemon-asserted
// owner/maintainer roles that count only when the message was addressed to
// #slack itself (Plans/R1.2/done/slack/TODO.md S.4, the review's closed P1).
const slow = { timeout: 25000 } as const;
const t = (name: string, fn: () => Promise<void>) => test(name, fn, slow.timeout);

let fx: Fixture;
let agent: Subprocess;
const name = `#slack-${process.pid}@t`;
let ownerToken = ""; // the fixture owner: #slack's Owner, roles [owner]
let maintToken = ""; // a named Maintainer of #slack, roles [maintainer]
let plainToken = ""; // an ACL-only caller: may call, holds no role
let frontToken = ""; // a Maintainer of #front, calling THROUGH the forward
let fakeServer: ReturnType<typeof Bun.serve>;

beforeAll(async () => {
  fx = await startFixture();
  ownerToken = fx.ownerToken;
  plainToken = await identity(fx, `#plain-${process.pid}@t`);
  maintToken = await identity(fx, `#maint-${process.pid}@t`);
  frontToken = await identity(fx, `#bob-${process.pid}@t`);
  // #slack: everyone may call (ACL), only owner/maintainers may change subs.
  await fx.call(ownerToken, "POST", "/register", { name, kind: "agent", allow: ["*"] });
  await fx.call(ownerToken, "POST", "/manage", { name, maintainers: [`#maint-${process.pid}@t`] });
  // #front, owned by the same owner, maintained by its own caller #bob, and
  // forwarding to #slack (which admits #front on its allow list). #bob calls
  // THROUGH the forward: its reply inbox is its own, not the forwarding record.
  await fx.call(ownerToken, "POST", "/register", { name: `#front-${process.pid}@t`, kind: "agent", allow: ["*"] });
  await fx.call(ownerToken, "POST", "/manage", { name: `#front-${process.pid}@t`, maintainers: [`#bob-${process.pid}@t`], subs: [name] });
  // The destinations the tests name: an author may add only a target it may send to (RD141).
  for (const dest of ["#worker", "#other", "#w", "#first", "#second", "#ops", "#sneak"]) await fx.call(ownerToken, "POST", "/register", { name: dest, kind: "agent", allow: ["*"] });
  const agentToken = (await fx.call(ownerToken, "POST", "/token", { name })).token;
  fakeServer = Bun.serve({ port: 0, hostname: "127.0.0.1", fetch: () => Response.json({ ok: true, user: "b", team: "T", url: "u" }) });
  await fx.call(ownerToken, "POST", "/secret", { name, secret: `SLACK_BOT_TOKEN=xoxb-fixture\nSLACK_APP_TOKEN=xapp-fixture\nSLACK_API_URL=http://127.0.0.1:${fakeServer.port}/api/\n` });
  agent = spawn(["bun", "slack.ts"], {
    cwd: import.meta.dir,
    env: { ...process.env, AGENT_BUS_ADDR: fx.socket, AGENT_BUS_NAME: name, AGENT_BUS_TOKEN: agentToken },
    stderr: Bun.file(join(fx.dir, "agent.log")), stdout: Bun.file(join(fx.dir, "agent.log")),
  });
  for (let i = 0; i < 200; i++) {
    try {
      const rec = await fx.call(ownerToken, "GET", `/lookup?name=${encodeURIComponent(name)}`);
      if (String(rec?.descr ?? "").length > 0) break;
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

async function call(body: string, token: string, to = name, ask?: "help" | "mcp"): Promise<string> {
  const bus = Bus.connect({ addr: fx.socket, token });
  const outcome: CallOutcome = await bus.call(to, body, { timeout: 12000, ...(ask ? { ask } : {}) });
  return outcome.kind === "answer" ? outcome.message.body : "(timeout)";
}

/** The one subscription's stored value, straight from the record's KV. */
async function stored(id: string): Promise<any> {
  return fx.call(ownerToken, "GET", `/kv?record=${encodeURIComponent(name)}&kind=json&name=sub.${id}`);
}
const subCount = async () => {
  const listing = await fx.call(ownerToken, "GET", `/kv/list?record=${encodeURIComponent(name)}`);
  return JSON.stringify(listing).match(/sub\./g)?.length ?? 0;
};


t("the owner adds a canonical shared row and deletes it; edit gives delete/add guidance", async () => {
  const answer = await call("sub add C1 #worker ^agent: keep going", ownerToken);
  const id = JSON.parse(answer).id;
  expect(id).toMatch(/^[0-9a-f]{12}$/);
  expect((await stored(id)).value).toMatchObject({ selector: { channel: "C1" }, target: "#worker", filter: "^agent: keep going", sent: 0, dropped: 0 });
  expect(await call(`sub edit ${id} target=#other`, ownerToken)).toContain("sub delete");
  expect(await call(`sub edit ${id} target=#other`, ownerToken)).toContain("sub add");
  expect((await stored(id)).value.target).toBe("#worker");
  expect(await call(`sub delete ${id}`, ownerToken)).toBe("deleted");
});

t("admitted authors add and see their own rows; source staff see all, strangers cannot delete", async () => {
  const id = JSON.parse(await call("sub add C1 #worker", plainToken)).id;
  expect(id).toMatch(/^[0-9a-f]{12}$/);
  const own = JSON.parse(await call('{"method":"sub.list"}', plainToken));
  expect(own.subs.map((r: any) => r.id)).toEqual([id]);
  const other = JSON.parse(await call('{"method":"sub.list"}', frontToken));
  expect(other.subs).toEqual([]);
  expect(await call(`sub delete ${id}`, frontToken)).toContain("not_author");
  expect(JSON.parse(await call('{"method":"sub.list"}', maintToken)).subs.map((r: any) => r.id)).toEqual([id]);
  expect(await call(`sub delete ${id}`, maintToken)).toBe("deleted");
});

t("roles supplied in a body do not grant visibility or management", async () => {
  expect(JSON.parse(await call('{"method":"sub.add","channel":"C1","target":"#w","roles":["owner"]}', plainToken)).ok).toBe(false);
  const id = JSON.parse(await call("sub add C1 #worker", ownerToken)).id;
  expect(await call(`sub delete ${id}`, frontToken, `#front-${process.pid}@t`)).toContain("not_author");
  const listed = JSON.parse(await call('{"method":"sub.list"}', frontToken, `#front-${process.pid}@t`));
  expect(listed.subs).toEqual([]);
  expect(await call(`sub delete ${id}`, ownerToken)).toBe("deleted");
});

t("shared filter grammar refuses malformed and unsupported patterns before storing", async () => {
  for (const bad of ["/(/", "/x/g", "x".repeat(201), "^(a+)+$", "[unclosed"]) {
    expect(await call(`sub add C1 #worker ${bad}`, ownerToken)).toContain("invalid_filter");
  }
  expect(await subCount()).toBe(0);
});

t("help exposes add/list/delete and explains creator visibility", async () => {
  const help = await call("", ownerToken, name, "help");
  expect(help).not.toContain("sub edit");
  expect(help).toContain("sub add"); expect(help).toContain("sub delete");
  expect(help).toContain("Owner and Maintainers see all");
});


t("RT33 a caller refused by the target cannot subscribe through the Slack handler", async () => {
  const blocked = `#rt33-slack-target-${process.pid}@t`, caller = `#plain-${process.pid}@t`;
  await fx.call(ownerToken, "POST", "/register", { name: blocked, kind: "agent", allow: [name] });
  const count = await subCount();
  expect(await call(`sub add C1 ${blocked}`, plainToken)).toContain("admission_refused");
  expect(await subCount()).toBe(count);
  expect(await call("sub list", plainToken)).not.toContain(blocked);
  await fx.call(ownerToken, "POST", "/manage", { name: blocked, allow: [name, caller] });
  const added = JSON.parse(await call(`sub add C1 ${blocked}`, plainToken));
  expect(added.id).toMatch(/^[0-9a-f]{12}$/);
  expect((await stored(added.id)).value.created_by).toBe(caller);
  expect(await call(`sub delete ${added.id}`, plainToken)).toBe("deleted");
});
