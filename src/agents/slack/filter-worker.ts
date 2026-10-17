// The filter worker: a subscription's regex is tested here, off the event
// loop that acks Slack and serves the bus. The main thread terminates the
// worker when a test runs past its limit and disables the subscription that
// needed it (Plans/R1.2/slack/README.md#behaviour-pinned-by-the-review).
declare const self: Worker;

self.onmessage = (event: MessageEvent) => {
  const { id, pattern, flags, text } = event.data as { id: number; pattern: string; flags: string; text: string };
  let hit = false;
  try {
    hit = new RegExp(pattern, flags).test(text);
  } catch {
    hit = false; // validated at add and edit; a pattern that still fails matches nothing
  }
  self.postMessage({ id, hit });
};
