// The shared vectors, run against the TypeScript twin. The Python engine runs
// the same file; a case that passes here and fails there is a parity bug.
import { expect, test } from "bun:test";
import vectors from "../../testdata/subscriptions.json";
import { checkText, compileFilter, FilterWorker, MATCH_MS, PATTERN_MS, SubscriptionError, Subscriptions, type Admission, type Options, type Port, type Row, type SendOutcome, type WarningCode } from "./subscriptions.ts";

const code = (fn: () => unknown): string | undefined => {
  try {
    fn();
  } catch (err) {
    return err instanceof SubscriptionError ? err.code : `not a SubscriptionError: ${err}`;
  }
  return undefined;
};

test("filter vectors: the common grammar and JS semantics", () => {
  expect(vectors.filters.length).toBeGreaterThan(0);
  for (const row of vectors.filters as { filter: string; text?: string; match?: boolean; error?: string }[]) {
    if (row.error) {
      expect({ filter: row.filter, code: code(() => { compileFilter(row.filter); if (row.text !== undefined) checkText(row.text); }) }).toEqual({ filter: row.filter, code: row.error });
      continue;
    }
    const re = compileFilter(row.filter);
    checkText(row.text!);
    expect({ filter: row.filter, text: row.text, match: re ? re.test(row.text!) : true }).toEqual({ filter: row.filter, text: row.text, match: row.match });
  }
});

type Step = Record<string, any>;

/** A port that answers from the step: its admission answers in order, its send outcome. */
function fake(initial: Row[]) {
  const store = new Map<string, unknown>(initial.map(r => [`sub.${r.id}`, structuredClone(r)]));
  const state = { checks: [] as unknown[], send: "must-not-send" as string, queries: [] as { who: string; target: string }[], writes: 0, attempts: 0,
    kind: "queue", store: "ok", counterStore: "ok" };
  const port: Port = {
    async admits(who, target): Promise<Admission> {
      state.queries.push({ who, target });
      if (!state.checks.length) throw new Error(`unscripted admission question ${who} -> ${target}`);
      const answer = state.checks.shift();
      return answer === true ? "yes" : answer === false || answer === "404" ? "no" : "unavailable";
    },
    async kind() { return state.kind; },
    async load() { return new Map(store); },
    async create(key, row) {
      state.writes++;
      if (store.has(key)) return false;
      store.set(key, structuredClone(row));
      // Applied, but the answer is lost: the engine must read the store again.
      if (state.store === "unknown_after") throw new Error("outcome unknown");
      return true;
    },
    // A write is a stored row or a counter write, as the Python harness counts.
    async remove(key) { return store.delete(key); },
    async counts(key, sent, dropped) {
      state.writes++;
      if (state.counterStore === "unknown") throw new Error("outcome unknown");
      Object.assign(store.get(key) as Row, { sent, dropped });
    },
    async send(): Promise<SendOutcome> {
      state.attempts++;
      if (state.send === "must-not-send") throw new Error("a send that must not happen");
      return state.send as SendOutcome;
    },
  };
  return { port, state, store };
}

for (const c of vectors.cases as Record<string, any>[]) {
  test(`case ${c.name}`, async () => {
    const { port, state } = fake(c.initial ?? []);
    const ids = [...(c.ids ?? [])];
    const warnings: WarningCode[] = [];
    const subs = new Subscriptions(port, c.source, {
      validateSelector: s => { if (typeof s !== "object" || s === null || Array.isArray(s)) throw new Error("selector"); },
      selectorMatches: (s, e) => Object.entries(s).every(([k, v]) => e[k] === v),
      warning: w => warnings.push(w),
      clock: () => new Date(c.now ?? "2026-01-01T00:00:00.000Z"),
      idFactory: () => ids.shift() ?? "ffffffffffff",
      ...(c.limits?.max_per_creator !== undefined ? { maxPerCreator: c.limits.max_per_creator } : {}),
      ...(c.limits?.max_subscriptions !== undefined ? { maxSubscriptions: c.limits.max_subscriptions } : {}),
    });
    await subs.reload();
    for (const [n, step] of (c.steps as Step[]).entries()) {
      const where = `${c.name} step ${n} ${step.op}`;
      state.checks = [...(step.checks ?? [])];
      state.send = step.send ?? "must-not-send";
      state.queries = [];
      state.writes = 0;
      state.attempts = 0;
      state.kind = step.kind ?? "queue";
      state.store = step.store ?? "ok";
      state.counterStore = step.counter_store ?? "ok";
      warnings.length = 0;
      const caller = { from: step.author, roles: step.roles ?? [], ...(step.original_to ? { original_to: step.original_to } : {}) };
      const want = step.expect;
      let got: Record<string, unknown> = {};
      try {
        if (step.op === "add") got.row = await subs.add(caller, step.target, step.selector, step.filter);
        else if (step.op === "list") got.rows = subs.list(caller);
        else if (step.op === "delete") got.deleted = await subs.delete(caller, step.id);
        else if (step.op === "deliver") {
          const deadline = step.deadline === "expired" ? Date.now() - 1000 : undefined;
          got = await subs.deliver(step.event, step.event.text, step.event.text, deadline === undefined ? {} : { deadline });
        } else throw new Error(`unknown op ${step.op}`);
      } catch (err) {
        if (!(err instanceof SubscriptionError)) throw err;
        got = { error: err.code };
      }
      const seen: Record<string, unknown> = { queries: state.queries, attempts: state.attempts, warnings: [...warnings], writes: state.writes };
      for (const field of Object.keys(seen)) if (field in want) got[field] = seen[field];
      expect({ where, ...got }).toEqual({ where, ...want });
      // Every scripted admission answer was asked for: none left over.
      expect({ where, unasked: state.checks }).toEqual({ where, unasked: [] });
    }
  });
}

