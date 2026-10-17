// Testing a filter within its limit: one worker, each test answered or the
// worker terminated at the limit and replaced. A pattern is tested against
// at most the first 4 KiB of a message (README § behaviour pinned by the
// review).
import { toRegExp } from "./subs.js";

export const filterLimitMs = Number(process.env.SLACK_AGENT_FILTER_MS ?? "50");
const maxText = 4096;

export type Verdict = "hit" | "miss" | "slow";

let worker: Worker | undefined;
let next = 0;
const waiting = new Map<number, (hit: boolean) => void>();

function spawnWorker(): Worker {
  // Beside this file, with this file's own extension: the source runs the
  // .ts worker, the bundle package.sh builds runs its .js one.
  const ext = import.meta.url.endsWith(".js") ? ".js" : ".ts";
  const w = new Worker(new URL("./filter-worker" + ext, import.meta.url).href);
  w.onmessage = (event: MessageEvent) => {
    const { id, hit } = event.data as { id: number; hit: boolean };
    waiting.get(id)?.(hit);
    waiting.delete(id);
  };
  w.unref?.();
  return w;
}

/** A subscription's filter against one message's text, or "slow" when it ran
 *  past the limit — the worker is then gone, and the next test gets a new one. */
export function testFilter(filter: string, text: string): Promise<Verdict> {
  const re = toRegExp(filter);
  const id = ++next;
  worker ??= spawnWorker();
  const w = worker;
  return new Promise(resolve => {
    const timer = setTimeout(() => {
      waiting.delete(id);
      w.terminate();
      if (worker === w) worker = undefined;
      resolve("slow");
    }, filterLimitMs);
    waiting.set(id, hit => {
      clearTimeout(timer);
      resolve(hit ? "hit" : "miss");
    });
    w.postMessage({ id, pattern: re.source, flags: re.flags, text: text.slice(0, maxText) });
  });
}

export function stopFilters(): void {
  worker?.terminate();
  worker = undefined;
}
