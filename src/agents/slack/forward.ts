// Forwarding (Plans/R1.2/slack/README.md#subscriptions-authority-errors and
// § behaviour pinned by the review): Socket Mode events pass through a
// bounded queue — the socket acks before anything here runs — are
// deduplicated by event_id over a window, filtered to plain messages,
// thread broadcasts and bots' messages, never one the agent posted itself, and sent
// as the agent to each subscription whose channel and filter match: topic
// `slack`, tag the channel id, body the message's JSON. A failed send is
// recorded, never replayed. Counters live in memory and are written to KV
// once per flush, only for subscriptions that still exist.
import type { Subscription } from "./subs.js";
import { testFilter } from "./filters.js";

const queueMax = Number(process.env.SLACK_AGENT_QUEUE ?? "1000");
const dedupMax = 1000;
const flushMs = Number(process.env.SLACK_AGENT_FLUSH_MS ?? "60000");

export type Identity = { botUser?: string; botId?: string; appId?: string };

export type ForwardDeps = {
  /** The subscriptions as they are now. */
  subs(): Promise<Subscription[]>;
  /** A subscription's channel, as given (#name or CID), to the CID it means. */
  channelId(channel: string): Promise<string | undefined>;
  /** A user id's name, or "". */
  userName(id: string): Promise<string>;
  /** A CID's #name, or "". */
  channelName(id: string): Promise<string>;
  /** Send as the agent. */
  send(to: string, body: string, topic: string, tag: string): Promise<void>;
  /** The bot's identity in Slack; undefined while it cannot be learned, and
   *  then nothing is forwarded. */
  identity(): Promise<Identity | undefined>;
  /** Whether a message in the bot's name is one this agent posted. The bot's
   *  token may be shared — CI, slack-post — so its name alone is not ours. */
  posted(channel: string, ts: string): Promise<boolean>;
  /** Add to a subscription's persisted count: once per flush, live ids only. */
  persistCount(id: string, forwarded: number): Promise<void>;
  /** Write its last error. */
  persistError(id: string, lastError: string): Promise<void>;
  /** Whether a subscription still exists. */
  exists(id: string): Promise<boolean>;
  /** Mark a subscription disabled, saying why; a deleted one stays deleted. */
  disable(id: string, why: string): Promise<void>;
  /** Remove a deleted subscription's counters that a flush wrote after it went. */
  forget(id: string): Promise<void>;
  /** Run a subscription's writes in its turn, with the edits and deletes. */
  serialize<T>(id: string, fn: () => Promise<T>): Promise<T>;
  log(line: string): void;
  scrub(text: string): string;
};

type Stat = { forwarded: number; lastError?: string; errorDirty?: boolean; dirty: boolean };

/** The subtypes forwarded; every other subtype is dropped. */
const forwardedSubtypes = new Set([undefined, "thread_broadcast", "bot_message"]);

export class Forwarder {
  private queue: { event: any; eventId: string }[] = [];
  private seen: string[] = [];
  private seenSet = new Set<string>();
  private pumping: Promise<void> | undefined;
  private flushTimer: ReturnType<typeof setInterval> | undefined;
  private stats = new Map<string, Stat>();
  dropped = 0;
  /** Counter writes made, for tests: one per live dirty id per flush. */
  writes = 0;

  constructor(private readonly deps: ForwardDeps) {}

  start(): void {
    this.flushTimer = setInterval(() => void this.flush(), flushMs);
  }

  /** Take one event from the socket: dedup, queue, and let the pump run. */
  admit(event: any, eventId: string): void {
    if (eventId !== "") {
      if (this.seenSet.has(eventId)) return;
      this.seenSet.add(eventId);
      this.seen.push(eventId);
      if (this.seen.length > dedupMax) this.seenSet.delete(this.seen.shift()!);
    }
    if (this.queue.length >= queueMax) {
      this.queue.shift();
      this.dropped++;
      this.deps.log(`forward: the queue is full; the oldest event was dropped (${this.dropped} so far)`);
    }
    this.queue.push({ event, eventId });
    this.pumping ??= this.pump().finally(() => (this.pumping = undefined));
  }

  /** In-memory counters a listing adds to what was flushed. */
  pending(id: string): { forwarded: number; lastError?: string } {
    const s = this.stats.get(id);
    return { forwarded: s?.forwarded ?? 0, lastError: s?.lastError };
  }

  private async pump(): Promise<void> {
    while (this.queue.length > 0) {
      const { event } = this.queue.shift()!;
      try {
        await this.handle(event);
      } catch (e) {
        this.deps.log(`forward: an event was not handled: ${this.deps.scrub(e instanceof Error ? e.message : String(e))}`);
      }
    }
  }

  private stat(id: string): Stat {
    let s = this.stats.get(id);
    if (!s) this.stats.set(id, (s = { forwarded: 0, dirty: false }));
    return s;
  }

