// The one way the face reaches the daemon. A refusal carries the daemon's own
// sentence; a transport failure never carries its text anywhere a visitor
// sees, because that text names the socket or the address.

// Every answer is parsed with `exact`: an integer past 2^53 arrives as its
// digit string, not a rounded number, so code reading a daemon number that
// may be that large must treat it as text.
export class Refusal extends Error {
  constructor(readonly status: number, readonly detail: string) { super(`${status} ${detail}`); }
}
export class Unreachable extends Error {
  constructor(readonly cause: unknown) { super("the daemon did not answer"); }
}

export type CallOptions = {
  cred?: string;                          // a typed token or a session id
  query?: Record<string, string | undefined>;
  body?: unknown;
  headers?: Record<string, string>;
  timeoutMs?: number;
};

export class Daemon {
  constructor(readonly addr: string) {}

  async call<T = any>(method: string, path: string, o: CallOptions = {}): Promise<T> {
    const q = new URLSearchParams();
    for (const [k, v] of Object.entries(o.query ?? {})) if (v != null) q.set(k, v);
    const qs = q.size ? `?${q}` : "";
    const headers: Record<string, string> = { ...(o.headers ?? {}) };
    if (o.cred) headers["X-Agent-Bus-Token"] = o.cred;
    let body: string | undefined;
    if (o.body !== undefined) { body = JSON.stringify(o.body); headers["Content-Type"] = "application/json"; }
    const socket = !/^https?:\/\//.test(this.addr);
    const url = socket ? `http://unix${path}${qs}` : `${this.addr.replace(/\/$/, "")}${path}${qs}`;
    let res: Response;
    try {
      res = await fetch(url, {
        method, headers, body,
        signal: AbortSignal.timeout(o.timeoutMs ?? 120_000),
        ...(socket ? { unix: this.addr } : {}),
      } as RequestInit);
    } catch (e) {
      console.error(`daemon ${method} ${path}: ${e}`);
      throw new Unreachable(e);
    }
    const text = await res.text();
    if (!res.ok) {
      let detail = text.trim();
      try { const j = JSON.parse(text); if (typeof j?.error === "string") detail = j.error; } catch {}
      throw new Refusal(res.status, detail);
    }
    if (!text) return undefined as T;
    try { return JSON.parse(text, exact) as T; } catch { return text as T; }
  }
}

/** A key-value int reaches ±2^62, past what a JS number holds exactly: an
 *  integer that does not fit is kept as its digits, never rounded. */
function exact(_key: string, v: unknown, ctx?: { source?: string }): unknown {
  return typeof v === "number" && !Number.isSafeInteger(v) && ctx?.source && /^-?\d+$/.test(ctx.source) ? ctx.source : v;
}
