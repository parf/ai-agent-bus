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

let made = "";

t("the owner adds, lists, edits fields and deletes, through the KV", async () => {
  expect((await fx.call(ownerToken, "GET", `/lookup?name=${encodeURIComponent(name)}`)).descr).toBe(manifest.description);
  const answer = await call("sub add C1 #worker ^agent: keep going", ownerToken);
  made = JSON.parse(answer).id;
  expect(typeof made).toBe("string");
  const value = await stored(made);
  expect(value.value.channel).toBe("C1");
  expect(value.value.target).toBe("#worker");
  expect(value.value.filter).toBe("^agent: keep going"); // multi-word survives
  expect(await call("sub list", plainToken)).toContain(`${made} C1 #worker ^agent: keep going`);
  expect(await call(`sub edit ${made} channel=C2 target=#other`, ownerToken)).toBe("edited");
  const edited = await stored(made);
  expect(edited.value.channel).toBe("C2");
  expect(edited.value.target).toBe("#other");
  expect(edited.value.filter).toBe("^agent: keep going"); // untouched field kept
  expect(await call(`sub edit ${made} filter=`, ownerToken)).toBe("edited");
  expect((await stored(made)).value.filter).toBeUndefined(); // filter= clears it
  expect(await call(`sub delete ${made}`, ownerToken)).toBe("deleted");
  expect(await subCount()).toBe(0);
});

t("an ACL-only caller lists but cannot change, and a body claiming roles changes nothing", async () => {
  expect(await call("sub list", plainToken)).toBe("(no subscriptions)");
  expect(await call("sub add C1 #w", plainToken)).toContain("error:");
  expect(JSON.parse(await call('{"method":"sub.add","channel":"C1","target":"#w","roles":["owner"]}', plainToken)).ok).toBe(false);
  expect(await subCount()).toBe(0);
});

t("a direct Maintainer is a positive control", async () => {
  const answer = await call("sub add C3 #ops", maintToken);
  expect(JSON.parse(answer).id).toBeDefined();
  expect(await call(`sub delete ${JSON.parse(answer).id}`, maintToken)).toBe("deleted");
});

t("a Maintainer arriving through a forward is refused: roles count only when addressed here", async () => {
  // #front's maintainer calls #front, which forwards to #slack with
  // original_to=#front and roles=[maintainer] — asserted for #front, not us.
  const answer = await call("sub add C9 #sneak", frontToken, `#front-${process.pid}@t`);
  expect(answer).toContain("error:");
  expect(await subCount()).toBe(0);
});

t("bad filters are refused on add and on edit, the old value kept", async () => {
  for (const bad of ["/x/g", "/x/y", "x".repeat(201), "[unclosed"]) {
    expect(await call(`sub add C1 #w ${bad}`, ownerToken)).toContain("error:");
  }
  const good = await call("sub add C1 #w ok", ownerToken);
  const id = JSON.parse(good).id;
  for (const bad of ["/x/g", "x".repeat(201), "[unclosed"]) {
    expect(await call(`sub edit ${id} filter=${bad}`, ownerToken)).toContain("error:");
  }
  expect((await stored(id)).value.filter).toBe("ok");
  await call(`sub delete ${id}`, ownerToken);
});

t("two disjoint edits of one subscription both land", async () => {
  const id = JSON.parse(await call("sub add C1 #first", ownerToken)).id;
  const [a, b] = await Promise.all([
    call(`sub edit ${id} channel=C7`, ownerToken),
    call(`sub edit ${id} target=#second`, ownerToken),
  ]);
  expect(a).toBe("edited");
  expect(b).toBe("edited");
  const value = (await stored(id)).value;
  expect(value.channel).toBe("C7");
  expect(value.target).toBe("#second");
  await call(`sub delete ${id}`, ownerToken);
});

// An ask for help describes the agent for an AI reader, as the caller may
// use it (docs/constitution.md#agent-self-description): every caller learns
// subscriptions follow their destination; only #slack's Owner and
// Maintainers are told they may change any, in the description and the
// short help.
t("manifest help describes destination management and restricts refused to direct managers", async () => {
  for (const token of [ownerToken, maintToken]) {
    const full = await call("", token, name, "help");
    expect(full).toContain("sub add <channel> <target>");
    expect(full).toContain("refused:");
    expect(await call("help", token)).toBe(full);
  }
  for (const token of [plainToken, frontToken]) {
    const full = await call("", token, name, "help");
    expect(full).toContain("post <channel>");
    expect(full).toContain("<text...>");
    expect(full).toContain("Bus destination; you must manage it");
    expect(full).not.toContain("refused:");
    expect(await call("help", token)).toBe(full);
  }
});

