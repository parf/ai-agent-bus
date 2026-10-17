import { expect, test } from "bun:test";

// The forwarder's own bounds (S.5): admitting an event never waits on the bus,
// so the socket's ack is never held by a slow send, and a queue that fills
// drops its oldest and counts it.
process.env.SLACK_AGENT_QUEUE = "3";
process.env.SLACK_AGENT_FLUSH_MS = "3600000";
const { Forwarder } = await import("./forward.ts");
// Read once at import; gone again before any other file's agent inherits them.
delete process.env.SLACK_AGENT_QUEUE;
delete process.env.SLACK_AGENT_FLUSH_MS;

test("a slow bus never slows admission, and a full queue drops its oldest and counts", async () => {
  const sent: string[] = [];
  let release!: () => void;
  const gate = new Promise<void>(r => (release = r));
  const f = new Forwarder({
    subs: async () => [{ id: "s1", channel: "C1", target: "#t", created_by: "o", created_at: "now" }],
    channelId: async c => c,
    channelName: async () => "#dev",
    send: async (_to, body) => { await gate; sent.push(JSON.parse(body).text); },
    identity: async () => ({}),
    persistCount: async () => {},
    persistError: async () => {},
    exists: async () => true,
    disable: async () => {},
    forget: async () => {},
    serialize: (_id, fn) => fn(),
    log: () => {},
    scrub: s => s,
  });
  const started = performance.now();
  for (let i = 0; i < 10; i++) f.admit({ type: "message", channel: "C1", user: "U1", text: `m${i}`, ts: `${i}` }, `E${i}`);
  expect(performance.now() - started).toBeLessThan(50); // ten admissions while the bus is stuck
  await Bun.sleep(20);
  release();
  await f.stop(2000);
  // One was in flight when the rest arrived; the queue of three kept the newest.
  expect(sent).toEqual(["m0", "m7", "m8", "m9"]);
  expect(f.dropped).toBe(6);
});

const quiet = {
  channelId: async (c: string) => c, channelName: async () => "", identity: async () => ({}),
  exists: async () => true, disable: async () => {}, forget: async () => {}, serialize: <T,>(_id: string, fn: () => Promise<T>) => fn(), log: () => {}, scrub: (s: string) => s,
};

test("a count that landed is not added again when the error after it failed (slack-review-S.5)", async () => {
  const counts: number[] = [];
  let failError = true;
  const f = new Forwarder({
    ...quiet,
    subs: async () => [{ id: "s1", channel: "C1", target: "#t", created_by: "o", created_at: "now" }],
    send: async () => { throw new Error("refused"); },
    persistCount: async (_id, n) => { counts.push(n); },
    persistError: async () => { if (failError) throw new Error("kv away"); },
  });
  // One refused send records an error; a second, successful path is not needed:
  // the count stays 0, and the error write fails on the first flush.
  f.admit({ type: "message", channel: "C1", user: "U1", text: "x", ts: "1" }, "E1");
  await Bun.sleep(20);
  await f.flush();
  failError = false;
  await f.flush();
  expect(counts).toEqual([]); // nothing was forwarded, nothing counted

  const counted: number[] = [];
  let errorFails = true;
  const g = new Forwarder({
    ...quiet,
    subs: async () => [
      { id: "ok", channel: "C1", target: "#t", created_by: "o", created_at: "now" },
      { id: "ok", channel: "C1", target: "#bad", created_by: "o", created_at: "now" },
    ],
    send: async to => { if (to === "#bad") throw new Error("refused"); },
    persistCount: async (_id, n) => { counted.push(n); },
    persistError: async () => { if (errorFails) throw new Error("kv away"); },
  });
  g.admit({ type: "message", channel: "C1", user: "U1", text: "x", ts: "1" }, "E1");
  await Bun.sleep(20);
  await g.flush(); // the count lands, the error write fails
  errorFails = false;
  await g.flush(); // the error lands now; the count is not written twice
  expect(counted).toEqual([1]);
});

test("stopping is bounded even when the final flush hangs (slack-review-S.5)", async () => {
  const f = new Forwarder({
    ...quiet,
    subs: async () => [{ id: "s1", channel: "C1", target: "#t", created_by: "o", created_at: "now" }],
    send: async () => {},
    persistCount: () => new Promise(() => {}), // a daemon that never answers
    persistError: async () => {},
  });
  f.admit({ type: "message", channel: "C1", user: "U1", text: "x", ts: "1" }, "E1");
  await Bun.sleep(20);
  // Bounded here: a stop that never returns fails this assertion, not the runner's timeout.
  const outcome = await Promise.race([f.stop(100).then(() => "stopped"), Bun.sleep(1000).then(() => "hung")]);
  expect(outcome).toBe("stopped");
});

test("a count written for a subscription deleted meanwhile is taken back (slack-review-S.5)", async () => {
  let present = true;
  const forgotten: string[] = [];
  const f = new Forwarder({
    ...quiet,
    subs: async () => [{ id: "s1", channel: "C1", target: "#t", created_by: "o", created_at: "now" }],
    send: async () => {},
    exists: async () => present,
    persistCount: async () => { present = false; }, // deleted while the count was written
    persistError: async () => {},
    forget: async id => { forgotten.push(id); },
  });
  f.admit({ type: "message", channel: "C1", user: "U1", text: "x", ts: "1" }, "E1");
  await Bun.sleep(20);
  await f.flush();
  expect(forgotten).toEqual(["s1"]);
});

test("while its own identity is unknown, nothing is forwarded (slack-review-S.5)", async () => {
  const sent: string[] = [];
  const f = new Forwarder({
    ...quiet,
    identity: async () => undefined,
    subs: async () => [{ id: "s1", channel: "C1", target: "#t", created_by: "o", created_at: "now" }],
    send: async (_to, body) => { sent.push(body); },
    persistCount: async () => {},
    persistError: async () => {},
  });
  f.admit({ type: "message", channel: "C1", user: "UBOT", text: "maybe mine", ts: "1" }, "E1");
  await Bun.sleep(30);
  expect(sent).toEqual([]);
});

test("a last error written for a subscription deleted meanwhile is taken back too (slack-review-S.5)", async () => {
  let present = true;
  const forgotten: string[] = [];
  const f = new Forwarder({
    ...quiet,
    subs: async () => [{ id: "s1", channel: "C1", target: "#t", created_by: "o", created_at: "now" }],
    send: async () => { throw new Error("refused"); }, // count stays 0; only the error is written
    exists: async () => present,
    persistCount: async () => {},
    persistError: async () => { present = false; }, // deleted while the error was written
    forget: async id => { forgotten.push(id); },
  });
  f.admit({ type: "message", channel: "C1", user: "U1", text: "x", ts: "1" }, "E1");
  await Bun.sleep(20);
  await f.flush();
  expect(forgotten).toEqual(["s1"]);
});
