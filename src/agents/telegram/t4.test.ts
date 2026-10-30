import { afterAll, beforeAll, expect, test } from "bun:test";
import { spawn, type Subprocess } from "bun";
import { join } from "node:path";
import { call as busCall, identity, startFixture, type Fixture } from "../slack/daemon.ts";
import { Bus } from "../../clients/typescript/src/index.js";
import { telegramWeb } from "./telegramapi.ts";

// TG.4: the other methods — channel, user, react, edit, delete — each with
// the pinned HTTP policy: a 429 waits retry_after (at most 30 s) and
// retries once, a write is never retried otherwise, a read retries once on
// 5xx (Plans/R1.0-Release/done/telegram/README.md § what carries over). And the
// tg4 P2-a: a bus failure during any method is answered as the bus, never
// as "not configured".

const slow = { timeout: 25000 } as const;
const t = (name: string, fn: () => Promise<void>) => test(name, fn, slow.timeout);

let fx: Fixture;
let callerToken: string;
let agentToken = "";
let fakePort = 0;
const name = `#telegram-t4-${process.pid}@t`;
let agent: Subprocess | undefined;
let calls: { method: string; body: any }[] = [];
let script: (method: string) => any = () => ({ ok: true, result: {} });

const config = { default: "#dev", aliases: { "#dev": "-100123", "@alice": "555" } };

beforeAll(async () => {
  fx = await startFixture();
  callerToken = await identity(fx, `#caller-t4-${process.pid}@t`);
  await fx.call(fx.ownerToken, "POST", "/register", { name, kind: "agent", allow: ["*"] });
  agentToken = (await fx.call(fx.ownerToken, "POST", "/token", { name })).token;
  const fake = Bun.serve({ port: 0, hostname: "127.0.0.1", async fetch(req) {
    const method = new URL(req.url).pathname.split("/").pop()!;
    if (method === "getUpdates") return Response.json({ ok: true, result: [] }); // the receiver polls; not recorded
    const body = await req.json().catch(() => ({}));
    calls.push({ method, body });
    const out = script(method);
    return out instanceof Response ? out : Response.json(out);
  } });
  fakePort = fake.port;
  await fx.call(fx.ownerToken, "POST", "/secret", { name, secret: `TELEGRAM_BOT_TOKEN=111:t4-bot\nTELEGRAM_API_URL=http://127.0.0.1:${fakePort}\n` });
  await fx.call(fx.ownerToken, "POST", "/configure", { name, config });
  agent = spawn(["bun", "telegram.ts"], {
    cwd: import.meta.dir,
    env: { ...process.env, AGENT_BUS_ADDR: fx.socket, AGENT_BUS_NAME: name, AGENT_BUS_TOKEN: agentToken },
    stderr: Bun.file(join(fx.dir, "t4-agent.log")),
    stdout: Bun.file(join(fx.dir, "t4-agent.log")),
  });
  for (let i = 0; i < 200; i++) {
    try {
      const rec = await busCall(fx.socket, fx.ownerToken, "GET", `/lookup?name=${encodeURIComponent(name)}`);
      if (String(rec?.descr ?? "").length > 0) return;
    } catch { /* not yet */ }
    await Bun.sleep(50);
  }
  throw new Error("the agent never registered");
}, 30000);

afterAll(async () => {
  agent?.kill("SIGKILL");
  await Bun.sleep(50);
  try { console.error("T4-AGENT-LOG:\n" + await Bun.file(join(fx.dir, "t4-agent.log")).text()); } catch {}
  await fx?.stop();
}, 15000);

async function call(body: string): Promise<string> {
  const bus = Bus.connect({ addr: fx.socket, token: callerToken });
  const outcome = await bus.call(name, body, { timeout: 10000 });
  return outcome.kind === "answer" ? outcome.message.body : "(timeout)";
}



t("channel resolves the chat and answers Telegram's own fields, in both forms", async () => {
  script = () => ({ ok: true, result: { id: -100123, title: "Dev", type: "supergroup", description: "the dev chat" } });
  expect(await call("channel #dev")).toBe("Dev (supergroup)");
  expect(calls.at(-1)).toEqual({ method: "getChat", body: { chat_id: "-100123" } });
  const answer = JSON.parse(await call(`{"method":"channel","channel":"#dev"}`));
  expect(answer).toEqual({ ok: true, id: -100123, title: "Dev", type: "supergroup", description: "the dev chat" });
  expect(calls.at(-1)).toEqual({ method: "getChat", body: { chat_id: "-100123" } });
});

t("user looks the member up through the declared default chat", async () => {
  script = () => ({ ok: true, result: { user: { id: 555, username: "alice", first_name: "Alice" }, status: "member" } });
  expect(await call("user @alice")).toBe("@alice (member)");
  expect(calls.at(-1)).toEqual({ method: "getChatMember", body: { chat_id: "-100123", user_id: "555" } });
  const answer = JSON.parse(await call(`{"method":"user","who":"555"}`));
  expect(answer).toEqual({ ok: true, id: 555, username: "alice", first_name: "Alice", status: "member" });
  expect(calls.at(-1)!.body.user_id).toBe("555");
});

