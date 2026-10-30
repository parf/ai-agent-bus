import { expect, test } from "bun:test";
import { Codex } from "./codex.ts";
import { startPush } from "./push.ts";

// J60: a manual /compact (or /review) runs as a turn that takes no steering, and
// the App Server answers turn/steer and turn/start with `activeTurnNotSteerable`
// until it ends. These checks run against a fake App Server: no Codex session, and
// no live window, is ever compacted.
const NOT_STEERABLE = { code: -32600, message: "the active turn cannot accept same-turn steering", data: { codexErrorInfo: { activeTurnNotSteerable: { turnKind: "compact" } } } };

type Fake = { url: string; seen: { method: string; params: any }[]; stop: () => void };
function fakeServer(decide: (method: string, params: any, n: number) => { result?: unknown; error?: unknown }): Fake {
  const seen: Fake["seen"] = [];
  const counts = new Map<string, number>();
  const server = Bun.serve({
    port: 0, hostname: "127.0.0.1",
    fetch: (req, s) => (s.upgrade(req) ? undefined : new Response("no", { status: 400 })),
    websocket: { message(ws, raw) {
      const m = JSON.parse(String(raw));
      if (m.id === undefined) return;
      seen.push({ method: m.method, params: m.params });
      const n = (counts.get(m.method) ?? 0) + 1;
      counts.set(m.method, n);
      ws.send(JSON.stringify({ jsonrpc: "2.0", id: m.id, ...decide(m.method, m.params, n) }));
    } },
  });
  return { url: `ws://127.0.0.1:${server.port}`, seen, stop: () => server.stop(true) };
}
const base = (method: string) => method === "thread/start" ? { result: { thread: { id: "t1", turns: [] } } } : { result: {} };
const quick = { steerWaitMs: 2_000, retryMs: 20 };
const attempts = (f: Fake, method: string) => f.seen.filter((c) => c.method === method);

async function codexOn(f: Fake, timing = quick) {
  const lines: string[] = [];
  const c = new Codex("/tmp", (s) => lines.push(s), f.url, "t", timing);
  await c.start();
  await c.openThread();
  return { c, lines };
}

test("a message waits out a /compact and is delivered by turn/start", async () => {
  const f = fakeServer((method, _p, n) => method === "turn/start" ? (n <= 3 ? { error: NOT_STEERABLE } : { result: { turn: { id: "t2" } } }) : base(method));
  const { c, lines } = await codexOn(f);
  try {
    expect(await c.deliver("hello", "m1")).toBe("turn/start");
    expect(attempts(f, "turn/start")).toHaveLength(4); // three refusals while compacting, then the turn
    expect(new Set(attempts(f, "turn/start").map((a) => a.params.clientUserMessageId))).toEqual(new Set(["agent-bus:m1"]));
    expect(lines.join("\n")).toContain("message m1 waits for it to end");
  } finally { c.stop(); f.stop(); }
});

test("a steer rejected because the turn is a /compact waits and starts a turn instead", async () => {
  let compacting = true;
  const f = fakeServer((method, _p, n) => {
    if (method === "turn/start") return n === 1 ? { result: { turn: { id: "t2" } } } : compacting ? { error: NOT_STEERABLE } : { result: { turn: { id: "t3" } } };
    if (method === "turn/steer") return { error: NOT_STEERABLE };
    return base(method);
  });
  const { c } = await codexOn(f);
  try {
    await c.deliver("one", "m1"); // a turn is now active
    setTimeout(() => (compacting = false), 120);
    expect(await c.deliver("two", "m2")).toBe("turn/start");
    expect(attempts(f, "turn/steer")).toHaveLength(1);
    expect(attempts(f, "turn/start").length).toBeGreaterThan(2);
  } finally { c.stop(); f.stop(); }
});

test("another error fails at once, without waiting", async () => {
  const f = fakeServer((method) => method === "turn/start" ? { error: { code: -32603, message: "boom" } } : base(method));
  const { c } = await codexOn(f, { steerWaitMs: 5_000, retryMs: 20 });
  try {
    const began = performance.now();
    await expect(c.deliver("hello", "m1")).rejects.toThrow("boom");
    expect(performance.now() - began).toBeLessThan(1_000);
    expect(attempts(f, "turn/start")).toHaveLength(1);
  } finally { c.stop(); f.stop(); }
});

test("an ordinary busy turn is still steered, never waited for", async () => {
  const f = fakeServer((method) => method === "turn/start" ? { result: { turn: { id: "t2" } } } : method === "turn/steer" ? { result: { turnId: "t2" } } : base(method));
  const { c } = await codexOn(f);
  try {
    expect(await c.deliver("one", "m1")).toBe("turn/start");
    expect(await c.deliver("two", "m2")).toBe("turn/steer");
    expect(f.seen.map((x) => x.method).filter((m) => m.startsWith("turn/"))).toEqual(["turn/start", "turn/steer"]);
    expect(attempts(f, "turn/steer")[0]!.params.expectedTurnId).toBe("t2");
  } finally { c.stop(); f.stop(); }
});

test("past the bound the message fails and its id is logged", async () => {
  const f = fakeServer((method) => method === "turn/start" ? { error: NOT_STEERABLE } : base(method));
  const { c, lines } = await codexOn(f, { steerWaitMs: 150, retryMs: 20 });
  try {
    await expect(c.deliver("hello", "m9")).rejects.toThrow("same-turn steering");
    expect(lines.join("\n")).toContain("message m9 was not delivered");
  } finally { c.stop(); f.stop(); }
});

// The push over the same fake: the next pushed message after a compaction arrives
// and the push keeps running, where it used to stop for good.
test("a session that compacts still receives the next pushed message", async () => {
  const f = fakeServer((method, _p, n) => method === "turn/start" ? (n <= 2 ? { error: NOT_STEERABLE } : { result: { turn: { id: `t${n}` } } }) : method === "turn/steer" ? { result: { turnId: "t3" } } : base(method));
  const { c } = await codexOn(f);
  const queue = [{ message_id: "m1", from: "#a@x", body: "first" }, { message_id: "m2", from: "#a@x", body: "second" }];
  const bus = { name: "#s@x", consume: async () => { const next = queue.shift(); if (next) return next; await Bun.sleep(40); return null; } } as any;
  const said: string[] = [];
  const push = startPush(bus, async (e) => { await c.deliver(e.body ?? "", e.message_id); }, (s) => said.push(s));
  try {
    const delivered = () => f.seen.filter((x) => x.method.startsWith("turn/")).map((x) => x.params.clientUserMessageId);
    for (let i = 0; i < 100 && !delivered().includes("agent-bus:m2"); i++) await Bun.sleep(20);
    expect(delivered()).toContain("agent-bus:m1");
    expect(delivered()).toContain("agent-bus:m2"); // the next pushed message after the compaction
    expect(push.running()).toBe(true);
    expect(push.ended()).toBeUndefined();
    expect(said.join("\n")).not.toContain("push: stopping");
  } finally { push.stop(); await push.done; c.stop(); f.stop(); }
});
