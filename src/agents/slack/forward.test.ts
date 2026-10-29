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
    maySend: async () => true, send: async (_to, body) => { await gate; sent.push(JSON.parse(body).text); },
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
    maySend: async () => true, send: async () => { throw new Error("refused"); },
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
    maySend: async () => true, send: async to => { if (to === "#bad") throw new Error("refused"); },
    persistCount: async (_id, n) => { counted.push(n); },
    persistError: async () => { if (errorFails) throw new Error("kv away"); },
  });
  g.admit({ type: "message", channel: "C1", user: "U1", text: "x", ts: "1" }, "E1");
  const pendingBy = Date.now() + 1000;
  while (g.pending("ok").forwarded !== 1 && Date.now() < pendingBy) await Bun.sleep(1);
  expect(g.pending("ok").forwarded).toBe(1);
  expect(counted).toEqual([]); // no write per event, independent of the real timer phase
  await g.flush(); // the count lands, the error write fails
  errorFails = false;
  await g.flush(); // the error lands now; the count is not written twice
  expect(counted).toEqual([1]);
});

test("stopping is bounded even when the final flush hangs (slack-review-S.5)", async () => {
  const f = new Forwarder({
    ...quiet,
    subs: async () => [{ id: "s1", channel: "C1", target: "#t", created_by: "o", created_at: "now" }],
    maySend: async () => true, send: async () => {},
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
    maySend: async () => true, send: async () => {},
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
    maySend: async () => true, send: async (_to, body) => { sent.push(body); },
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
    maySend: async () => true, send: async () => { throw new Error("refused"); }, // count stays 0; only the error is written
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
    maySend: async () => true, send: async to => { sent.push(to); },
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
    maySend: async () => true, send: async (_to, body) => { sent.push(JSON.parse(body).text); }, persistCount: async () => {}, persistError: async () => {},
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

test("a drop handler stops Slack subscriptions too", async () => {
  let subscriptions = 0, sends = 0;
  const f = new Forwarder({...quiet, route: async () => false,
    subs: async () => { subscriptions++; return []; }, send: async () => { sends++; },
    persistCount: async () => {}, persistError: async () => {}});
  f.admit({type: "message", channel: "C1", user: "U1", text: "discard", ts: "1"}, "drop");
  await f.stop(2000); expect(subscriptions).toBe(0); expect(sends).toBe(0);
});

// RD141: the author's right to send is asked before every forward, and a
// check that cannot be answered sends nothing.
test("the author's may-send is asked before every send; a no or a vanished target disables, an unanswered check only skips", async () => {
  const run = async (check: (who: string, to: string) => Promise<boolean>) => {
    const sent: string[] = [], asked: string[] = [], disabled: string[] = [];
    const f = new Forwarder({
      ...quiet,
      subs: async () => [{ id: "s1", channel: "C1", target: "#t", created_by: "author@t", created_at: "now" }],
      maySend: async (who, to) => { asked.push(`${who}>${to}`); return check(who, to); },
      send: async to => { sent.push(to); },
      disable: async (_id, why) => { disabled.push(why); },
      persistCount: async () => {}, persistError: async () => {},
    });
    f.admit({ type: "message", channel: "C1", user: "U1", text: "x", ts: "1" }, "E1");
    f.admit({ type: "message", channel: "C1", user: "U1", text: "y", ts: "2" }, "E2");
    await Bun.sleep(50);
    return { sent, asked, disabled };
  };
  const yes = await run(async () => true);
  expect(yes.sent).toEqual(["#t", "#t"]);
  expect(yes.asked).toEqual(["author@t>#t", "author@t>#t"]); // before each send, for the author
  const no = await run(async () => false);
  expect(no.sent).toEqual([]);
  expect(no.disabled.length).toBeGreaterThan(0);
  const gone = await run(async () => { throw Object.assign(new Error("no such name"), { status: 404 }); });
  expect(gone.sent).toEqual([]);
  expect(gone.disabled.length).toBeGreaterThan(0);
  const down = await run(async () => { throw new Error("daemon away"); });
  expect(down.sent).toEqual([]);
  expect(down.disabled).toEqual([]); // a check nobody could answer pauses nothing
});

test("an outage is asked once per event and logged once; a pair shared by subscriptions is asked once; an unknown author is disabled", async () => {
  const subs = [1, 2, 3, 4, 5].map(n => ({ id: `s${n}`, channel: "C1", target: n < 4 ? "#t" : `#u${n}`, created_by: "author@t", created_at: "now" }));
  subs.push({ id: "orphan", channel: "C1", target: "#t", created_by: "", created_at: "now" });
  const asked: string[] = [], logged: string[] = [], disabled: string[] = [];
  let down = true;
  const f = new Forwarder({
    ...quiet,
    subs: async () => subs,
    maySend: async (who, to) => { asked.push(`${who}>${to}`); if (down) throw new Error("daemon away"); return true; },
    send: async () => {},
    log: line => logged.push(line),
    disable: async (id, why) => { disabled.push(`${id}:${why}`); },
    persistCount: async () => {}, persistError: async () => {},
  });
  for (let i = 1; i <= 3; i++) f.admit({ type: "message", channel: "C1", user: "U1", text: "x", ts: String(i) }, `E${i}`);
  await Bun.sleep(80);
  expect(asked.length).toBe(3); // one failing call per event, however many subscriptions
  expect(logged.filter(l => l.includes("unavailable")).length).toBe(5); // once per subscription, not per event
  expect([...new Set(disabled)]).toEqual(["orphan:its author is unknown; disabled"]); // a real disable stops the later events
  down = false;
  asked.length = 0;
  f.admit({ type: "message", channel: "C1", user: "U1", text: "y", ts: "9" }, "E9");
  await Bun.sleep(40);
  expect(asked.sort()).toEqual(["author@t>#t", "author@t>#u4", "author@t>#u5"]); // the shared pair asked once
});
