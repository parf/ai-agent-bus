// Forwarding (Plans/R1.0-Release/telegram/README.md § receiving): every
// stored message goes to each subscription whose channel resolves to its
// chat — sent to the target with topic `telegram`, tag the chat id, and
// the README's body. A filter goes through the shared filter worker; the
// counters are the subscription's own (S.5): kept in memory, marked dirty
// per send, flushed on the minute through withSub, and a count whose
// subscription was deleted is taken back, never written (slack/forward.ts's
// stats design, reused).

import { testFilter } from "../slack/filters.ts";
import { withSub, type Subscription } from "../slack/subs.ts";
import type { SeenMessage } from "./seen.ts";

export type ForwardDeps = {
  subs: () => Promise<Subscription[]>;
  /** The subscription still exists in the store? A deleted subscription's
   *  pending count is taken back, never written. */
  exists: (id: string) => Promise<boolean>;
  resolve: (channel: string) => { ok: true; chat_id: string; named: string } | { ok: false; error: string };
  send: (to: string, body: string, topic: string, tag: string) => Promise<void>;
  persistCount: (id: string, forwarded: number) => Promise<void>;
  persistError: (id: string, lastError: string) => Promise<void>;
  disable: (id: string, why: string) => Promise<void>;
  /** The in-memory counter, for sub.list to show before a flush. */
  pending: (id: string) => { forwarded: number } | undefined;
  /** Refreshes the alias mapping the resolution reads: once per batch. */
  refreshConfig?: () => Promise<void>;
  log: (line: string) => void;
};

type Stat = { forwarded: number; lastError?: string; dirty: boolean; errorDirty: boolean };

export class Forwarder {
  private stats = new Map<string, Stat>();
  private stopping = false;

  constructor(private readonly deps: ForwardDeps) {}

  stop(): void {
    this.stopping = true;
  }

  /** The subscription's in-memory counter, or undefined when none. */
  pending(id: string): { forwarded: number } | undefined {
    const s = this.stats.get(id);
    return s ? { forwarded: s.forwarded } : undefined;
  }

  private stat(id: string): Stat {
    let s = this.stats.get(id);
    if (!s) {
      s = { forwarded: 0, dirty: false, errorDirty: false };
      this.stats.set(id, s);
    }
    return s;
  }

  /** Flushes the dirty counters, once a minute or at stop: each through
   *  withSub, with the exists take-back (slack/forward.ts's design). */
  async flush(): Promise<void> {
    for (const [id, s] of [...this.stats.entries()]) {
      if (!s.dirty) continue;
      await withSub(id, async () => {
        if (!(await this.deps.exists(id).catch(() => true))) {
          this.stats.delete(id); // deleted: nothing may be written back
          return;
        }
        if (s.forwarded > 0) {
          const delta = s.forwarded;
          s.forwarded = 0;
          try {
            await this.deps.persistCount(id, delta);
          } catch (e) {
            s.forwarded += delta; // not written: kept for the next flush
            this.deps.log(`forward: the count for ${id} was not written: ${e instanceof Error ? e.message.slice(0, 100) : "unknown"}`);
          }
        }
        if (s.lastError !== undefined && s.errorDirty) {
          try {
            await this.deps.persistError(id, s.lastError);
            s.errorDirty = false;
          } catch { /* kept for the next flush */ }
        }
        s.dirty = s.forwarded > 0 || s.errorDirty;
        if (!s.dirty) this.stats.delete(id);
      });
    }
  }

  /** One received message, to every matching subscription. A failed send is
   *  recorded (last_error), never replayed; a filter that cannot be
   *  evaluated in time disables the subscription, as Slack's does. */
  async message(m: SeenMessage, chatId: string, chatTitle?: string): Promise<void> {
    if (this.stopping) return;
    await this.deps.refreshConfig?.();
    for (const sub of await this.deps.subs()) {
      if (sub.disabled) continue;
      const dest = this.deps.resolve(sub.channel);
      if (!dest.ok || dest.chat_id !== chatId) continue;
      const verdict = sub.filter ? await testFilter(sub.filter, m.text ?? "") : "hit";
      if (verdict === "slow") {
        // Slack's semantics: a filter that cannot be evaluated in time
        // disables the subscription, with the reason recorded and logged —
        // never a silent drop.
        const why = "its filter took longer than the limit; disabled";
        this.deps.log(`forward: subscription ${sub.id}: ${why}`);
        await this.deps.disable(sub.id, why).catch(() => {});
        continue;
      }
      if (verdict === "miss") continue;
      const body = JSON.stringify({
        chat: chatId,
        ...(chatTitle ? { chat_name: chatTitle } : {}),
        user: m.from_id ?? 0,
        ...(m.from ? { user_name: m.from.startsWith("@") ? m.from : `@${m.from}` } : {}),
        text: m.text ?? "",
        message_id: m.message_id,
        ...(m.reply_to ? { reply_to: m.reply_to } : {}),
      });
      const s = this.stat(sub.id);
      try {
        await this.deps.send(sub.target, body, "telegram", chatId);
        s.forwarded++;
        s.dirty = true;
      } catch (e) {
        // Recorded, never replayed: the send may have happened.
        s.lastError = e instanceof Error ? e.message : String(e);
        s.errorDirty = true;
        s.dirty = true;
        this.deps.log(`forward: subscription ${sub.id} to ${sub.target}: ${s.lastError}`);
      }
    }
  }
}
