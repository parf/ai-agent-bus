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
    identity: () => ({}),
    persist: async () => {},
    exists: async () => true,
    disable: async () => {},
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
