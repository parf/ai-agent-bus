// The shared vectors, run against the TypeScript twin. The Python engine runs
// the same file; a case that passes here and fails there is a parity bug.
import { expect, test } from "bun:test";
import vectors from "../../testdata/subscriptions.json";
import { checkText, compileFilter, SubscriptionError, Subscriptions, type Admission, type Port, type Row, type SendOutcome, type WarningCode } from "./subscriptions.ts";

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
