import { expect, test } from "bun:test";
import { startPush, superviseDelivery, type Held, type Push } from "../mcp/push.ts";
import { Codex, transientDelivery } from "../mcp/codex.ts";
import { pushBinders } from "./push-binding.ts";

// J60b: the standard ab-codex and opencode push is started by the launcher (launcher.ts, through push-binding.ts),
// not by the MCP face, and the supervisor restarts it there. A message the failed push consumed is delivered first
// by the next one only if every start shares the launcher's one holder. These tests drive the launcher's own
// bindThread and bindSession, the real Codex client against a fake App Server answering a compaction with the live
// error text, and the supervisor's restart; nothing hands a holder in by hand.
const REAL_COMPACT = { code: -32603, message: "failed to submit turn input: ActiveTurnNotSteerable { turn_kind: Compact }" };

function fakeAppServer(decide: (method: string, params: any) => { result?: unknown; error?: unknown }) {
  const seen: { method: string; params: any }[] = [];
  const server = Bun.serve({
    port: 0, hostname: "127.0.0.1",
    fetch: (req, s) => (s.upgrade(req) ? undefined : new Response("no", { status: 400 })),
    websocket: { message(ws, raw) {
      const m = JSON.parse(String(raw));
      if (m.id === undefined) return;
      seen.push({ method: m.method, params: m.params });
      ws.send(JSON.stringify({ jsonrpc: "2.0", id: m.id, ...decide(m.method, m.params) }));
    } },
  });
  return { url: `ws://127.0.0.1:${server.port}`, seen, stop: () => server.stop(true) };
}

function inbox(...ids: string[]) {
  const queue = ids.map((id) => ({ message_id: id, from: "#a@x", to: "#s@x", body: `body of ${id}` }));
  return { name: "#s@x", consume: async () => { const next = queue.shift(); if (next) return next; await Bun.sleep(20); return null; } } as any;
}

// What the launcher does after a failed delivery: its supervisor restarts through the same bindThread/bindSession.
async function restartAfterFailure(first: () => Push | undefined, rebind: () => void) {
  let t = 0;
  const check = superviseDelivery(first, rebind, { note: () => {}, warn: () => {} }, { delays: [100, 100, 100], now: () => t });
  t = 1_000; await check(); // the wait starts when the stop is seen
  t = 2_000; await check(); // and the restart follows it
}

const binding = (over: Record<string, unknown>) => ({
  runtime: "codex", session: { id: "thread-1" }, bindings: { lock: () => true, save: () => {} },
  bus: () => inbox(), env: () => ({}), name: () => "#s@x", base: () => "#s@x", log: () => {}, warn: () => {}, ...over,
}) as any;

test("the launcher's bindThread delivers a message consumed during a compaction after its restart, before the next one", async () => {
  let compacting = true;
  const app = fakeAppServer((method) => method === "thread/start" ? { result: { thread: { id: "thread-1", turns: [] } } }
    : method === "turn/start" ? (compacting ? { error: REAL_COMPACT } : { result: { turn: { id: "t2" } } }) : { result: {} });
  const codex = new Codex("/tmp", () => {}, app.url, "t", { steerWaitMs: 100, retryMs: 20 });
  await codex.start(); await codex.openThread();
  let push: Push | undefined;
  const bus = inbox("m1", "m2");
  const { bindThread } = pushBinders(binding({ bus: () => bus, codex, setPush: (p: Push) => { push = p; } }));
  try {
    bindThread({ id: "thread-1" });
    await push!.done;
    expect(push!.ended()).toBe("delivery"); // the compaction outlasted the wait
    compacting = false;
    await restartAfterFailure(() => push, () => bindThread({ id: "thread-1" }));
    const sent = () => app.seen.filter((x) => x.method.startsWith("turn/")).map((x) => x.params.clientUserMessageId);
    for (let i = 0; i < 100 && !sent().includes("agent-bus:m2"); i++) await Bun.sleep(20);
    expect(sent().lastIndexOf("agent-bus:m1")).toBeLessThan(sent().indexOf("agent-bus:m2")); // m1 was not lost, and came first
  } finally { push?.stop(); await push?.done; codex.stop(); app.stop(); }
});

