// B.3 acceptance for the Codex push mode, against a fake App Server.
//
// The real one costs a model turn per check and cannot be driven into the
// states that matter — a thread already mid-turn, a steer the server refuses,
// a request the server expects us to answer. This speaks just enough of the
// protocol to put the client in each of them.
//
// It is a WebSocket server because that is how the client attaches to the
// *shared* App Server, which is the only kind that reaches a live session.

import { Codex, AppServerError } from "./codex.ts";

const CWD = "/tmp/agent-bus-fake-codex";

type Case = {
  steer?: { code: number; message: string };  // reject turn/steer this way
  resumeActive?: boolean;                     // resume reports a running turn
  listActive?: boolean;                       // thread/list's thread is mid-turn
};

let scenario: Case = {};
const seen: string[] = [];
let refusal: any;

const server = Bun.serve({
  port: 0,
  hostname: "127.0.0.1",
  fetch: (req, s) => (s.upgrade(req) ? undefined : new Response("no", { status: 400 })),
  websocket: {
    message(ws, raw): void {
      const msg = JSON.parse(String(raw));
      if (msg.method) seen.push(msg.method);
      const reply = (result: unknown) => void ws.send(JSON.stringify({ jsonrpc: "2.0", id: msg.id, result }));
      const fail = (error: unknown) => void ws.send(JSON.stringify({ jsonrpc: "2.0", id: msg.id, error }));
      const turns = (active: boolean) =>
        active ? [{ id: "turn-live", status: { type: "inProgress" } }] : [{ id: "turn-old", status: { type: "completed" } }];

      switch (msg.method) {
        case "initialize": return reply({ userAgent: "fake/0" });
        case "initialized": return;
        case "thread/list":
          return reply({ data: [{ id: "thread-1", cwd: CWD, turns: turns(!!scenario.listActive) }] });
        case "thread/resume":
          return reply({ thread: { id: "thread-1", cwd: CWD, turns: turns(!!scenario.resumeActive) } });
        case "thread/start":
          return reply({ thread: { id: "thread-1", cwd: CWD, turns: [] } });
        case "turn/steer":
          if (scenario.steer) return fail(scenario.steer);
          return reply({ turnId: "turn-steered" });
        case "turn/start":
          return reply({ turn: { id: "turn-new" } });
        default:
          if (msg.id !== undefined) return fail({ code: -32601, message: "no" });
      }
    },
    close() {},
  },
});
// Anything the client sends back to an id we invented lands here.
const url = `ws://127.0.0.1:${server.port}`;

let pass = 0, fail = 0;
const check = (label: string, cond: boolean, detail = "") => {
  if (cond) { console.log(`  ok   ${label}`); pass++; }
  else { console.log(`  FAIL ${label} ${detail}`); fail++; }
};
const quiet = () => {};

async function run(c: Case): Promise<Codex> {
  scenario = c;
  seen.length = 0;
  const codex = new Codex(CWD, quiet, url);
  await codex.start();
  return codex;
}

try {
  {
    // The face is Codex's MCP server, so it starts before the session has a
    // thread. Choosing one at start would pick the wrong one.
    scenario = {};
    seen.length = 0;
    const codex = new Codex(CWD, quiet, url);
    await codex.start();
    check("start does not choose a thread", !seen.includes("thread/list") && !codex.thread, seen.join(","));
    await codex.deliver("hello", "m0");
    check("the first message chooses it", seen.includes("thread/list") && !!codex.thread, seen.join(","));
    codex.stop();
  }
  {
    // A resumed thread that is already mid-turn must be steered, not raced.
    const codex = await run({ resumeActive: true });
    const how = await codex.deliver("hello", "m1");
    check("a thread resumed mid-turn is steered, not started", how === "turn/steer", `${how}; saw ${seen.join(",")}`);
    codex.stop();
  }
  {
    const codex = await run({ resumeActive: false });
    const how = await codex.deliver("hello", "m2");
    check("an idle thread gets a new turn", how === "turn/start", how);
    codex.stop();
  }
  {
    // Overload must not become "start another turn".
    const codex = await run({ resumeActive: true, steer: { code: -32001, message: "overloaded" } });
    let thrown: unknown;
    try { await codex.deliver("hello", "m3"); } catch (e) { thrown = e; }
    check(
      "an overloaded steer propagates instead of starting a second turn",
      thrown instanceof AppServerError && thrown.code === -32001 && !seen.includes("turn/start"),
      `${thrown}; saw ${seen.join(",")}`,
    );
    codex.stop();
  }
  {
    // A rejected steer asks the server what is running. The server still says
    // a turn is running, so starting another one would be wrong.
    const codex = await run({ resumeActive: true, steer: { code: -32602, message: "invalid params" } });
    let thrown: unknown;
    try { await codex.deliver("hello", "m4"); } catch (e) { thrown = e; }
    check(
      "a rejected steer refreshes and does not start a turn while one runs",
      thrown instanceof AppServerError && !seen.includes("turn/start"),
      `${thrown}; saw ${seen.join(",")}`,
    );
    codex.stop();
  }
  {
    // Same rejection, but the refresh says the turn is over: now start.
    scenario = { listActive: true, steer: { code: -32602, message: "invalid params" } };
    seen.length = 0;
    const codex = new Codex(CWD, quiet, url);
    await codex.start();
    scenario = { ...scenario, resumeActive: false };
    const how = await codex.deliver("hello", "m5");
    check("a rejected steer starts a turn once the server says none is running", how === "turn/start", `${how}; saw ${seen.join(",")}`);
    codex.stop();
  }
  {
    // Silence on a server request hangs the turn; refusing it does not.
    refusal = undefined;
    const answers: any[] = [];
    const spy = Bun.serve({
      port: 0, hostname: "127.0.0.1",
      fetch: (req, s) => (s.upgrade(req) ? undefined : new Response("no", { status: 400 })),
      websocket: {
        message(ws, raw): void {
          const msg = JSON.parse(String(raw));
          if (msg.id === 9001) { answers.push(msg); return; }
          const reply = (result: unknown) => void ws.send(JSON.stringify({ jsonrpc: "2.0", id: msg.id, result }));
          if (msg.method === "initialize") return reply({});
          if (msg.method === "initialized") return;
          if (msg.method === "thread/list") return reply({ data: [{ id: "t", cwd: CWD, turns: [] }] });
          if (msg.method === "thread/resume") return reply({ thread: { id: "t", cwd: CWD, turns: [] } });
          if (msg.method === "turn/start") {
            ws.send(JSON.stringify({ jsonrpc: "2.0", id: 9001, method: "item/commandExecution/requestApproval", params: {} }));
            reply({ turn: { id: "turn-new" } });
            return;
          }
        },
        close() {},
      },
    });
    const codex = new Codex(CWD, quiet, `ws://127.0.0.1:${spy.port}`);
    await codex.start();
    await codex.deliver("hello", "m6");
    await Bun.sleep(500);
    refusal = answers[0];
    check(
      "a server request is refused, not ignored",
      refusal?.error?.code === -32601,
      JSON.stringify(refusal),
    );
    codex.stop();
    spy.stop(true);
  }
} catch (e) {
  check("no exception", false, String(e));
} finally {
  server.stop(true);
}
console.log(`\ncodex: passed ${pass}, failed ${fail}`);
process.exit(fail === 0 ? 0 : 1);
