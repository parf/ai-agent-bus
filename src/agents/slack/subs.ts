// Subscriptions, in the agent's own KV (Plans/R1.2/slack/README.md
// #subscriptions-authority-errors): each subscription one JSON value
// `sub.<id>` with a short random hex id, created with mode `add` so a
// collision is refused, edited field-by-field through KV JSON ops so two
// disjoint edits both land, deleted outright. Authority (S.8): whoever
// manages a subscription's destination, as the daemon answers it, or the
// agent's own owner/maintainer roles, which count only on a message
// addressed to the agent itself — roles arriving through a forward were
// asserted for the forwarding record (slack-review-S.1's escalation).
//
// Only a KV "not found" means absent: any other failure — the daemon away, a
// refusal — is the call failing, never an empty store or a missing id
// (slack-review-S.4). The store is this agent's alone: a writer outside it
// editing between this agent's read and write is not guarded against.
import type { Bus, Message } from "../../clients/typescript/src/index.js";
import { BusError, KVNotFound, UnexpectedAnswer } from "../../clients/typescript/src/index.js";

export type Subscription = { id: string; channel: string; target: string; filter?: string; from_users?: string[]; created_by: string; created_at: string; disabled?: boolean };

/** Why `who`, a subscription's author, may not have it send to `target`, or
 *  undefined when the daemon says it may (RD141). The receiver's own ACL still
 *  decides what the agent may send; this is the author's right, asked at add,
 *  at an edit that moves the target, and before every forward. A daemon that
 *  cannot answer is a failed call, never a yes. */
export async function authorCannotSend(bus: Bus, self: string, who: string, target: string): Promise<string | undefined> {
  try {
    if (await bus.maySend(who, target)) return undefined;
  } catch (err) {
    if ((err as any)?.status === 404) return `${target} is not a destination this agent can see; its ACL must admit ${self}`;
    if (err instanceof UnexpectedAnswer) return `cannot check that ${who} may send to ${target}: the daemon does not answer may-send for another name; nothing stored`;
    throw err;
  }
  return `${who} may not send to ${target}`;
}

/** What a subscription call answers: the text form, and the JSON form's fields. */
export type SubResult = { ok: true; text: string; fields: Record<string, unknown> } | { ok: false; error: string };

/** Who may change any subscription: a daemon-asserted owner or maintainer
 *  role, on a message addressed here. A forward carries the forwarder's roles. */
export function mayChangeSubscriptions(e: Message, self: string): boolean {
  const addressed = typeof e.original_to === "string" ? e.original_to : "";
  if (addressed !== "" && addressed.toLowerCase() !== self) return false;
  const roles = e.roles;
  return roles.includes("owner") || roles.includes("maintainer");
}

/** `/pattern/flags` or a bare pattern, split. */
function split(filter: string): { pattern: string; flags: string } | undefined {
  if (!filter.startsWith("/")) return { pattern: filter, flags: "" };
  const close = filter.lastIndexOf("/");
  if (close < 1) return undefined;
  return { pattern: filter.slice(1, close), flags: filter.slice(close + 1) };
}

/** A filter as pinned: the PATTERN at most 200 characters (delimiters and
 *  flags not counted), flags i m s u only (g and y carry state between tests
 *  and are refused), valid or refused. */
export function filterProblem(filter: string | undefined): string | undefined {
  if (filter === undefined) return undefined;
  const parts = split(filter);
  if (!parts) return "a /filter/ needs its closing slash";
  if (parts.pattern.length > 200) return "a filter's pattern is at most 200 characters";
  if ([...parts.flags].some(f => !"imsu".includes(f))) return `filter flags are i m s u, not "${parts.flags}"`;
  try {
    new RegExp(parts.pattern, parts.flags);
  } catch (err) {
    return `not a valid filter: ${err instanceof Error ? err.message : String(err)}`;
  }
  return undefined;
}

/** `/pattern/flags` or a bare pattern, as the README's filter grammar. */
export function toRegExp(filter: string): RegExp {
  const parts = split(filter);
  if (!parts) throw new Error("a /filter/ needs its closing slash");
  return new RegExp(parts.pattern, parts.flags);
}

