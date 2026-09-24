import { spawn, spawnSync, type ChildProcess } from "node:child_process";
import { randomUUID } from "node:crypto";
import { appendFileSync, chmodSync, closeSync, existsSync, mkdirSync, mkdtempSync, openSync, readFileSync, realpathSync, renameSync, rmSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";
import { basename, dirname, join, resolve } from "node:path";
import { Bus, BusError, defaultName } from "../mcp/bus.ts";
import { Codex } from "../mcp/codex.ts";
import { Opencode } from "../mcp/opencode.ts";
import { sidecarMessage } from "../mcp/messages.ts";
import { startPush, sleep, type Push } from "../mcp/push.ts";
import { faceEvent, faceMarks } from "../mcp/face-mark.ts";
import { version } from "../mcp/version.ts";
import { Bindings, claudeSessions, claudeTitle, sameBase, type Session } from "./sessions.ts";
import { localAddress, runtimeBinary } from "./local.ts";
import { Terminal } from "./terminal.ts";
import { claimIdentity, releaseIdle } from "./identity.ts";
import { serveControl } from "./control.ts";
import { codexAuth } from "./runtime-auth.ts";

const runtime = process.argv[2];
const args = process.argv.slice(3);
// One line shape for everything we say, with the square carrying the state:
// green is fine, orange is worth knowing, red is broken. Good news goes to
// stdout; anything the user may need to act on goes to stderr, which is also
// where a terminal would have coloured it red whatever we meant.
const colour = (stream: NodeJS.WriteStream) => stream.isTTY && !process.env.NO_COLOR;
const say = (stream: NodeJS.WriteStream, code: string, s: string) =>
  stream.write(colour(stream)
    ? `\x1b[${code}m\u25a0\x1b[0m \x1b[1magent-bus\x1b[0m: \x1b[90m${s}\x1b[0m\n`
    : `agent-bus: ${s}\n`);
const note = (s: string) => say(process.stdout, "32", s);          // green: done
const warn = (s: string) => say(process.stderr, "38;5;208", s);    // orange: notice
const log = (s: string) => say(process.stderr, "31", s);           // red: broken
// The runtime adapters say both; a status line is good news, not a failure.
const adapterLog = (s: string, ok?: boolean) => ok ? note(s) : log(s);
// Make "agent-bus" resolvable as a channel server in this directory.
// Failure is not fatal: without it the session runs with tools and no channel,
// which is worth a warning and not worth refusing to start over.
// Claude keeps a whole account under one configuration home: its login, its
// MCP servers and its session transcripts. Everything here reads it from the
// same place, so an account flag moves all three together.
const claudeHome = () => process.env.CLAUDE_CONFIG_DIR || join(homedir(), ".claude");
const claudeConfig = () => process.env.CLAUDE_CONFIG_DIR ? join(process.env.CLAUDE_CONFIG_DIR, ".claude.json") : join(homedir(), ".claude.json");
function channelName(cwd: string, face: string) {
  try {
    const conf = claudeConfig();
    const registered = () => !!(existsSync(conf) && JSON.parse(readFileSync(conf, "utf8"))?.projects?.[cwd]?.mcpServers?.["agent-bus"]);
    if (registered()) return;
    const done = spawnSync("claude", ["mcp", "add", "--scope", "local", "agent-bus", "--", process.execPath, face],
      { cwd, stdio: "ignore" });
    // `claude mcp add` can say it added the server and exit 0 with nothing
    // written (a read-only configuration directory): believe the file.
    if (done.status !== 0 || !registered()) warn("could not register the channel server; this session has tools but no channel");
  } catch (e) { warn(`could not register the channel server: ${e}`); }
}

// A helper that dies at startup says why in its own log, and that log lives in
// the run directory cleanup removes. Its last lines go into the diagnostic.
function why(file: string): string {
  let text = "";
  try { text = readFileSync(file, "utf8"); } catch { return "it wrote nothing"; }
  const lines = text.replace(/\x1b\[[0-9;]*m/g, "").split("\n").map(l => l.trim()).filter(Boolean);
  return lines.length ? lines.slice(-4).join(" | ").slice(-600) : "it wrote nothing";
}

const option = (names: string[]) => {
  for (let i = 0; i < args.length; i++) for (const n of names) {
    if (args[i]!.startsWith(n + "=")) return args[i]!.slice(n.length + 1);
    if (args[i] === n && args[i + 1] && !args[i + 1]!.startsWith("-")) return args[i + 1];
  }
};
const has = (...names: string[]) => args.some(a => names.includes(a) || names.some(n => a.startsWith(n + "=")));
// One machine, several Claude accounts: -2, -3 and -4 name a sibling
// configuration home. The flag is the launcher's own and never reaches the
// runtime, which would refuse it.
const accounts = ["-2", "-3", "-4"];
type Child = { proc: ChildProcess; exited: Promise<number>; group: boolean };
const children: Child[] = [];
function start(command: string[], env: NodeJS.ProcessEnv, file?: number): Child {
  const proc = spawn(command[0]!, command.slice(1), {
    env, stdio: file === undefined ? "inherit" : ["ignore", file, file], detached: file !== undefined,
  });
  const exited = new Promise<number>(resolve => {
    proc.once("error", e => { log(e.message); resolve(1); });
    proc.once("exit", (code, signal) => resolve(code ?? (signal === "SIGINT" ? 130 : 143)));
  });
  const child = { proc, exited, group: file !== undefined };
  children.push(child);
  return child;
}
async function stop(child: Child): Promise<void> {
  const signal = (s: NodeJS.Signals) => {
    try { if (child.group && child.proc.pid) process.kill(-child.proc.pid, s); else child.proc.kill(s); } catch { /* already exited */ }
  };
  signal("SIGTERM");
  await Promise.race([child.exited, sleep(1500)]);
  // Group members may outlive the App Server; reap those too.
  signal("SIGKILL");
  await child.exited;
}

let bindings: Bindings | undefined, runDir: string | undefined, codex: Codex | undefined, opencode: Opencode | undefined, push: Push | undefined;
let poll: ReturnType<typeof setInterval> | undefined;
let terminal: Terminal | undefined;
let control: ReturnType<typeof serveControl> | undefined;
let stopping = false;
let cleanupPromise: Promise<void> | undefined;
function cleanup(): Promise<void> {
  return cleanupPromise ??= (async () => {
  stopping = true;
  clearInterval(poll);
  control?.stop();
  push?.stop();
  codex?.stop();
  opencode?.stop();
  for (const child of children.reverse()) await stop(child);
  bindings?.close();
  if (runDir) rmSync(runDir, { recursive: true, force: true });
  terminal?.restore();
  })();
}
process.on("SIGHUP", () => { void cleanup().then(() => process.exit(129)); });
process.on("SIGTERM", () => { void cleanup().then(() => process.exit(143)); });
// The foreground runtime receives terminal SIGINT directly.
process.on("SIGINT", () => {
  if (!children.some(c => !c.group && c.proc.exitCode === null)) void cleanup().then(() => process.exit(130));
});

async function main(): Promise<number> {
  if (runtime !== "claude" && runtime !== "codex" && runtime !== "opencode") throw new Error("expected claude, codex or opencode");
  if (has("--version", "-V")) { console.log(version); return 0; }
  if (has("--help", "-h")) {
    console.log(`ab-${runtime}: launch ${runtime} with agent-bus tools and messaging.
Discovers your local socket; AGENT_BUS_ADDR overrides discovery. AGENT_BUS_TOKEN supplies a token when needed.
AGENT_BUS_NAME sets the preferred bus identity. Automatic execution and continuation are enforced.${runtime === "claude" ? `
-2, -3 and -4 select a separate account: the configuration home becomes ~/.claude2, ~/.claude3 or ~/.claude4,
or the set CLAUDE_CONFIG_DIR with that digit appended. Each account keeps its own login, sessions and servers.` : ""}
Other arguments are forwarded to the runtime. Session state: XDG_STATE_HOME/agent-bus/sessions.
See docs/08-runner-role.md#smart-launchers.`);
    return 0;
  }
  const chosen = [...new Set(args.filter(a => accounts.includes(a)))];
  if (chosen.length) {
    if (runtime !== "claude") throw new Error(`${chosen[0]} selects a Claude account and applies to ab-claude only`);
    if (chosen.length > 1) throw new Error("select one account at a time");
    // Suffix whichever home is in force, so an explicit CLAUDE_CONFIG_DIR still parents its own accounts.
    const home = claudeHome().replace(/\/+$/, "") + chosen[0]!.slice(1);
    mkdirSync(home, { recursive: true, mode: 0o700 });
    process.env.CLAUDE_CONFIG_DIR = home;
    for (let i = args.indexOf(chosen[0]!); i >= 0; i = args.indexOf(chosen[0]!)) args.splice(i, 1);
    note(`account ${chosen[0]!.slice(1)}: ${home}`);
  }
  const binary = runtimeBinary(runtime, process.env);
  if (!binary) throw new Error(`${runtime} executable not found; install the runtime or set ${runtime.toUpperCase()}_BIN`);
  const dirOption = runtime === "codex" ? option(["-C", "--cd"]) : runtime === "opencode" ? option(["--dir"]) : undefined;
  const cwd = realpathSync(dirOption ? resolve(dirOption) : process.cwd());
  process.chdir(cwd);
  terminal = new Terminal(runtime, cwd);
  const address = localAddress(process.env);
  if (address) process.env.AGENT_BUS_ADDR = address;
  const cleanEnv = { ...process.env };
  for (const key of Object.keys(cleanEnv)) if (key.startsWith("AGENT_BUS_")) delete cleanEnv[key];
  if (terminal.active) {
    cleanEnv.CLAUDE_CODE_DISABLE_TERMINAL_TITLE = "1";
    cleanEnv.OPENCODE_DISABLE_TERMINAL_TITLE = "1";
  }
  const titleArgs = runtime === "codex" && terminal.active ? ["-c", "tui.terminal_title=[]"] : [];
  const configured = !!process.env.AGENT_BUS_TOKEN || !!process.env.AGENT_BUS_ADDR;
  if (!configured) {
    warn("bus is not configured; starting a plain runtime session");
    if (runtime === "claude") {
      const sessions = await claudeSessions(claudeHome(), cwd);
      terminal.set(option(["-n", "--name"]) || sessions[0]?.name || undefined);
      return await start([binary, ...claudeArgs(args), "--enable-auto-mode", ...(sessions.length ? ["--continue"] : [])], cleanEnv).exited;
    }
    if (runtime === "opencode") return await start([binary, ...(has("-c", "--continue", "-s", "--session") ? [] : ["--continue"]), ...args], cleanEnv).exited;
    return await start([binary, "resume", "--last", ...args, ...titleArgs, "--dangerously-bypass-approvals-and-sandbox"], cleanEnv).exited;
  }
  const owner = new Bus(process.env, true);
  await owner.status(); // Diagnose the bus before starting runtime sidecars.
  const state = join(process.env.XDG_STATE_HOME || join(homedir(), ".local/state"), "agent-bus/sessions");
  bindings = new Bindings(state, runtime);
  const runs = join(state, "runs");
  mkdirSync(runs, { recursive: true, mode: 0o700 });
  chmodSync(runs, 0o700);
  runDir = mkdtempSync(join(runs, runtime + "-"));
  const envFile = join(runDir, "bus-env.json");
  const face = realpathSync(join(import.meta.dir, existsSync(join(import.meta.dir, "../mcp/server.js")) ? "../mcp/server.js" : "../mcp/server.ts"));
  let session: Session, fresh = false;
  let remote: string | undefined;
  const tuiEnv: Record<string, string> = {};
  let runtimeArgs = [...args];
  let serverChild: Child | undefined;
  if (runtime === "claude") {
    if (has("--fork-session", "--from-pr", "--cloud", "--remote-control", "--background", "--bg")) throw new Error("this session mode cannot be bound by the launcher; start an interactive session");
    const home = claudeHome();
    const sessions = await claudeSessions(home, cwd);
    const requested = option(["-r", "--resume"]);
    if (has("-r", "--resume") && !requested) throw new Error("give a session ID or name to resume");
    const newID = option(["--session-id"]);
    if (newID) {
      if (!/^[0-9a-f-]{36}$/i.test(newID)) throw new Error("invalid session ID");
      if (!bindings.lock(`${runtime}:${newID}`)) throw new Error("session already has a launcher");
      session = { id: newID };
    } else {
      session = bindings.select(requested ? sessions : sessions.slice(0, 1), requested, fresh);
      if (!has("-r", "--resume", "-c", "--continue")) runtimeArgs = [...(session.file ? ["--continue"] : ["--session-id", session.id]), ...args];
      // Resolve continue ourselves so a simultaneous launch cannot select the same session.
      else if (has("-c", "--continue")) runtimeArgs = [...(session.file ? ["--continue"] : ["--session-id", session.id]), ...args.filter(a => a !== "-c" && a !== "--continue")];
    }
    runtimeArgs = [...claudeArgs(runtimeArgs), "--enable-auto-mode"];
    session.name = option(["-n", "--name"]) || session.name;
    session.file ||= join(home, "projects", cwd.replace(/[^a-zA-Z0-9]/g, "-"), `${session.id}.jsonl`);
  } else if (runtime === "opencode") {
    if (has("attach", "serve", "web", "run")) throw new Error("the launcher starts and owns the server; give it interactive arguments only");
    const tuiArgs: string[] = [];
    for (let i = 0; i < args.length; i++) {
      const a = args[i]!;
      // Already resolved before chdir; forwarding it would apply it twice.
      if (a === "--dir") { i++; continue; }
      if (a.startsWith("--dir=")) continue;
      tuiArgs.push(a);
    }
    // Reserve an ephemeral loopback port, then let the server bind it.
    const reservation = Bun.listen({ hostname: "127.0.0.1", port: 0, socket: { data() {} } });
    const port = reservation.port;
    reservation.stop(true);
    remote = `http://127.0.0.1:${port}`;
    // Anything that can reach this server can drive the session, so it gets a
    // password even on loopback. Passed by environment, never as a flag: an
    // argument is world-readable in ps.
    const password = randomUUID();
    const serverConfig = {
      // The launcher's enforced mode, the same promise the other two make.
      permission: "allow",
      mcp: {
        "agent-bus": {
          type: "local", command: [process.execPath, face],
          environment: { AGENT_BUS_SESSION_FILE: envFile }, enabled: true,
        },
      },
    };
    const serverEnv = { ...cleanEnv, OPENCODE_SERVER_PASSWORD: password, OPENCODE_CONFIG_CONTENT: JSON.stringify(serverConfig) };
    const fd = openSync(join(runDir, "server.log"), "a", 0o600);
    serverChild = start([binary, "serve", "--port", String(port), "--hostname", "127.0.0.1"], serverEnv, fd);
    closeSync(fd);
    opencode = new Opencode(cwd, adapterLog, remote, password);
    let ready = false;
    for (let i = 0; i < 200; i++) {
      if (serverChild.proc.exitCode !== null) throw new Error(`the opencode server exited during startup: ${why(join(runDir, "server.log"))}`);
      if (await opencode.probe()) { ready = true; break; }
      await sleep(50);
    }
    if (!ready) throw new Error(`the opencode server did not become ready: ${why(join(runDir, "server.log"))}`);
    await opencode.start();
    tuiEnv.OPENCODE_SERVER_PASSWORD = password;
    const listed = await opencode.sessions();
    const sessions: Session[] = listed.map(o => ({ id: o.id, name: o.title }));
    const requested = option(["-s", "--session"]);
    if (has("--fork")) throw new Error("a forked session cannot be bound by the launcher; start or resume one");
    session = bindings.select(requested ? sessions : sessions.slice(0, 1), requested, fresh);
    fresh = !sessions.some(s => s.id === session.id);
    if (fresh) {
      // OpenCode persists a new session before the first prompt, so attach
      // the TUI to the same explicit ID the inbox reader will use.
      const created = await opencode.openSession();
      if (!bindings.lock(`${runtime}:${created.id}`)) throw new Error("session already has a launcher");
      session = { id: created.id };
      fresh = false;
    }
    // Session selection is resolved here, so it must not be forwarded twice.
    runtimeArgs = tuiArgs.filter((a, i) => !["-c", "--continue", "-s", "--session"].includes(a) && !a.startsWith("--session=") && !a.startsWith("-s=") && !["-s", "--session"].includes(tuiArgs[i - 1] ?? ""));
  } else {
    if (has("--remote", "--remote-auth-token-env", "--worktree")) throw new Error("the launcher owns its App Server, authentication and working directory; remote/worktree mode is not supported");
    // CLI permission/config flags must configure the server, not its remote TUI.
    const serverArgs: string[] = [];
    const tuiArgs: string[] = [];
    for (let i = 0; i < args.length; i++) {
      const a = args[i]!;
      // Already resolved before chdir; forwarding a relative directory here
      // would apply it a second time in the remote TUI.
      if (a === "-C" || a === "--cd") { i++; continue; }
      if (a.startsWith("-C=") || a.startsWith("--cd=")) continue;
      const pair = ["-c", "--config", "-p", "--profile", "-s", "--sandbox", "-a", "--ask-for-approval"];
      const match = pair.find(n => a === n || a.startsWith(n + "="));
      if (match) {
        const v = a.includes("=") ? a.slice(a.indexOf("=") + 1) : args[++i];
        if (!v) throw new Error(`missing value for ${match}`);
        if (["-p", "--profile"].includes(match)) throw new Error("select profiles with CODEX_HOME before using the launcher");
        const key = ["-s", "--sandbox"].includes(match) ? "sandbox_mode" : ["-a", "--ask-for-approval"].includes(match) ? "approval_policy" : undefined;
        serverArgs.push("-c", key ? `${key}=${JSON.stringify(v)}` : v);
      } else if (a === "--dangerously-bypass-approvals-and-sandbox") {
        serverArgs.push("-c", 'approval_policy="never"', "-c", 'sandbox_mode="danger-full-access"');
      } else if (a === "--approve-for-me") throw new Error("set the desired approval policy explicitly for the App Server");
      else tuiArgs.push(a);
    }
    // Reserve an ephemeral loopback port; the App Server must then bind it.
    serverArgs.push("-c", 'approval_policy="never"', "-c", 'sandbox_mode="danger-full-access"');
    const reservation = Bun.listen({ hostname: "127.0.0.1", port: 0, socket: { data() {} } });
    remote = `ws://127.0.0.1:${reservation.port}`;
    reservation.stop(true);
    // Loopback is reachable by other local accounts. The native server checks
    // this capability before upgrading any connection; never fall back to an
    // unauthenticated listener when a runtime lacks these flags.
    const authentication = codexAuth(runDir);
    const password = authentication.token;
    serverArgs.push(...authentication.args);
    tuiEnv.AGENT_BUS_CODEX_AUTH_TOKEN = password;
    const mcp = {
      command: process.execPath, args: [face],
      env: { AGENT_BUS_SESSION_FILE: envFile }, enabled: true,
    };
    // TOML inline table; strings are JSON-escaped, never interpreted by a shell.
    serverArgs.push("-c", `mcp_servers.agent-bus={command=${JSON.stringify(mcp.command)},args=[${JSON.stringify(face)}],env={AGENT_BUS_SESSION_FILE=${JSON.stringify(envFile)}},enabled=true,default_tools_approval_mode="approve"}`);
    const fd = openSync(join(runDir, "app-server.log"), "a", 0o600);
    serverChild = start([binary, "app-server", ...serverArgs, "--listen", remote], cleanEnv, fd);
    closeSync(fd);
    let ready = false;
    for (let i = 0; i < 100; i++) {
      // An older Codex without --ws-auth and --remote-auth-token-env says so here too.
      if (serverChild.proc.exitCode !== null) throw new Error(`App Server exited during startup: ${why(join(runDir, "app-server.log"))}`);
      try { const response = await fetch(remote.replace("ws:", "http:"), { signal: AbortSignal.timeout(100) }); if (response) { ready = true; break; } } catch { /* not bound yet */ }
      await sleep(50);
    }
    if (!ready) throw new Error(`App Server did not become ready: ${why(join(runDir, "app-server.log"))}`);
    codex = new Codex(cwd, adapterLog, remote, password);
    await codex.start();
    const sessions = await codex.threads();
    const resumeIndex = tuiArgs.indexOf("resume");
    const requested = resumeIndex >= 0 && tuiArgs[resumeIndex + 1] && !tuiArgs[resumeIndex + 1]!.startsWith("-") ? tuiArgs[resumeIndex + 1] : undefined;
    if (tuiArgs.includes("fork") || tuiArgs.includes("exec")) throw new Error("the launcher supports interactive start and resume");
    session = bindings.select(requested ? sessions : sessions.slice(0, 1), requested, fresh);
    fresh = !sessions.some(s => s.id === session.id);
    runtimeArgs = tuiArgs.filter((a, i) => a !== "--last" && i !== resumeIndex && !(requested && i === resumeIndex + 1));
  }
  const explicit = process.env.AGENT_BUS_NAME?.trim().toLowerCase();
  const previous = bindings.saved(session, explicit);
  let base = explicit || defaultName({ ...process.env, AGENT_BUS_RUNTIME: runtime, AGENT_BUS_CWD: session.name || cwd }, "/");
  const saved = previous && (explicit || sameBase(previous, base)) ? previous.name : undefined;
  let { name, label } = await claimIdentity(owner, bindings, base, session.name || `${runtime}(${cwd})`, previous?.name, saved);
  bindings.save(session.id, name, base);
  if (previous && previous.name !== name) {
    try { await owner.unregister(previous.name); }
    catch (e) {
      if (!(e instanceof BusError && e.status === 404)) warn(`new address ${name}; old address ${previous.name} retained: ${e}`);
    }
  }
  const token = await owner.token(name);
  const addr = process.env.AGENT_BUS_ADDR;
  const env = {
    ...process.env, AGENT_BUS_NAME: name, AGENT_BUS_TOKEN: token, AGENT_BUS_RUNTIME: runtime,
    AGENT_BUS_DESCR: label, AGENT_BUS_PUSH: runtime === "claude" ? "claude" : "off",
    ...(addr && /^user-.*\.sock$/.test(basename(addr)) ? { AGENT_BUS_ADDR: join(dirname(addr), "bus.sock") } : {}),
  };
  let bus = new Bus(env);
  if ((await bus.status()).you !== name) throw new Error("bus listener did not authenticate the session identity");
  const faceEnv: Record<string, string> = {};
  for (const key of ["AGENT_BUS_NAME", "AGENT_BUS_TOKEN", "AGENT_BUS_RUNTIME", "AGENT_BUS_DESCR", "AGENT_BUS_PUSH", "AGENT_BUS_ADDR"] as const) {
    if (env[key] !== undefined) faceEnv[key] = env[key]!;
  }
  const saveEnv = () => {
    writeFileSync(envFile + ".new", JSON.stringify(faceEnv), { mode: 0o600 });
    renameSync(envFile + ".new", envFile);
  };
  saveEnv();
  const bindThread = (thread: { id: string }) => {
    if (thread.id !== session.id) {
      if (!bindings!.lock(`${runtime}:${thread.id}`)) throw new Error("runtime selected an already launched session");
      bindings!.save(thread.id, name, base);
      session.id = thread.id;
    }
    push = startPush(bus, async e => {
      await codex!.deliver(sidecarMessage(e), e.message_id);
    }, log);
  };
  const bindSession = (id: string) => {
    if (id !== session.id) {
      if (!bindings!.lock(`${runtime}:${id}`)) throw new Error("runtime selected an already launched session");
      bindings!.save(id, name, base);
      session.id = id;
    }
    opencode!.bind(id);
    push = startPush(bus, async e => {
      await opencode!.deliver(sidecarMessage(e), e.message_id);
    }, log);
  };
  if (codex) {
    const thread = fresh ? undefined : await codex.openThread(session.id);
    if (thread) bindThread(thread);
    runtimeArgs = ["--remote", remote!, "--remote-auth-token-env", "AGENT_BUS_CODEX_AUTH_TOKEN", "-C", cwd, ...(thread ? ["resume", thread.id] : []), ...runtimeArgs];
  } else if (opencode) {
    bindSession(session.id);
    runtimeArgs = ["attach", remote!, "--dir", cwd, "--session", session.id, ...runtimeArgs];
  } else {
    const config = join(runDir, "claude-mcp.json");
    writeFileSync(config, JSON.stringify({ mcpServers: { "agent-bus": { command: process.execPath, args: [face], env: { AGENT_BUS_SESSION_FILE: envFile } } } }), { mode: 0o600 });
    // The channel resolves its server name against the configured scopes only
    // — managed, user, project, local — and a --mcp-config server is in none of
    // them, so the session started with "no MCP server configured with that
    // name" and no channel. A name in the local scope satisfies that lookup;
    // the definition above still wins for the connection, which is what carries
    // this session's own env. Written through claude's own command so the file
    // it shares with every running session is not ours to rewrite, and only
    // when it is missing, since this persists per directory.
    channelName(cwd, face);
    runtimeArgs = ["--allowedTools", "mcp__agent-bus__*", "--mcp-config", config, "--dangerously-load-development-channels", "server:agent-bus", ...runtimeArgs];
  }
  let updating = Promise.resolve();
  const exclusive = <T>(work: () => Promise<T>): Promise<T> => {
    const next = updating.then(work);
    updating = next.then(() => {}, () => {});
    return next;
  };
  const currentTitle = () => codex ? (codex.thread ? codex.threadName() : undefined)
    : opencode ? opencode.title() : claudeTitle(session.file!);
  control = serveControl(title => exclusive(async () => {
    if (stopping) throw new Error("launcher is stopping");
    if (explicit) throw new Error("AGENT_BUS_NAME pins this address; remove it and restart the launcher before renaming");
    if (codex && !codex.thread) {
      const thread = await codex.loadedThread();
      if (!thread) throw new Error("Codex has not created its session yet; send the first message before renaming");
      bindThread(thread);
    }
    const nextTitle = title?.trim() || await currentTitle();
    if (!nextTitle) throw new Error("Rename the session first, or pass a name to ab_rename");
    const nextBase = defaultName({ ...process.env, AGENT_BUS_RUNTIME: runtime, AGENT_BUS_CWD: nextTitle }, "/");
    // A suffix is needed only while the base is occupied. This also lets a
    // session shed a suffix after an old broken-rename alias is cleaned up.
    const reuse = nextBase === base && (name === nextBase || (await owner.ls()).some(r => r.name === nextBase));
    const claimed = await claimIdentity(owner, bindings!, nextBase, nextTitle, name, reuse ? name : undefined);
    let moved: Bus;
    let token: string;
    try {
      token = await owner.token(claimed.name);
      moved = new Bus({ ...env, AGENT_BUS_NAME: claimed.name, AGENT_BUS_TOKEN: token });
      if ((await moved.status()).you !== claimed.name) throw new Error("new session credential did not authenticate");
      if (title) {
        if (codex) await codex.rename(nextTitle);
        else if (opencode) await opencode.rename(nextTitle);
        else appendFileSync(session.file!, JSON.stringify({ type: "custom-title", customTitle: nextTitle }) + "\n", { mode: 0o600 });
      }
    } catch (e) {
      if (claimed.name !== name) await releaseIdle(owner, claimed.name);
      throw e;
    }
    const old = name;
    push?.stop();
    await push?.done;
    push = undefined;
    bus = moved;
    name = claimed.name;
    label = claimed.label;
    base = nextBase;
    session.name = nextTitle;
    bindings!.save(session.id, name, base);
    Object.assign(faceEnv, { AGENT_BUS_NAME: name, AGENT_BUS_TOKEN: token, AGENT_BUS_DESCR: label });
    saveEnv();
    if (codex) bindThread({ id: session.id });
    else if (opencode) bindSession(session.id);
    terminal?.set(label);
    const released = old === name ? "address unchanged" : await releaseIdle(owner, old);
    return `registered as ${name} (${label}); ${released}`;
  }));
  Object.assign(faceEnv, control.env);
  saveEnv();
  // The same session, the same binding: what to type when this one cannot recover in place.
  const resumeHint = () => `resume this session with \`ab-${runtime} ${codex ? `resume ${session.id}` : opencode ? `--session ${session.id}` : `--resume ${session.id}`}\` in ${cwd}`;
  // The MCP face runs under the runtime, which neither restarts it nor tells us
  // when it dies: the session keeps typing, its bus tools fail, and a pushed
  // message would be consumed with nothing to answer it. So a lost face stops
  // delivery, is reported, and is brought back where the runtime allows it
  // (docs/08-runner-role.md#runtime-isolation-and-recovery).
  let faceLost = 0, faceStuck = false, faceResume: (() => void) | undefined;
  const watchFaces = async () => {
    if (stopping) return; // faces end with the session; that is no loss
    const event = faceEvent(!!faceLost, faceMarks(runDir!));
    if (event === "lost") {
      const wasPushing = !!push?.running();
      push?.stop();
      await push?.done;
      push = undefined;
      if (wasPushing) faceResume = () => codex ? bindThread({ id: session.id }) : bindSession(session.id);
      faceLost = Date.now();
      faceStuck = false;
      warn(`the agent-bus MCP server of this session stopped; its bus tools${runtime === "claude" ? " and bus messages" : ""} are unavailable and new messages stay queued for ${name}`);
      if (codex) { warn("reloading the agent-bus MCP server"); await codex.reloadMcp().catch(e => warn(`reload failed: ${e}`)); }
      else if (opencode) { warn("reconnecting the agent-bus MCP server"); await opencode.reconnectMcp("agent-bus").catch(e => warn(`reconnect failed: ${e}`)); }
      else warn("to restore it, type /mcp in this session, select agent-bus and choose Reconnect");
    } else if (event === "back" && faceLost) {
      faceLost = 0;
      faceResume?.();
      faceResume = undefined;
      note(`the agent-bus MCP server is back; bus delivery resumed for ${name}`);
    } else if (faceLost && !faceStuck && Date.now() - faceLost > 20_000) {
      faceStuck = true;
      warn(`the agent-bus MCP server is still down; ${runtime === "claude" ? "type /mcp and reconnect agent-bus, or " : ""}${resumeHint()}`);
    }
  };
  let refreshing = false;
  poll = setInterval(async () => {
    if (refreshing || stopping) return;
    refreshing = true;
    try { await exclusive(async () => {
      if (stopping) return;
      if (codex && !codex.thread) {
        const thread = await codex.loadedThread();
        if (thread) bindThread(thread);
      }
      const title = await currentTitle();
      if (title && title !== session.name) {
        const used = new Set((await owner.ls()).filter(r => r.name !== name).map(r => r.descr));
        let next = title;
        for (let n = 2; used.has(next) || (next !== label && !bindings!.lock(`label:${next}`)); n++) next = `${title} #${n}`;
        await owner.register({ name, kind: "agent", descr: next, personal: true });
        label = next;
        session.name = title;
        terminal?.set(next);
        faceEnv.AGENT_BUS_DESCR = next;
        saveEnv();
      }
      await watchFaces();
      if (push && !push.running()) warn(`bus push is inactive; ${resumeHint()}`);
    }); } catch (e) { warn(`session metadata refresh failed: ${e}`); }
    finally { refreshing = false; }
  }, 2000);
  note(`${label} → ${name}`);
  terminal.set(session.name ? label : undefined);
  const tui = start([binary, ...runtimeArgs, ...titleArgs], { ...cleanEnv, ...faceEnv, ...tuiEnv });
  if (serverChild) {
    const what = codex ? "App Server" : "opencode server";
    return await Promise.race([tui.exited, serverChild.exited.then(() => { throw new Error(`the ${what} stopped while the session was running; ${resumeHint()}`); })]);
  }
  return await tui.exited;
}

function claudeArgs(input: string[]): string[] {
  const out: string[] = [];
  for (let i = 0; i < input.length; i++) {
    const a = input[i]!;
    if (a === "--permission-mode") { i++; continue; }
    if (a.startsWith("--permission-mode=") || a === "--enable-auto-mode") continue;
    out.push(a);
  }
  return out;
}

let code = 1;
try { code = await main(); }
catch (e) { log(e instanceof Error ? e.message : String(e)); }
finally { await cleanup(); }
process.exit(code);
