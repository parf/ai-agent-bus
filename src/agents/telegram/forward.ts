// Forwarding (Plans/R1.0-Release/done/telegram/README.md § receiving): every
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
  /** Whether `who`, a subscription's author, may send to `to` by the daemon's rule; rejects with the daemon's 404 or any other error when it cannot say. */
  maySend: (who: string, to: string) => Promise<boolean>;
  persistCount: (id: string, forwarded: number) => Promise<void>;
  persistError: (id: string, lastError: string) => Promise<void>;
  disable: (id: string, why: string) => Promise<void>;
  /** The in-memory counter, for sub.list to show before a flush. */
  pending: (id: string) => { forwarded: number } | undefined;
  /** Refreshes the alias mapping the resolution reads: once per batch. */
  refreshConfig?: () => Promise<void>;
  log: (line: string) => void;
};

type Stat = { forwarded: number; unavailable?: boolean; lastError?: string; dirty: boolean; errorDirty: boolean };

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
  async message(m: SeenMessage, chatId: string, chatTitle?: string): Promise<boolean> {
    if (this.stopping) return false;
    let matched = false;
    await this.deps.refreshConfig?.();
    // One may-send answer per author and target for this event, and no further
    // calls after the first failure that is not a 404: they would fail alike.
    const checks = new Map<string, Promise<boolean>>();
    let down: unknown;
    const ask = (who: string, to: string): Promise<boolean> => {
      if (down !== undefined) return Promise.reject(down);
      const key = `${who}\n${to}`;
      let answer = checks.get(key);
      if (!answer) {
        answer = this.deps.maySend(who, to);
        checks.set(key, answer);
      }
      return answer.catch(e => {
        if ((e as any)?.status !== 404 && down === undefined) down = e ?? new Error("unavailable");
        throw e;
      });
    };
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
      matched = true;
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
      // The author's own right to send to the target, asked before every send
      // (RD141). A refusal or a vanished destination disables the subscription
      // visibly; a check that cannot be answered sends nothing, says so once
      // per outage, and is not asked again for the rest of this event.
      let stop: string | undefined;
      if (!sub.created_by) stop = "its author is unknown; disabled";
      else {
        try {
          if (!(await ask(sub.created_by, sub.target))) stop = "its author may no longer send to the target; disabled";
          else s.unavailable = false;
        } catch (e) {
          if ((e as any)?.status === 404) stop = "its target is gone or no longer visible to this agent; disabled";
          else {
            if (!s.unavailable) {
              s.unavailable = true;
              s.lastError = "the may-send check was unavailable; nothing sent";
              s.errorDirty = true;
              s.dirty = true;
              this.deps.log(`forward: subscription ${sub.id}: ${s.lastError}`);
            }
            continue;
          }
        }
      }
      if (stop !== undefined) {
        s.lastError = stop;
        s.errorDirty = true;
        s.dirty = true;
        this.deps.log(`forward: subscription ${sub.id}: ${stop}`);
        await this.deps.disable(sub.id, stop).catch(e => this.deps.log(`forward: subscription ${sub.id}: the disable was not stored: ${e instanceof Error ? e.message.slice(0, 100) : "unknown"}`));
        continue;
      }
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
    return matched;
  }
}
