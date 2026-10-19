// What the bot has seen, in memory, flushed to the agent's own KV: the
// last 200 messages per chat for `read`, and the chat and sender
// directories for `channels` and `users`, each bounded (tg5 review P3-2).
// Writes are dirty-key per batch (the tg5 review P3-1), with a full flush
// on the minute; the KV value is the ring as it stands, so a restart reads
// one consistent copy.

export type SeenMessage = {
  message_id: number;
  channel_post?: boolean;
  from_id?: number;
  from?: string;
  username?: string;
  name?: string;
  text?: string;
  reply_to?: number;
  date: number;
};

const DIR_LIMIT = 500;

export class Seen {
  // The ring per chat, keyed by the chat's id as a decimal string.
  private rings = new Map<string, SeenMessage[]>();
  private ids = new Map<string, Set<number>>();
  private chats = new Map<string, { id: string; title?: string; type: string }>();
  private users = new Map<string, { id: string; username?: string; first_name?: string }>();
  private readonly cap = 200;
  // The keys that changed since the last flush: the batch writes only
  // these; the minute flush writes everything.
  private dirty = new Set<string>();

  constructor(private readonly load?: { kv: Record<string, unknown> }) {
    if (load) this.restore(load.kv);
  }

  restore(kv: Record<string, unknown>): void {
    for (const [key, value] of Object.entries(kv)) {
      if (key.startsWith("seen:") && Array.isArray(value)) {
        const chat = key.slice(5);
        const ring = value as SeenMessage[];
        this.rings.set(chat, ring);
        this.ids.set(chat, new Set(ring.map(m => m.message_id)));
      } else if (key === "chats" && Array.isArray(value)) {
        for (const c of value as { id: string; title?: string; type: string }[]) this.chats.set(c.id, c);
      } else if (key === "users" && Array.isArray(value)) {
        for (const u of value as { id: string; username?: string; first_name?: string }[]) this.users.set(u.id, u);
      }
    }
  }

  /** The writes for the keys that changed since the last flush. */
  flushDirty(): Record<string, unknown> {
    const out: Record<string, unknown> = {};
    for (const key of this.dirty) {
      if (key === "chats") out.chats = [...this.chats.values()];
      else if (key === "users") out.users = [...this.users.values()];
      else if (key.startsWith("seen:")) out[key] = this.rings.get(key.slice(5)) ?? [];
    }
    return out;
  }

  /** The whole state, for the minute flush. */
  flushAll(): Record<string, unknown> {
    const out: Record<string, unknown> = { chats: [...this.chats.values()], users: [...this.users.values()] };
    for (const [chat, ring] of this.rings) out[`seen:${chat}`] = ring;
    return out;
  }

  /** Keep the batch dirty until every idempotent KV write succeeds. */
  async flush(write: (key: string, value: unknown) => Promise<void>, full = false): Promise<void> {
    const values = full ? this.flushAll() : this.flushDirty();
    for (const [key, value] of Object.entries(values)) await write(key, value);
    for (const key of Object.keys(values)) this.dirty.delete(key);
  }

  has(chatId: string, messageId: number): boolean { return this.ids.get(chatId)?.has(messageId) ?? false; }

  /** One received message: into its chat's ring and the sender directory.
   *  A message whose id is already in the ring is a re-delivery and is
   *  absorbed once (tg5 review P2-2). */
  message(chatId: string, chatTitle: string | undefined, chatType: string, m: SeenMessage): boolean {
    if (!this.chats.has(chatId)) {
      this.chats.set(chatId, { id: chatId, title: chatTitle, type: chatType });
      this.dirty.add("chats");
      this.evictOldest(this.chats);
    }
    if (m.from_id !== undefined) {
      const key = String(m.from_id);
      if (!this.users.has(key) && m.from) {
        this.users.set(key, { id: m.from_id, first_name: m.from });
        this.dirty.add("users");
        this.evictOldest(this.users);
      }
    }
    const ring = this.rings.get(chatId) ?? [];
    const ids = this.ids.get(chatId) ?? new Set<number>();
    // A message already in the ring is a re-delivery (a batch re-seen
    // after a failed flush) and is absorbed once — by id, not by tail: a
    // re-delivered batch repeats every message, not just the last.
    if (ids.has(m.message_id)) return false;
    ids.add(m.message_id);
    this.ids.set(chatId, ids);
    ring.push(m);
    if (ring.length > this.cap) {
      ring.splice(0, ring.length - this.cap);
      // The id set follows the ring's bound: the evicted ids may be seen
      // again if Telegram ever re-delivers them.
      const live = new Set(ring.map(x => x.message_id));
      this.ids.set(chatId, live);
    }
    this.rings.set(chatId, ring);
    this.dirty.add(`seen:${chatId}`);
    return true;
  }

  /** The last N messages of a chat, oldest first; N beyond the ring gives
   *  the whole ring. */
  read(chatId: string, n: number): SeenMessage[] {
    const ring = this.rings.get(chatId) ?? [];
    return ring.slice(Math.max(0, ring.length - n));
  }

  /** The chats the bot has seen, with their id, title and type. */
  chatList(): { id: string; title?: string; type: string }[] {
    return [...this.chats.values()];
  }

  /** The senders the bot has seen. */
  userList(): { id: string; username?: string; first_name?: string }[] {
    return [...this.users.values()];
  }

  /** The directories are bounded: beyond the limit, the oldest entry is
   *  dropped (tg5 review P3-2). */
  private evictOldest(map: Map<string, unknown>): void {
    while (map.size > DIR_LIMIT) {
      const oldest = map.keys().next().value;
      if (oldest === undefined) return;
      map.delete(oldest);
    }
  }
}
