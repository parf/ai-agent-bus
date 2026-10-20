import { expect, test } from "bun:test";
import { BusError } from "./bus.ts";
import { startPush } from "./push.ts";

// A push loop that keeps asking with a credential the daemon refuses costs one
// refusal per backoff for as long as the session runs, and delivers nothing.
// Success here is the loop *ending*, so every wait is bounded: a loop that did
// not stop must fail an assertion rather than hang the run.
const settled = async (done: Promise<void>, ms: number) =>
  await Promise.race([done.then(() => "stopped"), Bun.sleep(ms).then(() => "still running")]);

const refusing = (name: string, status: number, count: { calls: number }, message = "refused") => ({
  name,
  consume: async () => { count.calls++; throw new BusError(status, message); },
}) as any;
const SUSPENDED = '{"error":"user access is suspended"}';

for (const status of [401, 403]) { // a 403 that is not a suspension stays permanent
  test(`a ${status} refusal stops push after one attempt`, async () => {
    const count = { calls: 0 };
    const said: string[] = [];
    const push = startPush(refusing("#gone@srv1", status, count), async () => {}, (s) => said.push(s));
    expect(await settled(push.done, 500)).toBe("stopped");
    expect(count.calls).toBe(1);
    expect(push.running()).toBe(false);
    expect(said.join("\n")).toContain(`#gone@srv1 cannot read its inbox (${status}`);
  });
}

test("a transient failure is still retried", async () => {
  const count = { calls: 0 };
  const push = startPush(refusing("#busy@srv1", 503, count), async () => {}, () => {});
  expect(await settled(push.done, 300)).toBe("still running");
  expect(push.running()).toBe(true);
  expect(count.calls).toBe(1); // in its backoff, not stopped
  push.stop();
  await push.done;
});

// Suspension is reversible, so push waits and asks again instead of going off
// for the session's life; a stop during that long wait ends it at once.
test("a 403 suspension is retried after the long wait, and push resumes", async () => {
  const count = { calls: 0 };
  const said: string[] = [];
  let suspended = true;
  const delivered: string[] = [];
  const bus = {
    name: "#paused@srv1",
    consume: async () => {
      count.calls++;
      if (suspended) { suspended = false; throw new BusError(403, SUSPENDED); }
      if (count.calls === 2) return { message_id: "m1", from: "#a@srv1" };
      await Bun.sleep(50);
      return null;
    },
  } as any;
  const push = startPush(bus, async (e) => { delivered.push(e.message_id); }, (s) => said.push(s), 100);
  expect(await settled(push.done, 60)).toBe("still running");
  expect(count.calls).toBe(1); // waiting, not stopped
  expect(said.join("\n")).toContain("#paused@srv1 is suspended");
  await Bun.sleep(150);
  expect(delivered).toEqual(["m1"]);
  expect(said.join("\n")).toContain("#paused@srv1 reads its inbox again");
  push.stop();
  expect(await settled(push.done, 200)).toBe("stopped");
});

test("a stop during the suspension wait ends push at once", async () => {
  const count = { calls: 0 };
  const push = startPush(refusing("#paused@srv1", 403, count, SUSPENDED), async () => {}, () => {}, 60_000);
  await Bun.sleep(20);
  expect(push.running()).toBe(true); // waiting on the suspension, not stopped by it
  push.stop();
  expect(await settled(push.done, 200)).toBe("stopped");
  expect(count.calls).toBe(1);
});

test("a stop during the transient backoff ends push at once", async () => {
  const count = { calls: 0 };
  const push = startPush(refusing("#busy@srv1", 503, count), async () => {}, () => {});
  await Bun.sleep(20);
  push.stop();
  expect(await settled(push.done, 200)).toBe("stopped");
});

test("an unrelated reader is still refused without a rename handoff", async () => {
  const count = { calls: 0 }, said: string[] = [];
  const push = startPush(refusing("#other@srv1", 409, count), async () => {}, s => said.push(s));
  expect(await settled(push.done, 200)).toBe("stopped");
  expect(count.calls).toBe(1);
  expect(said.join("\n")).toContain("#other@srv1 already has a reader");
});

test("a held rename reader is retried, then the message is delivered once", async () => {
  const said: string[] = [], delivered: string[] = [];
  let calls = 0;
  const bus = {
    name: "#renamed@srv1",
    consume: async () => {
      if (++calls < 3) throw new BusError(409, "old read still held");
      return { message_id: "after-rename", from: "#a@srv1" };
    },
  } as any;
  const push = startPush(bus, async e => { delivered.push(e.message_id); push.stop(); }, s => said.push(s), undefined, 1_000);
  expect(await settled(push.done, 2_000)).toBe("stopped");
  expect(calls).toBe(3);
  expect(delivered).toEqual(["after-rename"]);
  expect(said.join("\n")).toContain("waiting for the previous reader after rename");
  expect(said.join("\n")).toContain("#renamed@srv1 reads its renamed inbox");
});

test("a rename handoff with a permanent competing reader stops within its bound", async () => {
  const count = { calls: 0 }, said: string[] = [];
  const push = startPush(refusing("#occupied@srv1", 409, count), async () => {}, s => said.push(s), undefined, 120);
  expect(await settled(push.done, 1_000)).toBe("stopped");
  expect(count.calls).toBeGreaterThan(1);
  expect(push.running()).toBe(false);
  expect(said.join("\n")).toContain("#occupied@srv1 still has a reader after 120ms following rename; push inactive");
});

test("stopping a rename handoff cancels its retry backoff", async () => {
  const count = { calls: 0 };
  const push = startPush(refusing("#stopping@srv1", 409, count), async () => {}, () => {}, undefined, 10_000);
  await Bun.sleep(20);
  expect(push.running()).toBe(true);
  push.stop();
  expect(await settled(push.done, 200)).toBe("stopped");
});
