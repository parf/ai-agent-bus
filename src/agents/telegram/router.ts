import { randomUUID } from "node:crypto";
// Session tags never expire; individual posted-message routes last seven days.
import type { Config } from "./config.ts";
import type { SeenMessage } from "./seen.ts";

export type Route = { to: string; topic?: string; tag?: string; re: string };
type StoredRoute = Route & { until: number };
type Refused = { id: string; username?: string; name?: string; chat: string; last_seen: string; count: number };
type AnswerRoute = { chat: string; message: number; sender: string; topic: string; tag: string; until: number };
type State = { notices?: Record<string, number>; answers?: Record<string, AnswerRoute>; refused?: Refused[]; tags: Record<string, string>; messages: Record<string, StoredRoute> };
export type RouterDeps = {
  get: () => Promise<string | undefined>;
  set: (value: State) => Promise<void>;
  config: () => Promise<Config>;
  send: (route: Omit<Route, "re"> & { re?: string }, body: string, reply: { name: string; topic: string; tag: string }) => Promise<string>;
  tell: (chat: string, message: number, text: string) => Promise<number>;
  self: string;
  fallback: (message: SeenMessage, chat: string, title?: string) => Promise<boolean>;
  log: (line: string) => void;
};
const lifetime = 7 * 24 * 60 * 60 * 1000;
const session = /^#[a-z0-9][a-z0-9._/-]*@[a-z0-9][a-z0-9.-]*$/;
const busName = /^(?:#[a-z0-9][a-z0-9._/-]*@[a-z0-9][a-z0-9.-]*|[a-z0-9][a-z0-9._-]*(?:@[a-z0-9][a-z0-9.-]*)?)$/;
export const shortTag = /^[a-z][a-z0-9]*$/;
export const agentName = session;

export class Router {
  private state: State = { tags: {}, messages: {} };
  private pending: Promise<unknown> = Promise.resolve();
  constructor(private readonly deps: RouterDeps) {}

  async load(): Promise<void> {
    const raw = await this.deps.get();
    if (raw === undefined) return;
    const state = JSON.parse(raw) as State;
    if (!state || !state.tags || !state.messages || Array.isArray(state.tags) || Array.isArray(state.messages)) throw Error("stored reply routes are malformed");
    const senders = new Set<string>();
    for (const [tag, sender] of Object.entries(state.tags)) {
      if (!shortTag.test(tag) || typeof sender !== "string" || !busName.test(sender) || senders.has(sender)) throw Error("stored reply tags are malformed");
      senders.add(sender);
    }
    for (const r of Object.values(state.messages)) {
      if (!r || !busName.test(r.to) || typeof r.re !== "string" || !Number.isFinite(r.until)) throw Error("stored reply mapping is malformed");
    }
    this.state = state;
  }

  // Bus calls and the poll loop may mutate state concurrently. Persist a
  // complete snapshot in order, publishing it in memory only after success.
  private change<T>(f: (next: State) => Promise<T>): Promise<T> {
    const result = this.pending.then(async () => {
      const next = structuredClone(this.state);
      for (const [who, until] of Object.entries(next.notices ?? {})) if (until <= Date.now()) delete next.notices![who];
      const value = await f(next);
      if (JSON.stringify(next) !== JSON.stringify(this.state)) await this.deps.set(next);
      this.state = next;
      return value;
    });
    this.pending = result.catch(() => {});
    return result;
  }

  private pins(state: State, config: Config): void {
    for (const [tag, sender] of Object.entries(config.tags ?? {})) {
      const previous = Object.entries(state.tags).find(([, who]) => who === sender)?.[0];
      if ((Object.hasOwn(state.tags, tag) && state.tags[tag] !== sender) || (previous && previous !== tag)) throw Error(`tags: #${tag} conflicts with a permanent session tag`);
      state.tags[tag] = sender;
    }
  }

  private assign(next: State, sender: string, config: Config): string {
    this.pins(next, config);
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

  async posted(chat: string, message: number, route: Route): Promise<void> {
    await this.change(async next => {
      for (const [key, old] of Object.entries(next.messages)) if (old.until <= Date.now()) delete next.messages[key];
      next.messages[`${chat}:${message}`] = { ...route, until: Date.now() + lifetime };
    });
  }

  refused(): Refused[] { return structuredClone(this.state.refused ?? []); }

  private async refuse(message: SeenMessage, chat: string, title?: string): Promise<boolean> {
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

  private async dispatch(route: Omit<Route, "re"> & { re?: string }, text: string, chat: string, message: number): Promise<void> {
    await this.change(async next => {
      const reply = { name: this.deps.self, topic: "telegram-router", tag: randomUUID() };
      const id = await this.deps.send(route, text, reply);
      next.answers ??= {};
      for (const [key, old] of Object.entries(next.answers)) if (old.until <= Date.now()) delete next.answers[key];
      next.answers[id] = { chat, message, sender: route.to, topic: reply.topic, tag: reply.tag, until: Date.now() + lifetime };
    });
  }

  async answer(message: { message_id: string; re?: string; from: string; topic?: string; tag?: string; body: string }): Promise<void> {
    // Dispatch publishes the bus-id mapping before this queued callback can
    // inspect it, including when a very fast session answers immediately.
    await this.change(async next => {
      const route = message.re ? next.answers?.[message.re] : undefined;
      if (!route || route.until <= Date.now() || route.sender !== message.from || route.topic !== message.topic || route.tag !== message.tag) {
        this.deps.log(`reply routing: dropped unmatched session reply ${message.message_id} from ${message.from}`);
        return;
      }
      const tag = this.assign(next, message.from, await this.deps.config());
      const text = `${message.body}\n\n#${tag}`;
      const mid = await this.deps.tell(route.chat, route.message, text);
      next.messages[`${route.chat}:${mid}`] = { to: message.from, topic: message.topic, tag: message.tag, re: message.message_id, until: Date.now() + lifetime };
    });
  }

  async incoming(message: SeenMessage, chat: string, title?: string): Promise<void> {
    const config = await this.deps.config();
    await this.change(async next => { this.pins(next, config); });
    // Only listed Telegram users may turn incoming text into bus traffic.
    const user = config.users?.[String(message.from_id)];
    if (!user) {
      if (await this.refuse(message, chat, title)) await this.deps.tell(chat, message.message_id, "we do not know you");
      return;
    }
    let route: (Omit<Route, "re"> & { re?: string }) | undefined;
    let text = message.text ?? "";
    if (message.reply_to !== undefined) {
      const stored = this.state.messages[`${chat}:${message.reply_to}`];
      if (!stored || stored.until <= Date.now()) {
        await this.deps.tell(chat, message.message_id, "that thread expired; use #tag");
        return;
      }
      route = { to: stored.to, topic: stored.topic, tag: stored.tag, re: stored.re };
    } else {
      const match = /^(#[^\s]+)(?:\s+([\s\S]*))?$/.exec(text);
      if (match) {
        const key = match[1]!.slice(1);
        const to = session.test(match[1]!) ? match[1]! : Object.hasOwn(this.state.tags, key) ? this.state.tags[key] : undefined;
        if (to) { route = { to }; text = match[2] ?? ""; }
      }
    }
    if (route) {
      await this.dispatch(route, `[${user} (TG)] ${text}`, chat, message.message_id);
      return;
    }
    if (await this.deps.fallback(message, chat, title)) return;
    if (config.route) {
      await this.dispatch({ to: config.route }, `[${user} (TG)] ${text}`, chat, message.message_id);
      return;
    }
    const tags = Object.keys(this.state.tags).sort().map(tag => `#${tag}`).join(", ") || "(none yet)";
    await this.deps.tell(chat, message.message_id, `I did not understand where to send this. Use a Reply or one of: ${tags}`);
  }
}
