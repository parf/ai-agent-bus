import { afterEach, expect, test } from "bun:test";
import { learnBotId } from "./identity.ts";
import { Receiver } from "./receive.ts";
import { Seen } from "./seen.ts";
import { telegramWeb } from "./telegramapi.ts";

// J90: getMe is asked again, at a capped backoff, until it names the bot; the agent serves meanwhile.
// Every ask goes through telegramWeb to a loopback fake (the J103 guard), never to Telegram.
process.env.AGENT_TEST_LOOPBACK_ONLY = "1";

type Reply = { status: number; body: string };
const refused: Reply = { status: 401, body: JSON.stringify({ ok: false, error_code: 401, description: "Unauthorized" }) };
const malformed: Reply = { status: 200, body: "not json" };
const me: Reply = { status: 200, body: JSON.stringify({ ok: true, result: { id: 777, is_bot: true, username: "fixture_bot" } }) };

let servers: ReturnType<typeof Bun.serve>[] = [];
afterEach(() => { for (const s of servers) s.stop(true); servers = []; });

/** A fake Telegram whose getMe answers next() each time, which may hold the answer; asks counts them. */
function fakeTelegram(next: () => Reply | Promise<Reply>) {
  const seen = { asks: 0 };
  const server = Bun.serve({ port: 0, hostname: "127.0.0.1", async fetch(req) {
    if (!new URL(req.url).pathname.endsWith("/getMe")) return new Response("no", { status: 404 });
    seen.asks++;
    const r = await next();
    return new Response(r.body, { status: r.status, headers: { "content-type": "application/json" } });
  } });
  servers.push(server);
  const ask = (signal?: AbortSignal) => telegramWeb("111:fixture", `http://127.0.0.1:${server.port}`, "read", "getMe", {}, Date.now() + 2000, signal);
  return { seen, ask };
}

/** Timer waits of the backoff's size are recorded and run at once. */
async function fastBackoff<T>(run: (waits: number[]) => Promise<T>): Promise<T> {
  const waits: number[] = [], original = globalThis.setTimeout;
  globalThis.setTimeout = ((fn: any, ms: number, ...args: any[]) => {
    if (Number(ms) <= 1000) { waits.push(Number(ms)); return original(fn, 1, ...args); }
    return original(fn, ms, ...args);
  }) as typeof setTimeout;
  try { return await run(waits); } finally { globalThis.setTimeout = original; }
}

async function until(done: () => boolean) {
  const end = Date.now() + 3000;
  while (!done() && Date.now() < end) await Bun.sleep(2);
  expect(done()).toBe(true);
}

test("getMe failing twice and then answering sets the bot id, with one warning and one recovery line", () => fastBackoff(async waits => {
  const replies = [refused, malformed, me];
  const tg = fakeTelegram(() => replies.shift() ?? me);
  const logs: string[] = [];
  let botId: string | undefined;
  const id = await learnBotId(tg.ask, v => { botId = v; }, { firstMs: 100, capMs: 400, log: l => logs.push(l) });
  expect(id).toBe("777");
  expect(botId).toBe("777");
  expect(tg.seen.asks).toBe(3);
  expect(waits).toEqual([100, 200]);
  expect(logs.length).toBe(2);
  expect(logs[0]).toContain("warning: the bot's Telegram identity is unknown (Unauthorized)");
  expect(logs[1]).toBe("the bot's Telegram identity is known now");
}));

test("getMe failing past the backoff cap warns once, keeps serving, and the id fills in when getMe recovers", () => fastBackoff(async waits => {
  let healthy = false;
  const tg = fakeTelegram(() => (healthy ? me : refused));
  const logs: string[] = [], forwarded: string[] = [];
  let polled = false;
  const receiver = new Receiver(new Seen(), async (_kind, method) => {
    if (method !== "getUpdates" || polled) return { ok: true, result: [] };
    polled = true;
    return { ok: true, result: [{ update_id: 5, message: { message_id: 5, date: 1, text: "from a person", from: { id: 42, username: "person" }, chat: { id: -100, title: "fixture", type: "supergroup" } } }] };
  }, async () => undefined, async () => {}, () => {});
  receiver.onMessage = m => { forwarded.push(String(m.text)); };
  // Distinct sizes, so the receiver's own poll timers are told apart from the backoff's.
  const learning = learnBotId(tg.ask, v => { receiver.botId = v; }, { firstMs: 101, capMs: 404, log: l => logs.push(l) });
  const backoff = () => waits.filter(w => w === 101 || w === 202 || w === 404);
  await receiver.load();
  receiver.start();
  try {
    // Past the cap: the waits doubled to 400 and stayed there, and the identity is still unknown.
    await until(() => backoff().filter(w => w === 404).length >= 3);
    expect(backoff().slice(0, 5)).toEqual([101, 202, 404, 404, 404]);
    expect(receiver.botId).toBeUndefined();
    // A message is still served while the identity is unknown.
    await until(() => forwarded.length === 1);
    expect(forwarded).toEqual(["from a person"]);
    // One warning for the state change, not one per retry.
    expect(logs.length).toBe(1);
    expect(logs[0]).toContain("warning: the bot's Telegram identity is unknown");
    const asked = tg.seen.asks;
    expect(asked).toBeGreaterThanOrEqual(5);
    healthy = true;
    expect(await learning).toBe("777");
    expect(receiver.botId).toBe("777");
    expect(logs).toEqual([logs[0], "the bot's Telegram identity is known now"]);
  } finally { receiver.stop(); }
}));

test("a stop ends the asking at once, with no timer left behind", async () => {
  const tg = fakeTelegram(() => refused);
  const stop = new AbortController(), logs: string[] = [];
  let set = false;
  const learning = learnBotId(tg.ask, () => { set = true; }, { firstMs: 60_000, capMs: 60_000, signal: stop.signal, log: l => logs.push(l) });
  await until(() => tg.seen.asks === 1 && logs.length === 1);
  const started = Date.now();
  stop.abort();
  expect(await learning).toBeUndefined();
  expect(Date.now() - started).toBeLessThan(1000);
  expect(set).toBe(false);
  expect(tg.seen.asks).toBe(1);
});

test("a stop during a held getMe ends the asking at once and nothing is set or said, even when the answer comes later", async () => {
  let release!: (r: Reply) => void;
  const held = new Promise<Reply>(r => { release = r; });
  const tg = fakeTelegram(() => held);
  const stop = new AbortController(), logs: string[] = [];
  let botId: string | undefined;
  let settled = false;
  const learning = learnBotId(tg.ask, v => { botId = v; }, { firstMs: 10, capMs: 10, signal: stop.signal, log: l => logs.push(l) });
  void learning.then(() => { settled = true; });
  await until(() => tg.seen.asks === 1);
  const stopped = Date.now();
  stop.abort();
  await until(() => settled);
  expect(Date.now() - stopped).toBeLessThan(200);
  expect(await learning).toBeUndefined();
  release(me); // the held answer arrives after the stop
  await Bun.sleep(50);
  expect(botId).toBeUndefined();
  expect(logs).toEqual([]);
  expect(tg.seen.asks).toBe(1);
});
