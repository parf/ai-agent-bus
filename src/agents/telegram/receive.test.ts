import { expect, test } from "bun:test";
import { Receiver } from "./receive.ts";
import { Seen } from "./seen.ts";

const update = (id: number) => ({ update_id: id, message: { message_id: id, date: 1, text: `message${id}`, from: { id: 42, username: "fixture" }, chat: { id: -100, title: "fixture", type: "supergroup" } } });
const realSleep = Bun.sleep;
const realNow = Date.now;
async function until(done: () => boolean) {
  const end = realNow() + 2000;
  while (!done() && realNow() < end) await realSleep(1);
  expect(done()).toBe(true);
}
async function fast(run: (waits: number[]) => Promise<void>) {
  const waits: number[] = [];
  Bun.sleep = (async (ms: any) => { waits.push(Number(ms)); await realSleep(1); }) as typeof Bun.sleep;
  try { await run(waits); } finally { Bun.sleep = realSleep; }
}

test("a refused seen write retries the batch before acknowledging its offset", () => fast(async () => {
  const seen = new Seen(), durable = new Map<string, unknown>();
  const polls: number[] = []; let writes = 0, done = false, forwards = 0;
  const receiver = new Receiver(seen, async (_, __, params) => {
    const offset = Number(params.offset); polls.push(offset);
    if (offset > 10) { receiver.stop(); done = true; return { ok: true, result: [] }; }
    if (polls.length > 4) { receiver.stop(); done = true; throw Error("batch never became durable"); }
    return { ok: true, result: [update(10)] };
  }, async () => undefined, async (key, value) => {
    if (key === "seen:-100" && ++writes === 1) throw Error("fixture write refused");
    if (key === "offset") expect(durable.has("seen:-100")).toBe(true);
    durable.set(key, structuredClone(value));
  }, () => {});
  receiver.onMessage = () => { forwards++; };
  try {
    await receiver.load(); receiver.start(); await until(() => done); await realSleep(5);
    expect(polls.slice(0, 3)).toEqual([0, 0, 11]);
    expect(writes).toBe(2); expect(durable.get("offset")).toBe("11");
    expect((durable.get("seen:-100") as any[]).length).toBe(1); expect(forwards).toBe(1);
  } finally { receiver.stop(); }
}));

test("a restart retries one failed KV read, restores history and forwards a replay only once", () => fast(async () => {
  const durable = new Map<string, string>(); let forwards = 0, offsetWrites = 0;
  const write = async (key: string, value: unknown) => { durable.set(key, typeof value === "string" ? value : JSON.stringify(value)); };
  const first = new Receiver(new Seen(), async () => ({ ok: true, result: [update(20)] }), async key => durable.get(key), async (key, value) => {
    if (key === "offset") { if (++offsetWrites === 3) first.stop(); throw Error("offset unavailable"); }
    await write(key, value);
  }, () => {});
  first.onMessage = () => { forwards++; };
  await first.load(); first.start(); await until(() => offsetWrites === 3); await realSleep(5);
  const seen = new Seen(); let reads = 0, done = false;
  const second = new Receiver(seen, async () => ({ ok: true, result: [update(20)] }), async key => {
    if (++reads === 1) throw Error("one startup read failure"); return durable.get(key);
  }, async (key, value) => { await write(key, value); if (key === "offset") { second.stop(); done = true; } }, () => {});
  second.onMessage = () => { forwards++; };
  try {
    await second.load();
    expect(reads).toBeGreaterThan(2);
    expect(seen.read("-100", 200).map(m => m.text)).toEqual(["message20"]);
    expect(seen.chatList().length).toBe(1); expect(seen.userList().length).toBe(1);
    second.start(); await until(() => done); await realSleep(5);
    expect(seen.read("-100", 200).length).toBe(1); expect(forwards).toBe(1);
  } finally { first.stop(); second.stop(); }
}));

test("poll refusals back off and keep their description for whoami", () => fast(async waits => {
  for (const status of [400, 409]) {
    const start = waits.length;
    const receiver = new Receiver(new Seen(), async () => { receiver.stop(); return { ok: false, status, description: `fixture ${status}` }; }, async () => undefined, async () => {}, () => {});
    await receiver.load(); receiver.start(); await until(() => waits.length > start);
    expect(waits[start]).toBe(status === 409 ? 30_000 : 5000);
    expect(receiver.pollError()).toBe(`fixture ${status}`); await realSleep(5);
  }
}));

test("seen directories stay bounded at 500", () => {
  const seen = new Seen();
  for (let i = 0; i < 510; i++) seen.message(String(i), String(i), "supergroup", { message_id: i, date: 1, from_id: i, from: String(i) });
  expect(seen.chatList().length).toBe(500); expect(seen.userList().length).toBe(500);
});

test("only dirty keys are written per batch; a full flush also writes unchanged rings", async () => {
  const seen = new Seen(); const keys: string[] = [];
  const write = async (key: string) => { keys.push(key); };
  seen.message("a", "a", "supergroup", { message_id: 1, date: 1 });
  await seen.flush(write); expect(keys.sort()).toEqual(["chats", "seen:a"]);
  keys.length = 0;
  seen.message("a", "a", "supergroup", { message_id: 2, date: 1 });
  await seen.flush(write); expect(keys).toEqual(["seen:a"]);
  keys.length = 0;
  await seen.flush(write); expect(keys).toEqual([]);
  await seen.flush(write, true); expect(keys.sort()).toEqual(["chats", "seen:a", "users"]);
});

test("the receiver writes the full stored history after a minute of batches", () => fast(async () => {
  let now = realNow(), polls = 0, done = false;
  const oldNow = Date.now; Date.now = () => now;
  const durable = new Map<string, string>([
    ["chats", JSON.stringify([{ id: "old", type: "supergroup" }])],
    ["seen:old", JSON.stringify([{ message_id: 1, date: 1, text: "earlier" }])],
  ]);
  const keys: string[] = [];
  const receiver = new Receiver(new Seen(), async () => {
    if (++polls === 1) { now += 60_001; return { ok: true, result: [update(30)] }; }
    receiver.stop(); done = true; return { ok: true, result: [] };
  }, async key => durable.get(key), async (key, value) => {
    keys.push(key); durable.set(key, typeof value === "string" ? value : JSON.stringify(value));
  }, () => {});
  try {
    await receiver.load(); receiver.start(); await until(() => done); await realSleep(5);
    expect(keys).toContain("seen:old");
    expect(keys.filter(key => key === "seen:-100").length).toBe(2); // dirty batch, then whole state
  } finally { receiver.stop(); Date.now = oldNow; }
}));
