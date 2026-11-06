// The bot's own Telegram id, so the receiver can tell the bot's own posts
// from everybody else's (receive.ts). The agent serves without it: an unknown
// id drops nothing, it only lets the bot's own posts through.

export type IdentityOptions = {
  /** First wait after a failed ask; it doubles up to capMs. */
  firstMs?: number;
  capMs?: number;
  /** Shutdown: no further ask is begun and no timer is left behind. */
  signal?: AbortSignal;
  log: (line: string) => void;
};

/** Asks getMe until it names the bot, waiting firstMs, doubling to capMs,
 *  between failed asks. It logs once when the id is unknown and once when it
 *  is learned, never per retry. Resolves with the id, or undefined on stop. */
export async function learnBotId(ask: (signal?: AbortSignal) => Promise<any>, set: (id: string) => void, o: IdentityOptions): Promise<string | undefined> {
  const cap = o.capMs ?? 30_000;
  let wait = o.firstMs ?? 1000, warned = false;
  while (!o.signal?.aborted) {
    let why: string;
    try {
      // The ask gets the stop signal, and a stop ends the wait for it: an answer that comes after a
      // stop is never used, and nothing is set or said once the agent is stopping.
      const me = await untilStopped(ask(o.signal), o.signal);
      if (o.signal?.aborted) return undefined;
      if (me?.ok === true && me.result?.id !== undefined && me.result.id !== null) {
        const id = String(me.result.id);
        set(id);
        if (warned) o.log("the bot's Telegram identity is known now");
        return id;
      }
      why = me?.ok === false ? String(me.description ?? me.error ?? "refused").slice(0, 120) : "the answer had no bot id";
    } catch (e) {
      if (o.signal?.aborted) return undefined;
      why = e instanceof Error ? e.message.slice(0, 120) : "getMe failed";
    }
    if (!warned) {
      o.log(`warning: the bot's Telegram identity is unknown (${why}); serving on and asking again, at most every ${Math.round(cap / 1000)} s. Until then the bot's own posts are not told apart.`);
      warned = true;
    }
    if (!(await pause(wait, o.signal))) return undefined;
    wait = Math.min(wait * 2, cap);
  }
  return undefined;
}

/** Waits ms, or less on abort; true when the wait ran out. */
function pause(ms: number, signal?: AbortSignal): Promise<boolean> {
  return new Promise(done => {
    if (signal?.aborted) return done(false);
    const timer = setTimeout(() => { signal?.removeEventListener("abort", stop); done(true); }, ms);
    const stop = () => { clearTimeout(timer); done(false); };
    signal?.addEventListener("abort", stop, { once: true });
  });
}

/** p, or undefined as soon as signal aborts; a late settle of p is dropped. */
function untilStopped<T>(p: Promise<T>, signal?: AbortSignal): Promise<T | undefined> {
  if (!signal) return p;
  if (signal.aborted) { p.catch(() => {}); return Promise.resolve(undefined); }
  return new Promise((resolve, reject) => {
    const stop = () => { p.catch(() => {}); resolve(undefined); };
    signal.addEventListener("abort", stop, { once: true });
    p.then(v => { signal.removeEventListener("abort", stop); resolve(v); }, e => { signal.removeEventListener("abort", stop); reject(e); });
  });
}
