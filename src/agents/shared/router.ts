import { randomUUID } from "node:crypto";
// Posted-message routes last seven days. An allocated session tag lasts while a route to its session
// does; a pinned tag stays. All of it is one KV value, kept under half the value limit.
import { own } from "./own.ts";
export type Config = { tags?: Record<string, string>; users?: Record<string, string>; route?: string; chats?: Record<string, { route?: string; tags?: boolean; anyone?: boolean; handler?: "drop" }> };
export type Incoming<Id = number> = { message_id: Id; answer_to?: Id; channel_post?: boolean; from_id?: Id; username?: string; name?: string; text?: string; reply_to?: Id; date: number };

export type Route = { to: string; topic?: string; tag?: string; re: string };
type StoredRoute = Route & { until: number };
type Refused = { id: string; username?: string; name?: string; chat: string; last_seen: string; count: number };
type AnswerRoute<Id> = { chat: string; message: Id; sender: string; topic: string; tag: string; until: number };
// gone: chat message keys whose route expired or was evicted, with the time, so a reply to one is
// answered rather than routed as a new message.
type State<Id> = { notices?: Record<string, number>; answers?: Record<string, AnswerRoute<Id>>; refused?: Refused[]; tags: Record<string, string>; messages: Record<string, StoredRoute>; gone?: Record<string, number> };
export type RouterDeps<Id = number> = {
  get: () => Promise<string | undefined>;
  set: (value: State<Id>) => Promise<void>;
  config: () => Promise<Config>;
  send: (route: Omit<Route, "re"> & { re?: string }, body: string, reply: { name: string; topic: string; tag: string }) => Promise<string>;
  tell: (chat: string, message: Id, text: string) => Promise<Id>;
  self: string;
  platform?: { label: string; name: string; topic: string };
  fallback: (message: Incoming<Id>, chat: string, title?: string) => Promise<boolean>;
  log: (line: string) => void;
};
const lifetime = 7 * 24 * 60 * 60 * 1000;
// One KV value holds at most 512 KiB (docs/03-records-kv.md#rules). The state stays under half of it,
// and the record of gone routes under an eighth of that, evicting the routes that expire first.
export const kvValueLimit = 512 * 1024;
export const stateBudget = kvValueLimit / 2;
const goneBudget = stateBudget / 8;
export const routeGone = "route expired or evicted";
const bytes = (value: unknown) => Buffer.byteLength(JSON.stringify(value));
const entryBytes = (key: string, value: unknown) => bytes(key) + bytes(value) + 2;
import { parseBusName, busName, agentName } from "./names.ts";
export { agentName } from "./names.ts";
export const shortTag = /^[a-z][a-z0-9]*$/;

// Telegram bounds message text by UTF-16 units. Keep surrogate pairs intact,
// and avoid flooding a chat even when the bus answer is much larger.
function answerChunks(text: string): string[] {
  const chunks: string[] = [];
  const marker = "… (truncated)";
  let offset = 0;
  while (offset < text.length && chunks.length < 8) {
    const truncate = chunks.length === 7 && text.length - offset > 4096;
    let end = Math.min(text.length, offset + 4096 - (truncate ? marker.length : 0));
    const last = text.charCodeAt(end - 1);
    if (end < text.length && last >= 0xd800 && last <= 0xdbff) end--;
    chunks.push(text.slice(offset, end) + (truncate ? marker : ""));
    offset = end;
  }
  return chunks;
}

export class Router<Id = number> {
  private state: State<Id> = { tags: {}, messages: {} };
  private pinned = new Set<string>();
  // Senders given a tag in the change being made: the caller is about to use it.
  private assigned = new Set<string>();
  private pending: Promise<unknown> = Promise.resolve();
  constructor(private readonly deps: RouterDeps<Id>) {}