const objectSelector: Options["validateSelector"] = s => { if (typeof s !== "object" || s === null || Array.isArray(s)) throw new Error("selector"); };

/** An engine over raw stored values, for what the vectors cannot hold. */
async function engine(stored: Record<string, unknown>, options: Partial<Options> = {}) {
  const { port, state, store } = fake([]);
  for (const [key, value] of Object.entries(stored)) store.set(key, structuredClone(value));
  const warnings: WarningCode[] = [];
  const subs = new Subscriptions(port, "#source@fixture.test", {
    validateSelector: objectSelector,
    selectorMatches: () => true,
    warning: w => warnings.push(w),
    idFactory: () => "0000000000ff",
    ...options,
  });
  await subs.reload();
  return { subs, port, state, store, warnings };
}

const good: Row = { id: "000000000001", target: "events@fixture.test", selector: {}, filter: "", created_by: "#writer@fixture.test",
  created_at: "2026-10-29T00:00:00.000Z", sent: 0, dropped: 0 };
const writer = { from: "#writer@fixture.test", roles: [] };
const manager = { from: "#manager@fixture.test", roles: ["maintainer"] };

test("damaged stored rows are listed to managers only, inert, and repaired only by a manager", async () => {
  const shapes: Record<string, unknown> = {
    "not an object": "text",
    "an array": [good],
    "a missing field": (({ dropped, ...r }) => r)(good),
    "an extra field": { ...good, paused: true },
    "a negative count": { ...good, sent: -1 },
    "an unsafe count": { ...good, dropped: 2 ** 53 },
    "a fractional count": { ...good, sent: 1.5 },
    "a local creation time": { ...good, created_at: "2026-10-29T00:00:00.000+01:00" },
    "an impossible creation time": { ...good, created_at: "2026-13-40T00:00:00.000Z" },
    "a filter that is not text": { ...good, filter: 7 },
    "a bad target": { ...good, target: "Events Queue" },
    "a creator that is no name": { ...good, created_by: null },
    "a refused selector": { ...good, selector: [] },
  };
  for (const [shape, value] of Object.entries(shapes)) {
    const v = typeof value === "object" && value !== null && !Array.isArray(value) ? { ...value, id: "000000000002" } : value;
    const { subs, state, warnings } = await engine({ "sub.000000000001": good, "sub.000000000002": v });
    const ids = (rows: { id: string }[]) => rows.map(r => r.id);
    expect({ shape, writer: ids(subs.list(writer)) }).toEqual({ shape, writer: ["000000000001"] });
    expect({ shape, manager: subs.list(manager) }).toEqual({ shape, manager: [good, { id: "000000000002", reason: "invalid_state" }] });
    // Delivery never reads it: only the good row asks and sends.
    state.checks = [true, true];
    state.send = "ok";
    expect({ shape, got: await subs.deliver({}, "x", "x") }).toEqual({ shape, got: { matched: 1, sent: 1, dropped: 0 } });
    expect({ shape, warnings, queries: state.queries.map(q => q.target) }).toEqual({ shape, warnings: [], queries: ["events@fixture.test", "#source@fixture.test"] });
    expect({ shape, code: await subs.delete(writer, "000000000002").catch(e => e.code) }).toEqual({ shape, code: "not_author" });
    expect({ shape, deleted: await subs.delete(manager, "000000000002") }).toEqual({ shape, deleted: true });
    expect({ shape, after: ids(subs.list(manager)) }).toEqual({ shape, after: ["000000000001"] });
  }
});

test("a damaged row still takes a place under the subscription limit", async () => {
  const { subs } = await engine({ "sub.000000000001": good, "sub.000000000002": "text" }, { maxSubscriptions: 2 });
  expect(await subs.add(writer, "events@fixture.test", {}).catch(e => e.code)).toBe("limit");
});

