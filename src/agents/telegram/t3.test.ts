import { afterAll, beforeAll, expect, test } from "bun:test";
import { spawn, type Subprocess } from "bun";
import { join } from "node:path";
import { call as busCall, identity, startFixture, type Fixture } from "../slack/daemon.ts";
import { Bus } from "../../clients/typescript/src/index.js";

// TG.3: post reaches the fake's sendMessage with the resolved chat id — an
// alias through the configuration, a numeric id as is, and the declared
// default when the call names no destination. Both refusals — an unknown
// alias, and no destination with no default — send nothing
// (Plans/R1.0-Release/telegram/TODO.md TG.3).

const slow = { timeout: 25000 } as const;
const t = (name: string, fn: () => Promise<void>) => test(name, fn, slow.timeout);

let fx: Fixture;
let callerToken = "";
let fake: { port: number };
let fakeServer: ReturnType<typeof Bun.serve>;
const sent: any[] = [];
const name = `#telegram-t3-${process.pid}@t`;
let agentToken = "";
let agent: Subprocess | undefined;

beforeAll(async () => {
  fx = await startFixture();
  callerToken = await identity(fx, `#caller-t3-${process.pid}@t`);
  await fx.call(fx.ownerToken, "POST", "/register", { name, kind: "agent", allow: ["*"] });
  const token = (await fx.call(fx.ownerToken, "POST", "/token", { name })).token;
  agentToken = token;
  fakeServer = Bun.serve({ port: 0, hostname: "127.0.0.1", async fetch(req) {
    const url = new URL(req.url);
    if (url.pathname.endsWith("/sendMessage")) {
      const body = await req.json();
      sent.push(body);
      return Response.json({ ok: true, result: { message_id: 4096, chat: { id: Number(body.chat_id) } } });
    }
    return Response.json({ ok: true, result: { id: 42, is_bot: true, username: "t3_bot" } });
  } });
  fake = { port: fakeServer.port };
  await fx.call(fx.ownerToken, "POST", "/secret", { name, secret: `TELEGRAM_BOT_TOKEN=111:t3-bot\nTELEGRAM_API_URL=http://127.0.0.1:${fake.port}\n` });
  agent = startAgent(token, name);
  for (let i = 0; i < 200; i++) {
    try {
      const rec = await busCall(fx.socket, fx.ownerToken, "GET", `/lookup?name=${encodeURIComponent(name)}`);
      if (String(rec?.descr ?? "").startsWith("telegram:")) return;
    } catch { /* not yet */ }
    await Bun.sleep(50);
  }
  throw new Error("the agent never registered");
}, 30000);

afterAll(async () => {
  agent?.kill("SIGKILL");
  await Bun.sleep(50);
  fakeServer?.stop(true);
  await fx?.stop();
}, 15000);

function startAgent(token: string, agentName: string): Subprocess {
  const proc = spawn(["bun", "telegram.ts"], {
    cwd: import.meta.dir,
    env: { ...process.env, AGENT_BUS_ADDR: fx.socket, AGENT_BUS_NAME: agentName, AGENT_BUS_TOKEN: token },
    stderr: Bun.file(join(fx.dir, `t3-agent.log`)),
    stdout: Bun.file(join(fx.dir, `t3-agent.log`)),
  });
  return proc;
}

async function call(body: string): Promise<string> {
  const bus = Bus.connect({ addr: fx.socket, token: callerToken });
  const outcome = await bus.call(name, body, { timeout: 10000 });
  return outcome.kind === "answer" ? outcome.message.body : "(timeout)";
}

const setConfig = (config: unknown) => fx.call(fx.ownerToken, "POST", "/configure", { name, config });

const defaultConfig = { default: "#dev", aliases: { "#dev": "-100123", "@alice": "555" } };

t("an aliased chat reaches sendMessage as its chat id", async () => {
  await setConfig(defaultConfig);
  expect(await call("post #dev hello there")).toBe("#dev message 4096");
  expect(sent.at(-1)).toEqual({ chat_id: "-100123", text: "hello there" });
});

t("a numeric chat id is used as is, declared or not", async () => {
  await setConfig(defaultConfig);
  expect(await call("post -100999 numbers work")).toBe("-100999 message 4096");
  expect(sent.at(-1)).toEqual({ chat_id: "-100999", text: "numbers work" });
});

t("an omitted destination takes the declared default", async () => {
  await setConfig(defaultConfig);
  expect(await call("post by default")).toBe("#dev message 4096");
  expect(sent.at(-1)).toEqual({ chat_id: "-100123", text: "by default" });
});

t("the JSON form carries channel and reply_to", async () => {
  await setConfig(defaultConfig);
  expect(await call(`{"method":"post","channel":"@alice","text":"hi","reply_to":7}`)).toBe("@alice message 4096");
  expect(sent.at(-1)).toEqual({ chat_id: "555", text: "hi", reply_to_message_id: 7 });
});

t("an unknown alias is refused with no send", async () => {
  await setConfig(defaultConfig);
  const before = sent.length;
  expect(await call("post #nope hi")).toBe("error: unknown alias #nope");
  expect(sent.length).toBe(before); // nothing reached Telegram
});

t("no destination and no default is refused, naming the key", async () => {
  await setConfig({ aliases: { "#dev": "-100123" } });
  const before = sent.length;
  expect(await call("post bare words")).toBe("error: no destination and no default in the configuration");
  expect(sent.length).toBe(before); // nothing reached Telegram
});

t("a changed configuration reaches the next call without a restart", async () => {
  await setConfig({ default: "@alice", aliases: { "#dev": "-100123", "@alice": "555" } });
  expect(await call("post moved")).toBe("@alice message 4096");
  expect(sent.at(-1)).toEqual({ chat_id: "555", text: "moved" });
});
