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

for (const status of [401, 403]) {
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
