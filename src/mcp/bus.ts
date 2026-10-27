import { PIN_ENV, pinnedCertificate } from "./pin.ts";
import os from "node:os";
import { statSync } from "node:fs";

export const ownerACL = "@owner";
export function runtimeTags(runtime?: string): string[] {
  const tag = runtime === "opencode1" ? "opencode" : runtime;
  return tag && ["claude","codex","opencode"].includes(tag) ? ["ai-cli", tag] : [];
}

// ab-* identities always admit the Owner's other Agents. Keep
// explicit grants an owner already added; registration is a merge, not a
// reason to erase them on launcher restart or rename.
export function withOwnerACL(allow: string[] | undefined): string[] {
  return [...new Set([...(allow ?? []), ownerACL])];
}

// The daemon, as seen from TypeScript. Every call carries a token and nothing
// else, as any other client does — docs/02-access.md#what-a-call-carries.
// HTTP+JSON over a unix socket or loopback TCP: bun's fetch speaks both.

export type Envelope = {
  message_id: string;
  from: string;
  to: string;
  topic?: string;
  tag?: string;
  body: string;
  /** What the sender holds toward the record it addressed, worked out by the daemon (Plans/R1.0-Release/roles.md#roles-are-for-agents). */
  roles?: string[];
  at: string;
  // A receipt is an ordinary message that says "got it" or "finished", not
  // an answer — a face that cannot tell the two apart hands a model an empty
  // body and leaves the real answer queued
  // (docs/04-messaging.md#receipts).
  receipt?: "ack" | "done";
  re?: string;
  // An ask is about the receiver itself, with no body: help for what it can
  // do, mcp for its tools (docs/constitution.md#agent-self-description).
  ask?: "help" | "help-config" | "help-secret" | "help-json" | "mcp" | "status";
  reply_to?: { name: string; topic?: string; tag?: string };
};

// A 📚 card's MCP descriptor (docs/03-records-resource.md#what-a-resource-is).
export type ResourceCard = { uri: string; template?: boolean; name?: string; title?: string; mimeType?: string;
  size?: number; icons?: unknown[]; annotations?: Record<string, unknown>; source?: string };

export type Record_ = { name: string; kind: string; addr?: string; descr?: string; owner: string;
  script?: string; // what agent-bus run serves an agent with (docs/06-dedicated-runner.md#script-agents)
  resource?: ResourceCard;
  // Resource management grants are a list. Older daemons used one group
  // string; the current daemon accepts that input only for migration and
  // always answers with this array form.
  maintainers?: string[];
  // Who may see and use it. A listing already leaves out what its caller may
  // not, so this is what a record says about itself, not a filter to apply
  // here (docs/02-access.md#acl).
  allow?: string[];
  // Personal records admit only their owner's cohort (docs/03-records.md#personal-and-shared).
  personal?: boolean; tags?: string[];
  // How to call it, and whether anything is actually serving it. A registry
  // entry says a name exists; these say whether a call through the bus will
  // reach anyone (docs/05-discovery.md#what-a-listing-answers).
  protocol?: string; reading?: boolean; readers?: number; queued?: number; last_used?: string;
  // The digest of a configuration, never the configuration itself: it is
  // how a caller sees that a record is configured, and that its setup
  // still matches the one it knew (docs/03-records-agent.md#why-a-digest-at-all).
  config_sha?: string };

/** The daemon's TCP port, plain or TLS, rather than a unix socket path. */
export function isTCP(addr: string): boolean {
  return addr.startsWith("http://") || addr.startsWith("https://");
}

export class BusError extends Error {
  constructor(readonly status: number, message: string) {
    super(message);
  }
}

export class Bus {
  readonly name: string;
  readonly #token: string;
  readonly #addr: string;
  readonly #env: Record<string, string | undefined>;
  #ca?: Promise<string>;

