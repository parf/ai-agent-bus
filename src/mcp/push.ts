// Push: the session stops polling and messages arrive on their own.
//
// One loop, two deliveries — long-poll the daemon, hand the envelope to a
// mode — because "how a message reaches a live session" is an adapter concern
// and nothing above it changes
// (docs/08-runner-role.md#adapters).
//
// The loop holds the inbox's one unfiltered read
// (docs/04-messaging.md#one-reader-per-inbox), so ab_consume stops being the
// reader while push is on.

import type { Bus, Envelope } from "./bus.ts";
import { BusError } from "./bus.ts";

export type Deliver = (e: Envelope) => Promise<void>;

const WAIT = "55s"; // just under the daemon's 60s ceiling
const BACKOFF_MS = 2_000;
// Refusals about who is asking, which asking again cannot change: an unknown
// credential, an identity that may not read here, and somebody else already
// holding the inbox.
const PERMANENT = new Set([401, 403, 409]);

export type Push = { readonly running: () => boolean; stop: () => void; done: Promise<void> };

export function startPush(bus: Bus, deliver: Deliver, log: (s: string) => void): Push {
  let stopped = false;
  const abort = new AbortController();
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
        // A configuration mistake, not a transient one. Retrying 409 would
        // steal the message from whoever legitimately owns the read; retrying
        // a refused credential costs the daemon one refusal per backoff for as
        // long as the session lives, and never delivers anything — which is
        // what a session outliving its principal did, at a refusal every two
        // seconds for a day.
        if (err instanceof BusError && PERMANENT.has(err.status)) {
          log(err.status === 409
            ? `push: ${bus.name} already has a reader; not starting a second one`
            : `push: ${bus.name} cannot read its inbox (${err.status} ${err.message}); push is off for this session`);
          stopped = true;
          return;
        }
        log(`push: consume failed: ${err instanceof BusError ? `${err.status} ${err.message}` : err}`);
        await sleep(BACKOFF_MS);
        continue;
      }
      if (stopped) return; // a message taken after stop would be lost anyway
      if (!e) continue; // deadline, no message
      try {
        await deliver(e);
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

export function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms));
}