  async load(): Promise<void> {
    const raw = await this.deps.get();
    if (raw === undefined) return;
    let state: State<Id>;
    try {
      state = Object.assign(Object.create(null), JSON.parse(raw)) as State<Id>;
      if (!state.tags || !state.messages || typeof state.tags !== "object" || typeof state.messages !== "object" || Array.isArray(state.tags) || Array.isArray(state.messages)) throw Error("malformed state");
    } catch {
      this.deps.log("reply routing: ignored malformed stored reply routes; starting with no stored routes");
      return;
    }
    const senders = new Set<string>();
    for (const [tag, sender] of Object.entries(state.tags)) {
      const name = parseBusName(sender);
      if (!shortTag.test(tag) || name === undefined || senders.has(name)) {
        delete state.tags[tag];
        this.deps.log(`reply routing: ignored malformed stored tag ${tag}`);
        continue;
      }
      state.tags[tag] = name;
      senders.add(name);
    }
    for (const [key, r] of Object.entries(state.messages)) {
      const name = parseBusName(r?.to);
      if (!r || name === undefined || typeof r.re !== "string" || !Number.isFinite(r.until)) {
        delete state.messages[key];
        this.deps.log(`reply routing: ignored malformed stored route ${key}`);
        continue;
      }
      r.to = name;
    }
    if (state.gone !== undefined && (typeof state.gone !== "object" || state.gone === null || Array.isArray(state.gone))) {
      delete state.gone;
      this.deps.log("reply routing: ignored malformed stored gone routes");
    }
    for (const [key, at] of Object.entries(state.gone ?? {})) if (!Number.isFinite(at)) delete state.gone![key];
    this.state = state;
  }

  // Bus calls and the poll loop may mutate state concurrently. Persist a
  // complete snapshot in order, publishing it in memory only after success.
  private change<T>(f: (next: State<Id>) => Promise<T>, landed = false): Promise<T> {
    const result = this.pending.then(async () => {
      const next = structuredClone(this.state);
      for (const [who, until] of Object.entries(next.notices ?? {})) if (until <= Date.now()) delete next.notices![who];
      const value = await f(next);
      this.trim(next);
      if (JSON.stringify(next) !== JSON.stringify(this.state)) {
        try { await this.deps.set(next); }
        catch (error) {
          if (!landed) throw error;
          this.deps.log("reply routing: could not persist a landed message route");
        }
      }
      this.state = next;
      return value;
    });
    this.pending = result.catch(() => {});
    return result;
  }

  // Drop expired routes, then evict the routes that expire first until the value fits its budget. A
  // removed chat route is remembered in gone; a session left with no route loses its allocated tag.
  private trim(next: State<Id>): void {
    const now = Date.now();
    const senders = new Set<string>();
    const drop = (kind: "messages" | "answers", key: string) => {
      if (kind === "messages") {
        senders.add(next.messages[key]!.to);
        delete next.messages[key];
        (next.gone ??= {})[key] = now;
      } else {
        senders.add(next.answers![key]!.sender);
        delete next.answers![key];
      }
    };
    for (const [key, r] of Object.entries(next.messages)) if (r.until <= now) drop("messages", key);
    for (const [key, r] of Object.entries(next.answers ?? {})) if (r.until <= now) drop("answers", key);
    let size = bytes(next);
    if (size > stateBudget) {
      const queue = [
        ...Object.entries(next.messages).map(([key, r]) => ({ kind: "messages" as const, key, until: r.until, size: entryBytes(key, r) })),
        ...Object.entries(next.answers ?? {}).map(([key, r]) => ({ kind: "answers" as const, key, until: r.until, size: entryBytes(key, r) })),
      ].sort((a, b) => a.until - b.until);
      let evicted = 0;
      for (const entry of queue) {
        // The running size is an estimate (a route removed adds its gone mark): measure before stopping.
        if (size <= stateBudget && (size = bytes(next)) <= stateBudget) break;
        drop(entry.kind, entry.key);
        size -= entry.size;
        evicted++;
      }
      this.deps.log(`reply routing: warning: evicted ${evicted} routes that expire first, to keep the stored routes under ${stateBudget} bytes`);
    }
    const gone = Object.entries(next.gone ?? {}).filter(([key, at]) => {
      if (at + lifetime > now) return true;
      delete next.gone![key];
      return false;
    }).sort((a, b) => a[1] - b[1]);
    let goneSize = gone.reduce((sum, [key, at]) => sum + entryBytes(key, at), 0);
    for (const [key, at] of gone) {
      if (goneSize <= goneBudget) break;
      delete next.gone![key];
      goneSize -= entryBytes(key, at);
    }
    if (next.gone && Object.keys(next.gone).length === 0) delete next.gone;
    for (const sender of this.assigned) senders.delete(sender);
    this.assigned.clear();
    if (senders.size === 0) return;
    for (const r of Object.values(next.messages)) senders.delete(r.to);
    for (const r of Object.values(next.answers ?? {})) senders.delete(r.sender);
    for (const [tag, sender] of Object.entries(next.tags)) if (senders.has(sender) && !this.pinned.has(tag)) delete next.tags[tag];
  }

