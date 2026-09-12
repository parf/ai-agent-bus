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

export type Push = { readonly running: () => boolean; stop: () => void };

export function startPush(bus: Bus, deliver: Deliver, log: (s: string) => void): Push {
  let stopped = false;
  const abort = new AbortController();
  const stop = () => {
    if (stopped) return;
    stopped = true;
    abort.abort(); // let go of the inbox now, rather than at the deadline
  };

  void (async () => {
    while (!stopped) {
      let e: Envelope | null = null;
      try {
        e = await bus.consume({ wait: WAIT }, abort.signal);
      } catch (err) {
        if (stopped) return;
        // 409 is somebody else holding this inbox. That is a configuration
        // mistake, not a transient one, and a loop that keeps retrying would
        // steal the message from whoever legitimately owns the read.
        if (err instanceof BusError && err.status === 409) {
          log(`push: ${bus.name} already has a reader; not starting a second one`);
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

  return { running: () => !stopped, stop };
}

export function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms));
}
