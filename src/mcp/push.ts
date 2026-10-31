// Push: the session stops polling and messages arrive on their own.
//
// One loop, two deliveries — long-poll the daemon, hand the envelope to a
// mode — because "how a message reaches a live session" is an adapter concern
// and nothing above it changes
// (docs/06-launchers.md#adapters).
//
// The loop holds the inbox's one unfiltered read
// (docs/04-messaging.md#one-reader-per-inbox), so ab_consume stops being the
// reader while push is on.

import type { Bus, Envelope } from "./bus.ts";
import { BusError } from "./bus.ts";
import { answerSessionAsk } from "./session-help.ts";

export type Deliver = (e: Envelope) => Promise<void>;

const WAIT = "55s"; // just under the daemon's 60s ceiling
const BACKOFF_MS = 2_000;
// A daemon restart takes seconds; a connection that is still down after this
// many attempts is a failure worth an error line, not a reconnect.
export const QUIET_ATTEMPTS = 8;
// Refusals about who is asking, which asking again cannot change: an unknown
// credential, and somebody else already holding the inbox.
const PERMANENT = new Set([401, 403, 409]);
// A suspended principal can be reactivated, which restores everything
// (docs/01-identity-and-authority.md), so push waits and asks again rather than
// going off for the rest of the session (owner, 2026-09-23). A 403 covers
// several refusals, so it is told by the daemon's own sentence for it
// (core.ErrInactive); every other 403 stays permanent.
export const SUSPENDED_RETRY_MS = 30 * 60_000;
// Cancelling fetch ends the client before the daemon necessarily removes its
// waiter. Only a rename handoff gets this bounded grace; other readers refuse.
export const RENAME_READER_RETRY_MS = 10_000;
const SUSPENDED = "user access is suspended";
const suspension = (err: unknown) => err instanceof BusError && err.status === 403 && err.message.includes(SUSPENDED);

/** Why a push ended: its own stop, a refusal that asking again cannot change, or
 *  a message that was consumed and could not be delivered (J60). */
export type PushEnd = "stopped" | "refused" | "delivery";
export type Push = { readonly running: () => boolean; readonly ended: () => PushEnd | undefined; stop: () => void; done: Promise<void> };

/** Messages a push took from an inbox and could not deliver, one per inbox name. They outlive the push: the next one
 *  for that name delivers its message before it reads again, so a consumed message is lost only if the process dies
 *  (J60b). A push reading another name (a fork, a rename) leaves them alone, and its own failure never overwrites
 *  another name's. */
export type Held = {
  kept?: Map<string, { message: Envelope; tries: number }>;
  /** Whether a failed delivery may succeed later (a turn that ends, a connection that returns). Any other
   *  failure is the app-server refusing the input itself, and asking again cannot change it. Default: yes. */
  transient?: (err: unknown) => boolean;
  /** One line to the session when a message is dropped, with its id, sender and error, never its body. */
  notice?: (s: string) => void;
};
/** Attempts before a message that keeps failing for a transient reason is dropped (J60b). */
export const DELIVERY_TRIES = 3;

