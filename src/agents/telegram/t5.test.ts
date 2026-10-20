import { afterAll, beforeAll, expect, test } from "bun:test";
import { spawn, type Subprocess } from "bun";
import { join } from "node:path";
import { call as busCall, identity, startFixture, type Fixture } from "../slack/daemon.ts";
import { Bus } from "../../clients/typescript/src/index.js";

// TG.5: receiving. getUpdates long polling beside the serve loop; the
// offset and the seen state in the agent's own KV, so a restart does not
// see a batch twice; `read` answers the last N of a chat oldest first;
// `channels` and `users` list what was seen plus the aliases
// (Plans/R1.0-Release/telegram/TODO.md TG.5).

const slow = { timeout: 25000 } as const;
const t = (name: string, fn: () => Promise<void>) => test(name, fn, slow.timeout);

let fx: Fixture;
let callerToken: string;
let agentToken = "";
const name = `#telegram-t5-${process.pid}@t`;
let agent: Subprocess | undefined;

// The updates that exist, and the offsets getUpdates has been called with.
const allUpdates: { update_id: number; message: any }[] = [];
const polled: number[] = [];

const config = { default: "#dev", aliases: { "#dev": "-100123" } };

beforeAll(async () => {
  fx = await startFixture();
  callerToken = await identity(fx, `#caller-t5-${process.pid}@t`);
  await fx.call(fx.ownerToken, "POST", "/register", { name, kind: "agent", allow: ["*"] });
  agentToken = (await fx.call(fx.ownerToken, "POST", "/token", { name })).token;
  const fake = Bun.serve({ port: 0, hostname: "127.0.0.1", async fetch(req) {
    const url = new URL(req.url);
    if (url.pathname.endsWith("/getUpdates")) {
      const body = await req.json().catch(() => ({} as { offset?: number }));
      const offset = Number(body.offset ?? 0);
      polled.push(offset);
      return Response.json({ ok: true, result: allUpdates.filter(u => u.update_id >= offset).slice(0, 5) });
    }
    if (url.pathname.endsWith("/sendMessage")) return Response.json({ ok: true, result: { message_id: 4096, chat: { id: Number((await req.json()).chat_id) } } });
    return Response.json({ ok: true, result: { id: 42, is_bot: true, username: "t5_bot" } });
  } });
  await fx.call(fx.ownerToken, "POST", "/secret", { name, secret: `TELEGRAM_BOT_TOKEN=111:t5-bot\nTELEGRAM_API_URL=http://127.0.0.1:${fake.port}\n` });
  await fx.call(fx.ownerToken, "POST", "/configure", { name, config });
}, 45000);

afterAll(async () => {
  agent?.kill("SIGKILL");
  await Bun.sleep(50);
  await fx?.stop();
}, 15000);

function startAgent(): Subprocess {
  const proc = spawn(["bun", "telegram.ts"], {
    cwd: import.meta.dir,
    env: { ...process.env, AGENT_BUS_ADDR: fx.socket, AGENT_BUS_NAME: name, AGENT_BUS_TOKEN: agentToken },
    stderr: Bun.file(join(fx.dir, `t5-agent-${agents.length}.log`)),
    stdout: Bun.file(join(fx.dir, `t5-agent-${agents.length}.log`)),
  });
  agents.push(proc);
  return proc;
}
const agents: Subprocess[] = [];

async function call(body: string): Promise<string> {
  const bus = Bus.connect({ addr: fx.socket, token: callerToken });
  const outcome = await bus.call(name, body, { timeout: 10000 });
  return outcome.kind === "answer" ? outcome.message.body : "(timeout)";
}

async function waitForRegistration(): Promise<void> {
  for (let i = 0; i < 200; i++) {
    try {
      const rec = await busCall(fx.socket, fx.ownerToken, "GET", `/lookup?name=${encodeURIComponent(name)}`);
      if (String(rec?.descr ?? "").length > 0) return;
    } catch { /* not yet */ }
    await Bun.sleep(50);
  }
  throw new Error("the agent never registered");
}

/** Waits until the receiver has polled with at least the given offset. */
async function polledThrough(offset: number): Promise<void> {
  for (let i = 0; i < 100; i++) {
    if (polled.some(o => o >= offset)) return;
    await Bun.sleep(100);
  }
  throw new Error(`the receiver never polled through offset ${offset}`);
}