/** Everything the subscriptions need of the bus: the agent's own record's KV. */
export type Store = { kv: { get(kind: "json" | "int" | "string", name: string): Promise<{ value: any }>; set(kind: "json" | "string", name: string, value: unknown, options: { mode: "set" | "add" | "replace" }): Promise<any>; delete(kind: "json" | "int" | "string", name: string): Promise<boolean>; increment?(name: string, options: { by: number }): Promise<bigint>; jsonApply(name: string, ops: { op: string; key: string; value?: unknown }[]): Promise<any>; list(): Promise<any> } };

/** One subscription's writes, one at a time. Every writer of a subscription
 *  and its counters is in this process — an edit, a delete, a disable, a
 *  flush — so taking turns per id keeps a field edit from being overwritten
 *  and a deleted subscription from being written back (slack-review-S.5). A
 *  writer outside the process is not guarded against. */
const turns = new Map<string, Promise<unknown>>();
export function withSub<T>(id: string, fn: () => Promise<T>): Promise<T> {
  const prev = turns.get(id) ?? Promise.resolve();
  const run = prev.then(fn, fn);
  const tail = run.catch(() => {});
  turns.set(id, tail);
  tail.then(() => { if (turns.get(id) === tail) turns.delete(id); });
  return run;
}

const newId = () => [...crypto.getRandomValues(new Uint8Array(4))].map(b => b.toString(16).padStart(2, "0")).join("");

const storeOf = (bus: Bus | Store, self: string): Store =>
  "kv" in bus ? (bus as Store) : (bus as unknown as { record: (n: string) => Store }).record(self);

export async function subAdd(bus: Bus | Store, self: string, e: Message, channel: string, target: string, filter: string | undefined, from_users?: string[]): Promise<SubResult> {
  const problem = filterProblem(filter);
  if (problem) return { ok: false, error: problem };
  const sub: Subscription = { id: "", channel, target, ...(from_users?.length ? { from_users } : {}), ...(filter !== undefined && filter !== "" ? { filter } : {}), created_by: e.from, created_at: new Date().toISOString() };
  for (let attempt = 0; attempt < 3; attempt++) {
    sub.id = newId();
    try {
      await storeOf(bus, self).kv.set("json", `sub.${sub.id}`, sub, { mode: "add" });
      return { ok: true, text: JSON.stringify({ id: sub.id }), fields: { id: sub.id } };
    } catch (err) {
      // The daemon refuses an add whose name is taken with 412: a collision retries with a new id.
      if (!(err instanceof BusError && err.status === 412)) throw err;
    }
  }
  return { ok: false, error: "could not pick a free subscription id" };
}

/** A value that may be absent: absent is undefined, any other failure throws. */
async function maybe(store: Store, kind: "int" | "string", name: string): Promise<any> {
  try {
    return (await store.kv.get(kind, name)).value;
  } catch (err) {
    if (err instanceof KVNotFound) return undefined;
    throw err;
  }
}

/** The counters a forwarder keeps in memory between flushes. */
export type Pending = (id: string) => { forwarded: number; lastError?: string };

export async function subList(bus: Bus | Store, self: string, pending?: Pending): Promise<SubResult> {
  const store = storeOf(bus, self);
  const listing = await store.kv.list();
  const names: string[] = [];
  for (const kind of Object.keys(listing ?? {})) {
    for (const entry of listing[kind] ?? []) if (typeof entry?.name === "string" && entry.name.startsWith("sub.")) names.push(entry.name);
  }
  names.sort();
  const subs: (Subscription & { forwarded: number; last_error?: string })[] = [];
  for (const name of names) {
    let sub: Subscription;
    try {
      sub = (await store.kv.get("json", name)).value;
    } catch (err) {
      if (!(err instanceof KVNotFound)) throw err; // deleted between list and get is the only skip
      continue;
    }
    const flushed = Number((await maybe(store, "int", `stat.${sub.id}.forwarded`)) ?? 0);
    const now = pending?.(sub.id);
    const lastError = now?.lastError ?? (await maybe(store, "string", `stat.${sub.id}.last_error`));
    subs.push({ ...sub, forwarded: flushed + (now?.forwarded ?? 0), ...(lastError ? { last_error: lastError } : {}) });
  }
  const text = subs.map(s => `${s.id} ${s.channel} ${s.target}${s.from_users?.length ? ` from=${s.from_users.join(",")}` : ""}${s.filter ? ` ${s.filter}` : ""} by=${s.created_by} at=${s.created_at} forwarded=${s.forwarded}${s.disabled ? " disabled" : ""}${s.last_error ? ` last_error=${s.last_error}` : ""}`).join("\n") || "(no subscriptions)";
  return { ok: true, text, fields: { subs } };
}

