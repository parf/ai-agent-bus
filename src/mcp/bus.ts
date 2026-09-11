import os from "node:os";

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
  // A receipt is an ordinary message that says "got it" or "finished", not
  // an answer — a face that cannot tell the two apart hands a model an empty
  // body and leaves the real answer queued
  // (docs/04-messaging.md#receipts).
  receipt?: "ack" | "done";
  re?: string;
  reply_to?: { service: string; topic?: string; tag?: string };
};

export type Record_ = { name: string; kind: string; addr?: string; descr?: string; owner: string;
  // How to call it, and whether anything is actually serving it. A registry
  // entry says a name exists; these say whether a call through the bus will
  // reach anyone (docs/05-discovery.md#what-a-listing-answers).
  protocol?: string; reading?: boolean; queued?: number;
  // The digest of a configuration, never the configuration itself: it is
  // how a caller sees that a service is configured, and that its setup
  // still matches the one it knew (docs/03-services-and-topics.md#why-a-digest-at-all).
  config_sha?: string };

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
    this.name = (env.AGENT_BUS_NAME || defaultName(env)).trim().toLowerCase();
    this.#token = env.AGENT_BUS_TOKEN ?? "";
    this.#addr = env.AGENT_BUS_ADDR ?? defaultSocket(env);
    if (!this.#token) throw new Error("set AGENT_BUS_TOKEN");
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
  async send(msg: { to: string; body: string; topic?: string; tag?: string; receipt?: string; re?: string }): Promise<Envelope> {
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

// A session that was launched by a plugin manifest cannot be told its own
// name, so it derives one: runtime plus where it is working, which is how a
// human refers to a session anyway. V1 names its channels the same way.
// The rule is docs/01-identity.md#names — a-z0-9._- either side, 64 total.
export function defaultName(env: NodeJS.ProcessEnv = process.env): string {
  const realm = slug(env.AGENT_BUS_REALM || hostname());
  const runtime = slug(env.AGENT_BUS_RUNTIME || "agent");
  const where = slug(env.AGENT_BUS_CWD || process.cwd());
  // Trim from the front: the tail of a path is the part that identifies it.
  const room = MAX_NAME - realm.length - 1 - runtime.length - 1;
  const tail = where.length > room ? where.slice(where.length - room) : where;
  return `${runtime}.${trimEdges(tail)}@${realm}`;
}

const MAX_NAME = 64;

function slug(s: string): string {
  return trimEdges(
    s.normalize("NFKD").replace(/[^\x20-\x7e]/g, "").toLowerCase().replace(/[^a-z0-9._-]+/g, "-"),
  );
}

// The first character must be a letter or a digit.
function trimEdges(s: string): string {
  return s.replace(/^[^a-z0-9]+/, "").replace(/[^a-z0-9]+$/, "") || "x";
}

function hostname(): string {
  return os.hostname() || "localhost";
}

function defaultSocket(env: NodeJS.ProcessEnv): string {
  const dir = env.XDG_RUNTIME_DIR;
  return dir ? `${dir}/agent-bus/bus.sock` : `/tmp/agent-bus-${process.getuid?.() ?? 0}.sock`;
}
