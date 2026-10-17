import { afterAll, beforeAll, expect, test } from "bun:test";
import { startFixture, type Fixture } from "./daemon.ts";
import { Bus, type Message } from "../../clients/typescript/src/index.js";
import { filterProblem, subAdd, subEdit, subList, type Store } from "./subs.ts";

// The subscription store's own rules (slack-review-S.4), through a real KV:
// only "not found" means absent, the pattern is what the 200-character cap
// counts, and two edits that truly overlap both land.
/** The real store's methods, one of them replaced: a class's methods do not spread. */
function wrap(over: Partial<Store["kv"]>): Store {
  const kv = store.kv;
  return { kv: {
    get: (k, n) => kv.get(k, n), set: (k, n, v, o) => kv.set(k, n, v, o), delete: (k, n) => kv.delete(k, n),
    jsonApply: (n, ops) => kv.jsonApply(n, ops), list: () => kv.list(), ...over,
  } };
}

let fx: Fixture;
let store: Store;
const self = `#slack-subs-${process.pid}@t`;
const msg = { from: "owner@t", roles: ["owner"] } as unknown as Message;

beforeAll(async () => {
  fx = await startFixture();
  await fx.call(fx.ownerToken, "POST", "/register", { name: self, kind: "agent" });
  store = Bus.connect({ addr: fx.socket, token: fx.ownerToken }).record(self) as unknown as Store;
}, 30000);

afterAll(async () => {
  await fx?.stop();
}, 15000);

test("a failing store is the call failing, never an empty list or a missing id", async () => {
  const broken = wrap({
    list: async () => ({ json: [{ name: "sub.dead" }] }),
    get: async () => { throw new Error("transport failed"); },
  });
  await expect(subList(broken, self)).rejects.toThrow("transport failed");
  await expect(subEdit(broken, self, "dead", { target: "#x" })).rejects.toThrow("transport failed");
  // A real "not found" is still an answer.
  expect(await subEdit(store, self, "nosuch", { target: "#x" })).toEqual({ ok: false, error: "no subscription nosuch" });
});

test("the 200-character cap counts the pattern, not its delimiters or flags", () => {
  const p200 = "x".repeat(200);
  expect(filterProblem(p200)).toBeUndefined();
  expect(filterProblem(`/${p200}/i`)).toBeUndefined();
  expect(filterProblem("x".repeat(201))).toContain("at most 200");
  expect(filterProblem(`/${"x".repeat(201)}/i`)).toContain("at most 200");
  expect(filterProblem("/a/g")).toContain("flags are i m s u");
});

test("two edits of one subscription that overlap both land", async () => {
  const made = await subAdd(store, self, msg, "C1", "#w", "^a");
  if (!made.ok) throw new Error(made.error);
  const id = made.fields.id as string;
  // Both edits read before either writes: a whole-value read-modify-write
  // would let the second overwrite the first.
  let reads = 0;
  let release!: () => void;
  const bothRead = new Promise<void>(r => (release = r));
  const barrier = wrap({
    get: async (kind: "json", name: string) => {
      const got = await store.kv.get(kind, name);
      if (++reads === 2) release();
      await bothRead;
      return got;
    },
  });
  const [a, b] = await Promise.all([
    subEdit(barrier, self, id, { channel: "C2" }),
    subEdit(barrier, self, id, { target: "#other" }),
  ]);
  expect(a.ok && b.ok).toBe(true);
  const value = (await store.kv.get("json", `sub.${id}`)).value;
  expect(value.channel).toBe("C2");
  expect(value.target).toBe("#other");
  expect(value.filter).toBe("^a");
});