t("react sets the emoji on the message", async () => {
  script = () => ({ ok: true, result: true });
  expect(await call("react #dev 42 👍")).toBe("reacted 👍 on message 42");
  expect(calls.at(-1)).toEqual({ method: "setMessageReaction", body: { chat_id: "-100123", message_id: 42, reaction: [{ type: "emoji", emoji: "👍" }] } });
  const answer = JSON.parse(await call(`{"method":"react","channel":"#dev","id":42,"emoji":"👍"}`));
  expect(answer).toEqual({ ok: true });
});

t("edit rewrites the message text", async () => {
  script = () => ({ ok: true, result: { message_id: 42, text: "fixed" } });
  expect(await call("edit #dev 42 fixed text")).toBe("message 42 edited");
  expect(calls.at(-1)).toEqual({ method: "editMessageText", body: { chat_id: "-100123", message_id: 42, text: "fixed text" } });
  const answer = JSON.parse(await call(`{"method":"edit","channel":"#dev","id":42,"text":"fixed"}`));
  expect(answer).toEqual({ ok: true, message_id: 42, text: "fixed" });
});

t("delete removes the message", async () => {
  script = () => ({ ok: true, result: true });
  expect(await call("delete #dev 42")).toBe("message 42 deleted");
  expect(calls.at(-1)).toEqual({ method: "deleteMessage", body: { chat_id: "-100123", message_id: 42 } });
  const answer = JSON.parse(await call(`{"method":"delete","channel":"#dev","id":42}`));
  expect(answer).toEqual({ ok: true, message_id: 42 });
});

t("a 429 waits retry_after and retries the write once", async () => {
  let n = 0;
  script = () => (++n === 1 ? new Response(JSON.stringify({ ok: false, parameters: { retry_after: 1 } }), { status: 429 }) : Response.json({ ok: true, result: { message_id: 7, chat: { id: -100123 } } }));
  const answer = await call("post #dev retried");
  expect(n).toBe(2); // the write was attempted again after the wait
  expect(answer).toBe("#dev message 7");
});

t("a 429 whose wait cannot fit the deadline is refused, not waited out", async () => {
  // Directly against the policy: a retry_after beyond the deadline refuses
  // at once (the unit seam the agent's 60 s default cannot test cheaply).
  let n = 0;
  const fake = Bun.serve({ port: 0, hostname: "127.0.0.1", async fetch(req) {
    await req.json().catch(() => ({}));
    n++;
    return Response.json({ ok: false, parameters: { retry_after: 45 }, description: "Too Many Requests" }, { status: 429 });
  } });
  try {
    const r = await telegramWeb("111:t", `http://127.0.0.1:${fake.port}`, "write", "sendMessage", { chat_id: 1, text: "x" }, Date.now() + 5000);
    expect(r.ok).toBe(false);
    expect(n).toBe(1); // one look, then the refusal: no 45 s wait
  } finally {
    fake.stop(true);
  }
});

t("a write is never retried on a 5xx", async () => {
  let n = 0;
  script = () => { n++; return new Response("upstream exploded", { status: 502 }); };
  const answer = await call("post #dev once only");
  expect(n).toBe(1); // exactly one attempt for a write
  expect(answer).toContain("error:");
});

t("a read retries once on a 5xx and succeeds", async () => {
  let n = 0;
  script = () => { n++; return n === 1 ? new Response("upstream exploded", { status: 500 }) : { ok: true, result: { id: -100123, title: "Dev", type: "supergroup" } }; };
  expect(await call("channel #dev")).toBe("Dev (supergroup)");
  expect(n).toBe(2);
});

