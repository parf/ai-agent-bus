// Push into a live Codex session, through the App Server.
//
// `codex app-server` speaks newline-delimited JSON-RPC on stdio — B.0 settled
// that, and it is why this runs on bun with no WebSocket
// (Plans/PoC/DONE.md: B.0). The sequence is V1's, minus the JetStream parts:
//
//   initialize → thread/list newest for this cwd → thread/resume, else
//   thread/start → per message: turn/steer if a turn is running, else
//   turn/start
//
// Taken from V1 because it hit the case: steering an active turn instead of
// queueing a second one, and falling back to turn/start when the server says
// the turn we expected is gone. Skipped from V1: the adapter journal,
// delivery events, ADAPTER_PENDING recovery, sandbox policy plumbing and
// thread re-selection — all of them serve a durability contract PoC does not
// have (docs/12-stages.md#poc).

import { resolve } from "node:path";

type Rpc = { jsonrpc: "2.0"; id?: number | string; method?: string; params?: unknown; result?: any; error?: { code: number; message: string } };
type Thread = { id: string; cwd?: string; turns?: { id: string; status?: unknown }[] };

export class AppServerError extends Error {
  constructor(readonly code: number, message: string) {
    super(message);
  }
}

export class Codex {
  #proc?: ReturnType<typeof Bun.spawn>;
  #stdin?: import("bun").FileSink;
  #next = 1;
  #pending = new Map<number, { ok: (v: any) => void; fail: (e: Error) => void; timer: Timer }>();
  #thread?: string;
  #activeTurn?: string;
  readonly #cwd: string;
  readonly #log: (s: string) => void;

  constructor(cwd: string, log: (s: string) => void) {
    this.#cwd = resolve(cwd);
    this.#log = log;
  }

  get thread(): string | undefined {
    return this.#thread;
  }

  async start(): Promise<string> {
    const proc = Bun.spawn(["codex", "app-server"], { stdin: "pipe", stdout: "pipe", stderr: "pipe" });
    this.#proc = proc;
    this.#stdin = proc.stdin as import("bun").FileSink;
    void this.#read(proc.stdout);
    void this.#drainStderr(proc.stderr);

    await this.#request("initialize", {
      clientInfo: { name: "agent-bus", title: "agent-bus", version: "0.1.0" },
      // The deltas are a token stream; we only care about turn boundaries.
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
    this.#thread = res.thread.id;
    this.#log(`codex: ${found ? "resumed" : "started"} thread ${this.#thread} in ${this.#cwd}`);
    return this.#thread;
  }

  // Put text in front of the session: steer the turn it is running, or start
  // one. `id` becomes clientUserMessageId so the message is identifiable in
  // the thread.
  async deliver(text: string, id: string): Promise<"turn/steer" | "turn/start"> {
    if (!this.#thread) throw new Error("codex: not started");
    const common = {
      threadId: this.#thread,
      clientUserMessageId: `agent-bus:${id}`,
      input: [{ type: "text", text, text_elements: [] }],
    };
    if (this.#activeTurn) {
      try {
        const r = await this.#request<{ turnId: string }>("turn/steer", {
          ...common,
          expectedTurnId: this.#activeTurn,
        });
        this.#activeTurn = r.turnId;
        return "turn/steer";
      } catch (err) {
        // -32001 is overload: retrying as a new turn would not help either,
        // so let it out. Anything else means our idea of the active turn is
        // stale — V1's lesson — and turn/start is the answer.
        if (err instanceof AppServerError && err.code === -32001) throw err;
        this.#log(`codex: steer rejected (${err}); starting a turn instead`);
        this.#activeTurn = undefined;
      }
    }
    const r = await this.#request<{ turn: { id: string } }>("turn/start", common);
    this.#activeTurn = r.turn.id;
    return "turn/start";
  }

  stop(): void {
    for (const p of this.#pending.values()) {
      clearTimeout(p.timer);
      p.fail(new Error("codex: closed"));
    }
    this.#pending.clear();
    this.#stdin?.end();
    this.#proc?.kill();
  }

  #request<T = any>(method: string, params?: unknown, timeoutMs = 30_000): Promise<T> {
    const id = this.#next++;
    const line = JSON.stringify({ jsonrpc: "2.0", id, method, params }) + "\n";
    return new Promise<T>((ok, fail) => {
      const timer = setTimeout(() => {
        this.#pending.delete(id);
        fail(new Error(`codex: ${method} timed out after ${timeoutMs}ms`));
      }, timeoutMs);
      this.#pending.set(id, { ok, fail, timer });
      this.#write(line).catch((e) => {
        clearTimeout(timer);
        this.#pending.delete(id);
        fail(e);
      });
    });
  }

  #notify(method: string, params?: unknown): void {
    void this.#write(JSON.stringify({ jsonrpc: "2.0", method, params }) + "\n");
  }

  // Bun buffers a FileSink; without the flush the app-server never sees the
  // line and every request times out (B.0).
  async #write(line: string): Promise<void> {
    if (!this.#stdin) throw new Error("codex: not started");
    this.#stdin.write(line);
    await this.#stdin.flush();
  }

  async #read(stream: ReadableStream<Uint8Array>): Promise<void> {
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
    // The server went away; nobody is going to answer what is in flight.
    for (const [id, p] of this.#pending) {
      clearTimeout(p.timer);
      p.fail(new Error("codex: app-server exited"));
      this.#pending.delete(id);
    }
  }

  async #drainStderr(stream: ReadableStream<Uint8Array>): Promise<void> {
    const decoder = new TextDecoder();
    for await (const chunk of stream) {
      const s = decoder.decode(chunk).trim();
      if (s) this.#log(`codex stderr: ${s}`);
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
    if (typeof msg.id === "number" && (msg.result !== undefined || msg.error)) {
      const p = this.#pending.get(msg.id);
      if (!p) return;
      clearTimeout(p.timer);
      this.#pending.delete(msg.id);
      if (msg.error) p.fail(new AppServerError(msg.error.code, msg.error.message));
      else p.ok(msg.result);
      return;
    }
    if (msg.method) this.#notification(msg.method, msg.params);
  }

  // Only turn boundaries matter: they decide steer vs start.
  #notification(method: string, params: unknown): void {
    const p = (params ?? {}) as { threadId?: string; turn?: { id?: string } };
    if (p.threadId !== this.#thread || typeof p.turn?.id !== "string") return;
    if (method === "turn/started") this.#activeTurn = p.turn.id;
    else if (method === "turn/completed" && this.#activeTurn === p.turn.id) this.#activeTurn = undefined;
  }
}
