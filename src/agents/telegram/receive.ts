// Receiving (Plans/R1.0-Release/telegram/README.md § receiving): getUpdates
// long polling, 30 s; the offset in the agent's own KV, written after handling the batch. History persistence is best effort per chat; a
// failed history write stays dirty and cannot block other chats.
// A failed poll — a 409 (another poller or a webhook on the token), a 429
// after its retry, a malformed answer, a 5xx — is logged with its
// description and retried after a wait (a 409 longer); the last poll error
// is kept for whoami to name (tg5 review P2-1). The store's batch is
// flushed before the offset is written; failed chat-history writes are
// logged and retried on a later flush, while the offset can advance.

import { Seen, type SeenMessage } from "./seen.ts";

type Update = {
  update_id: number;
  message?: {
    message_id: number;
    date: number;
    text?: string;
    from?: { id: number; username?: string; first_name?: string; last_name?: string };
    chat?: { id: number; title?: string; type: string };
    reply_to_message?: { message_id: number };
  };
  channel_post?: Update["message"];
  edited_message?: unknown;
};

export class Receiver {
  private offset = 0;
  private stopping = false;
  private wake?: () => void;
  private lastPollError = "";
  /** Set by the agent: every stored message goes to the matching
   *  subscriptions (TG.6). The bot's own id, when known, keeps the bot's
   *  own posts out. */
  // Returning false discards the message before any seen history is recorded.
  onMessage?: (m: SeenMessage, chatId: string, chatTitle?: string) => void | false | Promise<void | false>;
  botId?: string;
  private attempts = new Map<string, number>();

  constructor(
    private readonly seen: Seen,
    private readonly web: (kind: "read" | "write", method: string, params: Record<string, unknown>) => Promise<any>,
    private readonly kvGet: (key: string) => Promise<string | undefined>,
    private readonly kvSet: (key: string, value: unknown) => Promise<void>,
    private readonly log: (line: string) => void,
  ) {}

  /** The last poll failure's description, for whoami to name (a 409 means
   *  another poller or a webhook holds the token). */
  pollError(): string {
    return this.lastPollError;
  }

  /** Restore the offset and seen history together, retrying unavailable KV
   *  reads before polling or serving calls. */
  async load(): Promise<void> {
    for (let attempt = 1; attempt <= 3; attempt++) {
      try {
        const stored = await this.kvGet("offset");
        const state: Record<string, unknown> = {};
        for (const key of ["chats", "users"]) state[key] = await this.storedArray(key);
        for (const chat of state.chats as { id: string }[]) {
          const key = `seen:${chat.id}`;
          state[key] = await this.storedArray(key);
        }
        this.seen.restore(state);
        this.offset = Number(stored) || 0;
        return;
      } catch (e) {
        const wait = attempt * 1000;
        this.log(`the receive state read failed (attempt ${attempt}): ${e instanceof Error ? e.message.slice(0, 100) : "unknown"}`);
        if (attempt < 3) await Bun.sleep(wait);
      }
    }
    throw new Error("the receive state could not be read from KV");
  }

  private async storedArray(key: string): Promise<any[]> {
    const raw = await this.kvGet(key);
    if (raw === undefined) return [];
    let value: unknown;
    try { value = JSON.parse(raw); } catch {
      this.log(`stored ${key} is not JSON; starting it empty`);
      return [];
    }
    if (!Array.isArray(value)) {
      this.log(`stored ${key} is not an array; starting it empty`);
      return [];
    }
    return value;
  }

