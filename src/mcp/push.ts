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

export type Push = { readonly running: () => boolean; stop: () => void; done: Promise<void> };

export function startPush(bus: Bus, deliver: Deliver, log: (s: string) => void, suspendedRetryMs = SUSPENDED_RETRY_MS, readerReleaseMs = 0, sessionEnv: NodeJS.ProcessEnv = process.env, onRefused?: (why: string) => void, onReading?: () => void): Push {
  let stopped = false;
  let suspended = false;
  const abort = new AbortController();
  const readerBudget = Math.min(readerReleaseMs, RENAME_READER_RETRY_MS);
  const readerDeadline = performance.now() + readerBudget;
  let handoff = readerBudget > 0, readerWaited = false, readerBackoff = 50, reading = false, down = 0;
  const stop = () => {
    if (stopped) return;
    stopped = true;
    abort.abort(); // let go of the inbox now, rather than at the deadline
  };

  const done = (async () => {
    while (!stopped) {
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
      if (stopped) return; // a message taken after stop would be lost anyway
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
      try {
        if (!await answerSessionAsk(bus, e, sessionEnv)) await deliver(e);
      } catch (err) {
        // At-most-once: the daemon has already handed this message over, so a
        // failed delivery loses it. Stop rather than keep draining an inbox
        // into a delivery path that is not working — the rest stays queued
        // for whoever reads next.
        log(`push: message ${e.message_id} from ${e.from} was consumed but not delivered: ${err}`);
        log(`push: stopping; later messages stay in ${bus.name}'s inbox`);
        stopped = true;
        return;
      }
    }
  })();

  return { running: () => !stopped, stop, done };
}

// A stop ends the wait at once: a half-hour retry must not keep a stopped
// push alive.
export function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((r) => {
    const t = setTimeout(r, ms);
    signal?.addEventListener("abort", () => { clearTimeout(t); r(); }, { once: true });
  });
}