/** Asked in a subscription's turn, with it as stored: why the caller may
 *  not change it, or undefined when it may. */
export type Guard = (sub: Subscription) => Promise<string | undefined>;

export async function subEdit(bus: Bus | Store, self: string, subId: string, edits: { channel?: string; target?: string; filter?: string; from_users?: string[] }, guard?: Guard): Promise<SubResult> {
  if (edits.filter !== undefined && edits.filter !== "") {
    const problem = filterProblem(edits.filter);
    if (problem) return { ok: false, error: problem };
  }
  return withSub(subId, () => editNow(bus, self, subId, edits, guard));
}

/** The subscription as stored, or undefined when there is none. */
async function stored(store: Store, subId: string): Promise<Subscription | undefined> {
  try {
    return (await store.kv.get("json", `sub.${subId}`)).value as Subscription;
  } catch (err) {
    if (err instanceof KVNotFound) return undefined;
    throw err;
  }
}

async function editNow(bus: Bus | Store, self: string, subId: string, edits: { channel?: string; target?: string; filter?: string; from_users?: string[] }, guard?: Guard): Promise<SubResult> {
  const store = storeOf(bus, self);
  const name = `sub.${subId}`;
  const sub = await stored(store, subId); // an edit of what is not there says so
  if (!sub) return { ok: false, error: `no subscription ${subId}` };
  const refused = guard && (await guard(sub));
  if (refused) return { ok: false, error: refused };
  const ops: { op: string; key: string; value?: unknown }[] = [];
  if (edits.channel !== undefined) ops.push({ op: "set", key: "channel", value: edits.channel });
  if (edits.target !== undefined) ops.push({ op: "set", key: "target", value: edits.target });
  if (edits.filter !== undefined) ops.push(edits.filter === "" ? { op: "unset", key: "filter" } : { op: "set", key: "filter", value: edits.filter });
  if (edits.from_users !== undefined) ops.push(edits.from_users.length ? { op: "set", key: "from_users", value: edits.from_users } : { op: "unset", key: "from_users" });
  // A filter edit replaces the filter a slow one was disabled for: the subscription runs again.
  if (edits.filter !== undefined && sub.disabled) ops.push({ op: "unset", key: "disabled" });
  await store.kv.jsonApply(name, ops); // field-by-field: disjoint edits both land
  return { ok: true, text: "edited", fields: { id: subId } };
}

/** Mark a subscription disabled. Checked first, in its turn: a JSON op on a
 *  value that is gone would create it again as {disabled: true}, so one
 *  deleted meanwhile stays deleted (slack-review-S.5). A filter edit enables it again. */
export function subDisable(bus: Bus | Store, self: string, subId: string): Promise<void> {
  // In its turn: a delete cannot come between the check and the write, and
  // the JSON op sets one field, so an edit before or after it keeps its own.
  return withSub(subId, async () => {
    const kv = storeOf(bus, self).kv;
    await kv.get("json", `sub.${subId}`); // gone: refused, never written back
    await kv.jsonApply(`sub.${subId}`, [{ op: "set", key: "disabled", value: true }]);
  });
}

export function subDelete(bus: Bus | Store, self: string, subId: string, guard?: Guard): Promise<SubResult> {
  return withSub(subId, () => deleteNow(bus, self, subId, guard));
}

async function deleteNow(bus: Bus | Store, self: string, subId: string, guard?: Guard): Promise<SubResult> {
  const store = storeOf(bus, self);
  if (guard) {
    const sub = await stored(store, subId);
    if (!sub) return { ok: false, error: `no subscription ${subId}` };
    const refused = await guard(sub);
    if (refused) return { ok: false, error: refused };
  }
  const gone = await store.kv.delete("json", `sub.${subId}`);
  if (gone) {
    // Its counters go with it; a forwarder's flush checks it still exists.
    for (const [kind, name] of [["int", `stat.${subId}.forwarded`], ["string", `stat.${subId}.last_error`]] as const) {
      await store.kv.delete(kind, name).catch(err => { if (!(err instanceof KVNotFound)) throw err; });
    }
  }
  return gone ? { ok: true, text: "deleted", fields: { id: subId } } : { ok: false, error: `no subscription ${subId}` };
}
