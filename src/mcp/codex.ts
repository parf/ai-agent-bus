// Push into a live Codex session, through the App Server.
//
// **Which App Server matters more than the protocol.** Spawning our own gets
// a JSON-RPC connection that can resume a thread's saved history — but that
// is a second process, and steering it does not reach the session a person is
// typing in. V1 solved this by topology, not by cleverness
// (`/rd/bin/ai-codex`): a launcher starts one App Server, the TUI attaches to
// it with `--remote`, and the notifier attaches to the same one. Steering
// then reaches the live turn.
//
// So this attaches when it is told where (`AGENT_BUS_CODEX_WS`), and spawns
// its own only as a fallback — which is honest about being a headless thread.
//
// | Transport | What it is |
// |---|---|
// | `ws://127.0.0.1:PORT` | the shared server. **Loopback WebSocket**, spoken by bun's built-in `WebSocket` — no dependency, and none of V1's `ws` + `permessage-deflate` |
// | stdio | our own `codex app-server`, NDJSON (B.0) |
//
// `--listen unix://` is a WebSocket too, and bun cannot open one over a unix
// socket. A loopback port avoids it, and the App Server binds localhost only,
// which is the daemon's own rule.
//
// The call sequence is V1's, minus the JetStream parts: initialize →
// thread/list newest for this cwd → thread/resume, else thread/start → per
// message turn/steer if a turn is running, else turn/start.

import { resolve } from "node:path";

import { Pending, drain, lines } from "./rpc.ts";
// The only MCP server this face will approve a tool call for: its own.
const MCP_SERVER_NAME = "agent-bus";

type Rpc = {
  jsonrpc?: "2.0";
  id?: number | string;
  method?: string;
  params?: unknown;
  result?: any;
  error?: { code: number; message: string };
};
type Turn = { id: string; status?: unknown };
type Thread = { id: string; cwd?: string; turns?: Turn[] };

export class AppServerError extends Error {
  constructor(readonly code: number, message: string) {
    super(message);
  }
}

// Only what both transports need: a line out, lines in, and an end.
type Wire = {
  write(line: string): Promise<void>;
  close(): void;
};

