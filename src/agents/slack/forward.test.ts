import { expect, test } from "bun:test";
process.env.SLACK_AGENT_QUEUE = "3";
const { Forwarder } = await import("./forward.ts");
delete process.env.SLACK_AGENT_QUEUE;
const event = (text = "x", extra = {}) => ({ type: "message", channel: "C1", user: "U1", text, ts: text, ...extra });
function fixture(over: any = {}) {
  const sent: string[] = [], logs: string[] = [];
  let closed = 0;
  const subscriptions = { deliver: async (_channel: string, _author: string, _text: string, body: string) => { sent.push(JSON.parse(body).text); return { matched: 1, sent: 1, dropped: 0 }; }, close: () => { closed++; } };
  const f = new Forwarder({ subscriptions, userName: async () => "Ada", channelName: async () => "#dev", identity: async () => ({}), posted: async () => false, log: s => logs.push(s), scrub: s => s, ...over });
  return { f, sent, logs, closed: () => closed };
}

test("slow forwarding never delays admission; a full queue keeps newest events and counts losses", async () => {
  const sent: string[] = []; let release!: () => void;
  const gate = new Promise<void>(r => { release = r; });
  const { f } = fixture({ subscriptions: { deliver: async (_c: string, _a: string, text: string) => { await gate; sent.push(text); }, close: () => {} } });
  const started = performance.now();
  for (let i = 0; i < 10; i++) f.admit(event(`m${i}`), `E${i}`);
  expect(performance.now() - started).toBeLessThan(50);
  release(); await f.stop(2000);
  expect(sent).toEqual(["m0", "m7", "m8", "m9"]); expect(f.dropped).toBe(6);
});

test("duplicate provider event ids do not make duplicate copies", async () => {
  const x = fixture(); x.f.admit(event("one"), "same"); x.f.admit(event("one"), "same"); x.f.admit(event("two"), "different");
  await x.f.stop(2000); expect(x.sent).toEqual(["one", "two"]);
});

test("unknown identity and own posted messages forward nothing; shared-token messages still forward", async () => {
  const unknown = fixture({ identity: async () => undefined }); unknown.f.admit(event(), "unknown"); await unknown.f.stop(1000); expect(unknown.sent).toEqual([]);
  const own = fixture({ identity: async () => ({ botUser: "UBOT" }), posted: async (_c: string, ts: string) => ts === "mine" });
  own.f.admit(event("mine", { user: "UBOT" }), "own"); own.f.admit(event("shared", { user: "UBOT" }), "shared");
  await own.f.stop(1000); expect(own.sent).toEqual(["shared"]);
});

test("edits and deletes are not forwarded; thread broadcasts and bot posts are", async () => {
  const x = fixture();
  for (const subtype of ["message_changed", "message_deleted", "thread_broadcast", "bot_message"]) x.f.admit(event(subtype, { subtype }), subtype);
  await x.f.stop(1000); expect(x.sent).toEqual(["thread_broadcast", "bot_message"]);
});

test("routing drop stops subscriptions, while a routing failure does not gate them", async () => {
  const drop = fixture({ route: async () => false }); drop.f.admit(event(), "drop"); await drop.f.stop(1000); expect(drop.sent).toEqual([]);
  let attempts = 0;
  const fail = fixture({ route: async () => { attempts++; throw Object.assign(Error("no"), { status: 403 }); } });
  fail.f.admit(event(), "failure"); await fail.f.stop(1000); expect(fail.sent).toEqual(["x"]); expect(attempts).toBe(1);
});

test("pre-send routing failures retry boundedly without retrying subscription copies", async () => {
  let attempts = 0;
  const x = fixture({ route: async () => { attempts++; throw Error("unavailable"); } });
  x.f.admit(event(), "routing"); await x.f.stop(2000); expect(attempts).toBe(3); expect(x.sent).toEqual(["x"]);
});

test("stop is bounded, closes shared workers and refuses further intake", async () => {
  let closed = 0;
  const x = fixture({ subscriptions: { deliver: () => new Promise(() => {}), close: () => { closed++; } } });
  x.f.admit(event(), "one");
  const result = await Promise.race([x.f.stop(50).then(() => "stopped"), Bun.sleep(1000).then(() => "hung")]);
  expect(result).toBe("stopped"); expect(closed).toBe(1);
});
