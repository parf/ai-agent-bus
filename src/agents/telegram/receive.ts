// Receiving (Plans/R1.0-Release/telegram/README.md § receiving): getUpdates
// long polling, 30 s; the offset in the agent's own KV, written after the
// store's batch flush is durable, so a restart does not see a batch twice.
// A failed poll — a 409 (another poller or a webhook on the token), a 429
// after its retry, a malformed answer, a 5xx — is logged with its
// description and retried after a wait (a 409 longer); the last poll error
// is kept for whoami to name (tg5 review P2-1). The store's batch is
// flushed BEFORE the offset is written (tg5 review P2-2): a failed flush
// then re-delivers the batch, whose messages are absorbed once.

import { telegramWeb } from "./telegramapi.ts";
import { Seen, type SeenMessage } from "./seen.ts";

type Update = {
  update_id: number;
  message?: {
    message_id: number;
    date: number;
    text?: string;
    from?: { id: number; username?: string; first_name?: string };
    chat?: { id: number; title?: string; type: string };
    reply_to_message?: { message_id: number };
  };
  edited_message?: unknown;
};

export class Receiver {
  private offset = 0;
  private stopping = false;
  private lastPollError = "";
  /** Set by the agent: every stored message goes to the matching
   *  subscriptions (TG.6). The bot's own id, when known, keeps the bot's
   *  own posts out. */
  onMessage?: (m: SeenMessage, chatId: string, chatTitle?: string) => void;
  botId?: string;

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

  /** Reads the offset from KV — retried, never silently zero (tg5 review
   *  P2-2a) — and runs the poll loop until stop(). */
  async load(): Promise<void> {
    for (let attempt = 1; attempt <= 3; attempt++) {
      try {
        const stored = await this.kvGet("offset");
        this.offset = Number(stored) || 0;
        return;
      } catch (e) {
        const wait = attempt * 1000;
        this.log(`the offset read failed (attempt ${attempt}): ${e instanceof Error ? e.message.slice(0, 100) : "unknown"}`);
        if (attempt < 3) await Bun.sleep(wait);
      }
    }
    throw new Error("the offset could not be read from KV");
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
            await Bun.sleep(url409(r) ? 30_000 : 5_000);
            continue;
          }
          this.lastPollError = "";
          // A getMe-shaped answer (a result that is no array) is treated as
          // no updates: the poll asks Telegram, which always answers an array.
          const updates = (Array.isArray(r.result) ? r.result : []) as Update[];
          if (updates.length === 0) {
            // The real long poll holds; a fast fake does not — a floor
            // against hot-spinning.
            await Bun.sleep(1000);
            continue;
          }
          this.offset = updates[updates.length - 1]!.update_id + 1;
          for (const u of updates) this.absorb(u);
          // The store's batch first, THEN the offset: a failed flush makes
          // the next poll re-deliver the batch, whose messages are
          // absorbed once (tg5 review P2-2b).
          for (const [key, value] of Object.entries(this.seen.flushDirty())) {
            await this.kvSet(key, value);
          }
          await this.writeOffset();
          // The full flush once a minute (tg5 review P3-1): the
          // directories and any ring that the dirty writes missed.
          if (Date.now() - fullSince >= 60_000) {
            for (const [key, value] of Object.entries(this.seen.flushAll())) {
              await this.kvSet(key, value);
            }
            fullSince = Date.now();
          }
        } catch (e) {
          this.lastPollError = e instanceof Error ? e.message.slice(0, 120) : "unknown";
          this.log(`getUpdates failed: ${this.lastPollError}`);
          await Bun.sleep(5000);
        }
      }
    })();
  }

  stop(): void {
    this.stopping = true;
  }

  /** The offset write, retried: KV lagging memory must not lose the
   *  position (tg5 review P2-2b). */
  private async writeOffset(): Promise<void> {
    for (let attempt = 1; attempt <= 3; attempt++) {
      try {
        await this.kvSet("offset", String(this.offset));
        return;
      } catch (e) {
        this.log(`the offset write failed (attempt ${attempt}): ${e instanceof Error ? e.message.slice(0, 100) : "unknown"}`);
        if (attempt < 3) await Bun.sleep(500 * attempt);
      }
    }
  }

  private absorb(u: Update): void {
    const m = u.message;
    if (!m || !m.chat) return; // edits (edited_message) and non-message updates are not seen
    const chatId = String(m.chat.id);
    const message: SeenMessage = {
      message_id: m.message_id,
      date: m.date,
      ...(m.from?.id !== undefined ? { from_id: m.from.id } : {}),
      ...(m.from?.username || m.from?.first_name ? { from: m.from.username ?? m.from.first_name } : {}),
      ...(m.text ? { text: m.text } : {}),
      ...(m.reply_to_message ? { reply_to: m.reply_to_message.message_id } : {}),
    };
    // Stored once, then handed to the forwarder: the dedup covers the
    // forwarder too — a re-delivered batch is not forwarded twice, and the
    // bot's own posts are not forwarded at all (tg5 review P2-2, README
    // § receiving).
    this.seen.message(chatId, m.chat.title, m.chat.type, message);
    if (this.botId !== undefined && m.from?.id !== undefined && String(m.from.id) === this.botId) return;
    this.onMessage?.(message, chatId, m.chat.title);
  }
}

function url409(r: { status?: number; description?: string; error?: string }): boolean {
  return r.status === 409 || (r.description ?? "").includes("409") || (r.description ?? "").includes("Conflict");
}