export class Codex {
  #wire?: Wire;
  #proc?: ReturnType<typeof Bun.spawn>;
  #pending = new Pending();
  #thread?: string;
  #activeTurn?: string;
  // Bumped by every notification that changes the active turn, so a slow RPC
  // response cannot resurrect a turn that has already completed.
  #turnEpoch = 0;
  #closed = false;
  readonly #cwd: string;
  readonly #url?: string;
  readonly #log: (s: string) => void;
  // Approvals go to the person in the TUI, never to us. Without this the App
  // Server asks *the client that started the turn* — this face — and a face
  // that approves nothing turns every tool call the session makes into "user
  // rejected". Observed live: it killed the session's own ab_reply.
  // `approvalPolicy` matches what the session is already configured for; it
  // is not a way to switch protections off, which is why it is read and not
  // assumed.
  readonly #policy = {
    approvalPolicy: (process.env.AGENT_BUS_CODEX_APPROVAL ?? "on-request") as "never" | "on-request",
    approvalsReviewer: "user" as const,
  };

  constructor(cwd: string, log: (s: string) => void, url = process.env.AGENT_BUS_CODEX_WS) {
    this.#cwd = resolve(cwd);
    this.#log = log;
    this.#url = url?.trim() || undefined;
  }

  get thread(): string | undefined {
    return this.#thread;
  }

  get shared(): boolean {
    return !!this.#url;
  }

  // Connect and handshake only. The thread is chosen at the first delivery,
  // not here: this face is Codex's own MCP server, so it starts *before* the
  // session has a thread, and picking one now would pick the wrong one — our
  // own, headless, instead of the one the person is typing in.
  async start(): Promise<void> {
    this.#wire = this.#url ? await this.#connect(this.#url) : this.#spawn();

    await this.#request("initialize", {
      clientInfo: { name: "agent-bus", title: "agent-bus", version: "0.1.0" },
      // The deltas are a token stream; only turn boundaries matter here.
      capabilities: {
        experimentalApi: false,
        optOutNotificationMethods: [
          "item/agentMessage/delta",
          "item/commandExecution/outputDelta",
          "item/reasoning/summaryTextDelta",
          "item/reasoning/textDelta",
        ],
      },
    });
    this.#notify("initialized");
    this.#log(
      `codex: attached to ${this.#url ? `the shared app-server ${this.#url}` : "its own app-server"}` +
        `; the thread for ${this.#cwd} is chosen at the first message`,
    );
  }

  // Newest thread for this directory, or a fresh one. Chosen once and kept:
  // following a person who opens a *new* session in the same directory would
  // need re-selection machinery nobody has asked for. Restart the face.
  async #ensureThread(): Promise<void> {
    if (this.#thread) return;
    const listed = await this.#request<{ data?: Thread[] }>("thread/list", {
      cwd: this.#cwd,
      limit: 100,
      sortKey: "recency_at",
      sortDirection: "desc",
      sourceKinds: ["cli", "vscode"],
      archived: false,
    });
    const found = (listed.data ?? []).find((t) => t.cwd && resolve(t.cwd) === this.#cwd);
    const res = found
      ? await this.#request<{ thread: Thread }>("thread/resume", { threadId: found.id, ...this.#policy }, 60_000)
      : await this.#request<{ thread: Thread }>("thread/start", { cwd: this.#cwd, ...this.#policy });
    this.#setThread(res.thread);
    this.#log(
      `codex: ${found ? "attached to the session's thread" : "no session here yet, started a thread"} ` +
        `${this.#thread} in ${this.#cwd}${this.#activeTurn ? `, turn ${this.#activeTurn} already running` : ""}`,
    );
  }

  // Put text in front of the session: steer the turn it is running, or start
  // one. `id` becomes clientUserMessageId so the message is identifiable.
  async deliver(text: string, id: string): Promise<"turn/steer" | "turn/start"> {
    await this.#ensureThread();
    if (!this.#thread) throw new Error("codex: no thread");
    const common = {
      threadId: this.#thread,
      clientUserMessageId: `agent-bus:${id}`,
      input: [{ type: "text", text, text_elements: [] }],
    };

    if (this.#activeTurn) {
      const epoch = this.#turnEpoch;
      try {
        const r = await this.#request<{ turnId: string }>("turn/steer", {
          ...common,
          expectedTurnId: this.#activeTurn,
        });
        this.#setActiveTurn(r.turnId, epoch);
        return "turn/steer";
      } catch (err) {
        // A rejection is not a licence to start a second turn: the steer may
        // have landed. Only a *protocol* rejection means our idea of the
        // active turn was wrong, and even then only after the server has
        // confirmed no turn is running. Overload and timeouts propagate.
        if (!(err instanceof AppServerError)) throw err;
        if (err.code === -32001) throw err; // overload: retrying is not the fix
        this.#log(`codex: steer rejected (${err.code} ${err.message}); asking the server what is running`);
        await this.#refresh();
        if (this.#activeTurn) throw err; // a turn *is* running — do not add another
      }
    }
    const epoch = this.#turnEpoch;
    const r = await this.#request<{ turn: Turn }>("turn/start", { ...common, ...this.#policy });
    this.#setActiveTurn(r.turn.id, epoch);
    return "turn/start";
  }

  stop(): void {
    if (this.#closed) return;
    this.#closed = true;
    this.#pending.failAll("codex: closed");
    this.#wire?.close();
    this.#proc?.kill();
  }

  // Ask the server what is actually running, rather than guessing from a
  // rejection — V1's lesson, in one call.
  async #refresh(): Promise<void> {
    try {
      const r = await this.#request<{ thread: Thread }>("thread/resume", { threadId: this.#thread, ...this.#policy }, 60_000);
      this.#setThread(r.thread);
    } catch (err) {
      this.#log(`codex: could not refresh the thread: ${err}`);
    }
  }

  #setThread(thread: Thread): void {
    this.#thread = thread.id;
    // A resumed thread may already be mid-turn; treating it as idle would
    // start a second one.
    this.#turnEpoch++;
    this.#activeTurn = latestActiveTurn(thread.turns)?.id;
  }

  // A response only sets the active turn if no notification has changed it
  // since the request went out.
  #setActiveTurn(id: string, epoch: number): void {
    if (epoch !== this.#turnEpoch) return;
    this.#activeTurn = id;
  }

  #request<T = any>(method: string, params?: unknown, timeoutMs = 30_000): Promise<T> {
    return this.#pending.request(
      (id) => this.#write(JSON.stringify({ jsonrpc: "2.0", id, method, params }) + "\n"),
      timeoutMs,
      () => new Error(`codex: ${method} timed out after ${timeoutMs}ms`),
    ) as Promise<T>;
  }

  #notify(method: string, params?: unknown): void {
    this.#write(JSON.stringify({ jsonrpc: "2.0", method, params }) + "\n").catch((e) =>
      this.#log(`codex: could not send ${method}: ${e}`),
    );
  }

  #respond(id: number | string, body: { result: unknown } | { error: { code: number; message: string } }): void {
    this.#write(JSON.stringify({ jsonrpc: "2.0", id, ...body }) + "\n").catch((e) =>
      this.#log(`codex: could not answer request ${id}: ${e}`),
    );
  }

  #write(line: string): Promise<void> {
    if (!this.#wire) return Promise.reject(new Error("codex: not started"));
    return this.#wire.write(line);
  }

  // --- transports -------------------------------------------------------

  async #connect(url: string): Promise<Wire> {
    const ws = new WebSocket(url);
    await new Promise<void>((ok, fail) => {
      const t = setTimeout(() => fail(new Error(`codex: ${url} did not open in 15s`)), 15_000);
      ws.onopen = () => { clearTimeout(t); ok(); };
      ws.onerror = () => { clearTimeout(t); fail(new Error(`codex: cannot reach the app-server at ${url}`)); };
    });
    ws.onmessage = (e) => {
      // A frame may carry more than one line.
      for (const line of String(e.data).split("\n")) if (line.trim()) this.#dispatch(line.trim());
    };
    ws.onclose = () => this.#gone("the shared app-server closed the connection");
    ws.onerror = () => this.#gone("the shared app-server connection failed");
    return {
      write: async (line) => ws.send(line),
      close: () => ws.close(),
    };
  }

  #spawn(): Wire {
    const proc = Bun.spawn(["codex", "app-server"], { stdin: "pipe", stdout: "pipe", stderr: "pipe" });
    this.#proc = proc;
    const stdin = proc.stdin as import("bun").FileSink;
    void this.#readLines(proc.stdout);
    void this.#drainStderr(proc.stderr);
    return {
      // Bun buffers a FileSink; without the flush the app-server never sees
      // the line and every request times out (B.0).
      write: async (line) => { stdin.write(line); await stdin.flush(); },
      close: () => stdin.end(),
    };
  }

  async #readLines(stream: ReadableStream<Uint8Array>): Promise<void> {
    try {
      for await (const line of lines(stream)) this.#dispatch(line);
      this.#gone("the app-server exited");
    } catch (err) {
      this.#gone(`the app-server connection failed: ${err}`);
    }
  }

  async #drainStderr(stream: ReadableStream<Uint8Array>): Promise<void> {
    try {
      await drain(stream, (s) => {
        const line = s.trim();
        if (line) this.#log(`codex stderr: ${line}`);
      });
    } catch { /* the process is gone; #gone has already been said */ }
  }

  // Nobody is going to answer what is in flight.
  #gone(why: string): void {
    if (this.#closed) return;
    this.#closed = true;
    this.#log(`codex: ${why}`);
    this.#pending.failAll(`codex: ${why}`);
  }

  #dispatch(line: string): void {
    let msg: Rpc;
    try {
      msg = JSON.parse(line) as Rpc;
    } catch {
      this.#log(`codex: unparsable line: ${line.slice(0, 200)}`);
      return;
    }
    if (msg.id !== undefined && (msg.result !== undefined || msg.error)) {
      this.#pending.settle(
        msg.id,
        msg.error ? new AppServerError(msg.error.code, msg.error.message) : null,
        msg.result,
      );
      return;
    }
    // A message with both an id and a method is a *request*: the server is
    // waiting for us, and silence hangs the turn.
    if (msg.id !== undefined && msg.method) {
      this.#serverRequest(msg.id, msg.method, msg.params);
      return;
    }
    if (msg.method) this.#notification(msg.method, msg.params);
  }

  // A turn started by a bus message has no human at the keyboard, so the
  // App Server asks *us* to approve what the session wants to do. Approving
  // in general would be handing a remote peer the user's permissions. The
  // one thing this face will approve is **a tool call into the agent-bus MCP
  // server itself** — answering the peer is what the turn is for, and that
  // answer is an ab_ call.
  // Everything else is declined and shows up in the session as such.
  #serverRequest(id: number | string, method: string, params: unknown): void {
    if (method !== "mcpServer/elicitation/request") {
      this.#log(`codex: refusing server request ${method}`);
      this.#respond(id, { error: { code: -32601, message: `agent-bus does not implement ${method}` } });
      return;
    }
    const p = (params ?? {}) as { serverName?: string; _meta?: { codex_approval_kind?: string } };
    const ours = p.serverName === MCP_SERVER_NAME && p._meta?.codex_approval_kind === "mcp_tool_call";
    this.#log(`codex: ${ours ? "approving" : "declining"} an approval for ${p.serverName ?? "?"}`);
    this.#respond(id, { result: { action: ours ? "accept" : "decline", content: {} } });
  }

  // Only turn boundaries matter: they decide steer vs start.
  #notification(method: string, params: unknown): void {
    const p = (params ?? {}) as { threadId?: string; turn?: { id?: string } };
    if (p.threadId !== this.#thread || typeof p.turn?.id !== "string") return;
    if (method === "turn/started") {
      this.#turnEpoch++;
      this.#activeTurn = p.turn.id;
    } else if (method === "turn/completed" && this.#activeTurn === p.turn.id) {
      this.#turnEpoch++;
      this.#activeTurn = undefined;
    }
  }
}

function latestActiveTurn(turns?: Turn[]): Turn | undefined {
  if (!turns?.length) return undefined;
  for (let i = turns.length - 1; i >= 0; i--) {
    const t = turns[i]!;
    const status = (t.status ?? {}) as { type?: string };
    // Anything that is not finished is a turn we must steer, not race.
    if (status.type && status.type !== "completed" && status.type !== "failed" && status.type !== "aborted") {
      return t;
    }
  }
  return undefined;
}
