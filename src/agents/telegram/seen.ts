// What the bot has seen, in memory, flushed to the agent's own KV once a
// minute (Plans/R1.0-Release/telegram/README.md § receiving): the last 200
// messages per chat for `read`, and every chat and sender the bot has seen
// for `channels` and `users`. A flush is whole-for-the-key: the KV value
// is the ring as it stands, so a restart reads one consistent copy.

import type { Config } from "./config.ts";

export type SeenMessage = {
  message_id: number;
  from_id?: number;
  from?: string;
  text?: string;
  reply_to?: number;
  date: number;
};

export class Seen {
  // The ring per chat, keyed by the chat's id as a decimal string.
  private rings = new Map<string, SeenMessage[]>();
  private chats = new Map<string, { id: string; title?: string; type: string }>();
  private users = new Map<string, { id: string; username?: string; first_name?: string }>();
  private readonly cap = 200;

  constructor(private readonly load?: { kv: Record<string, unknown> }) {
    if (load) this.restore(load.kv);
  }

  private restore(kv: Record<string, unknown>): void {
    for (const [key, value] of Object.entries(kv)) {
      if (key.startsWith("seen:") && Array.isArray(value)) {
        this.rings.set(key.slice(5), value as SeenMessage[]);
      } else if (key === "chats" && Array.isArray(value)) {
        for (const c of value as { id: string; title?: string; type: string }[]) this.chats.set(c.id, c);
      } else if (key === "users" && Array.isArray(value)) {
        for (const u of value as { id: string; username?: string; first_name?: string }[]) this.users.set(u.id, u);
      }
    }
  }

  /** The KV writes for the current state: one value per chat ring, plus the
   *  chat and sender directories. Written after each batch, not per message. */
  flush(): Record<string, unknown> {
    const out: Record<string, unknown> = { chats: [...this.chats.values()], users: [...this.users.values()] };
    for (const [chat, ring] of this.rings) out[`seen:${chat}`] = ring;
    return out;
  }

  /** One received message: into its chat's ring and the sender directory. */
  message(chatId: string, chatTitle: string | undefined, chatType: string, m: SeenMessage): void {
    if (!this.chats.has(chatId)) this.chats.set(chatId, { id: chatId, title: chatTitle, type: chatType });
    if (m.from_id !== undefined) {
      const key = String(m.from_id);
      if (!this.users.has(key) && m.from) this.users.set(key, { id: m.from_id, first_name: m.from });
    }
    const ring = this.rings.get(chatId) ?? [];
    ring.push(m);
    if (ring.length > this.cap) ring.splice(0, ring.length - this.cap);
    this.rings.set(chatId, ring);
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
}

/** The destination a chat name resolves to, for the store's keys: an
 *  aliased or numeric chat as the configuration already spells it. */
export function chatKey(chatId: string): string {
  return chatId;
}

export type { Config };