  private async handle(event: any): Promise<void> {
    if (event?.type !== "message") return;
    if (!forwardedSubtypes.has(event.subtype)) return;
    const me = await this.deps.identity();
    if (!me) {
      // Not knowing which posts are its own, it forwards none: one dropped
      // message is better than a loop of its own replies (slack-review-S.5).
      this.deps.log("forward: the agent's own Slack identity is not known yet; an event was not forwarded");
      return;
    }
    const channel = typeof event.channel === "string" ? event.channel : "";
    if (!channel) return;
    const bots = (me.botUser && event.user === me.botUser) || (me.botId && event.bot_id === me.botId) || (me.appId && event.app_id === me.appId);
    if (bots && (await this.deps.posted(channel, typeof event.ts === "string" ? event.ts : ""))) return;
    const text = typeof event.text === "string" ? event.text : "";
    const subs = await this.deps.subs();
    let body: string | undefined;
    for (const sub of subs) {
      if ((sub as any).disabled === true) continue;
      // One subscription's failed lookup skips it alone, never the others.
      const id = await this.deps.channelId(sub.channel).catch(e => {
        this.deps.log(`forward: subscription ${sub.id}: its channel ${sub.channel} was not resolved: ${this.deps.scrub(e instanceof Error ? e.message : String(e))}`);
        return undefined;
      });
      if (id !== channel) continue;
      if (sub.filter) {
        const verdict = await testFilter(sub.filter, text);
        if (verdict === "slow") {
          const why = "its filter took longer than the limit; disabled";
          this.stat(sub.id).lastError = why;
          this.stat(sub.id).errorDirty = true;
          this.stat(sub.id).dirty = true;
          this.deps.log(`forward: subscription ${sub.id}: ${why}`);
          await this.deps.disable(sub.id, why).catch(() => {});
          continue;
        }
        if (verdict === "miss") continue;
      }
      if (body === undefined) {
        const who = typeof event.user === "string" ? await this.deps.userName(event.user) : "";
        body = JSON.stringify({
          channel,
          channel_name: await this.deps.channelName(channel),
          user: typeof event.user === "string" ? event.user : typeof event.bot_id === "string" ? event.bot_id : "",
          ...(who ? { user_name: who } : {}),
          text,
          ts: typeof event.ts === "string" ? event.ts : "",
          ...(typeof event.thread_ts === "string" ? { thread_ts: event.thread_ts } : {}),
        });
      }
      const s = this.stat(sub.id);
      try {
        await this.deps.send(sub.target, body, "slack", channel);
        s.forwarded++;
      } catch (e) {
        // Recorded, never replayed: the send may have happened.
        s.lastError = this.deps.scrub(e instanceof Error ? e.message : String(e));
        s.errorDirty = true;
        this.deps.log(`forward: subscription ${sub.id} to ${sub.target}: ${s.lastError}`);
      }
      s.dirty = true;
    }
  }

  /** Write the counters that changed, once, for subscriptions still there.
   *  The count and the last error are written apart: a count that landed is
   *  never added again because the error after it failed (slack-review-S.5). */
  async flush(): Promise<void> {
    for (const [id, s] of [...this.stats]) {
      if (!s.dirty) continue;
      await this.deps.serialize(id, () => this.flushOne(id, s));
    }
  }

  private async flushOne(id: string, s: Stat): Promise<void> {
    {
      if (!(await this.deps.exists(id).catch(() => true))) {
        this.stats.delete(id); // deleted: nothing may write it back
        return;
      }
      if (s.forwarded > 0) {
        const delta = s.forwarded;
        s.forwarded = 0;
        try {
          await this.deps.persistCount(id, delta);
          this.writes++;
          // Deleted while the count was written: take back what the write made.
          if (await this.goneSince(id)) return;
        } catch (e) {
          s.forwarded += delta; // not written: kept for the next flush
          this.deps.log(`forward: the count for ${id} was not written: ${this.deps.scrub(e instanceof Error ? e.message : String(e))}`);
        }
      }
      if (s.errorDirty && s.lastError !== undefined) {
        try {
          await this.deps.persistError(id, s.lastError);
          s.errorDirty = false;
          if (await this.goneSince(id)) return;
        } catch (e) {
          this.deps.log(`forward: the last error for ${id} was not written: ${this.deps.scrub(e instanceof Error ? e.message : String(e))}`);
        }
      }
      s.dirty = s.forwarded > 0 || s.errorDirty === true;
    }
  }

  /** Deleted while a counter was written: take back what the write made. */
  private async goneSince(id: string): Promise<boolean> {
    if (await this.deps.exists(id).catch(() => true)) return false;
    await this.deps.forget(id).catch(() => {});
    this.stats.delete(id);
    return true;
  }

  /** Stop: no more flushes on a timer; drain the queue and flush once, all
   *  within drainMs — a daemon that does not answer cannot hold shutdown. */
  async stop(drainMs = 5000): Promise<void> {
    clearInterval(this.flushTimer);
    this.flushTimer = undefined;
    const bound = Bun.sleep(drainMs);
    await Promise.race([(async () => {
      if (this.pumping) await this.pumping;
      await this.flush();
    })(), bound]);
  }

  get armed(): boolean {
    return this.flushTimer !== undefined;
  }
}

/** The messages this agent posted, by channel and ts, for a while: its own
 *  posts are told from others' in the same bot's name by this, not by name.
 *  An event can outrun the post's answer, so a question waits, boundedly,
 *  for the posts still in flight. */
export class Posted {
  private at = new Map<string, number>();
  private flying = 0;
  private settle: (() => void)[] = [];

  constructor(private readonly keepMs = 10 * 60_000, private readonly most = 1000, private readonly waitMs = 5000) {}

  /** A post is being sent. */
  begin(): void {
    this.flying++;
  }

  /** It was answered: where it landed, or nothing when it failed. */
  end(channel?: string, ts?: string): void {
    if (channel && ts) {
      this.at.set(`${channel} ${ts}`, Date.now());
      const old = Date.now() - this.keepMs;
      for (const [k, when] of this.at) {
        if (this.at.size <= this.most && when >= old) break;
        this.at.delete(k);
      }
    }
    if (--this.flying === 0) for (const f of this.settle.splice(0)) f();
  }

  async mine(channel: string, ts: string): Promise<boolean> {
    const k = `${channel} ${ts}`;
    if (!this.at.has(k) && this.flying > 0) {
      await new Promise<void>(done => {
        const t = setTimeout(done, this.waitMs);
        this.settle.push(() => { clearTimeout(t); done(); });
      });
    }
    return this.at.has(k);
  }
}
