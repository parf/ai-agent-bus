// Receiving (Plans/R1.0-Release/telegram/README.md § receiving): getUpdates
// long polling, 30 s; the offset in the agent's own KV, written after each
// non-empty batch, so a restart does not see a batch again. Each received
// message goes to the seen store for `read`, `channels` and `users`.
// Forwarding to subscription targets is TG.6, with the shared subscriptions
// module; in groups the bot sees every message only with privacy mode off
// in BotFather — the agent's README says so.

import { telegramWeb } from "./telegramapi.ts";
import { Seen, type SeenMessage } from "./seen.ts";

/** The pinned-policy call the receiver makes: the secret is read fresh for
 *  every poll, exactly as the methods read theirs. */
export type Web = (kind: "read" | "write", method: string, params: Record<string, unknown>) => Promise<any>;

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
  /** Set by the agent: every stored message goes to the matching
   *  subscriptions (TG.6). The bot's own id, when known, keeps the bot's
   *  own posts out. */
  onMessage?: (m: SeenMessage, chatId: string, chatTitle?: string) => void;
  botId?: string;

  constructor(
    private readonly seen: Seen,
    private readonly web: Web,
    private readonly kvGet: (key: string) => Promise<string | undefined>,
    private readonly kvSet: (key: string, value: unknown) => Promise<void>,
    private readonly log: (line: string) => void,
  ) {}

  /** Reads the offset from KV and runs the poll loop until stop(). A
   *  failed poll is logged and retried after 5 s; the bus is never blocked
   *  by it. */
  start(): Promise<void> {
    void (async () => {
      try {
        const stored = await this.kvGet("offset");
        if (stored !== undefined) this.offset = Number(stored) || 0;
      } catch {
        /* start from the beginning; the batch dup is TG.6's forwarding concern */
      }
      while (!this.stopping) {
        try {
          this.log(`poll offset=${this.offset}`);
          const r = await this.web("read", "getUpdates", { offset: this.offset, timeout: 30 });
          const updates = (r.result ?? []) as Update[];
          this.log(`poll got ${updates.length} update(s)`);
          if (updates.length === 0) await Bun.sleep(1000); // a floor against hot-spinning: the real long poll holds, a fast fake does not
          if (updates.length > 0) {
            this.offset = updates[updates.length - 1]!.update_id + 1;
            for (const u of updates) this.absorb(u);
            // The offset is written with the batch: a restart resumes after
            // it, never seeing it twice (README § receiving).
            await this.kvSet("offset", String(this.offset));
            await this.flush();
          }
        } catch (e) {
          this.log(`getUpdates failed: ${e instanceof Error ? e.message.slice(0, 120) : "unknown"}`);
          await Bun.sleep(5000);
        }
      }
    })();
    return Promise.resolve();
  }

  stop(): void {
    this.stopping = true;
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
    this.seen.message(chatId, m.chat.title, m.chat.type, message);
    // The bot's own posts are not forwarded (README § receiving).
    if (this.botId !== undefined && m.from?.id !== undefined && String(m.from.id) === this.botId) return;
    this.onMessage?.(message, chatId, m.chat.title);
  }

  /** The whole seen state into the agent's own KV, after each batch. */
  private async flush(): Promise<void> {
    for (const [key, value] of Object.entries(this.seen.flush())) {
      await this.kvSet(key, value);
    }
  }
}
