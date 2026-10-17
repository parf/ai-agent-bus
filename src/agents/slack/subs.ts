// Subscriptions, in the agent's own KV (Plans/R1.2/slack/README.md
// #subscriptions-authority-errors): each subscription one JSON value
// `sub.<id>` with a short random hex id, created with mode `add` so a
// collision is refused, edited field-by-field through KV JSON ops so two
// disjoint edits both land, deleted outright. Authority: the daemon's
// owner/maintainer roles, which count only on a message addressed to the
// agent itself — roles arriving through a forward were asserted for the
// forwarding record (slack-review-S.1's escalation, closed by design).
import type { Bus, Message } from "../../clients/typescript/src/index.js";
import { KVNotFound } from "../../clients/typescript/src/index.js";

export type Subscription = { id: string; channel: string; target: string; filter?: string; created_by: string; created_at: string };

/** Who may change subscriptions: a daemon-asserted owner or maintainer role,
 *  on a message addressed here. A forward carries the forwarder's roles. */
export function mayChangeSubscriptions(e: Message, self: string): boolean {
  const addressed = typeof e.original_to === "string" ? e.original_to : "";
  if (addressed !== "" && addressed.toLowerCase() !== self) return false;
  const roles = e.roles;
  return roles.includes("owner") || roles.includes("maintainer");
}

/** A filter as pinned: flags i m s u only (g and y carry state between tests
 *  and are refused), at most 200 characters, valid or refused. */
export function filterProblem(filter: string | undefined): string | undefined {
  if (filter === undefined) return undefined;
  if (filter.length > 200) return "a filter is at most 200 characters";
  if (filter.startsWith("/")) {
    const close = filter.lastIndexOf("/");
    if (close < 1) return "a /filter/ needs its closing slash";
    const flags = filter.slice(close + 1);
    if ([...flags].some(f => !"imsu".includes(f))) return `filter flags are i m s u, not "${flags}"`;
  }
  try {
    toRegExp(filter);
  } catch (err) {
    return `not a valid filter: ${err instanceof Error ? err.message : String(err)}`;
  }
  return undefined;
}

/** `/pattern/flags` or a bare pattern, as the README's filter grammar. */
export function toRegExp(filter: string): RegExp {
  if (filter.startsWith("/")) {
    const close = filter.lastIndexOf("/");
    if (close < 1) throw new Error("a /filter/ needs its closing slash");
    return new RegExp(filter.slice(1, close), filter.slice(close + 1));
  }
  return new RegExp(filter);
}

/** Everything S.4 needs of the bus: the agent's own record's KV. */
export type Store = { kv: { get(kind: "json", name: string): Promise<{ value: any }>; set(kind: "json", name: string, value: unknown, options: { mode: "set" | "add" | "replace" }): Promise<any>; delete(kind: "json", name: string): Promise<boolean>; jsonApply(name: string, ops: { op: string; key: string; value?: unknown }[]): Promise<any>; list(): Promise<any> } };

const id = () => [...crypto.getRandomValues(new Uint8Array(4))].map(b => b.toString(16).padStart(2, "0")).join("");

export async function subAdd(bus: Bus, self: string, e: Message, channel: string, target: string, filter: string | undefined): Promise<string> {
  const problem = filterProblem(filter);
  if (problem) return `error: ${problem}`;
  const sub: Subscription = { id: "", channel, target, ...(filter !== undefined ? { filter } : {}), created_by: e.from, created_at: new Date().toISOString() };
  for (let attempt = 0; attempt < 3; attempt++) {
    sub.id = id();
    try {
      await (bus as unknown as { record: (n: string) => Store }).record(self).kv.set("json", `sub.${sub.id}`, sub, { mode: "add" });
      return JSON.stringify({ id: sub.id });
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      if (!message.includes("exists")) throw err; // a collision retries with a new id
    }
  }
  return "error: could not pick a free subscription id";
}

export async function subList(bus: Bus, self: string): Promise<string> {
  const store = (bus as unknown as { record: (n: string) => Store }).record(self);
  const listing = await store.kv.list();
  const names: string[] = [];
  for (const kind of Object.keys(listing ?? {})) {
    for (const entry of listing[kind] ?? []) if (typeof entry?.name === "string" && entry.name.startsWith("sub.")) names.push(entry.name);
  }
  names.sort();
  const rows: string[] = [];
  for (const name of names) {
    try {
      const sub = (await store.kv.get("json", name)).value;
      rows.push(`${sub.id} ${sub.channel} ${sub.target}${sub.filter ? ` ${sub.filter}` : ""} by=${sub.created_by} at=${sub.created_at}`);
    } catch { /* deleted between list and get */ }
  }
  return rows.join("\n") || "(no subscriptions)";
}

export async function subEdit(bus: Bus, self: string, subId: string, edits: { channel?: string; target?: string; filter?: string }): Promise<string> {
  if (edits.filter !== undefined) {
    const problem = filterProblem(edits.filter);
    if (problem) return `error: ${problem}`;
  }
  const store = (bus as unknown as { record: (n: string) => Store }).record(self);
  const name = `sub.${subId}`;
  try {
    await store.kv.get("json", name); // an edit of what is not there says so
  } catch {
    return `error: no subscription ${subId}`;
  }
  const ops: { op: string; key: string; value?: unknown }[] = [];
  if (edits.channel !== undefined) ops.push({ op: "set", key: "channel", value: edits.channel });
  if (edits.target !== undefined) ops.push({ op: "set", key: "target", value: edits.target });
  if (edits.filter !== undefined) ops.push({ op: "set", key: "filter", value: edits.filter });
  await store.kv.jsonApply(name, ops); // field-by-field: disjoint edits both land
  return "edited";
}

export async function subDelete(bus: Bus, self: string, subId: string): Promise<string> {
  const store = (bus as unknown as { record: (n: string) => Store }).record(self);
  const gone = await store.kv.delete("json", `sub.${subId}`);
  return gone ? "deleted" : `error: no subscription ${subId}`;
}
