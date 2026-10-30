import { expect, test } from "bun:test";
import { startPush, superviseDelivery, type Push } from "./push.ts";

// J60: a push that stopped because a delivery failed is started again, a few times.
const fakePush = (end: "delivery" | "refused" | "stopped" | undefined, running = false): Push => ({ running: () => running, ended: () => end, stop() {}, done: Promise.resolve() });

test("a push that stopped after a failed delivery is restarted three times, then said once", async () => {
  let current: Push = fakePush("delivery");
  const restarts: number[] = [];
  const said: { kind: string; s: string }[] = [];
  let t = 0;
  const check = superviseDelivery(() => current, () => { restarts.push(t); current = fakePush("delivery"); }, { note: (s) => said.push({ kind: "note", s }), warn: (s) => said.push({ kind: "warn", s }) }, { delays: [2_000, 10_000, 30_000], now: () => t });
  for (t = 0; t < 120_000; t += 2_000) await check();
  expect(restarts).toEqual([2_000, 12_000, 42_000]); // the 2 s, 10 s and 30 s waits, each from the restart before
  expect(said.filter((x) => x.kind === "note")).toHaveLength(3);
  const warns = said.filter((x) => x.kind === "warn");
  expect(warns).toHaveLength(1); // one final line, no 2-second loop
  expect(warns[0]!.s).toContain("restarted 3 times");
});

test("a refusal or the push's own stop is never restarted", async () => {
  for (const end of ["refused", "stopped", undefined] as const) {
    let restarts = 0;
    const check = superviseDelivery(() => fakePush(end), () => { restarts++; }, { note: () => {}, warn: () => {} }, { now: () => 0 });
    for (let i = 0; i < 5; i++) await check();
    expect(restarts).toBe(0);
  }
});

test("a restarted push that stays healthy gets its tries back", async () => {
  let current: Push = fakePush("delivery");
  let t = 0, restarts = 0;
  const check = superviseDelivery(() => current, () => { restarts++; current = fakePush(undefined, true); }, { note: () => {}, warn: () => {} }, { delays: [100, 100, 100], settledMs: 1_000, now: () => t });
  await check(); // restart 1
  t = 5_000; await check(); // healthy long enough: tries reset
  current = fakePush("delivery");
  t = 6_000; await check(); // restart again, as a new first try
  expect(restarts).toBe(2);
});

// The reason the real push reports is what the supervisor reads.
test("a push says why it ended: a failed delivery, a refusal, its own stop", async () => {
  const one = { message_id: "m1", from: "#a@x" };
  const failing = startPush({ name: "#s@x", consume: async () => one } as any, async () => { throw new Error("no thread"); }, () => {});
  await failing.done;
  expect([failing.running(), failing.ended()]).toEqual([false, "delivery"]);
  const refused = startPush({ name: "#s@x", consume: async () => { throw new (await import("./bus.ts")).BusError(409, "has a reader"); } } as any, async () => {}, () => {});
  await refused.done;
  expect(refused.ended()).toBe("refused");
  const own = startPush({ name: "#s@x", consume: async () => { await Bun.sleep(30); return null; } } as any, async () => {}, () => {});
  own.stop();
  await own.done;
  expect(own.ended()).toBe("stopped");
});

// Each wait is the delay after the stop is seen or after the restart before it: no early retry.
test("no retry comes before its delay", async () => {
  let current: Push = fakePush("delivery");
  const restartedAt: number[] = [];
  let t = 0;
  const check = superviseDelivery(() => current, () => { restartedAt.push(t); current = fakePush("delivery"); }, { note: () => {}, warn: () => {} }, { delays: [2_000, 10_000, 30_000], now: () => t });
  for (t = 0; t <= 60_000; t += 500) await check();
  expect(restartedAt).toHaveLength(3);
  expect(restartedAt[0]!).toBeGreaterThanOrEqual(2_000);
  expect(restartedAt[1]! - restartedAt[0]!).toBeGreaterThanOrEqual(10_000);
  expect(restartedAt[2]! - restartedAt[1]!).toBeGreaterThanOrEqual(30_000);
});