  constructor(env = process.env, allowLocal = false) {
    this.name = (env.AGENT_BUS_NAME || defaultName(env)).trim().toLowerCase();
    this.#token = env.AGENT_BUS_TOKEN ?? "";
    this.#addr = env.AGENT_BUS_ADDR ?? defaultSocket(env);
    this.#env = env;
    if (!this.#token && !(allowLocal && !isTCP(this.#addr))) throw new Error("set AGENT_BUS_TOKEN");
  }

  /** A principal's credential, asked for with this one's. The daemon allows
   *  it for a name you own, or for anyone if you are the daemon's owner.
   *  See docs/02-access.md#getting-a-token. */
  async token(name: string): Promise<string> {
    const got = await this.#call("POST", "/token", { name });
    return got.token;
  }

  /** A client for another principal. The token is the identity, so becoming
   *  somebody else means holding their credential — which a caller cannot do
   *  without being allowed to. */
  async as(name: string): Promise<Bus> {
    return new Bus({ ...this.#env, AGENT_BUS_NAME: name, AGENT_BUS_TOKEN: await this.token(name) });
  }

  async #call(method: string, path: string, body?: unknown, signal?: AbortSignal, headers: Record<string, string> = {}, raw = false): Promise<any> {
    const overTCP = isTCP(this.#addr);
    const url = (overTCP ? this.#addr.replace(/\/$/, "") : "http://localhost") + path;
    // A pinned https:// daemon: its certificate, once checked against the
    // pin, is the only one trusted (docs/02-access-remote.md#over-https).
    const pin = this.#env[PIN_ENV];
    let ca: string | undefined;
    if (this.#addr.startsWith("https://") && pin) {
      this.#ca ??= pinnedCertificate(this.#addr, pin).catch((err) => { this.#ca = undefined; throw err; });
      ca = await this.#ca;
    }
    const res = await fetch(url, {
      method,
      ...(signal ? { signal } : {}),
      ...(overTCP ? {} : { unix: this.#addr }),
      ...(ca ? { tls: { ca } } : {}),
      headers: {
        "X-Agent-Bus-Token": this.#token,
        "Content-Type": "application/json",
        ...headers,
      },
      ...(body === undefined ? {} : { body: JSON.stringify(body) }),
    }).catch((err: Error) => {
      if (signal?.aborted) throw err;
      // Nothing listening at a unix path is said as the Go CLI says it.
      const where = !overTCP && ["FailedToOpenSocket", "ENOENT"].includes((err as { code?: string }).code ?? "") ? "no daemon found at" : "failed at";
      throw new Error(`agent-bus ${method} ${path.split("?")[0]} ${where} ${this.#addr}: ${err.message}`, { cause: err });
    });
    if (res.status === 204) return null; // nothing arrived before the deadline

    const text = await res.text();
    if (!res.ok) throw new BusError(res.status, text.trim() || res.statusText);
    return raw ? text : text ? JSON.parse(text, exact) : null;
  }

  register(rec: { name: string; kind?: string; addr?: string; descr?: string; allow?: string[]; personal?: boolean; tags?: string[] }, createOnly = false): Promise<Record_> {
    return this.#call("POST", "/register", rec, undefined, createOnly ? { "If-None-Match": "*" } : {});
  }

  ls(kind?: string): Promise<Record_[]> {
    return this.#call("GET", "/ls" + (kind ? `?kind=${encodeURIComponent(kind)}` : ""));
  }

  unregister(name: string): Promise<null> { return this.#call("POST", "/unregister", { name }); }

  status(): Promise<{ you: string }> { return this.#call("GET", "/status"); }

  /** One POST the face passes through, answered as the daemon said it. */
  async post(path: string, body?: unknown, signal?: AbortSignal): Promise<unknown> { return this.#call("POST", path, body, signal); }

  /** A record's secret, as the plain text it is stored as (docs/03-records-service.md#secrets). */
  secret(name: string): Promise<string> {
    return this.#call("GET", `/secret?name=${encodeURIComponent(name)}`, undefined, undefined, {}, true);
  }

  /** One GET the face passes through, answered as the daemon said it. */
  async get(path: string, signal?: AbortSignal): Promise<unknown> { return this.#call("GET", path, undefined, signal); }

  // A send that fails after the request left is not a send that did not
  // happen. The daemon does not deduplicate, so the caller is told the
  // outcome is unknown rather than invited to resend: there are no retries
  // here, and a resend would be a second message.
  async send(msg: { to: string; body: string; topic?: string; tag?: string; receipt?: string; re?: string; ask?: string }): Promise<Envelope> {
    try {
      return await this.#call("POST", "/send", msg);
    } catch (err) {
      if (err instanceof BusError) throw err; // the daemon answered: it did not accept
      throw new Error(`delivery outcome unknown, do not automatically resend: ${err}`);
    }
  }

  consume(
    opts: { inbox?: string; topic?: string; tag?: string; wait?: string } = {},
    signal?: AbortSignal,
  ): Promise<Envelope | null> {
    const q = new URLSearchParams();
    for (const [k, v] of Object.entries(opts)) if (v) q.set(k, v);
    return this.#call("GET", "/consume" + (q.size ? `?${q}` : ""), undefined, signal);
  }
}

// A session that was launched by a plugin manifest cannot be told its own
// name, so it derives one: runtime plus where it is working, which is how a
// human refers to a session anyway. The NATS version names channels the same way.
// The rule is docs/01-identity-and-authority.md#names — a-z0-9._- either side, 64 total,
// and an Agent's name begins with "#", which counts toward the 64.
export function defaultName(env: NodeJS.ProcessEnv = process.env, separator: "." | "/" = "."): string {
  const realm = slug(env.AGENT_BUS_REALM || hostname());
  const runtime = slug(env.AGENT_BUS_RUNTIME || "agent");
  const where = slug(env.AGENT_BUS_CWD || process.cwd());
  // Trim from the front: the tail of a path is the part that identifies it.
  const room = MAX_NAME - AGENT_PREFIX.length - realm.length - 1 - runtime.length - 1;
  const tail = where.length > room ? where.slice(where.length - room) : where;
  return `${AGENT_PREFIX}${runtime}${separator}${trimEdges(tail)}@${realm}`;
}

// AGENT_PREFIX begins every Agent's name (docs/constitution.md#actors-and-ascii-textarea-syntax).
export const AGENT_PREFIX = "#";

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

// A key-value int reaches ±2^62, past what a JS number holds exactly: an
// integer that does not fit stays its own digits, and JSON.stringify writes
// them back unrounded (docs/03-records-kv.md#rules).
function exact(_key: string, v: unknown, ctx?: { source?: string }): unknown {
  return typeof v === "number" && !Number.isSafeInteger(v) && ctx?.source && /^-?\d+$/.test(ctx.source)
    ? (JSON as unknown as { rawJSON(s: string): unknown }).rawJSON(ctx.source) : v;
}

// SystemRuntimeDir in the Go CLI: where a daemon installed for the host listens.
export const SYSTEM_RUNTIME_DIR = "/run/agent-bus";

// The account this process runs as, from its uid, as the daemon names its
// socket and the Go CLI finds it. Bun's userInfo() reports $USER instead, or
// "unknown" without it, and a caller may unset or change that at will.
export function accountName(uid = process.getuid!()): string | undefined {
  const found = Bun.spawnSync(["id", "-nu", String(uid)], { stdout: "pipe", stderr: "ignore" });
  return (found.exitCode === 0 && found.stdout.toString().trim()) || undefined;
}

// The Go CLI's discovery (docs/02-access.md#local-socket): the login's
// directory, then the installed one; bus.sock with a token, the account's own
// socket without. With nothing listening it answers the installed path, never
// a guess in world-writable /tmp, where a socket anyone made would be handed
// the token; the call then fails naming that path.
export function defaultSocket(env: NodeJS.ProcessEnv): string {
  const dirs = [...(env.XDG_RUNTIME_DIR ? [`${env.XDG_RUNTIME_DIR}/agent-bus`] : []), SYSTEM_RUNTIME_DIR];
  return clientSocket(dirs, env.AGENT_BUS_TOKEN ? undefined : accountName());
}
export function clientSocket(dirs: string[], account?: string): string {
  const paths = dirs.map(dir => `${dir}/${account ? `user-${account}.sock` : "bus.sock"}`);
  return paths.find(path => { try { return statSync(path).isSocket(); } catch { return false; } }) ?? paths.at(-1)!;
}
