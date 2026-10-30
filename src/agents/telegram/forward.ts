// Telegram-specific body and live selector mapping; shared subscriptions do
// filtering, both author checks, sending and counters without pause or retry.
import type { ChatSubscriptions } from "../shared/subscription-port.ts";
import type { SeenMessage } from "./seen.ts";
export type ForwardDeps = {
  subscriptions: Pick<ChatSubscriptions, "deliver" | "close">;
  refreshConfig?: () => Promise<void>;
};
export class Forwarder {
  private stopping = false;
  constructor(private readonly deps: ForwardDeps) {}
  stop(): void { this.stopping = true; this.deps.subscriptions.close(); }
  async message(m: SeenMessage, chatId: string, chatTitle?: string): Promise<boolean> {
    if (this.stopping) return false;
    await this.deps.refreshConfig?.();
    const body = JSON.stringify({
      chat: chatId, ...(chatTitle ? { chat_name: chatTitle } : {}), user: m.from_id ?? 0,
      ...(m.from ? { user_name: m.from.startsWith("@") ? m.from : `@${m.from}` } : {}),
      text: m.text ?? "", message_id: m.message_id, ...(m.reply_to ? { reply_to: m.reply_to } : {}),
    });
    const result = await this.deps.subscriptions.deliver(chatId, String(m.from_id ?? ""), m.text ?? "", body, "telegram");
    return result.matched > 0;
  }
}