t("--help in a body is ordinary text, not an ask", async () => {
  expect(await call("--help", ownerToken)).not.toContain("Slack on the bus");
});

// S.8 (Plans/R1.2/done/slack/TODO.md): a subscription is added, edited and deleted
// by whoever manages its destination, as the daemon answers it, or by
// #slack's own Owner and Maintainers; its creator counts for nothing.
const d1 = `#d1-${process.pid}@t`; // maintained by #plain and #bob
const d2 = `#d2-${process.pid}@t`; // the fixture owner's alone
t("S.8: whoever manages the destination adds, edits and deletes; nobody else does", async () => {
  for (const d of [d1, d2]) await fx.call(ownerToken, "POST", "/register", { name: d, kind: "agent", allow: ["*"] });
  await fx.call(ownerToken, "POST", "/manage", { name: d1, maintainers: [`#plain-${process.pid}@t`, `#bob-${process.pid}@t`] });
  const before = await subCount();
  // An ACL-only caller managing d1 adds, edits and deletes there.
  const mine = JSON.parse(await call(`sub add C1 ${d1}`, plainToken)).id;
  expect(typeof mine).toBe("string");
  expect(await call(`sub edit ${mine} filter=/x/i`, plainToken)).toBe("edited");
  // It may not aim at a destination it does not control: refused, nothing written.
  expect(await call(`sub add C1 ${d2}`, plainToken)).toContain(`does not control ${d2}`);
  expect(await call(`sub edit ${mine} target=${d2}`, plainToken)).toContain(`does not control ${d2}`);
  expect((await stored(mine)).value.target).toBe(d1);
  // One on d2, made by the owner: the plain caller changes neither.
  const theirs = JSON.parse(await call(`sub add C2 ${d2}`, ownerToken)).id;
  expect(await call(`sub edit ${theirs} filter=x`, plainToken)).toContain(`does not control ${d2}`);
  expect(await call(`sub delete ${theirs}`, plainToken)).toContain(`does not control ${d2}`);
  expect((await stored(theirs)).value.filter).toBeUndefined();
  // Nor takes it over by moving it to a destination it does control.
  expect(await call(`sub edit ${theirs} target=${d1}`, plainToken)).toContain(`does not control ${d2}`);
  expect((await stored(theirs)).value.target).toBe(d2);
  // #slack's Maintainer changes any, even to a destination it does not manage.
  expect(await call(`sub edit ${mine} target=${d2}`, maintToken)).toBe("edited");
  expect(await call(`sub edit ${mine} target=${d1}`, maintToken)).toBe("edited");
  // Control is the sender's: #bob, maintaining d1, reaches #slack through a forward.
  const via = JSON.parse(await call(`sub add C3 ${d1}`, frontToken, `#front-${process.pid}@t`)).id;
  expect(typeof via).toBe("string");
  // A creator who loses control keeps its subscription: control is not rechecked.
  await fx.call(ownerToken, "POST", "/manage", { name: d1, maintainers: [] });
  expect(await call("sub list", ownerToken)).toContain(`${mine} C1 ${d1}`);
  expect(await call(`sub delete ${mine}`, plainToken)).toContain(`does not control ${d1}`);
  for (const id of [mine, theirs, via]) expect(await call(`sub delete ${id}`, ownerToken)).toBe("deleted");
  expect(await subCount()).toBe(before);
});

t("an ask for its MCP tools answers the MCP shape for none, and changes nothing", async () => {
  const before = await subCount();
  expect(await call("", plainToken, name, "mcp")).toBe('{"tools":[]}');
  expect(await subCount()).toBe(before);
});

// RD141: even one who may change any subscription needs the right to send to
// the target itself, at add and when an edit moves the target.
t("the author needs may-send to the target, whatever else it may change", async () => {
  const guarded = `#guarded-${process.pid}@t`;
  await fx.call(ownerToken, "POST", "/register", { name: guarded, kind: "agent", allow: [name] }); // the agent sees it; #maint does not
  const before = await subCount();
  expect(await call(`sub add C1 ${guarded}`, maintToken)).toContain(`may not send to ${guarded}`);
  expect(await subCount()).toBe(before); // nothing stored
  const id = JSON.parse(await call("sub add C1 #first", maintToken)).id;
  expect(await call(`sub edit ${id} target=${guarded}`, maintToken)).toContain(`may not send to ${guarded}`);
  expect((await stored(id)).value.target).toBe("#first");
  expect(await call(`sub delete ${id}`, maintToken)).toBe("deleted");
  const mine = JSON.parse(await call(`sub add C1 ${guarded}`, ownerToken)).id;
  expect(await call(`sub delete ${mine}`, ownerToken)).toBe("deleted");
});
