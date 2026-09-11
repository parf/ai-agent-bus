// The smokes drive rpc.ts end to end, but only in the easy shape: one request
// in flight at a time, and every chunk a whole line. Mutation showed that —
// five of six mutants lived through all three suites. These are the cases that
// kill them, and they are exactly the cases the easy shape never reaches.

import { expect, test } from "bun:test";
import { Pending, drain, lines } from "./rpc.ts";

/** A stream that hands over exactly these chunks, boundaries and all. */
function chunks(...parts: string[]): ReadableStream<Uint8Array> {
  const enc = new TextEncoder();
  return new ReadableStream({
    start(c) {
      for (const p of parts) c.enqueue(enc.encode(p));
      c.close();
    },
  });
}

async function collect(s: ReadableStream<Uint8Array>): Promise<string[]> {
  const out: string[] = [];
  for await (const line of lines(s)) out.push(line);
  return out;
}

test("a line split across chunks arrives whole", async () => {
  expect(await collect(chunks('{"id":1,', '"result":"ok"}\n'))).toEqual(['{"id":1,"result":"ok"}']);
});

test("many lines in one chunk, and one line over many chunks", async () => {
  expect(await collect(chunks("a\nb\nc", "d\n", "e", "f", "g\n"))).toEqual(["a", "b", "cd", "efg"]);
});

test("a trailing line with no newline is not yielded", async () => {
  // The other end is mid-write, not done. Yielding it would hand a parser
  // half a JSON object.
  expect(await collect(chunks("whole\n", "half"))).toEqual(["whole"]);
});

test("blank lines are skipped", async () => {
  expect(await collect(chunks("a\n\n\nb\n"))).toEqual(["a", "b"]);
});

test("drain reports every piece in order and keeps none of it", async () => {
  const seen: string[] = [];
  // Returning the whole history is what made a long-lived app-server's
  // stderr grow without bound, so there is nothing to return.
  expect(await drain(chunks("one", "two"), (s) => seen.push(s))).toBeUndefined();
  expect(seen).toEqual(["one", "two"]);
});

test("concurrent requests get their own answers", async () => {
  const p = new Pending();
  const a = p.issue(5_000, () => new Error("a"));
  const b = p.issue(5_000, () => new Error("b"));
  const c = p.issue(5_000, () => new Error("c"));
  expect(new Set([a.id, b.id, c.id]).size).toBe(3);
  expect(p.size).toBe(3);

  // Answered out of order, which is the normal case: a slow call and a fast
  // one are in flight together and the fast one lands first.
  p.settle(c.id, null, "third");
  p.settle(a.id, null, "first");
  p.settle(b.id, null, "second");
  expect(await Promise.all([a.answer, b.answer, c.answer])).toEqual(["first", "second", "third"]);
  expect(p.size).toBe(0);
});

test("an answer to an id nobody waits on is dropped, not misdelivered", async () => {
  const p = new Pending();
  const one = p.issue(5_000, () => new Error("one"));
  expect(p.settle(one.id + 999, null, "not yours")).toBe(false);
  expect(p.settle("not-a-number", null, "nor this")).toBe(false);
  expect(p.size).toBe(1);
  p.settle(one.id, null, "mine");
  expect(await one.answer).toBe("mine");
});

test("a settled request is not settled twice", async () => {
  const p = new Pending();
  const one = p.issue(5_000, () => new Error("one"));
  expect(p.settle(one.id, null, "first")).toBe(true);
  expect(p.settle(one.id, null, "second")).toBe(false);
  expect(await one.answer).toBe("first");
});

test("nothing is kept after an answer", async () => {
  const p = new Pending();
  for (let i = 0; i < 100; i++) {
    const { id, answer } = p.issue(5_000, () => new Error("x"));
    p.settle(id, null, i);
    await answer;
  }
  // The owner's rule: do not pollute memory with data nobody will read again.
  expect(p.size).toBe(0);
});

test("a reserved id collides with nothing in flight", async () => {
  const p = new Pending();
  const live = p.issue(5_000, () => new Error("live"));
  const spare = p.reserve();
  const later = p.issue(5_000, () => new Error("later"));
  expect(new Set([live.id, spare, later.id]).size).toBe(3);
  // Reserved means nobody is waiting: an answer to it changes nothing.
  expect(p.settle(spare, null, "ignored")).toBe(false);
  p.settle(live.id, null, 1);
  p.settle(later.id, null, 2);
  expect(await Promise.all([live.answer, later.answer])).toEqual([1, 2]);
});

test("a timeout rejects with the caller's own error and forgets the entry", async () => {
  const p = new Pending();
  const { answer } = p.issue(10, () => new Error("timeout on tools/call"));
  await expect(answer).rejects.toThrow("timeout on tools/call");
  expect(p.size).toBe(0);
});

test("failAll rejects everyone in flight and leaves nothing behind", async () => {
  const p = new Pending();
  const waiting = [1, 2, 3].map(() => p.issue(5_000, () => new Error("never")).answer);
  p.failAll("server exited");
  for (const a of waiting) await expect(a).rejects.toThrow("server exited");
  expect(p.size).toBe(0);
  // A timer still armed here would hold the process open for five seconds.
});

test("request writes, and a write that fails rejects the wait", async () => {
  const p = new Pending();
  const written: number[] = [];
  const ok = p.request((id) => { written.push(id); }, 5_000, () => new Error("never"));
  expect(written.length).toBe(1);
  p.settle(written[0]!, null, "answered");
  expect(await ok).toBe("answered");

  const broken = p.request(() => Promise.reject(new Error("pipe closed")), 5_000, () => new Error("never"));
  await expect(broken).rejects.toThrow("pipe closed");
  // A write that threw must not leave its id waiting for an answer that
  // cannot come.
  expect(p.size).toBe(0);
});