export function startPush(bus: Bus, deliver: Deliver, log: (s: string) => void, suspendedRetryMs = SUSPENDED_RETRY_MS, readerReleaseMs = 0, sessionEnv: NodeJS.ProcessEnv = process.env, onRefused?: (why: string) => void, onReading?: () => void, held: Held = {}): Push {
  let stopped = false;
  let end: PushEnd | undefined;
  let suspended = false;
  const abort = new AbortController();
  const readerBudget = Math.min(readerReleaseMs, RENAME_READER_RETRY_MS);
  const readerDeadline = performance.now() + readerBudget;
  let handoff = readerBudget > 0, readerWaited = false, readerBackoff = 50, reading = false, down = 0;
  const stop = () => {
    if (stopped) return;
    stopped = true;
    end ??= "stopped";
    abort.abort(); // let go of the inbox now, rather than at the deadline
  };

  // Delivers one message. A failure that may clear is kept for the next push, which delivers it first, and ends
  // this one; the third failure of it, or any failure asking again cannot change, drops it with one line to the log
  // and one to the session naming its id, sender and error (never its body), and the inbox keeps flowing.
  const slots = () => (held.kept ??= new Map());
  const release = (e: Envelope) => { if (slots().get(bus.name)?.message.message_id === e.message_id) slots().delete(bus.name); };
  const hand = async (e: Envelope): Promise<"delivered" | "dropped" | "kept"> => {
    // The answer to a session ask is a send on the bus: if its reply is lost it may have gone, so it is never
    // replayed, and an ask held from before is let go with it. Only the delivery into the session is held and
    // tried again.
    let asked: boolean;
    try {
      asked = await answerSessionAsk(bus, e, sessionEnv);
    } catch (err) {
      release(e);
      const why = `push: message ${e.message_id} from ${e.from} was consumed and dropped: its answer may already have been sent: ${err}`;
      log(why);
      held.notice?.(why.replace(/^push: /, ""));
      return "dropped";
    }
    try {
      if (!asked) await deliver(e);
      release(e);
      return "delivered";
    } catch (err) {
      const before = slots().get(bus.name);
      const tries = (before?.message.message_id === e.message_id ? before.tries : 0) + 1;
      const transient = held.transient?.(err) ?? true;
      if (!transient || tries >= DELIVERY_TRIES) {
        release(e);
        const why = `push: message ${e.message_id} from ${e.from} was consumed and dropped: ${transient ? `not delivered after ${tries} tries` : "the app-server refused it"}: ${err}`;
        log(why);
        held.notice?.(why.replace(/^push: /, ""));
        return "dropped";
      }
      slots().set(bus.name, { message: e, tries });
      log(`push: message ${e.message_id} from ${e.from} was consumed but not delivered (try ${tries} of ${DELIVERY_TRIES}), and is kept to be delivered first: ${err}`);
      log(`push: stopping; later messages stay in ${bus.name}'s inbox`);
      stopped = true;
      end = "delivery";
      return "kept";
    }
  };

  const done = (async () => {
    while (!stopped) {
      const keptHere = slots().get(bus.name);
      if (keptHere) {
        const kept = keptHere.message;
        const result = await hand(kept);
        if (result === "kept") return;
        if (result === "delivered") log(`push: message ${kept.message_id} from ${kept.from}, kept after a failed delivery, was delivered`);
        continue;
      }
      let e: Envelope | null = null;
      try {
        e = await bus.consume({ wait: WAIT }, abort.signal);
      } catch (err) {
        if (stopped) return;
        // A refused credential costs the daemon one refusal per backoff for as
        // long as the session lives, and never delivers anything — which is
        // what a session outliving its principal did, at a refusal every two
        // seconds for a day.
        if (suspension(err)) {
          if (!suspended) log(`push: ${bus.name} is suspended (${(err as BusError).message}); asking again every ${Math.round(suspendedRetryMs / 60_000)} minutes`);
          suspended = true;
          await sleep(suspendedRetryMs, abort.signal);
          continue;
        }
        if (err instanceof BusError && err.status === 409 && handoff && performance.now() < readerDeadline) {
          if (!readerWaited) log(`push: ${bus.name} waiting for the previous reader after rename (up to ${readerBudget}ms)`);
          readerWaited = true;
          await sleep(Math.min(readerBackoff, Math.max(0, readerDeadline - performance.now())), abort.signal);
          readerBackoff = Math.min(readerBackoff * 2, 1_000);
          continue;
        }
        if (err instanceof BusError && PERMANENT.has(err.status)) {
          log(err.status === 409
            ? handoff
              ? `push: ${bus.name} still has a reader after ${readerBudget}ms following rename; push inactive, not starting a second reader`
              : `push: ${bus.name} already has a reader; not starting a second one`
            : `push: ${bus.name} cannot read its inbox (${err.status} ${err.message}); push is off for this session`);
          // Somebody else holds the inbox: say so where the session sees it, not
          // only in the runtime's MCP log (docs/06-launchers.md#one-live-session-per-name).
          if (err.status === 409) onRefused?.(`${bus.name} is already read by another session; this one receives nothing under that name`);
          stopped = true;
          end = "refused";
          return;
        }
        // A daemon restart resets or refuses the socket for a moment: a quiet
        // one-line "reconnecting", and an error only when it outlasts the
        // restart (J15). A refusal the daemon answered is always said.
        if (!(err instanceof BusError)) {
          if (++down === 1) log(`push: ${bus.name} lost the daemon; reconnecting`);
          if (down === QUIET_ATTEMPTS) log(`push: consume failed: ${err}`);
        } else log(`push: consume failed: ${err.status} ${err.message}`);
        await sleep(Number(sessionEnv.AGENT_BUS_PUSH_BACKOFF_MS) || BACKOFF_MS, abort.signal);
        continue;
      }
      if (stopped) { // stopped while the daemon was handing it over: kept for the next push, not lost
        if (e) slots().set(bus.name, { message: e, tries: 0 });
        return;
      }
      handoff = false; // the first successful consume proves the old waiter is gone
      if (down) { if (down < QUIET_ATTEMPTS) log(`push: ${bus.name} reconnected`); down = 0; }
      if (!reading) { reading = true; onReading?.(); }
      if (readerWaited) {
        readerWaited = false;
        log(`push: ${bus.name} reads its renamed inbox`);
      }
      if (suspended) {
        suspended = false;
        log(`push: ${bus.name} reads its inbox again`);
      }
      if (!e) continue; // deadline, no message
      if (await hand(e) === "kept") return;
    }
  })();

  return { running: () => !stopped, ended: () => end, stop, done };
}

