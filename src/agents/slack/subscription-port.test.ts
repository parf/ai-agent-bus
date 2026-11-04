import { afterAll, beforeAll, expect, test } from "bun:test";
import { Bus } from "../../clients/typescript/src/index.js";
import { startFixture, identity, type Fixture } from "./daemon.ts";
import { ChatPort, ChatSubscriptions } from "../shared/subscription-port.ts";
let fx: Fixture;
beforeAll(async () => { fx = await startFixture(); }, 30000);
afterAll(async () => { await fx?.stop(); }, 15000);

async function setup(label: string) {
  const source = `#${label}-${process.pid}@fixture`, target = `#${label}-target-${process.pid}@fixture`;
  const token = await identity(fx, source);
  await fx.call(fx.ownerToken, "POST", "/register", { name: target, kind: "agent", allow: ["*"], bound: 1 });
  const targetToken = (await fx.call(fx.ownerToken, "POST", "/token", { name: target })).token;
  const bus = Bus.connect({ addr: fx.socket, token, expectName: source }), receiver = Bus.connect({ addr: fx.socket, token: targetToken });
  const warnings: string[] = [], caller = { from: fx.owner, roles: ["owner"] };
  const make = () => new ChatSubscriptions(new ChatPort(bus, source), { resolve: async c => c, warning: (code, id) => warnings.push(`${id}:${code}`) });
  return { source, target, token, bus, receiver, warnings, caller, make };
}

test("chat Port sends without wait through the real daemon: accepted, full, then exactly one next copy", async () => {
  const f = await setup("port"), s = f.make();
  const sends: unknown[] = [], send = f.bus.send.bind(f.bus);
  f.bus.send = async (target, body, options) => {
    sends.push(options);
    return send(target, body, options);
  };
  try {
    const row = await s.add(f.caller, "C1", f.target);
    expect(await s.deliver("C1", "U1", "first", "first", "chat")).toEqual({ matched: 1, sent: 1, dropped: 0 });
    expect(await s.deliver("C1", "U1", "full", "full", "chat")).toEqual({ matched: 1, sent: 0, dropped: 1 });
    expect(f.warnings).toEqual([`${row.id}:send_refused`]);
    expect((await f.receiver.consume({ wait: 100 }))?.body).toBe("first");
    expect(await s.deliver("C1", "U1", "next", "next", "chat")).toEqual({ matched: 1, sent: 1, dropped: 0 });
    const copies = [(await f.receiver.consume({ wait: 100 }))?.body];
    const extra = await f.receiver.consume({ wait: 100 }); if (extra) copies.push(extra.body);
    expect(copies).toEqual(["next"]);
    // Pin the Port API boundary too: the SDK may normalize wait=0 away.
    expect(sends).toHaveLength(3);
    for (const options of sends) expect(Object.hasOwn(options as object, "wait")).toBe(false);
    expect(await s.list(f.caller)).toEqual([expect.objectContaining({ id: row.id, sent: 2, dropped: 1 })]);
  } finally { s.close(); await f.bus.close(); await f.receiver.close(); }
}, 10000);

test("the real daemon's revoked author grant drops a copy and never sends as the helper instead", async () => {
  const f = await setup("revoked"), s = f.make(), creator = `#author-${process.pid}@fixture`;
  await identity(fx, creator);
  try {
    const row = await s.add({ from: creator, roles: [] }, "C1", f.target);
    await fx.call(fx.ownerToken, "POST", "/manage", { name: f.target, allow: [f.source] });
    expect(await s.deliver("C1", "U1", "revoked", "revoked", "chat")).toEqual({ matched: 1, sent: 0, dropped: 1 });
    expect(await f.receiver.consume({ wait: 100 })).toBeNull();
    expect(f.warnings).toEqual([`${row.id}:admission_refused`]);
  } finally { s.close(); await f.bus.close(); await f.receiver.close(); }
}, 10000);

// Slack and Telegram both construct this shared ChatSubscriptions/ChatPort.
test("chat authority refusal or unavailability stores nothing at add and never sends a blocked copy", async () => {
  const f = await setup("authority"), s = f.make(), author = `#port-author-${process.pid}@fixture`;
  await identity(fx, author);
  const caller = { from: author, roles: [] };
  const maySend = f.bus.maySend.bind(f.bus);
  try {
    await fx.call(fx.ownerToken, "POST", "/manage", { name: f.target, allow: [f.source] });
    await expect(s.add(caller, "C1", f.target)).rejects.toMatchObject({ code: "admission_refused" });
    expect(await s.list(caller)).toEqual([]);
    await fx.call(fx.ownerToken, "POST", "/manage", { name: f.target, allow: [f.source, author] });
    f.bus.maySend = async () => { throw Error("fixture authority unavailable"); };
    await expect(s.add(caller, "C1", f.target)).rejects.toMatchObject({ code: "admission_unavailable" });
    expect(await s.list(caller)).toEqual([]);
    f.bus.maySend = maySend;
    const row = await s.add(caller, "C1", f.target);
    f.bus.maySend = async () => { throw Error("fixture authority unavailable"); };
    expect(await s.deliver("C1", "U1", "blocked", "blocked", "chat")).toEqual({ matched: 1, sent: 0, dropped: 1 });
    expect(await f.receiver.consume({ wait: 100 })).toBeNull();
    expect(f.warnings).toEqual([`${row.id}:admission_unavailable`]);
    f.bus.maySend = maySend;
    expect(await s.deliver("C1", "U1", "recovered", "recovered", "chat")).toEqual({ matched: 1, sent: 1, dropped: 0 });
    expect((await f.receiver.consume({ wait: 100 }))?.body).toBe("recovered");
    expect(await f.receiver.consume({ wait: 100 })).toBeNull();
  } finally { f.bus.maySend = maySend; s.close(); await f.bus.close(); await f.receiver.close(); }
}, 10000);
