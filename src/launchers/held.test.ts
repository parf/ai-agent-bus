import { expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { startPush, superviseDelivery, type Held, type Push } from "../mcp/push.ts";
import { transientDelivery } from "../mcp/codex.ts";

// J60b: the standard ab-codex and opencode push is started by launcher.ts, not by the MCP face, and the supervisor
// restarts it there. A message the failed push consumed is delivered first by the next one only if every start
// shares the launcher's one holder; launcher.ts is a script, so its call sites are read, not imported.
const source = readFileSync(new URL("./launcher.ts", import.meta.url), "utf8");
const body = (name: string) => {
  const from = source.indexOf(`const ${name} = `);
  expect(from).toBeGreaterThan(0);
  return source.slice(from, source.indexOf("\n  };", from));
};

test("the launcher starts every push through one helper that passes its one holder", () => {
  expect(source.match(/\bstartPush\(/g)).toHaveLength(1); // the helper's own call
  const helper = source.slice(source.indexOf("const pushInto"), source.indexOf("const bindThread"));
  expect(helper).toMatch(/startPush\(bus, deliver, log, undefined, readerReleaseMs, env, undefined, undefined, held\)/);
  expect(source).toMatch(/const held: Held = \{ transient: transientDelivery, notice: \(s\) => warn\(s\) \};/);
  expect(source.match(/const held\b/g)).toHaveLength(1); // one holder, never one per start
  for (const bind of ["bindThread", "bindSession"]) {
    expect(body(bind)).toContain("push = pushInto(");
    expect(body(bind)).not.toContain("startPush(");
  }
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
  expect(held.message).toBeUndefined();
  expect(notices).toHaveLength(1);
  expect(notices[0]).not.toContain("SECRET");
  current!.stop(); await current!.done;
});
