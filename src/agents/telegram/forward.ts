// Forwarding (Plans/R1.0-Release/telegram/README.md § receiving): every
// received message goes to each subscription whose channel resolves to its
// chat — sent to the target with topic `telegram`, tag the chat id, and
// the README's body. A filter goes through the shared filter worker; the
// counters are the subscription's own (S.5), written through withSub so a
// concurrent edit cannot lose them.

import { testFilter } from "../slack/filters.ts";
import { withSub, type Subscription } from "../slack/subs.ts";
import type { SeenMessage } from "./seen.ts";

export type ForwardDeps = {
  subs: () => Promise<Subscription[]>;
  resolve: (channel: string) => { ok: true; chat_id: string; named: string } | { ok: false; error: string };
  send: (to: string, body: string, topic: string, tag: string) => Promise<void>;
  persistCount: (id: string, forwarded: number) => Promise<void>;
  persistError: (id: string, lastError: string) => Promise<void>;
  log: (line: string) => void;
};

export class Forwarder {
  constructor(private readonly deps: ForwardDeps) {}

  /** One received message, to every matching subscription. A failed send is
   *  recorded, never replayed (S.5). */
  async message(m: SeenMessage, chatId: string, chatTitle?: string): Promise<void> {
    for (const sub of await this.deps.subs()) {
      if (sub.disabled) continue;
      const dest = this.deps.resolve(sub.channel);
      this.deps.log(`forward: sub ${sub.id} channel ${sub.channel} → ${dest.ok ? dest.chat_id : dest.error} vs message chat ${chatId}`);
      if (!dest.ok || dest.chat_id !== chatId) continue;
      const verdict = sub.filter ? await testFilter(sub.filter, m.text ?? "") : "hit";
      if (verdict !== "hit") continue;
      const body = JSON.stringify({
        chat: chatId,
        ...(chatTitle ? { chat_name: chatTitle } : {}),
        user: m.from_id ?? 0,
        ...(m.from ? { user_name: m.from.startsWith("@") ? m.from : `@${m.from}` } : {}),
        text: m.text ?? "",
        message_id: m.message_id,
        ...(m.reply_to ? { reply_to: m.reply_to } : {}),
      });
      try {
        await this.deps.send(sub.target, body, "telegram", chatId);
        await this.deps.persistCount(sub.id, 1);
      } catch (e) {
        // Recorded, never replayed: the send may have happened.
        const lastError = e instanceof Error ? e.message : String(e);
        this.deps.log(`forward: subscription ${sub.id} to ${sub.target}: ${lastError}`);
        await this.deps.persistError(sub.id, lastError);
      }
    }
  }
}
