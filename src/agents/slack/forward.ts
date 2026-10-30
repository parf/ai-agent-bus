// Slack-specific intake, deduplication, routing and self-echo suppression.
// Shared subscriptions own filtering, admission and one-shot send accounting.
import type { ChatSubscriptions } from "../shared/subscription-port.ts";
const queueMax = Number(process.env.SLACK_AGENT_QUEUE ?? "1000");
const dedupMax = 1000;
const forwardedSubtypes = new Set([undefined, "thread_broadcast", "bot_message"]);
export type Identity = { botUser?: string; botId?: string; appId?: string };
export type ForwardDeps = {
  route?(event: any, identity: Identity): Promise<void | false>;
  subscriptions: Pick<ChatSubscriptions, "deliver" | "close">;
  userName(id: string): Promise<string>;
  channelName(id: string): Promise<string>;
  identity(): Promise<Identity | undefined>;
  posted(channel: string, ts: string): Promise<boolean>;
  log(line: string): void;
  scrub(text: string): string;
};
export class Forwarder {
  private queue: { event: any; eventId: string }[] = [];
  private seen: string[] = [];
  private seenSet = new Set<string>();
  private pumping: Promise<void> | undefined;
  private stopping = false;
  dropped = 0;
  constructor(private readonly deps: ForwardDeps) {}
  /** Take one event from the socket: dedup, queue, and let the pump run. */
  admit(event: any, eventId: string): void {
    if (this.stopping) return;
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

  private async pump(): Promise<void> {
    while (this.queue.length > 0) {
      const { event } = this.queue.shift()!;
      try { await this.handle(event); }
      catch { this.deps.log("forward: subscription handling unavailable; event not replayed"); }
    }
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
    if (this.deps.route) {
      for (let attempt = 1; attempt <= 3; attempt++) {
        try { if (await this.deps.route(event, me) === false) return; break; }
        catch (error) {
          const status = (error as { status?: number })?.status;
          const refused = status !== undefined && status >= 400 && status < 500 && status !== 408 && status !== 429;
          if (refused || attempt === 3) {
            this.deps.log(`routing: message ${channel}:${event.ts}: ${refused ? "delivery refused" : "handling failed after 3 attempts"}; moving on`);
            break;
          }
          await Bun.sleep(attempt * 250);
        }
      }
    }
    const text = typeof event.text === "string" ? event.text : "";
    const who = typeof event.user === "string" ? await this.deps.userName(event.user) : "";
    const body = JSON.stringify({
      channel, channel_name: await this.deps.channelName(channel),
      user: typeof event.user === "string" ? event.user : typeof event.bot_id === "string" ? event.bot_id : "",
      ...(who ? { user_name: who } : {}), text,
      ts: typeof event.ts === "string" ? event.ts : "",
      ...(typeof event.thread_ts === "string" ? { thread_ts: event.thread_ts } : {}),
    });
    await this.deps.subscriptions.deliver(channel, event.bot_id ?? event.user ?? "", text, body, "slack");
  }
  async stop(drainMs = 5000): Promise<void> {
    this.stopping = true;
    await Promise.race([this.pumping, Bun.sleep(drainMs)]);
    this.deps.subscriptions.close();
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
