// What the bot has seen, in memory, flushed to the agent's own KV: the
// newest messages per chat for `read` (200 entries and 64 KiB UTF-8 JSON),
// and the chat and sender
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
const RING_BYTES = 64 * 1024;

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
        const ring = this.bounded(value as SeenMessage[]);
        this.rings.set(chat, ring);
        this.ids.set(chat, new Set(ring.map(m => m.message_id)));
      } else if (key === "chats" && Array.isArray(value)) {
        for (const c of value as { id: string; title?: string; type: string }[]) this.chats.set(c.id, c);
      } else if (key === "users" && Array.isArray(value)) {
        for (const u of value as { id: string; username?: string; first_name?: string }[]) this.users.set(String(u.id), { ...u, id: String(u.id) });
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

  /** Failed keys stay dirty; the receiver can report and skip a history-key failure. */
  async flush(write: (key: string, value: unknown) => Promise<void>, full = false, historyError?: (line: string) => void): Promise<void> {
    const values = full ? this.flushAll() : this.flushDirty();
    for (const [key, value] of Object.entries(values)) {
      try { await write(key, value); this.dirty.delete(key); }
      catch (error) {
        if (!historyError || !key.startsWith("seen:")) throw error;
        historyError(`seen history write failed for ${key}; retaining it for retry and advancing the update offset`);
      }
    }
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
        this.users.set(key, { id: key, first_name: m.from, ...(m.username ? { username: m.username } : {}) });
        this.dirty.add("users");
        this.evictOldest(this.users);
      }
    }
    let ring = this.rings.get(chatId) ?? [];
    const ids = this.ids.get(chatId) ?? new Set<number>();
    // A message already in the ring is a re-delivery (a batch re-seen
    // after a failed flush) and is absorbed once — by id, not by tail: a
    // re-delivered batch repeats every message, not just the last.
    if (ids.has(m.message_id)) return false;
    ids.add(m.message_id);
    this.ids.set(chatId, ids);
    ring.push(m);
    ring = this.bounded(ring);
    this.ids.set(chatId, new Set(ring.map(item => item.message_id)));
    this.rings.set(chatId, ring);
    this.dirty.add(`seen:${chatId}`);
    return true;
  }

  /** Keep the newest contiguous history under both the count and UTF-8 KV bounds. */
  private bounded(ring: SeenMessage[]): SeenMessage[] {
    let start = ring.length, bytes = 2;
    while (start > 0 && ring.length - start < this.cap) {
      const size = Buffer.byteLength(JSON.stringify(ring[start - 1])) + (start < ring.length ? 1 : 0);
      if (bytes + size > RING_BYTES) break;
      bytes += size;
      start--;
    }
    return ring.slice(start);
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
