// Push into a live opencode session, over its HTTP server.
//
// **opencode is already the topology the other two adapters had to build.**
// Its TUI is a client of a local server, so there is no second process to
// reconcile: the launcher starts `opencode serve`, the person's TUI attaches
// to it with `opencode attach <url>`, and a prompt posted to that server
// lands in the turn they are watching (docs/08-runner-role.md#adapters).
//
// | | |
// |---|---|
// | transport | HTTP, plus one `GET /event` stream. No JSON-RPC, no WebSocket |
// | push | `POST /session/{id}/prompt_async` — accepted with `204`, the turn runs on its own |
// | liveness | `session.idle` and `session.status` say when a turn ends, which neither of the other two runtimes tells us |
// | selection | the launcher attaches the TUI to the same explicit session ID it binds here |
//
// The one thing worth being careful about: the server the launcher starts is
// on loopback with a password, because anything that can reach it can drive
// the session. `OPENCODE_SERVER_PASSWORD` is how opencode itself wants that
// said, so it is what this uses — never a flag, which `ps` would show.

import { resolve } from "node:path";

import { lines } from "./rpc.ts";

export type Session = { id: string; title?: string | null; directory?: string; parentID?: string };

export class OpencodeError extends Error {
  constructor(readonly status: number, message: string) {
    super(message);
  }
}

export class Opencode {
  readonly #cwd: string;
  readonly #url: string;
  readonly #log: (s: string) => void;
  readonly #auth?: string;
  #session?: string;
  #busy = false;
  #events?: AbortController;
  #closed = false;

  constructor(cwd: string, log: (s: string) => void, url: string, password?: string, username = "opencode") {
    this.#cwd = resolve(cwd);
    this.#log = log;
    this.#url = url.replace(/\/+$/, "");
    if (password) this.#auth = "Basic " + Buffer.from(`${username}:${password}`).toString("base64");
  }

  get session(): string | undefined {
    return this.#session;
  }

  get busy(): boolean {
    return this.#busy;
  }

  // Handshake is a plain GET: if the config comes back, the server is up and
  // the password is right. Then the event stream, which is what makes this
  // adapter able to answer "has the turn finished" at all.
  /** Is the server up and the password right? Used while it is starting. */
  async probe(timeoutMs = 300): Promise<boolean> {
    try { await this.#request("GET", "/config", undefined, timeoutMs); return true; }
    catch { return false; }
  }

  async start(): Promise<void> {
    await this.#request("GET", "/config");
    void this.#listen();
    this.#log(`opencode: attached to ${this.#url}; the launcher binds the session for ${this.#cwd}`);
  }

  /** Sessions in this directory, newest first. Children are not sessions a person types in. */
  async sessions(): Promise<Session[]> {
    const all = await this.#request<Session[]>("GET", `/session?directory=${encodeURIComponent(this.#cwd)}`);
    return all.filter(s => !s.parentID && (!s.directory || resolve(s.directory) === this.#cwd));
  }

  async openSession(id?: string): Promise<Session> {
    const session = id
      ? await this.#request<Session>("GET", `/session/${encodeURIComponent(id)}`)
      : await this.#request<Session>("POST", `/session?directory=${encodeURIComponent(this.#cwd)}`, {});
    this.#session = session.id;
    return session;
  }

  bind(id: string): void {
    this.#session = id;
  }

  async rename(title: string): Promise<void> {
    if (!this.#session) throw new Error("opencode: no session bound yet");
    await this.#request("PATCH", `/session/${encodeURIComponent(this.#session)}`, { title });
  }

  async title(): Promise<string | undefined> {
    if (!this.#session) return undefined;
    const session = await this.#request<Session>("GET", `/session/${encodeURIComponent(this.#session)}`);
    return session.title || undefined;
  }

  // Put text in front of the session. `prompt_async` answers 204 and lets the
  // turn run, which is what a push wants: the reply comes back over the bus,
  // not over this request. opencode admits a prompt while a turn is running,
  // so unlike Codex there is no steer-or-start decision to get wrong.
  async deliver(text: string, id: string): Promise<void> {
    if (!this.#session) throw new Error("opencode: no session bound yet");
    await this.#request("POST", `/session/${encodeURIComponent(this.#session)}/prompt_async?directory=${encodeURIComponent(this.#cwd)}`, {
      parts: [{ type: "text", text }],
      // Identifies the turn in the session's own history as ours.
      metadata: { source: "agent-bus", message_id: id },
    });
  }

  stop(): void {
    if (this.#closed) return;
    this.#closed = true;
    this.#events?.abort();
  }

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

  // Server-sent events. Only status and idle matter here, and everything else — the
  // token deltas, the tool calls — is the session's business, not ours.
  async #listen(): Promise<void> {
    while (!this.#closed) {
      this.#events = new AbortController();
      try {
        const headers: Record<string, string> = { accept: "text/event-stream" };
        if (this.#auth) headers.authorization = this.#auth;
        const response = await fetch(this.#url + "/event", { headers, signal: this.#events.signal });
        if (!response.ok || !response.body) throw new Error(`event stream answered ${response.status}`);
        for await (const line of lines(response.body)) {
          if (!line.startsWith("data:")) continue;
          try { this.#event(JSON.parse(line.slice(5).trim())); } catch { /* a keepalive or a partial frame */ }
        }
      } catch (e) {
        if (this.#closed) return;
        this.#log(`opencode: event stream dropped (${e instanceof Error ? e.message : e}); reconnecting`);
      }
      if (this.#closed) return;
      await new Promise(r => setTimeout(r, 1_000));
    }
  }

  #event(event: { type?: string; properties?: any }): void {
    const p = event.properties ?? {};
    switch (event.type) {
      case "session.idle":
        if (p.sessionID === this.#session) this.#busy = false;
        return;
      case "session.status":
        if (p.sessionID === this.#session) this.#busy = !!p.status && Object.keys(p.status).length > 0;
        return;
    }
  }
}
