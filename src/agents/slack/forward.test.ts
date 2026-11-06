import { expect, test } from "bun:test";
process.env.SLACK_AGENT_QUEUE = "3";
// A distinct module instance pins this fixture even if J22 imported the default first.
const { Forwarder, identityGiveMs, identityRetry } = await import("./forward.ts?queue-fixture");
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
  const unknown = fixture({ identity: async () => undefined, identityRetryMs: [1, 1] }); unknown.f.admit(event(), "unknown"); await unknown.f.stop(1000); expect(unknown.sent).toEqual([]);
  const own = fixture({ identity: async () => ({ botUser: "UBOT" }), posted: async (_c: string, ts: string) => ts === "mine" });
  own.f.admit(event("mine", { user: "UBOT" }), "own"); own.f.admit(event("shared", { user: "UBOT" }), "shared");
  await own.f.stop(1000); expect(own.sent).toEqual(["shared"]);
});

test("an identity that Slack has not given yet is asked for again, and the event waits instead of being lost", async () => {
  let asked = 0;
  const x = fixture({ identity: async () => (++asked < 3 ? undefined : {}), identityRetryMs: [1, 1, 1] });
  x.f.admit(event("late"), "late-1"); x.f.admit(event("later"), "late-2");
  await x.f.stop(2000);
  expect(x.sent).toEqual(["late", "later"]); expect(asked).toBe(4); // 3 for the first event, 1 for the second
  expect(x.logs.filter(l => l.includes("warning"))).toEqual([]);
});

test("a definitive refusal of the identity is not waited out, a try-again answer is, and each is counted", async () => {
  for (const [retryable, calls] of [[false, 1], [true, 4], [undefined, 4]] as const) {
    let asked = 0;
    const x = fixture({ identity: async () => { asked++; return undefined; }, identityRetryMs: [1, 1, 1], ...(retryable === undefined ? {} : { identityRetryable: () => retryable }) });
    x.f.admit(event("refused"), "r-" + retryable);
    await x.f.stop(2000);
    expect(asked).toBe(calls);
    expect(x.sent).toEqual([]);
    expect(x.logs.filter(l => l.includes("warning")).length).toBe(1);
  }
});

test("a bus or secret hiccup is asked again per event (4 calls) but does not hold the start; a refusal is neither", async () => {
  expect(identityRetry("slack-transient")).toEqual({ atStart: true, perEvent: true });
  expect(identityRetry("bus")).toEqual({ atStart: false, perEvent: true });
  expect(identityRetry("refused")).toEqual({ atStart: false, perEvent: false });
  for (const cause of ["slack-transient", "bus", "refused"] as const) {
    let asked = 0;
    const x = fixture({ identity: async () => { asked++; return undefined; }, identityRetryMs: [1, 1, 1], identityRetryable: () => identityRetry(cause).perEvent });
    x.f.admit(event("hiccup"), "h-" + cause);
    await x.f.stop(2000);
    expect(asked).toBe(cause === "refused" ? 1 : 4);
  }
});

test("an event lost for want of an identity is logged at warning with its channel and ts, never its text", async () => {
  const x = fixture({ identity: async () => undefined, identityRetryMs: [1, 1] });
  x.f.admit(event("secret body text", { ts: "1700.5" }), "gone");
  await x.f.stop(2000);
  expect(x.sent).toEqual([]);
  const lines = x.logs.filter(l => l.includes("warning"));
  expect(lines.length).toBe(1); expect(lines[0]).toContain("C1"); expect(lines[0]).toContain("1700.5"); expect(lines[0]).not.toContain("secret body text");
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

test("the start-up identity wait takes only a positive whole number; anything else is an error", () => {
  expect(identityGiveMs(undefined)).toEqual({ ms: 20_000 });
  expect(identityGiveMs("1500")).toEqual({ ms: 1500 });
  for (const bad of ["abc", "", "-5", "0", "1.5", "1e3", " 7", "99999999999"]) {
    expect(identityGiveMs(bad)).toEqual({ error: "SLACK_AGENT_IDENTITY_GIVE_MS must be a positive whole number of milliseconds" });
  }
});
