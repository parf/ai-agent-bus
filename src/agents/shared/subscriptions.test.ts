// The shared vectors, run against the TypeScript twin. The Python engine runs
// the same file; a case that passes here and fails there is a parity bug.
import { expect, test } from "bun:test";
import vectors from "../../testdata/subscriptions.json";
import { checkText, compileFilter, SubscriptionError, Subscriptions, type Admission, type Options, type Port, type Row, type SendOutcome, type WarningCode } from "./subscriptions.ts";

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
    async remove(key) { state.writes++; return store.delete(key); },
    async counts(key, sent, dropped) {
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
          got = { ...(await subs.deliver(step.event, step.event.text, step.event.text, deadline === undefined ? {} : { deadline })), warnings: [...warnings], attempts: state.attempts };
        } else throw new Error(`unknown op ${step.op}`);
      } catch (err) {
        if (!(err instanceof SubscriptionError)) throw err;
        got = { error: err.code };
      }
      if ("queries" in want) got.queries = state.queries;
      if ("writes" in want) got.writes = state.writes;
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
  return { subs, state, store, warnings };
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