t("a bus that does not answer /config is answered as the bus during a post (tg4 P2-a)", async () => {
  // A second agent, reached through a proxy whose GET /config fails: the
  // start path exits nonzero (not 78), a post through it is answered as
  // the bus in both forms, and with /config passing again it posts.
  const name2 = `#telegram-t4p-${process.pid}@t`;
  await fx.call(fx.ownerToken, "POST", "/register", { name: name2, kind: "agent", allow: ["*"] });
  const token2 = (await fx.call(fx.ownerToken, "POST", "/token", { name: name2 })).token;
  await fx.call(fx.ownerToken, "POST", "/secret", { name: name2, secret: `TELEGRAM_BOT_TOKEN=111:t4p\nTELEGRAM_API_URL=http://127.0.0.1:${fakePort}\n` });
  await fx.call(fx.ownerToken, "POST", "/configure", { name: name2, config });
  const proxyPath = join(fx.dir, `t4-proxy-${process.pid}.sock`);
  let failing = true;
  const server = Bun.serve({ unix: proxyPath, async fetch(req) {
    const url = new URL(req.url);
    if (failing && url.pathname === "/config") return new Response("the store is busy", { status: 503 });
    return fetch(`http://unix${url.pathname}${url.search}`, { method: req.method, headers: req.headers, body: req.body, unix: fx.socket } as RequestInit);
  } });
  try {
    const through = spawn(["bun", "telegram.ts"], {
      cwd: import.meta.dir,
      env: { ...process.env, AGENT_BUS_ADDR: proxyPath, AGENT_BUS_NAME: name2, AGENT_BUS_TOKEN: token2 },
      stderr: Bun.file(join(fx.dir, "t4-proxy-agent.log")),
      stdout: Bun.file(join(fx.dir, "t4-proxy-agent.log")),
    });
    const code = await Promise.race([through.exited, Bun.sleep(15_000).then(() => -1)]);
    expect(code).not.toBe(78);
    expect(code).not.toBe(0);
    const log = await Bun.file(join(fx.dir, "t4-proxy-agent.log")).text();
    expect(log).toContain("telegram: the bus did not answer");
    // Passing through: it serves, and a post goes to Telegram.
    failing = false;
    const serving = spawn(["bun", "telegram.ts"], {
      cwd: import.meta.dir,
      env: { ...process.env, AGENT_BUS_ADDR: proxyPath, AGENT_BUS_NAME: name2, AGENT_BUS_TOKEN: token2 },
      stderr: Bun.file(join(fx.dir, "t4-proxy-agent.log")),
      stdout: Bun.file(join(fx.dir, "t4-proxy-agent.log")),
    });
    const before = calls.length;
    const bus = Bus.connect({ addr: fx.socket, token: callerToken });
    let answered = "";
    for (let i = 0; i < 40 && !answered; i++) {
      const outcome = await bus.call(name2, "post #dev through the proxy", { timeout: 5000 }).catch(() => ({ kind: "timeout" }));
      if (outcome.kind === "answer") answered = outcome.message.body;
      await Bun.sleep(100);
    }
    expect(answered).toContain("message");
    expect(calls.length).toBeGreaterThan(before); // sendMessage reached Telegram
    serving.kill("SIGKILL");
  } finally {
    server.stop(true);
  }
}, 40000);

t("a 429 whose retry_after outlives the caller's deadline refuses at once (tg4 P2-a)", async () => {
  // A second agent on a fake whose sendMessage answers 429 with
  // retry_after=3: the caller's bus deadline of 1.5 s must be answered
  // "error: ratelimited" in time, after exactly one attempt.
  const name2 = `#telegram-t4r-${process.pid}@t`;
  await fx.call(fx.ownerToken, "POST", "/register", { name: name2, kind: "agent", allow: ["*"] });
  const token2 = (await fx.call(fx.ownerToken, "POST", "/token", { name: name2 })).token;
  let n = 0;
  const own = Bun.serve({ port: 0, hostname: "127.0.0.1", async fetch(req) {
    await req.json().catch(() => ({}));
    if (new URL(req.url).pathname.endsWith("/sendMessage")) n++; // only sendMessage attempts count; the receiver's getUpdates polls don't
    return Response.json({ ok: false, parameters: { retry_after: 3 }, description: "Too Many Requests" }, { status: 429 });
  } });
  await fx.call(fx.ownerToken, "POST", "/secret", { name: name2, secret: `TELEGRAM_BOT_TOKEN=111:t4r\nTELEGRAM_API_URL=http://127.0.0.1:${own.port}\n` });
  await fx.call(fx.ownerToken, "POST", "/configure", { name: name2, config });
  try {
    const proc = spawn(["bun", "telegram.ts"], {
      cwd: import.meta.dir,
      env: { ...process.env, AGENT_BUS_ADDR: fx.socket, AGENT_BUS_NAME: name2, AGENT_BUS_TOKEN: token2 },
      stderr: Bun.file(join(fx.dir, "t4-retry-agent.log")),
      stdout: Bun.file(join(fx.dir, "t4-retry-agent.log")),
    });
    for (let i = 0; i < 200; i++) {
      try {
        const rec = await busCall(fx.socket, fx.ownerToken, "GET", `/lookup?name=${encodeURIComponent(name2)}`);
        if (String(rec?.descr ?? "").length > 0) break;
      } catch { /* not yet */ }
      await Bun.sleep(50);
    }
    await Bun.sleep(200);
    const started = Date.now();
    const bus = Bus.connect({ addr: fx.socket, token: callerToken });
    const outcome = await bus.call(name2, "post #dev too slow", { timeout: 1500 });
    const elapsed = Date.now() - started;
    expect(outcome.kind).toBe("answer");
    expect(outcome.kind === "answer" && outcome.message.body).toBe("error: ratelimited");
    expect(elapsed).toBeLessThan(2800); // answered within the caller's deadline, not at 3029 ms
    expect(n).toBe(1); // one attempt, no retry after the caller was gone
    proc.kill("SIGKILL");
  } finally {
    own.stop(true);
  }
}, 20000);
