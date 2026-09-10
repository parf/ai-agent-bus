// Push: the session stops polling and messages arrive on their own.
//
// One loop, two deliveries. The loop is the same either way — long-poll the
// daemon, hand the envelope to a mode — because "how a message reaches a live
// session" is an adapter concern and nothing above it changes
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
  const loop = (async () => {
    while (!stopped) {
      let e: Envelope | null = null;
      try {
        e = await bus.consume({ wait: WAIT });
      } catch (err) {
        // 409 means somebody else holds this inbox: that is a configuration
        // mistake, not a transient one, but the loop keeps trying so the
        // session recovers when the other reader goes away.
        log(`push: consume failed: ${err instanceof BusError ? `${err.status} ${err.message}` : err}`);
        await sleep(BACKOFF_MS);
        continue;
      }
      if (!e) continue; // deadline, no message
      try {
        await deliver(e);
      } catch (err) {
        // At-most-once: the daemon has already handed this message over, so a
        // failed delivery loses it. Say so rather than pretending otherwise.
        log(`push: message ${e.message_id} from ${e.from} was consumed but not delivered: ${err}`);
      }
    }
  })();
  void loop;
  return { running: () => !stopped, stop: () => (stopped = true) };
}

export function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms));
}
