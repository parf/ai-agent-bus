// The daemon, as seen from TypeScript. Every call carries the same two
// parameters as any other client — see docs/02-access.md#two-parameters.
// HTTP+JSON over a unix socket or loopback TCP: bun's fetch speaks both.

export type Envelope = {
  message_id: string;
  from: string;
  to: string;
  topic?: string;
  tag?: string;
  body: string;
  at: string;
};

export type Record_ = { name: string; kind: string; addr?: string; descr?: string; owner: string };

export class BusError extends Error {
  constructor(readonly status: number, message: string) {
    super(message);
  }
}

export class Bus {
  readonly name: string;
  readonly #token: string;
  readonly #addr: string;

  constructor(env = process.env) {
    this.name = (env.AGENT_BUS_NAME ?? "").trim().toLowerCase();
    this.#token = env.AGENT_BUS_TOKEN ?? "";
    this.#addr = env.AGENT_BUS_ADDR ?? defaultSocket(env);
    if (!this.name || !this.#token) {
      throw new Error("set AGENT_BUS_NAME (user@realm) and AGENT_BUS_TOKEN");
    }
  }

  async #call(method: string, path: string, body?: unknown, signal?: AbortSignal): Promise<any> {
    const overTCP = this.#addr.startsWith("http://");
    const url = (overTCP ? this.#addr.replace(/\/$/, "") : "http://localhost") + path;
    const res = await fetch(url, {
      method,
      ...(signal ? { signal } : {}),
      ...(overTCP ? {} : { unix: this.#addr }),
      headers: {
        "X-Agent-Bus-User": this.name,
        "X-Agent-Bus-Token": this.#token,
        "Content-Type": "application/json",
      },
      ...(body === undefined ? {} : { body: JSON.stringify(body) }),
    });
    if (res.status === 204) return null; // nothing arrived before the deadline

    const text = await res.text();
    if (!res.ok) throw new BusError(res.status, text.trim() || res.statusText);
    return text ? JSON.parse(text) : null;
  }

  register(rec: { name: string; kind?: string; addr?: string; descr?: string }): Promise<Record_> {
    return this.#call("POST", "/register", rec);
  }

  ls(kind?: string): Promise<Record_[]> {
    return this.#call("GET", "/ls" + (kind ? `?kind=${encodeURIComponent(kind)}` : ""));
  }

  // A send that fails after the request left is not a send that did not
  // happen. The daemon does not deduplicate, so the caller is told the
  // outcome is unknown rather than invited to resend
  // (docs/12-stages.md#poc: no persistence, no retries).
  async send(msg: { to: string; body: string; topic?: string; tag?: string }): Promise<Envelope> {
    try {
      return await this.#call("POST", "/send", msg);
    } catch (err) {
      if (err instanceof BusError) throw err; // the daemon answered: it did not accept
      throw new Error(`delivery outcome unknown, do not automatically resend: ${err}`);
    }
  }

  consume(
    opts: { topic?: string; tag?: string; wait?: string } = {},
    signal?: AbortSignal,
  ): Promise<Envelope | null> {
    const q = new URLSearchParams();
    for (const [k, v] of Object.entries(opts)) if (v) q.set(k, v);
    return this.#call("GET", "/consume" + (q.size ? `?${q}` : ""), undefined, signal);
  }
}

function defaultSocket(env: NodeJS.ProcessEnv): string {
  const dir = env.XDG_RUNTIME_DIR;
  return dir ? `${dir}/agent-bus/bus.sock` : `/tmp/agent-bus-${process.getuid?.() ?? 0}.sock`;
}
