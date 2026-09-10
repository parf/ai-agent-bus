// Push into a live Codex session, through the App Server.
//
// **Which App Server matters more than the protocol.** Spawning our own gets
// a JSON-RPC connection that can resume a thread's saved history — but that
// is a second process, and steering it does not reach the session a person is
// typing in. V1 solved this by topology, not by cleverness
// (`/rd/bin/ai-codex`): a launcher starts one App Server, the TUI attaches to
// it with `--remote`, and the notifier attaches to the same one. Steering
// then reaches the live turn because it is the same server.
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
// socket — which is exactly what forced V1 onto Node. A loopback port avoids
// it, and the App Server binds localhost only, which is the PoC's exposure
// rule anyway (docs/12-stages.md#poc).
//
// The call sequence is V1's, minus the JetStream parts: initialize →
// thread/list newest for this cwd → thread/resume, else thread/start → per
// message turn/steer if a turn is running, else turn/start.

import { resolve } from "node:path";

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
  #next = 1;
  #pending = new Map<number, { ok: (v: any) => void; fail: (e: Error) => void; timer: Timer }>();
  #thread?: string;
  #activeTurn?: string;
  // Bumped by every notification that changes the active turn, so a slow RPC
  // response cannot resurrect a turn that has already completed.
  #turnEpoch = 0;
  #closed = false;
  readonly #cwd: string;
  readonly #url?: string;
  readonly #log: (s: string) => void;

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
  // need V1's re-selection machinery; PoC says restart the face
  // (Plans/PoC/DONE.md: B).
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
      ? await this.#request<{ thread: Thread }>("thread/resume", { threadId: found.id }, 60_000)
      : await this.#request<{ thread: Thread }>("thread/start", { cwd: this.#cwd });
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
    const r = await this.#request<{ turn: Turn }>("turn/start", common);
    this.#setActiveTurn(r.turn.id, epoch);
    return "turn/start";
  }

  stop(): void {
    if (this.#closed) return;
    this.#closed = true;
    for (const p of this.#pending.values()) {
      clearTimeout(p.timer);
      p.fail(new Error("codex: closed"));
    }
    this.#pending.clear();
    this.#wire?.close();
    this.#proc?.kill();
  }

  // Ask the server what is actually running, rather than guessing from a
  // rejection — V1's lesson, in one call.
  async #refresh(): Promise<void> {
    try {
      const r = await this.#request<{ thread: Thread }>("thread/resume", { threadId: this.#thread }, 60_000);
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
    const id = this.#next++;
    const line = JSON.stringify({ jsonrpc: "2.0", id, method, params }) + "\n";
    return new Promise<T>((ok, fail) => {
      const timer = setTimeout(() => {
        this.#pending.delete(id);
        fail(new Error(`codex: ${method} timed out after ${timeoutMs}ms`));
      }, timeoutMs);
      // Registered before the write, so a fast answer cannot arrive first.
      this.#pending.set(id, { ok, fail, timer });
      this.#write(line).catch((e) => {
        clearTimeout(timer);
        this.#pending.delete(id);
        fail(e);
      });
    });
  }

  #notify(method: string, params?: unknown): void {
    this.#write(JSON.stringify({ jsonrpc: "2.0", method, params }) + "\n").catch((e) =>
      this.#log(`codex: could not send ${method}: ${e}`),
    );
  }

  #respond(id: number | string, error: { code: number; message: string }): void {
    this.#write(JSON.stringify({ jsonrpc: "2.0", id, error }) + "\n").catch((e) =>
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
      const decoder = new TextDecoder();
      let buf = "";
      for await (const chunk of stream) {
        buf += decoder.decode(chunk, { stream: true });
        let nl: number;
        while ((nl = buf.indexOf("\n")) >= 0) {
          const line = buf.slice(0, nl).trim();
          buf = buf.slice(nl + 1);
          if (line) this.#dispatch(line);
        }
      }
      this.#gone("the app-server exited");
    } catch (err) {
      this.#gone(`the app-server connection failed: ${err}`);
    }
  }

  async #drainStderr(stream: ReadableStream<Uint8Array>): Promise<void> {
    try {
      const decoder = new TextDecoder();
      for await (const chunk of stream) {
        const s = decoder.decode(chunk).trim();
        if (s) this.#log(`codex stderr: ${s}`);
      }
    } catch { /* the process is gone; #gone has already been said */ }
  }

  // Nobody is going to answer what is in flight.
  #gone(why: string): void {
    if (this.#closed) return;
    this.#closed = true;
    this.#log(`codex: ${why}`);
    for (const [id, p] of this.#pending) {
      clearTimeout(p.timer);
      p.fail(new Error(`codex: ${why}`));
      this.#pending.delete(id);
    }
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
      const p = typeof msg.id === "number" ? this.#pending.get(msg.id) : undefined;
      if (!p) return;
      clearTimeout(p.timer);
      this.#pending.delete(msg.id as number);
      if (msg.error) p.fail(new AppServerError(msg.error.code, msg.error.message));
      else p.ok(msg.result);
      return;
    }
    // A message with both an id and a method is a *request*: the server is
    // waiting for us. Refusing is the honest answer — this client approves
    // nothing, and silence would hang the turn.
    if (msg.id !== undefined && msg.method) {
      this.#log(`codex: refusing server request ${msg.method}`);
      this.#respond(msg.id, { code: -32601, message: `agent-bus does not implement ${msg.method}` });
      return;
    }
    if (msg.method) this.#notification(msg.method, msg.params);
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
