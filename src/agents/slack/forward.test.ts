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
    userName: async () => "",
    send: async (_to, body) => { await gate; sent.push(JSON.parse(body).text); },
    identity: async () => ({}), posted: async () => false,
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
  channelId: async (c: string) => c, channelName: async () => "", userName: async () => "", identity: async () => ({}), posted: async () => false,
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
    posted: async () => false,
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

test("one subscription whose channel lookup fails is skipped; the others still forward", async () => {
  const sent: string[] = [];
  const f = new Forwarder({
    ...quiet,
    subs: async () => [
      { id: "bad", channel: "#renamed", target: "#a", created_by: "o", created_at: "now" },
      { id: "good", channel: "C1", target: "#b", created_by: "o", created_at: "now" },
    ],
    channelId: async c => { if (c === "#renamed") throw new Error("ratelimited"); return c; },
    send: async to => { sent.push(to); },
    persistCount: async () => {},
    persistError: async () => {},
  });
  f.admit({ type: "message", channel: "C1", user: "U1", text: "hi", ts: "1" }, "E1");
  await f.stop(2000);
  expect(sent).toEqual(["#b"]);
});

test("S.10 routing failures never gate independent subscriptions", async () => {
  const sent: string[] = [];
  const f = new Forwarder({ ...quiet,
    route: async () => { throw Error("routing unavailable"); },
    subs: async () => [{id:"s",channel:"C1",target:"#t",created_by:"o",created_at:"now"}],
    send: async (_to, body) => { sent.push(JSON.parse(body).text); }, persistCount: async () => {}, persistError: async () => {},
  });
  f.admit({type:"message",channel:"C1",user:"U9",text:"subscription",ts:"1"},"event");
  await f.stop(1000);
  expect(sent).toEqual(["subscription"]);
});

test("S.10 pre-send routing failure retries once; permanent and poison failures are bounded", async () => {
  for(const failure of [1,Infinity,403]) {
    let attempts=0,delivered=0;const logs:string[]=[];
    const f=new Forwarder({...quiet,subs:async()=>[],persistCount:async()=>{},persistError:async()=>{},send:async()=>{},
      route:async()=>{attempts++;if(attempts>=4)throw Object.assign(Error("fixture ceiling"),{status:403});if(failure===403)throw Object.assign(Error("no"),{status:403});if(attempts<=failure)throw Error("before send");delivered++;},log:line=>logs.push(line)});
    f.admit({type:"message",channel:"C1",user:"U1",text:"question",ts:"1.000001"},"event");await f.stop(2000);
    expect(attempts).toBe(failure===1?2:failure===403?1:3);expect(delivered).toBe(failure===1?1:0);
    if(failure!==1)expect(logs.join()).toContain("message C1:1.000001");
  }
});