/** J60: a push that stopped because a delivery failed is started again, after a
 *  growing wait, at most `delays.length` times; a refusal or its own stop is never
 *  restarted. After the last try one line says so and nothing repeats. `check` is
 *  called on a timer and holds no timer of its own. */
export function superviseDelivery(
  current: () => Push | undefined,
  restart: () => Promise<void> | void,
  say: { note: (s: string) => void; warn: (s: string) => void },
  opts: { delays?: number[]; settledMs?: number; now?: () => number } = {},
): () => Promise<void> {
  const delays = opts.delays ?? [2_000, 10_000, 30_000];
  const settledMs = opts.settledMs ?? 60_000;
  const now = opts.now ?? Date.now;
  let tries = 0, gaveUp = false, since = 0;
  let due: number | undefined;
  return async () => {
    const push = current();
    if (!push) return;
    if (push.running()) {
      // Healthy for a while: a later failure gets its own tries.
      if (tries > 0 && now() - since >= settledMs) { tries = 0; gaveUp = false; due = undefined; }
      return;
    }
    if (push.ended() !== "delivery") return;
    if (tries >= delays.length) {
      if (!gaveUp) {
        gaveUp = true;
        say.warn(`bus push stopped after a failed delivery and was restarted ${tries} times; it stays off. Resume this session to reattach it`);
      }
      return;
    }
    due ??= now() + delays[tries]!; // the first wait starts when the stop is seen, the next ones at each restart
    if (now() < due) return;
    tries++;
    say.note(`bus push stopped after a failed delivery; restarting it (${tries} of ${delays.length})`);
    await push.done;
    await restart();
    since = now();
    due = tries < delays.length ? now() + delays[tries]! : undefined;
  };
}

// A stop ends the wait at once: a half-hour retry must not keep a stopped
// push alive.
export function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((r) => {
    const t = setTimeout(r, ms);
    signal?.addEventListener("abort", () => { clearTimeout(t); r(); }, { once: true });
  });
}
