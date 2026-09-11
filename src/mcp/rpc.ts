// Newline-delimited JSON-RPC, the parts that were written three times: a line
// reader over a byte stream, and the map that matches an answer to the
// request waiting for it. The Codex adapter needs both, and so does each
// harness that drives a server over stdio.
//
// This is deliberately not a JSON-RPC framework. It knows nothing about
// methods, notifications or server-initiated requests — those differ in every
// caller, and belong where they are handled. See docs/10-modules.md.

/** Whole lines out of a byte stream. Partial chunks are held until complete. */
export async function* lines(stream: ReadableStream<Uint8Array>): AsyncGenerator<string> {
  const decoder = new TextDecoder();
  let buf = "";
  for await (const chunk of stream) {
    buf += decoder.decode(chunk, { stream: true });
    let nl: number;
    while ((nl = buf.indexOf("\n")) >= 0) {
      const line = buf.slice(0, nl).trim();
      buf = buf.slice(nl + 1);
      if (line) yield line;
    }
  }
}

/**
 * Read a stream to its end, handing over each piece as it arrives. Draining
 * from the start matters: a harness that only reads stderr in its failure
 * path waits forever on a server that has not exited.
 *
 * Nothing is kept here. A caller that wants the history accumulates it —
 * a long-lived app-server's stderr would otherwise grow until it exits.
 */
export async function drain(stream: ReadableStream<Uint8Array>, onText: (s: string) => void): Promise<void> {
  const decoder = new TextDecoder();
  for await (const chunk of stream) onText(decoder.decode(chunk, { stream: true }));
}

type Entry = { ok: (v: any) => void; fail: (e: Error) => void; timer: ReturnType<typeof setTimeout> };

/**
 * The requests still waiting for an answer. `issue` reserves an id and hands
 * back the promise; the caller writes the line itself, because how a line
 * reaches the other end is the caller's business.
 */
export class Pending {
  #next = 1;
  #open = new Map<number, Entry>();

  issue(timeoutMs: number, onTimeout: () => Error): { id: number; answer: Promise<any> } {
    const id = this.#next++;
    const answer = new Promise<any>((ok, fail) => {
      const timer = setTimeout(() => {
        this.#open.delete(id);
        fail(onTimeout());
      }, timeoutMs);
      // Registered before the caller writes, so a fast answer cannot arrive
      // before anything is waiting for it.
      this.#open.set(id, { ok, fail, timer });
    });
    return { id, answer };
  }

  /**
   * issue, write, and fail the wait if the write itself failed — the shape
   * every caller had written out by hand. How a line reaches the other end
   * stays the caller's business; only the bookkeeping is shared.
   */
  request(write: (id: number) => Promise<unknown> | unknown, timeoutMs: number, onTimeout: () => Error): Promise<any> {
    const { id, answer } = this.issue(timeoutMs, onTimeout);
    const failed = (e: unknown) => this.settle(id, e instanceof Error ? e : new Error(String(e)));
    // Written now, not on the next tick: callers issue requests back to back
    // and the order they reach the wire is theirs, not the scheduler's.
    try {
      Promise.resolve(write(id)).catch(failed);
    } catch (e) {
      failed(e);
    }
    return answer;
  }

  /** An id nobody waits on — for a request whose answer is never collected. */
  reserve(): number {
    return this.#next++;
  }

  /** Hand an answer, or a failure, to whoever is waiting. */
  settle(id: unknown, err: Error | null, result?: unknown): boolean {
    if (typeof id !== "number") return false;
    const e = this.#open.get(id);
    if (!e) return false;
    clearTimeout(e.timer);
    this.#open.delete(id);
    if (err) e.fail(err);
    else e.ok(result);
    return true;
  }

  /** The other end is gone: nothing in flight will ever be answered. */
  failAll(reason: string): void {
    const err = new Error(reason);
    for (const [id, e] of this.#open) {
      clearTimeout(e.timer);
      this.#open.delete(id);
      e.fail(err);
    }
  }

  get size(): number {
    return this.#open.size;
  }
}