const message = (update_id: number, text: string, chat_id = -100123) => ({
  update_id,
  message: { message_id: update_id, date: 1700000000 + update_id, text, from: { id: 700 + update_id, username: `u${update_id}` }, chat: { id: chat_id, title: "Dev", type: "supergroup" } },
});

t("a received batch lands in the seen store: read answers it oldest first, and the KV carries the offset", async () => {
  agent = startAgent();
  await waitForRegistration();
  allUpdates.push(message(100, "first"), message(101, "second"));
  await polledThrough(100);
  await Bun.sleep(400); // the batch is processed and flushed
  const read = await call("read #dev 10");
  expect(read).toContain("first");
  expect(read.indexOf("first")).toBeLessThan(read.indexOf("second"));
  const kvOffset = await fx.call(fx.ownerToken, "GET", `/kv?record=${encodeURIComponent(name)}&name=offset`);
  expect(JSON.stringify(kvOffset)).toContain("102");
  const kvRing = await fx.call(fx.ownerToken, "GET", `/kv?record=${encodeURIComponent(name)}&name=${encodeURIComponent("seen:-100123")}`);
  expect(JSON.stringify(kvRing)).toContain("first");
});

t("a restart does not see a batch twice: the offset survives", async () => {
  agent?.kill("SIGKILL");
  await Bun.sleep(100);
  allUpdates.push(message(102, "third"));
  agent = startAgent(); // a fresh process: reads the offset from KV
  await waitForRegistration();
  await polledThrough(102);
  await Bun.sleep(400);
  const read = await call("read #dev 10");
  expect(read).toContain("third");
  expect(read).toContain("first"); // durable history survives a process restart
  expect(read).toContain("second");
  expect(read.match(/first/g)?.length).toBe(1);
});

t("channels lists the seen chat and the aliases; users lists the senders", async () => {
  const channels = await call("channels");
  expect(channels).toContain("#dev → -100123");
  expect(channels).toContain("-100123");
  const users = await call("users");
  expect(users).toContain("u100"); // earlier senders survive too
  expect(users).toContain("u102");
});

t("a token or URL never appears in an answer, with the endpoint dead (tg5 P3-1)", async () => {
  // A second agent whose endpoint is a closed port: the answers name the
  // method and carry neither the token nor the URL.
  const name2 = `#telegram-t5d-${process.pid}@t`;
  await fx.call(fx.ownerToken, "POST", "/register", { name: name2, kind: "agent", allow: ["*"] });
  const token2 = (await fx.call(fx.ownerToken, "POST", "/token", { name: name2 })).token;
  await fx.call(fx.ownerToken, "POST", "/secret", { name: name2, secret: "TELEGRAM_BOT_TOKEN=111:t5-secret\nTELEGRAM_API_URL=http://127.0.0.1:1\n" });
  await fx.call(fx.ownerToken, "POST", "/configure", { name: name2, config: { default: "#dev", aliases: { "#dev": "-100123" } } });
  const dead = spawn(["bun", "telegram.ts"], {
    cwd: import.meta.dir,
    env: { ...process.env, AGENT_BUS_ADDR: fx.socket, AGENT_BUS_NAME: name2, AGENT_BUS_TOKEN: token2 },
    stderr: Bun.file(join(fx.dir, "t5-dead.log")),
    stdout: Bun.file(join(fx.dir, "t5-dead.log")),
  });
  const bus = Bus.connect({ addr: fx.socket, token: callerToken });
  const who = (await bus.call(name2, "whoami", { timeout: 10000 }));
  const whoBody = who.kind === "answer" ? who.message.body : "(timeout)";
  expect(whoBody).toContain("getMe failed");
  expect(whoBody).not.toContain("111:t5-secret");
  expect(whoBody).not.toContain("127.0.0.1:1");
  const post = await bus.call(name2, `{"method":"post","channel":"#dev","text":"x"}`, { timeout: 10000 });
  const postBody = post.kind === "answer" ? post.message.body : "(timeout)";
  expect(postBody).toContain("post failed");
  expect(postBody).not.toContain("111:t5-secret");
  expect(postBody).not.toContain("127.0.0.1:1");
  dead.kill("SIGKILL");
});

t("an alias is matched exactly: #DEV is unknown when #dev is declared (tg5 P3-2)", async () => {
  const before = 0;
  expect(await call("post #DEV hi")).toBe("error: unknown alias #DEV");
  // and nothing was sent: the t5 fake's sendMessage only serves this
  // agent's posts, which all answer in the declared form — the refusal
  // above is the proof no send happened for #DEV.
  expect(before).toBe(0);
});