test("the launcher's bindSession does the same for OpenCode", async () => {
  let broken = true;
  const delivered: string[] = [];
  const opencode = { bind: () => {}, deliver: async (_text: string, id: string) => { if (broken && id === "m1") throw new Error("opencode: prompt timed out"); delivered.push(id); } };
  let push: Push | undefined;
  const bus = inbox("m1", "m2");
  const { bindSession } = pushBinders(binding({ runtime: "opencode", bus: () => bus, opencode, setPush: (p: Push) => { push = p; } }));
  try {
    bindSession("thread-1");
    await push!.done;
    expect(push!.ended()).toBe("delivery");
    broken = false;
    await restartAfterFailure(() => push, () => bindSession("thread-1"));
    for (let i = 0; i < 100 && delivered.length < 2; i++) await Bun.sleep(20);
    expect(delivered).toEqual(["m1", "m2"]);
  } finally { push?.stop(); await push?.done; }
});

// The supervisor's restart, as the launcher makes it, with the launcher's classifier: the first push fails a
// delivery that may clear, the restart delivers that message before the next one.
test("a restart delivers the message the failed push consumed before the next one", async () => {
  const held: Held = { transient: transientDelivery, notice: () => {} };
  const queue = [{ message_id: "m1", from: "#a@x", body: "one" }, { message_id: "m2", from: "#a@x", body: "two" }];
  const bus = { name: "#s@x", consume: async () => { const next = queue.shift(); if (next) return next; await Bun.sleep(20); return null; } } as any;
  let broken = true;
  const delivered: string[] = [];
  const deliver = async (e: any) => { if (broken && e.message_id === "m1") throw new Error("codex: turn/start timed out after 30000ms"); delivered.push(e.message_id); };
  let current: Push | undefined;
  const start = () => { current = startPush(bus, deliver, () => {}, undefined, 0, process.env, undefined, undefined, held); };
  start();
  let t = 0;
  const check = superviseDelivery(() => current, () => { broken = false; start(); }, { note: () => {}, warn: () => {} }, { delays: [100, 100, 100], now: () => t });
  await current!.done;
  expect(current!.ended()).toBe("delivery");
  t = 1_000; await check(); // the wait starts when the stop is seen
  t = 2_000; await check(); // and the restart follows it
  for (let i = 0; i < 100 && delivered.length < 2; i++) await Bun.sleep(20);
  expect(delivered).toEqual(["m1", "m2"]);
  current!.stop(); await current!.done;
});

// The supervisor gives its own retry budget back after a push has run for settledMs; a delivery that outlasts that
// (Codex's five-minute wait) crosses it on every try. The cap is therefore the held message's own count, not the
// supervisor's: with a clock that crosses the reset on every check, the message is tried three times, dropped, and
// the next one flows.
test("a message whose every try outlasts the supervisor's settle time is still tried three times, then dropped", async () => {
  const notices: string[] = [];
  const held: Held = { transient: transientDelivery, notice: (s) => notices.push(s) };
  const queue = [{ message_id: "m1", from: "#a@x", body: "SECRET" }, { message_id: "m2", from: "#a@x", body: "two" }];
  const bus = { name: "#s@x", consume: async () => { const next = queue.shift(); if (next) return next; await Bun.sleep(20); return null; } } as any;
  let tries = 0;
  const delivered: string[] = [];
  const deliver = async (e: any) => {
    if (e.message_id === "m1") { tries++; await Bun.sleep(25); throw new Error("codex: turn/start timed out after 30000ms"); }
    delivered.push(e.message_id);
  };
  let current: Push | undefined;
  let starts = 0;
  const start = () => { starts++; current = startPush(bus, deliver, () => {}, undefined, 0, process.env, undefined, undefined, held); };
  start();
  let t = 0;
  const check = superviseDelivery(() => current, start, { note: () => {}, warn: () => {} }, { delays: [10, 10, 10], settledMs: 60_000, now: () => t });
  for (let i = 0; i < 300 && !delivered.includes("m2"); i++) { t += 61_000; await check(); await Bun.sleep(10); }
  expect(tries).toBe(3);
  expect(starts).toBe(3);
  expect(delivered).toEqual(["m2"]);
  expect(held.kept?.get("#s@x")).toBeUndefined();
  expect(notices).toHaveLength(1);
  expect(notices[0]).not.toContain("SECRET");
  current!.stop(); await current!.done;
});
