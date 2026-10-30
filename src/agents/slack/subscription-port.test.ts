import { afterAll, beforeAll, expect, test } from "bun:test";
import { spawn } from "bun";
import { join } from "node:path";
import { Bus } from "../../clients/typescript/src/index.js";
import { startFixture, identity, type Fixture } from "./daemon.ts";
import { ChatPort, ChatSubscriptions } from "../shared/subscription-port.ts";
import { migrationId } from "../shared/subscription-migration.ts";
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

test("SIGKILL between new-row commit and old-key delete recovers one listed row and exactly one delivery", async () => {
  const f = await setup("migration"), kv = f.bus.record(f.source).kv;
  const oldId = "deadbeef", nextId = migrationId(`sub.${oldId}`);
  await kv.set("json", `sub.${oldId}`, { id: oldId, channel: "C1", target: f.target, created_by: fx.owner, created_at: "2026-10-30T00:00:00.000Z" }, { mode: "add" });
  await kv.set("int", `stat.${oldId}.forwarded`, 6, { mode: "set" });
  const code = `
    import { Bus } from ${JSON.stringify(new URL("../../clients/typescript/src/index.ts", import.meta.url).pathname)};
    import { ChatPort, ChatSubscriptions } from ${JSON.stringify(new URL("../shared/subscription-port.ts", import.meta.url).pathname)};
    const bus = Bus.connect({expectName: process.env.AGENT_BUS_NAME});
    const kv = bus.record(process.env.AGENT_BUS_NAME).kv, set = kv.set.bind(kv);
    kv.set = async (...args) => {
      const result = await set(...args);
      if(args[1] === ${JSON.stringify(`sub.${nextId}`)}) { console.log("new-row-committed"); await new Promise(() => {}); }
      return result;
    };
    const s = new ChatSubscriptions(new ChatPort(bus, process.env.AGENT_BUS_NAME), {resolve: async c => c, warning: () => {}});
    await s.ready();
  `;
  const childPath = join(fx.dir, "migration.local.ts"); await Bun.write(childPath, code);
  const child = spawn(["bun", childPath], { env: { ...process.env, AGENT_BUS_NAME: f.source, AGENT_BUS_ADDR: fx.socket, AGENT_BUS_TOKEN: f.token }, stdout: "pipe", stderr: "pipe" });
  const reader = child.stdout.getReader();
  const s = f.make();
  try {
    const chunk = await Promise.race([reader.read().then(r => new TextDecoder().decode(r.value)), Bun.sleep(3000).then(() => "deadline")]);
    expect(chunk).toContain("new-row-committed");
    child.kill("SIGKILL");
    expect(await Promise.race([child.exited.then(() => "stopped"), Bun.sleep(2000).then(() => "hung")])).toBe("stopped");
    expect((await kv.get("json", `sub.${oldId}`)).value).toBeDefined();
    expect((await kv.get("json", `sub.${nextId}`)).value).toBeDefined();
    const listed = await s.list(f.caller);
    expect(listed).toEqual([expect.objectContaining({ id: nextId, sent: 6 })]);
    await expect(kv.get("json", `sub.${oldId}`)).rejects.toThrow();
    expect(await s.list(f.caller)).toEqual(listed); // a second pass is unchanged
    await s.deliver("C1", "U1", "one", "one", "chat");
    const copies = [(await f.receiver.consume({ wait: 100 }))?.body];
    // deliver() waits for all sends; this short probe cannot race a later delivery.
    const extra = await f.receiver.consume({ wait: 100 }); if (extra) copies.push(extra.body);
    expect(copies).toEqual(["one"]);
  } finally { child.kill(); await child.exited; await reader.cancel(); s.close(); await f.bus.close(); await f.receiver.close(); }
}, 12000);
