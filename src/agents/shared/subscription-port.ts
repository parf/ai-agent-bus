// The chat agents supply KV/transport and selectors; all subscription policy
// and delivery accounting live in the shared TypeScript engine.
import { Bus, BusError, KVNotFound } from "../../clients/typescript/src/index.js";
import { Subscriptions, SubscriptionError, type Caller, type Port, type Row, type WarningCode } from "./subscriptions.ts";
import { migrateSubscriptions, validAuthors } from "./subscription-migration.ts";

export const staff = (e: Caller, source: string) => (!e.original_to || e.original_to.toLowerCase() === source.toLowerCase()) && e.roles.some(r => r === "owner" || r === "maintainer");

export function validateSelector(selector: Record<string, unknown>): void {
  const keys = Object.keys(selector).sort().join();
  if (keys !== "channel" && keys !== "channel,from_users") throw Error("invalid selector");
  if (typeof selector.channel !== "string" || !selector.channel || selector.channel.length > 256 || /[\x00-\x20\x7f]/.test(selector.channel)) throw Error("invalid channel");
  if (selector.from_users !== undefined && !validAuthors(selector.from_users)) throw Error("invalid authors");
}

export class ChatPort implements Port {
  deadline = Infinity;
  constructor(readonly bus: Bus, readonly source: string) {}
  operation() {
    const timeout = Math.min(1000, this.deadline - Date.now());
    if (timeout <= 0) throw new SubscriptionError("admission_unavailable");
    return { timeout };
  }
  async admits(who: string, target: string) {
    try { return await this.bus.maySend(who, target, this.operation()) ? "yes" as const : "no" as const; }
    catch (e) { return e instanceof BusError && e.status === 404 ? "no" as const : "unavailable" as const; }
  }
  async kind(target: string) {
    try { return (await this.bus.request("GET", "/lookup", undefined, { ...this.operation(), query: { name: target } })).kind; }
    catch { return undefined; }
  }
  async load(): Promise<Map<string, unknown>> {
    const kv = this.bus.record(this.source).kv;
    await migrateSubscriptions(kv, this.operation.bind(this));
    const listing = await kv.list(this.operation());
    const rows = new Map<string, unknown>();
    for (const item of listing?.json ?? []) {
      if (typeof item.name !== "string" || !item.name.startsWith("sub.")) continue;
      try { rows.set(item.name, (await kv.get("json", item.name, this.operation())).value); }
      catch (e) { if (!(e instanceof KVNotFound)) throw e; }
    }
    return rows;
  }
  async create(key: string, row: Row) {
    try { await this.bus.record(this.source).kv.set("json", key, row as any, { ...this.operation(), mode: "add" }); return true; }
    catch (e) { if (e instanceof BusError && e.status === 412) return false; throw e; }
  }
  remove(key: string) { return this.bus.record(this.source).kv.delete("json", key, this.operation()); }
  async counts(key: string, sent: number, dropped: number) {
    const kv = this.bus.record(this.source).kv;
    // All local subscription operations share ChatSubscriptions' turn. Do not
    // create a row removed by an external operator between load and counters.
    await kv.get("json", key, this.operation());
    await kv.jsonApply(key, [{ op: "set", key: "sent", value: sent }, { op: "set", key: "dropped", value: dropped }], this.operation());
  }
  async send(target: string, body: string, options: { topic?: string; tag?: string; deadline?: number }) {
    try {
      // No wait: this is a forward, not a call expecting a reply. The real
      // daemon refuses an explicit zero wait too (B.29's adapter regression).
      await this.bus.send(target, body, { ...this.operation(), topic: options.topic, tag: options.tag });
      return "ok" as const;
    } catch (e) {
      return e instanceof BusError && e.status >= 400 && e.status < 500 ? "refused" as const : "unknown" as const;
    }
  }
}

export type ChatOptions = {
  resolve: (channel: string) => Promise<string | undefined>;
  warning: (code: WarningCode, id?: string) => void;
};
export class ChatSubscriptions {
  readonly engine: Subscriptions;
  private tail: Promise<unknown> = Promise.resolve();
  constructor(readonly port: ChatPort, private options: ChatOptions) {
    this.engine = new Subscriptions(port, port.source, {
      validateSelector,
      selectorMatches: (selector, event) => Array.isArray(event.channels) && event.channels.includes(selector.channel) &&
        (!Array.isArray(selector.from_users) || selector.from_users.length === 0 || selector.from_users.includes(event.author)),
      warning: options.warning,
    });
  }
  // Socket events and bus calls can overlap. Serialize the engine's full
  // load/operation/counter cycle so delete cannot race a counter write.
  private run<T>(fn: () => Promise<T> | T, deadline = Date.now() + 15_000): Promise<T> {
    const next = this.tail.then(async () => {
      this.port.deadline = deadline;
      await this.engine.reload(); // migration completes before anything serves
      return fn();
    });
    this.tail = next.catch(() => {});
    return next;
  }
  ready() { return this.run(() => undefined); }
  add(caller: Caller, channel: string, target: string, filter = "", from_users?: string[]) {
    return this.run(() => this.engine.add(caller, target, { channel, ...(from_users?.length ? { from_users } : {}) }, filter));
  }
  list(caller: Caller) { return this.run(() => this.engine.list(caller)); }
  delete(caller: Caller, id: string) { return this.run(() => this.engine.delete(caller, id)); }
  deliver(channel: string, author: string, text: string, body: string, topic: string) {
    return this.run(async () => {
      // Internal enumeration supplies selector resolution, never user-facing
      // visibility. A user's list always uses their authenticated caller.
      const rows = this.engine.list({ from: this.port.source, roles: ["owner"] });
      const channels: string[] = [];
      for (const selector of new Set(rows.flatMap(r => "selector" in r ? [String(r.selector.channel)] : []))) {
        try { if (await this.options.resolve(selector) === channel) channels.push(selector); }
        catch { /* one unresolved provider selector cannot stop the others */ }
      }
      return this.engine.deliver({ channels, author }, text, body, { topic, tag: channel, deadline: this.port.deadline });
    });
  }
  close() { this.engine.close(); }
}

export function subscriptionList(rows: Awaited<ReturnType<ChatSubscriptions["list"]>>) {
  const text = rows.map(r => "selector" in r
    ? `${r.id} ${r.selector.channel} ${r.target}${Array.isArray(r.selector.from_users) && r.selector.from_users.length ? ` from=${r.selector.from_users.join(",")}` : ""} filter=${JSON.stringify(r.filter)} by=${r.created_by} at=${r.created_at} sent=${r.sent} dropped=${r.dropped}${r.reason ? ` reason=${r.reason}` : ""}`
    : `${r.id} reason=${r.reason}`).join("\n") || "(no subscriptions)";
  return { ok: true as const, text, fields: { subs: rows } };
}
