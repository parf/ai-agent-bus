// Push into a live opencode 2 session, over its HTTP server.
//
// **opencode is already the topology the other two adapters had to build.**
// Its TUI is a client of a local server, so there is no second process to
// reconcile: the launcher starts `opencode serve`, the person's TUI connects
// to it with `opencode --server <url>`, and a prompt posted to that server
// lands in the turn they are watching (docs/06-launchers.md#adapters).
// opencode 1.x is still spoken by opencode1.ts, behind `ab-opencode1`.
//
// | | |
// |---|---|
// | transport | HTTP only: one `GET /api/event` stream exists, but it is volatile by contract and nothing here needs it, so this client holds no long-lived connection |
// | push | `POST /api/session/{id}/prompt` with `delivery: "queue"` — the server's durable inbox admits it at once (200) and runs the turn when the session is idle, so a message never corrupts an in-flight turn |
// | liveness | `session.execution.*` events exist; the bus reply is correlated by the session itself, so acceptance — not completion — is what this adapter acknowledges |
// | selection | the launcher starts the TUI on the same explicit session ID it binds here |
// | shapes | every response is wrapped: `{data: …}`; `204` has no body |
//
// The one thing worth being careful about: the server the launcher starts is
// on loopback with a password, because anything that can reach it can drive
// the session. `OPENCODE_SERVER_PASSWORD` is how opencode itself wants that
// said, so it is what this uses — never a flag, which `ps` would show.
// Unauthenticated `/config` now answers the web UI with 200, so the probe
// must ask for `/api/config`, which only a v2 server with the password has.

import { resolve } from "node:path";

export type Session = { id: string; title?: string | null; parentID?: string | null; location?: { directory?: string } };

export class OpencodeError extends Error {
  constructor(readonly status: number, message: string) {
    super(message);
  }
}

export class Opencode {
  readonly #cwd: string;
  readonly #url: string;
  // ok marks a status line rather than a problem; the launcher colours it.
  readonly #log: (s: string, ok?: boolean) => void;
  readonly #auth?: string;
  #session?: string;

  constructor(cwd: string, log: (s: string, ok?: boolean) => void, url: string, password?: string, username = "opencode") {
    this.#cwd = resolve(cwd);
    this.#log = log;
    this.#url = url.replace(/\/+$/, "");
    if (password) this.#auth = "Basic " + Buffer.from(`${username}:${password}`).toString("base64");
  }

  // Handshake is a plain GET: if the config documents come back, the server
  // is up, speaks v2 and the password is right.
  /** Is the server up, v2 and the password right? Used while it is starting. */
  async probe(timeoutMs = 300): Promise<boolean> {
    try { await this.#request("GET", "/api/config", undefined, timeoutMs); return true; }
    catch { return false; }
  }

  async start(): Promise<void> {
    await this.#request("GET", "/api/config");
    this.#log(`opencode: attached to ${this.#url}; the launcher binds the session for ${this.#cwd}`, true);
  }

  /** Sessions in this directory, newest first. Children are not sessions a person types in. */
  async sessions(): Promise<Session[]> {
    const page = await this.#request<{ data: Session[] }>("GET", `/api/session?directory=${encodeURIComponent(this.#cwd)}`);
    return (page.data ?? []).filter(s => !s.parentID && (!s.location?.directory || resolve(s.location.directory) === this.#cwd));
  }

  async openSession(): Promise<Session> {
    // Created in the server's own location, which is the launch directory.
    const page = await this.#request<{ data: Session }>("POST", "/api/session", {});
    this.#session = page.data.id;
    return page.data;
  }

  bind(id: string): void {
    this.#session = id;
  }

  get session(): string | undefined {
    return this.#session;
  }

  /** Start a configured MCP server again after its process died. The server's default location is the launch directory. */
  async reconnectMcp(name: string): Promise<void> {
    await this.#request("POST", `/api/experimental/mcp/${encodeURIComponent(name)}/disconnect`);
    await this.#request("POST", `/api/experimental/mcp/${encodeURIComponent(name)}/connect`);
  }

  async rename(title: string): Promise<void> {
    if (!this.#session) throw new Error("opencode: no session bound yet");
    await this.#request("PATCH", `/api/session/${encodeURIComponent(this.#session)}`, { title });
  }

  async title(): Promise<string | undefined> {
    if (!this.#session) return undefined;
    const page = await this.#request<{ data: Session }>("GET", `/api/session/${encodeURIComponent(this.#session)}`);
    return page.data.title || undefined;
  }

  // Put text in front of the session. The prompt endpoint is asynchronous by
  // design: the durable inbox takes the message at once, so the request
  // cannot be held hostage by a running turn. `queue` waits for idle rather
  // than steering the turn, and the reply comes back over the bus, not over
  // this request — which is why acceptance is all this adapter acknowledges.
  async deliver(text: string, id: string): Promise<void> {
    if (!this.#session) throw new Error("opencode: no session bound yet");
    await this.#request("POST", `/api/session/${encodeURIComponent(this.#session)}/prompt`, {
      text,
      // Identifies the turn in the session's own history as ours.
      metadata: { source: "agent-bus", message_id: id },
      delivery: "queue",
    });
  }

  // No event stream and no polling live here: v2 makes it volatile by
  // contract, and nothing the launcher does needs per-turn state. Kept as a
  // method because the launcher's cleanup calls it unconditionally.
  stop(): void {}

  async #request<T = any>(method: string, path: string, body?: unknown, timeoutMs = 30_000): Promise<T> {
    const headers: Record<string, string> = {};
    if (this.#auth) headers.authorization = this.#auth;
    if (body !== undefined) headers["content-type"] = "application/json";
    let response: Response;
    try {
      response = await fetch(this.#url + path, {
        method, headers,
        body: body === undefined ? undefined : JSON.stringify(body),
        signal: AbortSignal.timeout(timeoutMs),
      });
    } catch (e) {
      throw new Error(`opencode: ${method} ${path} failed: ${e instanceof Error ? e.message : e}`);
    }
    if (!response.ok) {
      throw new OpencodeError(response.status, `opencode: ${method} ${path} answered ${response.status} ${(await response.text()).slice(0, 200)}`);
    }
    if (response.status === 204) return undefined as T;
    const text = await response.text();
    return (text ? JSON.parse(text) : undefined) as T;
  }
}