test("an expired deadline drops each selector candidate before its filter is tested", async () => {
  let tested = 0;
  const { subs, state, warnings } = await engine({ "sub.000000000001": { ...good, filter: "/nomatch/" } },
    { matcher: async () => { tested++; return "miss"; } });
  expect(await subs.deliver({}, "x", "x", { deadline: Date.now() - 1 })).toEqual({ matched: 1, sent: 0, dropped: 1 });
  expect({ tested, warnings, queries: state.queries, attempts: state.attempts }).toEqual({ tested: 0, warnings: ["deadline"], queries: [], attempts: 0 });
});

// Exponential backtracking inside the common grammar: seconds natively.
const slow = "(a|aa)+$";
const slowText = "a".repeat(60) + "b";

test("a pathological filter runs out of time in the worker; the next filter gets a fresh worker", async () => {
  const { subs, state, warnings } = await engine({
    "sub.000000000001": { ...good, filter: slow },
    "sub.000000000002": { ...good, id: "000000000002", filter: "/b$/" },
  });
  state.checks = [true, true];
  state.send = "ok";
  const started = Date.now();
  const got = await subs.deliver({}, slowText, "x");
  const took = Date.now() - started;
  expect({ got, attempts: state.attempts, warnings }).toEqual({ got: { matched: 2, sent: 1, dropped: 1 }, attempts: 1, warnings: ["filter_timeout"] });
  expect(subs.list(writer).map(r => [r.id, (r as Row).sent, (r as Row).dropped])).toEqual([["000000000001", 0, 1], ["000000000002", 1, 0]]);
  expect(took).toBeLessThan(MATCH_MS);
});

test("one delivery's matching stops at its budget however many filters are slow", async () => {
  const count = Math.ceil((2 * MATCH_MS) / PATTERN_MS);
  const stored: Record<string, unknown> = {};
  for (let i = 1; i <= count; i++) {
    const id = i.toString(16).padStart(12, "0");
    stored[`sub.${id}`] = { ...good, id, filter: `${slow}|z${i}` };
  }
  const { subs, warnings } = await engine(stored, { maxSubscriptions: count });
  const started = Date.now();
  await subs.deliver({}, slowText, "x");
  const took = Date.now() - started;
  expect({ warnings: warnings.length, every: new Set(warnings) }).toEqual({ warnings: count, every: new Set(["filter_timeout"]) });
  expect(took).toBeGreaterThanOrEqual(MATCH_MS - PATTERN_MS);
  expect(took).toBeLessThan(MATCH_MS + 5 * PATTERN_MS);
});

test("a closed worker answers every later test slow at once and starts no worker", async () => {
  const worker = new FilterWorker();
  worker.close();
  const spawned: unknown[] = [];
  const Real = globalThis.Worker;
  globalThis.Worker = class extends Real { constructor(...args: ConstructorParameters<typeof Worker>) { super(...args); spawned.push(this); } } as typeof Worker;
  try {
    const started = Date.now();
    expect(await worker.test(/a/, "a", Date.now() + MATCH_MS)).toBe("slow");
    expect({ spawned: spawned.length, quick: Date.now() - started < PATTERN_MS }).toEqual({ spawned: 0, quick: true });
  } finally {
    globalThis.Worker = Real;
    for (const w of spawned) (w as Worker).terminate();
  }
});

test("close during a worker's start answers at once, not at the batch's end", async () => {
  const worker = new FilterWorker();
  const verdict = worker.test(/a/, "a", Date.now() + MATCH_MS);
  // Before the new worker can have said it is ready.
  await Promise.resolve();
  const started = Date.now();
  worker.close();
  expect(await verdict).toBe("slow");
  expect(Date.now() - started).toBeLessThan(PATTERN_MS);
});

test("close terminates a test in flight; the worker answers nothing after", async () => {
  const worker = new FilterWorker();
  const verdict = worker.test(new RegExp(slow), slowText, Date.now() + MATCH_MS);
  await Bun.sleep(10);
  const started = Date.now();
  worker.close();
  expect(await verdict).toBe("slow");
  expect(Date.now() - started).toBeLessThan(PATTERN_MS);
  expect(await worker.test(/a/, "a", Date.now() + MATCH_MS)).toBe("slow");
});

test("a send never starts once close has returned, and a later delivery is refused", async () => {
  const { subs, port, state, warnings } = await engine({ "sub.000000000001": good });
  const ask = port.admits;
  // The engine closes while a copy waits on its admission answer.
  port.admits = async (who, target) => { subs.close(); return ask(who, target); };
  state.checks = [true, true];
  state.send = "ok";
  expect(await subs.deliver({}, "x", "x")).toEqual({ matched: 1, sent: 0, dropped: 1 });
  expect({ attempts: state.attempts, warnings }).toEqual({ attempts: 0, warnings: ["deadline"] });
  state.queries = [];
  expect(await subs.deliver({}, "x", "x").catch(e => e.code)).toBe("store_unknown");
  expect(state.queries).toEqual([]);
});
