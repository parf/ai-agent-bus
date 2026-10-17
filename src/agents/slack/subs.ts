// Subscriptions, in the agent's own KV (Plans/R1.2/slack/README.md
// #subscriptions-authority-errors): each subscription one JSON value
// `sub.<id>` with a short random hex id, created with mode `add` so a
// collision is refused, edited field-by-field through KV JSON ops so two
// disjoint edits both land, deleted outright. Authority: the daemon's
// owner/maintainer roles, which count only on a message addressed to the
// agent itself — roles arriving through a forward were asserted for the
// forwarding record (slack-review-S.1's escalation, closed by design).
//
// Only a KV "not found" means absent: any other failure — the daemon away, a
// refusal — is the call failing, never an empty store or a missing id
// (slack-review-S.4). The store is this agent's alone: a writer outside it
// editing between this agent's read and write is not guarded against.
import type { Bus, Message } from "../../clients/typescript/src/index.js";
import { KVNotFound } from "../../clients/typescript/src/index.js";

export type Subscription = { id: string; channel: string; target: string; filter?: string; created_by: string; created_at: string };

/** What a subscription call answers: the text form, and the JSON form's fields. */
export type SubResult = { ok: true; text: string; fields: Record<string, unknown> } | { ok: false; error: string };

/** Who may change subscriptions: a daemon-asserted owner or maintainer role,
 *  on a message addressed here. A forward carries the forwarder's roles. */
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
export type Store = { kv: { get(kind: "json", name: string): Promise<{ value: any }>; set(kind: "json", name: string, value: unknown, options: { mode: "set" | "add" | "replace" }): Promise<any>; delete(kind: "json", name: string): Promise<boolean>; jsonApply(name: string, ops: { op: string; key: string; value?: unknown }[]): Promise<any>; list(): Promise<any> } };

const newId = () => [...crypto.getRandomValues(new Uint8Array(4))].map(b => b.toString(16).padStart(2, "0")).join("");

const storeOf = (bus: Bus | Store, self: string): Store =>
  "kv" in bus ? (bus as Store) : (bus as unknown as { record: (n: string) => Store }).record(self);

export async function subAdd(bus: Bus | Store, self: string, e: Message, channel: string, target: string, filter: string | undefined): Promise<SubResult> {
  const problem = filterProblem(filter);
  if (problem) return { ok: false, error: problem };
  const sub: Subscription = { id: "", channel, target, ...(filter !== undefined && filter !== "" ? { filter } : {}), created_by: e.from, created_at: new Date().toISOString() };
  for (let attempt = 0; attempt < 3; attempt++) {
    sub.id = newId();
    try {
      await storeOf(bus, self).kv.set("json", `sub.${sub.id}`, sub, { mode: "add" });
      return { ok: true, text: JSON.stringify({ id: sub.id }), fields: { id: sub.id } };
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      if (!message.includes("exists")) throw err; // a collision retries with a new id
    }
  }
  return { ok: false, error: "could not pick a free subscription id" };
}

export async function subList(bus: Bus | Store, self: string): Promise<SubResult> {
  const store = storeOf(bus, self);
  const listing = await store.kv.list();
  const names: string[] = [];
  for (const kind of Object.keys(listing ?? {})) {
    for (const entry of listing[kind] ?? []) if (typeof entry?.name === "string" && entry.name.startsWith("sub.")) names.push(entry.name);
  }
  names.sort();
  const subs: Subscription[] = [];
  for (const name of names) {
    try {
      subs.push((await store.kv.get("json", name)).value);
    } catch (err) {
      if (!(err instanceof KVNotFound)) throw err; // deleted between list and get is the only skip
    }
  }
  const text = subs.map(s => `${s.id} ${s.channel} ${s.target}${s.filter ? ` ${s.filter}` : ""} by=${s.created_by} at=${s.created_at}`).join("\n") || "(no subscriptions)";
  return { ok: true, text, fields: { subs } };
}

export async function subEdit(bus: Bus | Store, self: string, subId: string, edits: { channel?: string; target?: string; filter?: string }): Promise<SubResult> {
  if (edits.filter !== undefined && edits.filter !== "") {
    const problem = filterProblem(edits.filter);
    if (problem) return { ok: false, error: problem };
  }
  const store = storeOf(bus, self);
  const name = `sub.${subId}`;
  try {
    await store.kv.get("json", name); // an edit of what is not there says so
  } catch (err) {
    if (err instanceof KVNotFound) return { ok: false, error: `no subscription ${subId}` };
    throw err;
  }
  const ops: { op: string; key: string; value?: unknown }[] = [];
  if (edits.channel !== undefined) ops.push({ op: "set", key: "channel", value: edits.channel });
  if (edits.target !== undefined) ops.push({ op: "set", key: "target", value: edits.target });
  if (edits.filter !== undefined) ops.push(edits.filter === "" ? { op: "unset", key: "filter" } : { op: "set", key: "filter", value: edits.filter });
  await store.kv.jsonApply(name, ops); // field-by-field: disjoint edits both land
  return { ok: true, text: "edited", fields: { id: subId } };
}

export async function subDelete(bus: Bus | Store, self: string, subId: string): Promise<SubResult> {
  const gone = await storeOf(bus, self).kv.delete("json", `sub.${subId}`);
  return gone ? { ok: true, text: "deleted", fields: { id: subId } } : { ok: false, error: `no subscription ${subId}` };
}