  private pins(state: State<Id>, config: Config): void {
    this.pinned = new Set(Object.keys(config.tags ?? {}));
    for (const [tag, sender] of Object.entries(config.tags ?? {})) {
      const previous = Object.entries(state.tags).find(([, who]) => who === sender)?.[0];
      if ((Object.hasOwn(state.tags, tag) && state.tags[tag] !== sender) || (previous && previous !== tag)) throw Error(`tags: #${tag} conflicts with a permanent session tag`);
      state.tags[tag] = sender;
    }
  }

  private assign(next: State<Id>, sender: string, config: Config): string {
    this.pins(next, config);
    this.assigned.add(sender);
    const existing = Object.entries(next.tags).find(([, who]) => who === sender)?.[0];
    if (existing) return existing;
    const first = sender.replace(/^#/, "").match(/[a-z]/)?.[0] ?? "a";
    let tag = first, n = 1;
    while (Object.hasOwn(next.tags, tag)) tag = first + n++;
    next.tags[tag] = sender;
    return tag;
  }

  async tag(sender: string, config: Config): Promise<string> {
    return this.change(async next => this.assign(next, sender, config));
  }

  async posted(chat: string, message: Id, route: Route): Promise<void> {
    await this.change(async next => {
      next.messages[`${chat}:${message}`] = { ...route, until: Date.now() + lifetime };
      delete next.gone?.[`${chat}:${message}`];
    }, true);
  }

  /** A message a reply routes from: a live route, or one that expired or was evicted, which the reply
   *  is told about. */
  mapped(chat: string, message: Id): boolean {
    const key = `${chat}:${message}`;
    return !!own(this.state.messages, key) || own(this.state.gone, key) !== undefined;
  }

  refused(): Refused[] { return structuredClone(this.state.refused ?? []); }

  private async refuse(message: Incoming<Id>, chat: string, title?: string): Promise<boolean> {
    this.deps.log("reply routing: " + JSON.stringify({ user: String(message.from_id ?? "unknown"), username: message.username ? `@${message.username}` : "", name: message.name ?? "", chat, title: title ?? "", message: message.message_id, reason: "not a listed user", text: (message.text ?? "").slice(0, 40) }));
    return this.change(async next => {
      const id = String(message.from_id ?? "unknown");
      const old = next.refused?.find(sender => sender.id === id);
      next.notices ??= {};
      const notify = !Object.hasOwn(next.notices, id);
      if (notify) next.notices[id] = Date.now() + 3_600_000;
      next.refused = [...(next.refused ?? []).filter(sender => sender.id !== id), { id, username: message.username, name: message.name, chat, last_seen: new Date().toISOString(), count: (old?.count ?? 0) + 1 }].slice(-20);
      return notify;
    });
  }

  private async dispatch(route: Omit<Route, "re"> & { re?: string }, text: string, chat: string, message: Id): Promise<void> {
    await this.change(async next => {
      const reply = { name: this.deps.self, topic: this.deps.platform?.topic ?? "telegram-router", tag: randomUUID() };
      const id = await this.deps.send(route, text, reply);
      next.answers ??= {};
      next.answers[id] = { chat, message, sender: route.to, topic: reply.topic, tag: reply.tag, until: Date.now() + lifetime };
    }, true);
  }

  /** Wait for an in-flight dispatch to publish its mapping before classifying
   *  a very fast MCP answer. Matching the correlation is opt-in at serve(). */
  async matchesReply(message: { re?: string; topic?: string; tag?: string }): Promise<boolean> {
    await this.pending;
    return Object.values(this.state.answers ?? {}).some(route => route.until > Date.now() && route.topic === message.topic && route.tag === message.tag);
  }

  async answer(message: { message_id: string; re?: string; from: string; topic?: string; tag?: string; body: string }): Promise<void> {
    // Dispatch publishes the bus-id mapping before this queued callback can
    // inspect it, including when a very fast session answers immediately.
    await this.change(async next => {
      const route = message.re ? own(next.answers, message.re) :
        Object.values(next.answers ?? {}).find(candidate => candidate.sender === message.from && candidate.topic === message.topic && candidate.tag === message.tag);
      if (message.re && (!route || route.until <= Date.now())) {
        // The dispatch it answers expired or was evicted: say so, never drop it silently.
        this.deps.log(`reply routing: warning: ${routeGone}: session reply ${message.message_id} from ${message.from}`);
        if (message.body.startsWith("error: ")) return;
        try {
          await this.deps.send({ to: message.from, topic: message.topic, tag: message.tag },
            `error: ${routeGone}; your answer was not posted to the chat`, { name: this.deps.self, topic: message.topic ?? "", tag: message.tag ?? "" });
        } catch { this.deps.log("reply routing: could not notify the session of an expired or evicted route"); }
        return;
      }
      if (!route || route.until <= Date.now() || route.sender !== message.from || route.topic !== message.topic || route.tag !== message.tag) {
        this.deps.log(`reply routing: dropped unmatched session reply ${message.message_id} from ${message.from}`);
        return;
      }
      const tag = this.assign(next, message.from, await this.deps.config());
      const text = `${message.body}\n\n#${tag}`;
      for (const chunk of answerChunks(text)) {
        let mid: Id;
        try { mid = await this.deps.tell(route.chat, route.message, chunk); }
        catch {
          this.deps.log("reply routing: could not post a session answer");
          try {
            await this.deps.send({ to: message.from, topic: message.topic, tag: message.tag },
              "error: could not post your answer to the chat", { name: this.deps.self, topic: message.topic ?? "", tag: message.tag ?? "" });
          } catch { this.deps.log("reply routing: could not notify the session of a posting failure"); }
          return;
        }
        next.messages[`${route.chat}:${mid}`] = { to: message.from, topic: message.topic, tag: message.tag, re: message.message_id, until: Date.now() + lifetime };
      }
    }, true);
  }

  async incoming(message: Incoming<Id>, chat: string, title?: string, snapshot?: Config): Promise<void | false> {
    const config = snapshot ?? await this.deps.config();
    const chatConfig = own(config.chats, chat);
    if (chatConfig?.handler === "drop") return false;
    await this.change(async next => { this.pins(next, config); });
    // Channel posts are Telegram-admin-authorized announcements, not a
    // human identity. Do not echo our own known channel publications.
    if (message.channel_post && own(this.state.messages, `${chat}:${message.message_id}`)) return;
    const listed = own(config.users, String(message.from_id));
    const channelAdmin = message.channel_post === true && chatConfig?.route !== undefined;
    const anyoneOnly = !listed;
    const user = listed ??
      (channelAdmin ? `channel ${chat}` :
      chatConfig?.anyone === true ? `${this.deps.platform?.name ?? "Telegram"} user ${message.from_id ?? "unknown"}` : undefined);
    if (!user) {
      if (await this.refuse(message, chat, title)) await this.deps.tell(chat, message.answer_to ?? message.message_id, "we do not know you");
      return;
    }
    let route: (Omit<Route, "re"> & { re?: string }) | undefined;
    let text = message.text ?? "";
    if (message.reply_to !== undefined) {
      const key = `${chat}:${message.reply_to}`;
      const stored = own(this.state.messages, key);
      if ((stored && stored.until <= Date.now()) || (!stored && own(this.state.gone, key) !== undefined)) {
        this.deps.log(`reply routing: warning: ${routeGone}: a reply to ${key}`);
        await this.deps.tell(chat, message.answer_to ?? message.message_id, `${routeGone}; use #tag`);
        return;
      }
      if (stored) route = { to: stored.to, topic: stored.topic, tag: stored.tag, re: stored.re };
    }
    if (!route && !anyoneOnly && chatConfig?.tags !== false) {
      const match = /^(#[^\s]+)(?:\s+([\s\S]*))?$/.exec(text);
      if (match) {
        const key = match[1]!.slice(1);
        const to = own(this.state.tags, key) ?? (agentName.test(match[1]!) ? parseBusName(match[1]!) : undefined);
        if (to) { route = { to }; text = match[2] ?? ""; }
      }
    }
    if (route) {
      await this.dispatch(route, `[${user} (${this.deps.platform?.label ?? "TG"})] ${text}`, chat, message.answer_to ?? message.message_id);
      return;
    }
    if (!anyoneOnly && await this.deps.fallback(message, chat, title)) return;
    const target = chatConfig?.route ?? (anyoneOnly ? undefined : config.route);
    if (target) {
      await this.dispatch({ to: target }, `[${user} (${this.deps.platform?.label ?? "TG"})] ${text}`, chat, message.answer_to ?? message.message_id);
      return;
    }
    if (anyoneOnly) {
      await this.deps.tell(chat, message.answer_to ?? message.message_id, "I did not understand where to send this. This chat has no route; reply to a session post here.");
      return;
    }
    const tags = Object.keys(this.state.tags).sort().map(tag => `#${tag}`).join(", ") || "(none yet)";
    await this.deps.tell(chat, message.answer_to ?? message.message_id, `I did not understand where to send this. Use a Reply or one of: ${tags}`);
  }
}