  start(): void {
    void (async () => {
      let fullSince = Date.now();
      while (!this.stopping) {
        try {
          const r = await this.web("read", "getUpdates", { offset: this.offset, timeout: 30 });
          if (r.ok !== true) {
            // A 409 means another poller or a webhook holds the token: back
            // off for a long while. Anything else: a short wait. The
            // description is kept for whoami either way (tg5 review P2-1).
            this.lastPollError = r.description ?? r.error ?? "unknown answer";
            this.log(`getUpdates refused: ${this.lastPollError}`);
            await this.pause(url409(r) ? 30_000 : 5_000);
            continue;
          }
          this.lastPollError = "";
          // A getMe-shaped answer (a result that is no array) is treated as
          // no updates: the poll asks Telegram, which always answers an array.
          const updates = (Array.isArray(r.result) ? r.result : []) as Update[];
          if (updates.length === 0) {
            // The real long poll holds; a fast fake does not — a floor
            // against hot-spinning.
            await this.pause(1000);
            continue;
          }
          const nextOffset = updates[updates.length - 1]!.update_id + 1;
          for (const u of updates) await this.absorb(u);
          // Persist history first, retaining any failed chat key for retry.
          // A broken history key must not stop every chat's polling.
          await this.seen.flush(this.kvSet, false, this.log);
          await this.writeOffset(nextOffset);
          this.offset = nextOffset;
          // The full flush once a minute (tg5 review P3-1): the
          // directories and any ring that the dirty writes missed.
          if (Date.now() - fullSince >= 60_000) {
            await this.seen.flush(this.kvSet, true, this.log);
            fullSince = Date.now();
          }
        } catch (e) {
          if (this.stopping) return; // intentional poll cancellation is not a poll error
          this.lastPollError = e instanceof Error ? e.message.slice(0, 120) : "unknown";
          this.log(`getUpdates failed: ${this.lastPollError}`);
          await this.pause(5000);
        }
      }
    })();
  }

  stop(): void {
    this.stopping = true;
    this.wake?.();
  }

  private pause(ms: number): Promise<void> {
    if (this.stopping) return Promise.resolve();
    return new Promise(resolve => {
      const done = () => { clearTimeout(timer); this.wake = undefined; resolve(); };
      const timer = setTimeout(done, ms);
      this.wake = done;
    });
  }

  /** The offset write, retried: KV lagging memory must not lose the
   *  position (tg5 review P2-2b). */
  private async writeOffset(offset: number): Promise<void> {
    for (let attempt = 1; attempt <= 3; attempt++) {
      try {
        await this.kvSet("offset", String(offset));
        return;
      } catch (e) {
        this.log(`the offset write failed (attempt ${attempt}): ${e instanceof Error ? e.message.slice(0, 100) : "unknown"}`);
        if (attempt < 3) await Bun.sleep(500 * attempt);
      }
    }
    throw new Error("the offset could not be written to KV");
  }

  private async absorb(u: Update): Promise<void> {
    const m = u.message ?? u.channel_post;
    if (!m || !m.chat) return; // edits (edited_message) and non-message updates are not seen
    const chatId = String(m.chat.id);
    const message: SeenMessage = {
      message_id: m.message_id,
      ...(u.channel_post && m.chat.type === "channel" ? { channel_post: true } : {}),
      date: m.date,
      ...(m.from?.id !== undefined ? { from_id: m.from.id } : {}),
      ...(m.from?.username || m.from?.first_name ? { from: m.from.username ?? m.from.first_name } : {}),
      ...(m.from?.username ? { username: m.from.username } : {}),
      ...(m.from?.first_name ? { name: [m.from.first_name, m.from.last_name].filter(Boolean).join(" ") } : {}),
      ...(m.text ? { text: m.text } : {}),
      ...(m.reply_to_message ? { reply_to: m.reply_to_message.message_id } : {}),
    };
    if (this.seen.has(chatId, message.message_id)) return;
    const key = `${chatId}:${message.message_id}`;
    if (!(this.botId !== undefined && m.from?.id !== undefined && String(m.from.id) === this.botId)) {
      try { if (await this.onMessage?.(message, chatId, m.chat.title) === false) { this.attempts.delete(key); return; } }
      catch (error) {
        const attempt = (this.attempts.get(key) ?? 0) + 1;
        const status = (error as { status?: number })?.status;
        const refused = status !== undefined && status >= 400 && status < 500 && status !== 408 && status !== 429;
        if (!refused && attempt < 3) { this.attempts.set(key, attempt); throw error; }
        this.log(`message ${key}: ${refused ? "delivery refused" : "handling failed after 3 attempts"}; moving on`);
      }
    }
    this.attempts.delete(key);
    // Publish history only after handling succeeds, a definitive refusal, or
    // the bounded retry policy. Transient failures leave the offset behind.
    this.seen.message(chatId, m.chat.title, m.chat.type, message);
  }
}

function url409(r: { status?: number; description?: string; error?: string }): boolean {
  return r.status === 409 || (r.description ?? "").includes("409") || (r.description ?? "").includes("Conflict");
}
