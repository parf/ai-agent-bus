import { Bus } from "./bus.ts";
import { shareFixtureInbox } from "./smoke-access.ts";
// Runtime boundary fixture. The bus and MCP face are real in smoke.sh.
import { appendFileSync, existsSync, readFileSync, writeFileSync } from "node:fs";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";

const args = process.argv.slice(2);
const value = (flag: string) => args[args.indexOf(flag) + 1];
const file = process.env.TEST_EVENTS!;
const event = (kind: string, data: unknown = {}) => appendFileSync(file, JSON.stringify({ kind, data, pid: process.pid }) + "\n");
event("argv", args);
event("config-dir", process.env.CLAUDE_CONFIG_DIR || null);
const savedTitle = () => process.env.TEST_TITLE_FILE && existsSync(process.env.TEST_TITLE_FILE)
  ? readFileSync(process.env.TEST_TITLE_FILE, "utf8") : process.env.TEST_TITLE || null;
const saveTitle = (title: string) => { if (process.env.TEST_TITLE_FILE) writeFileSync(process.env.TEST_TITLE_FILE, title); };
if (process.env.TEST_FAIL_START && (args[0] === "app-server" || args[0] === "serve")) process.exit(7);

// opencode: the server the launcher owns, and the TUI attaches to it.
if (args[0] === "serve") {
  const config = JSON.parse(process.env.OPENCODE_CONFIG_CONTENT || "{}");
  event("permission", config.permission);
  writeFileSync(file + ".mcp", JSON.stringify(config.mcp?.["agent-bus"]));
  const id = process.env.TEST_SESSION_ID || "opencode-session";
  const session = { id, title: savedTitle(), directory: process.cwd() };
  const expected = process.env.OPENCODE_SERVER_PASSWORD && "Basic " + Buffer.from(`opencode:${process.env.OPENCODE_SERVER_PASSWORD}`).toString("base64");
  Bun.serve({
    hostname: "127.0.0.1", port: Number(value("--port")),
    async fetch(req) {
      const url = new URL(req.url);
      // Anything that can reach this server can drive the session.
      if (expected && req.headers.get("authorization") !== expected) return new Response("unauthorized", { status: 401 });
      event("http", { method: req.method, path: url.pathname });
      if (url.pathname === "/config") return Response.json(config);
      if (url.pathname === "/session" && req.method === "GET") return Response.json(process.env.TEST_NO_HISTORY ? [] : [session]);
      if (url.pathname === "/session" && req.method === "POST") return Response.json(session);
      if (url.pathname === "/event") {
        return new Response(new ReadableStream({
          async start(controller) {
            // The TUI says which session it opened; that is what the launcher binds.
            for (let i = 0; i < 600 && !existsSync(file + ".tui-ready"); i++) await Bun.sleep(50);
            // The real TUI does not announce initial session creation through
            // tui.session.select. The launcher must attach to an explicit ID.
            controller.enqueue(new TextEncoder().encode(`data: ${JSON.stringify({ type: "server.connected", properties: {} })}\n\n`));
          },
        }), { headers: { "content-type": "text/event-stream" } });
      }
      const one = url.pathname.match(/^\/session\/([^/]+)$/);
      if (one && req.method === "PATCH") {
        session.title = (await req.json() as any).title;
        event("runtime-renamed", session.title);
        saveTitle(session.title!);
        return Response.json(session);
      }
      if (one && req.method === "GET") return Response.json({ ...session, id: one[1] });
      if (url.pathname.endsWith("/prompt_async") && req.method === "POST") {
        writeFileSync(file + ".delivered", (await req.json() as any).parts[0].text);
        return new Response(null, { status: 204 });
      }
      return new Response("not found", { status: 404 });
    },
  });
  event("server-ready");
} else if (args[0] === "app-server") {
  let config: any;
  for (let i = 0; i < args.length; i++) if (args[i] === "-c") {
    const parsed = Bun.TOML.parse(args[++i]!) as Record<string, any>;
    if (parsed.mcp_servers) config = (parsed.mcp_servers as any)["agent-bus"];
    if (parsed.approval_policy) event("approval", parsed.approval_policy);
    if (parsed.sandbox_mode) event("sandbox", parsed.sandbox_mode);
  }
  writeFileSync(file + ".mcp", JSON.stringify(config));
  const url = new URL(value("--listen")!);
  const token = args.includes("--ws-token-file") ? readFileSync(value("--ws-token-file")!, "utf8") : undefined;
  const expected = value("--ws-auth") === "capability-token" && token ? `Bearer ${token}` : undefined;
  const thread = { id: process.env.TEST_SESSION_ID || "codex-session", cwd: process.cwd(), name: savedTitle(), turns: [] };
  Bun.serve({ hostname: "127.0.0.1", port: Number(url.port),
    fetch(req, server) {
      if (expected && req.headers.get("authorization") !== expected) return new Response("unauthorized", { status: 401 });
      return server.upgrade(req) ? undefined : new Response("ready");
    },
    websocket: {
      message(ws, data) {
        const m = JSON.parse(String(data));
        event(m.method, m.params);
        let result: any = {};
        if (m.method === "thread/list") result = { data: process.env.TEST_NO_HISTORY ? [] : [thread] };
        if (m.method === "thread/loaded/list") result = { data: existsSync(file + ".tui-ready") ? [thread.id] : [] };
        if (["thread/resume", "thread/start", "thread/read"].includes(m.method)) result = { thread };
        if (m.method === "thread/name/set") {
          thread.name = m.params.name;
          event("runtime-renamed", thread.name);
          saveTitle(thread.name!);
        }
        if (m.method === "turn/start" || m.method === "turn/steer") {
          writeFileSync(file + ".delivered", m.params.input[0].text);
          result = { turn: { id: "turn" }, turnId: "turn" };
        }
        if (m.id !== undefined) ws.send(JSON.stringify({ id: m.id, result }));
      }, close() {},
    },
  });
  event("server-ready");
} else {
  const kind = process.env.TEST_RUNTIME!;
  event("terminal-env", kind === "claude" ? process.env.CLAUDE_CODE_DISABLE_TERMINAL_TITLE : process.env.OPENCODE_DISABLE_TERMINAL_TITLE);
  let config: any;
  if (kind === "claude" && args.includes("--mcp-config")) config = JSON.parse(readFileSync(value("--mcp-config")!, "utf8")).mcpServers["agent-bus"];
  if (kind === "codex" && args.includes("--remote")) config = JSON.parse(readFileSync(file + ".mcp", "utf8"));
  if (kind === "opencode" && args[0] === "attach") {
    if (value("--session") !== (process.env.TEST_SESSION_ID || "opencode-session")) {
      event("wrong-session"); process.exit(19);
    }
    // opencode declares one command array; the client here wants it split.
    const declared = JSON.parse(readFileSync(file + ".mcp", "utf8"));
    if (declared) config = { command: declared.command[0], args: declared.command.slice(1), env: declared.environment };
  }
  if (!config) { event("plain"); process.exit(Number(process.env.TEST_EXIT || 0)); }
  writeFileSync(file + ".tui-ready", "ready");
  if (kind === "claude" && !args.includes("mcp__agent-bus__*")) { event("tools-not-authorized"); process.exit(18); }
  if (kind === "codex" && config.default_tools_approval_mode !== "approve") { event("tools-not-authorized"); process.exit(18); }
  if (kind === "codex") {
    const remote = value("--remote")!;
    const token = process.env[value("--remote-auth-token-env")!];
    const denied = await fetch(remote.replace("ws:", "http:"));
    event("runtime-denied", denied.status);
    // An actual handshake pins the TUI's credential channel, not just the
    // presence of an environment variable in a fixture.
    const ws = new WebSocket(remote, token ? { headers: { Authorization: `Bearer ${token}` } } : undefined);
    await new Promise<void>((ok, fail) => {
      const timer = setTimeout(() => fail(new Error("TUI could not authenticate")), 3000);
      ws.onopen = () => { clearTimeout(timer); event("tui-authenticated"); ws.close(); ok(); };
      ws.onerror = () => { clearTimeout(timer); fail(new Error("TUI authentication refused")); };
    });
  }
  // The TUI reaches its own server only because the launcher gave it the password.
  if (kind === "opencode" && !process.env.OPENCODE_SERVER_PASSWORD) { event("tools-not-authorized"); process.exit(18); }
  const busEnv = JSON.parse(readFileSync(config.env.AGENT_BUS_SESSION_FILE, "utf8"));
  event("identity", { name: busEnv.AGENT_BUS_NAME, label: busEnv.AGENT_BUS_DESCR, ownerLeaked: !!process.env.AGENT_BUS_OWNER_TOKEN });
  const client = new Client({ name: "launcher-test", version: "1" });
  let delivered = "";
  client.fallbackNotificationHandler = async n => {
    if (n.method === "notifications/claude/channel" && args.includes("--dangerously-load-development-channels")) delivered = String((n.params as any).content);
  };
  const transport = new StdioClientTransport({ command: config.command, args: config.args, env: { ...process.env, ...config.env } as Record<string, string> });
  await client.connect(transport);
  await shareFixtureInbox(new Bus(busEnv));
  event("mcp-pid", transport.pid);
  event("tools", (await client.listTools()).tools.map(t => t.name));
  event("listed", await client.callTool({ name: "ab_ls", arguments: {} }));
  event("sent", await client.callTool({ name: "ab_send", arguments: { to: process.env.TEST_PEER, text: "launcher-question", topic: "launch", tag: process.env.TEST_TAG || "test" } }));
  if (process.env.TEST_DENIED) event("denied", await client.callTool({ name: "ab_send", arguments: { to: process.env.TEST_DENIED, text: "must refuse" } }));
  for (let i = 0; i < 200; i++) {
    if (kind !== "claude" && existsSync(file + ".delivered")) delivered = readFileSync(file + ".delivered", "utf8");
    if (delivered.includes("launcher-answer")) break;
    await Bun.sleep(25);
  }
  event("delivered", delivered);
  if (process.env.TEST_RENAME) {
    if (busEnv.AGENT_BUS_CONTROL_ADDR) {
      const denied = await fetch(busEnv.AGENT_BUS_CONTROL_ADDR + "/rename", { method: "POST", body: JSON.stringify({ name: "unauthorized" }) });
      event("control-denied", denied.status);
    }
    const second = new Client({ name: "second-face", version: "1" });
    const secondTransport = new StdioClientTransport({ command: config.command, args: config.args, env: { ...process.env, ...config.env } as Record<string, string> });
    await second.connect(secondTransport);
    event("mcp-pid", secondTransport.pid);
    const renamed = await Promise.all([client, second].map(c => c.callTool({ name: "ab_rename", arguments: { name: process.env.TEST_RENAME } })));
    for (const result of renamed) event("renamed", result);
    const repeat = await client.callTool({ name: "ab_rename", arguments: {} });
    event("rename-repeat", repeat);
    await second.close();
    const after = JSON.parse(readFileSync(config.env.AGENT_BUS_SESSION_FILE, "utf8"));
    await shareFixtureInbox(new Bus(after));
    event("after-rename", { name: after.AGENT_BUS_NAME, label: after.AGENT_BUS_DESCR });
    if (existsSync(file + ".delivered")) writeFileSync(file + ".delivered", "");
    delivered = "";
    event("sent-after-rename", await client.callTool({ name: "ab_send", arguments: { to: process.env.TEST_PEER, text: "after-rename", topic: "renamed", tag: process.env.TEST_TAG || "test" } }));
    for (let i = 0; i < 120; i++) {
      if (kind !== "claude" && existsSync(file + ".delivered")) delivered = readFileSync(file + ".delivered", "utf8");
      if (delivered.includes("launcher-answer")) break;
      await Bun.sleep(25);
    }
    event("rename-delivered", delivered);
  }
  if (process.env.TEST_HOLD) await Bun.sleep(Number(process.env.TEST_HOLD));
  await client.close();
  process.exit(delivered.includes("launcher-answer") ? Number(process.env.TEST_EXIT || 0) : 9);
}
