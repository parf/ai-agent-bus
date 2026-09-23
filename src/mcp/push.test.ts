import { expect, test } from "bun:test";
import { BusError } from "./bus.ts";
import { startPush } from "./push.ts";

// A push loop that keeps asking with a credential the daemon refuses costs one
// refusal per backoff for as long as the session runs, and delivers nothing.
// Success here is the loop *ending*, so every wait is bounded: a loop that did
// not stop must fail an assertion rather than hang the run.
const settled = async (done: Promise<void>, ms: number) =>
  await Promise.race([done.then(() => "stopped"), Bun.sleep(ms).then(() => "still running")]);

const refusing = (name: string, status: number, count: { calls: number }) => ({
  name,
  consume: async () => { count.calls++; throw new BusError(status, "refused"); },
}) as any;

for (const status of [401]) {
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
      if (suspended) { suspended = false; throw new BusError(403, "user access is suspended"); }
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
  const push = startPush(refusing("#paused@srv1", 403, count), async () => {}, () => {}, 60_000);
  await Bun.sleep(20);
  push.stop();
  expect(await settled(push.done, 200)).toBe("stopped");
  expect(count.calls).toBe(1);
});
