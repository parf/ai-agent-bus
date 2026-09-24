// H.9.6 live-runtime recovery: one launcher-owned native TUI on a PTY against a
// disposable daemon, kept open across a graceful daemon restart and a bus-child
// crash, then a killed MCP face whose printed recovery is followed.
// bun runtime-recovery.ts BUILT_PROGRAM_DIR NEW_EVIDENCE_DIR codex|opencode|claude [SIGNED_IN_CLAUDE_CONFIG_DIR]
//
// Codex and OpenCode take model decisions from a deterministic loopback
// fixture; Claude uses the real model under a copied claude.ai login. The
// fixture never calls MCP or the bus: the runtime executes every tool.
import { mkdirSync, readFileSync, readdirSync, writeFileSync, openSync, closeSync, copyFileSync, chmodSync, existsSync, appendFileSync } from "node:fs";
import { resolve, join } from "node:path";
import { userInfo } from "node:os";
import { randomBytes } from "node:crypto";
import { Database } from "bun:sqlite";
import { Bus } from "../mcp/bus.ts";

const bin = resolve(process.argv[2]!);
const out = resolve(process.argv[3]!); mkdirSync(out); // no overwriting evidence
const runtime = process.argv[4] as "codex" | "opencode" | "claude";
if (!["codex", "opencode", "claude"].includes(runtime)) throw new Error("expected codex, opencode or claude");
const signedIn = runtime === "claude" ? resolve(process.argv[5] ?? "") : "";
const live = runtime === "claude";
const name = `#${runtime}/recovery@fixture`;
const home = join(out, "home"), cwd = join(out, "work with spaces"), config = join(out, "config");
for (const d of [home, cwd]) mkdirSync(d, { mode: 0o700 });
const env: Record<string, string> = { PATH: process.env.PATH!, TERM: "xterm-256color", LANG: "C.UTF-8" };
const say = (s: string) => { const line = `${new Date().toISOString()} ${s}`; console.log(line); appendFileSync(join(out, "phases.log"), line + "\n"); };

// ---- model fixture (codex, opencode) ---------------------------------------
type Seen = { deliveries: number; history: boolean; sent?: boolean; failure?: string };
const seen = new Map<number, Seen>();
const canaries: string[] = [];
const errors: string[] = [];
let sequence = 0;
function userTexts(body: any): string[] {
  if (runtime === "codex") return (body.input ?? []).filter((x: any) => x.role === "user").map((x: any) => JSON.stringify(x.content));
  return (body.messages ?? []).filter((m: any) => m.role === "user").map((m: any) => JSON.stringify(m.content));
}
function toolOutput(body: any, id: string): string | undefined {
  if (runtime === "codex") { const o = (body.input ?? []).findLast((x: any) => x.type === "function_call_output" && x.call_id === id); return o && JSON.stringify(o.output); }
  const o = (body.messages ?? []).findLast((m: any) => m.role === "tool" && m.tool_call_id === id); return o && JSON.stringify(o.content);
}
function decide(body: any): { text?: string; call?: { id: string; name: string; args: unknown } } {
  if (runtime === "opencode" && JSON.stringify(body.messages?.[0]?.content ?? "").includes("You are a title generator")) return { text: "Recovery session" };
  const users = userTexts(body), last = users.at(-1) ?? "";
  if (last.includes("KEYBOARD")) return { text: "KEYBOARD-OK" };
  const ping = last.match(/PING-(\d+)-([a-f0-9]+)/);
  if (!ping) throw new Error("unexpected model input: " + last.slice(0, 300));
  const n = Number(ping[1]);
  const record = seen.get(n) ?? { deliveries: 0, history: false };
  seen.set(n, record);
  // Each delivered message is one user item; a duplicate delivery is a second.
  record.deliveries = users.filter(u => u.includes(ping[0])).length;
  // The same session holds every earlier exchange.
  record.history = canaries.slice(0, n - 1).every(c => users.some(u => u.includes(`PING-${canaries.indexOf(c) + 1}-${c}`)));
  const tools = runtime === "codex"
    ? body.tools?.find((t: any) => t.name === "mcp__agent_bus")?.tools?.map((t: any) => t.name)
    : body.tools?.map((t: any) => t.function?.name?.replace(/^agent-bus_/, ""));
  const id = `pong-${n}`;
  const output = toolOutput(body, id);
  if (output === undefined) {
    if (!tools?.includes("ab_send")) { record.failure = "ab_send not declared"; return { text: `PONG-FAILED-${n}: the agent-bus tools are not available` }; }
    // The route the pushed message spells out, not one this fixture knows.
    const route = JSON.parse(last.match(/Reply using ab_send with (\{.*?\}) and text/)?.[1]?.replace(/\\"/g, '"') ?? "null");
    if (!route?.to) throw new Error("pushed message spelled out no reply route: " + last.slice(0, 300));
    return { call: { id, name: "ab_send", args: { ...route, text: `PONG-${ping[2]}` } } };
  }
  if (output.includes("the bus accepted")) { record.sent = true; return { text: `PONG-SENT-${n}` }; }
  record.failure = output;
  return { text: `PONG-FAILED-${n}: ${output.slice(0, 200)}` };
}
function stream(item: { text?: string; call?: { id: string; name: string; args: unknown } }): Response {
  const seq = ++sequence;
  if (runtime === "opencode") {
    const id = `chatcmpl-${seq}`;
    const chunk = (delta: unknown, finish_reason: string | null = null) => ({ id, object: "chat.completion.chunk", created: Math.floor(Date.now() / 1000), model: "test", choices: [{ index: 0, delta, finish_reason }] });
    const delta = item.call ? { role: "assistant", tool_calls: [{ index: 0, id: item.call.id, type: "function", function: { name: "agent-bus_" + item.call.name, arguments: JSON.stringify(item.call.args) } }] } : { role: "assistant", content: item.text };
    return new Response([chunk(delta), chunk({}, item.call ? "tool_calls" : "stop")].map(e => `data: ${JSON.stringify(e)}\n\n`).join("") + "data: [DONE]\n\n", { headers: { "content-type": "text/event-stream" } });
  }
  const it: any = item.call
    ? { type: "function_call", id: `fc-${seq}`, call_id: item.call.id, namespace: "mcp__agent_bus", name: item.call.name, arguments: JSON.stringify(item.call.args), status: "completed" }
    : { type: "message", id: `msg-${seq}`, role: "assistant", status: "completed", content: [{ type: "output_text", text: item.text, annotations: [] }] };
  const response = { id: `resp-${seq}`, object: "response", created_at: Math.floor(Date.now() / 1000), status: "completed", output: [it], usage: { input_tokens: 1, output_tokens: 1, total_tokens: 2 } };
  const events: any[] = [{ type: "response.created", response: { ...response, status: "in_progress", output: [] } }, { type: "response.output_item.added", output_index: 0, item: { ...it, ...(item.call ? { arguments: "" } : { content: [] }), status: "in_progress" } }];
  if (item.call) events.push({ type: "response.function_call_arguments.delta", output_index: 0, item_id: it.id, delta: it.arguments });
  else events.push({ type: "response.content_part.added", output_index: 0, content_index: 0, item_id: it.id, part: { type: "output_text", text: "", annotations: [] } }, { type: "response.output_text.delta", output_index: 0, content_index: 0, item_id: it.id, delta: item.text });
  events.push({ type: "response.output_item.done", output_index: 0, item: it }, { type: "response.completed", response });
  return new Response(events.map((event, sequence_number) => `event: ${event.type}\ndata: ${JSON.stringify({ ...event, sequence_number })}\n\n`).join(""), { headers: { "content-type": "text/event-stream" } });
}
const provider = live ? undefined : Bun.serve({ hostname: "127.0.0.1", port: 0, async fetch(req) {
  try { const body = await req.json(); appendFileSync(join(out, "provider.jsonl"), JSON.stringify(body) + "\n"); return stream(decide(body)); }
  catch (e) { errors.push(String(e)); return new Response(String(e), { status: 500 }); }
} });

// ---- checks and bounded waits ---------------------------------------------
let checks = 0, failed = 0;
const results: { phase: string; label: string; ok: boolean }[] = [];
let phase = "start";
// A soft check fails the run but lets the later phases still be measured.
function check(ok: boolean, label: string, soft = false) {
  console.log(`${ok ? "ok" : "FAIL"} [${phase}] ${label}`);
  results.push({ phase, label, ok });
  if (ok) checks++; else { failed++; if (!soft) throw new Error(`[${phase}] ${label}`); }
}
let proc: ReturnType<typeof Bun.spawn> | undefined, output = "";
const screenOf = (o: string) => o.replace(/\x1b\[[0-9;?<>=]*[a-zA-Z~]/g, "").replace(/\x1b\][^\x07\x1b]*(\x07|\x1b\\)/g, "").replace(/\x1b[()][0-9A-Za-z]/g, "");
const screen = () => screenOf(output);
const flat = () => screen().replace(/\s+/g, "");
async function until(test: () => boolean | Promise<boolean>, label: string, ms = 20000) {
  const deadline = Date.now() + ms;
  while (Date.now() < deadline) {
    if (errors.length) throw new Error(errors.join("\n"));
    if (await test()) return;
    if (proc && proc.exitCode !== null) throw new Error(`launcher exited (${proc.exitCode}): ${label}`);
    await Bun.sleep(100);
  }
  throw new Error(`timed out: ${label}`);
}

// ---- disposable daemon ----------------------------------------------------
let daemon: ReturnType<typeof Bun.spawn> | undefined, daemons = 0;
function startDaemon() {
  const fd = openSync(join(out, `daemon-${++daemons}.log`), "w");
  daemon = Bun.spawn([join(bin, "agent-busd"), "-addr", "127.0.0.1:0", "-socket", join(out, "bus.sock"), "-owner", "owner@fixture", "-db", join(out, "bus.db"), "-create", "-flush-every", "0"], { env, stdout: fd, stderr: fd });
  closeSync(fd);
}
const children = (pid: number) => Bun.spawnSync(["pgrep", "-P", String(pid)]).stdout.toString().split(/\s+/).filter(Boolean).map(Number);
function descendants(pid: number): number[] { return children(pid).flatMap(c => [c, ...descendants(c)]); }
const cmdline = (pid: number) => { try { return readFileSync(`/proc/${pid}/cmdline`, "utf8").replace(/\0/g, " "); } catch { return ""; } };

startDaemon();
let owner!: Bus, peer!: Bus, parked!: Bus;
async function ready(label: string) {
  await until(async () => { try { return (await owner.status()).you === "owner@fixture"; } catch { return false; } }, `daemon answers after ${label}`, 15000);
}

// ---- Claude's disposable signed-in profile ---------------------------------
const runtimeEnv: Record<string, string> = { HOME: home, XDG_CONFIG_HOME: join(home, "config"), XDG_DATA_HOME: join(home, "data"), XDG_STATE_HOME: join(home, "state"), XDG_CACHE_HOME: join(home, "cache") };
if (runtime === "claude") {
  mkdirSync(config, { mode: 0o700 });
  copyFileSync(join(signedIn, ".credentials.json"), join(config, ".credentials.json"));
  chmodSync(join(config, ".credentials.json"), 0o600);
  const account = JSON.parse(readFileSync(join(signedIn, ".claude.json"), "utf8"));
  writeFileSync(join(config, ".claude.json"), JSON.stringify({ hasCompletedOnboarding: true, theme: "dark", oauthAccount: account.oauthAccount, userID: account.userID, projects: { [cwd]: { hasTrustDialogAccepted: true } } }), { mode: 0o600 });
  runtimeEnv.CLAUDE_CONFIG_DIR = config;
} else if (runtime === "codex") {
  runtimeEnv.CODEX_HOME = home;
  writeFileSync(join(home, "config.toml"), `model="fixture"\nmodel_provider="fixture"\n[model_providers.fixture]\nname="fixture"\nbase_url="http://127.0.0.1:${provider!.port}/v1"\nwire_api="responses"\nrequires_openai_auth=false\n[projects.${JSON.stringify(cwd)}]\ntrust_level="trusted"\n`);
} else {
  runtimeEnv.OPENCODE_DISABLE_MODELS_FETCH = "1";
  const dir = join(home, "config/opencode"); mkdirSync(dir, { recursive: true });
  writeFileSync(join(dir, "opencode.json"), JSON.stringify({ model: "fixture/test", small_model: "fixture/test", enabled_providers: ["fixture"], plugin: [], provider: { fixture: { npm: "@ai-sdk/openai-compatible", name: "Fixture", options: { baseURL: `http://127.0.0.1:${provider!.port}/v1`, apiKey: "fixture-only" }, models: { test: { name: "Fixture", limit: { context: 100000, output: 10000 } } } } } }));
}

// ---- session binding: bus name, launcher binding, runtime session id -------
function runtimeSessions(): string[] {
  if (runtime === "codex") {
    const found: string[] = [];
    const walk = (d: string) => { if (!existsSync(d)) return; for (const e of readdirSync(d, { withFileTypes: true })) e.isDirectory() ? walk(join(d, e.name)) : e.name.startsWith("rollout-") && found.push(e.name.match(/([0-9a-f-]{36})\.jsonl$/)?.[1] ?? e.name); };
    walk(join(home, "sessions"));
    return found.sort();
  }
  if (runtime === "opencode") {
    const db = join(home, "data/opencode/opencode.db");
    if (!existsSync(db)) return [];
    const h = new Database(db, { readonly: true });
    try { return (h.query("select id from session where parent_id is null").all() as { id: string }[]).map(r => r.id).sort(); } finally { h.close(); }
  }
  const dir = join(config, "projects", cwd.replace(/[^a-zA-Z0-9]/g, "-"));
  return existsSync(dir) ? readdirSync(dir).filter(f => /^[0-9a-f-]{36}\.jsonl$/.test(f)).map(f => f.slice(0, -6)).sort() : [];
}
function launcherBindings(): string {
  const dir = join(home, "state/agent-bus/sessions");
  return JSON.stringify(readdirSync(dir).filter(f => f.endsWith(".json")).sort().map(f => [f, JSON.parse(readFileSync(join(dir, f), "utf8"))]));
}
let binding: { sessions: string; bindings: string } | undefined;
async function sameBinding() {
  const now = { sessions: JSON.stringify(runtimeSessions()), bindings: launcherBindings() };
  if (!binding) { binding = now; check(JSON.parse(now.sessions).length === 1, `one runtime session ${now.sessions}`); return; }
  check(now.sessions === binding.sessions, `same runtime session id ${now.sessions}`);
  check(now.bindings === binding.bindings, "same launcher binding (session id -> bus name)");
  const rows = (await owner.ls()).filter(r => r.name.startsWith(`#${runtime}/`));
  check(rows.length === 1 && rows[0]!.name === name, `one bus address, unchanged: ${rows.map(r => r.name).join(",")}`);
}

// ---- one correlated exchange ------------------------------------------------
async function exchange(n: number, ms = live ? 180000 : 30000) {
  const canary = randomBytes(8).toString("hex");
  canaries[n - 1] = canary;
  const topic = "recovery", tag = `x${n}-${randomBytes(3).toString("hex")}`;
  const body = live
    ? `Acceptance test ${n}. Answer this message with ab_reply, body exactly: PONG-${canary}. Do nothing else.`
    : `PING-${n}-${canary}`;
  await peer.send({ to: name, topic, tag, body });
  let reply: any;
  const deadline = Date.now() + ms;
  while (!reply && Date.now() < deadline) {
    if (errors.length) throw new Error(errors.join("\n"));
    if (proc && proc.exitCode !== null) throw new Error(`launcher exited during exchange ${n}`);
    try { reply = await peer.consume({ topic, tag, wait: "2s" }); } catch { await Bun.sleep(200); }
  }
  check(!!reply, `exchange ${n}: a correlated reply arrives`);
  check(reply.from === name, `exchange ${n}: reply comes from the same session address (${reply.from})`);
  check(reply.body.includes(`PONG-${canary}`), `exchange ${n}: reply carries this exchange's random value`);
  // Nothing else follows: no duplicate reply, nothing misrouted to the peer.
  const extra = await peer.consume({ wait: "3s" });
  check(extra === null, `exchange ${n}: exactly one reply, nothing else in the peer inbox`);
  if (live) {
    const lines = transcript();
    const events = lines.split("\n").filter(l => l.includes(`<channel source=\\"agent-bus\\"`) && l.includes(`tag=\\"${tag}\\"`) && l.includes('"type":"user"'));
    check(events.length === 1, `exchange ${n}: delivered once as a channel event (${events.length})`);
    check(canaries.slice(0, n - 1).every(c => lines.includes(`PONG-${c}`)), `exchange ${n}: the same transcript holds every earlier exchange`);
  } else {
    const s = seen.get(n);
    check(s?.deliveries === 1, `exchange ${n}: delivered to the model once (${s?.deliveries})`);
    check(!!s?.history, `exchange ${n}: the model turn carries every earlier exchange (same conversation)`);
    check(!!s?.sent, `exchange ${n}: the runtime's own MCP ab_send carried the reply`);
  }
  await sameBinding();
}
function transcript(): string {
  const dir = join(config, "projects");
  if (!existsSync(dir)) return "";
  return readdirSync(dir).flatMap(d => readdirSync(join(dir, d)).filter(f => f.endsWith(".jsonl")).map(f => readFileSync(join(dir, d, f), "utf8"))).join("\n");
}
async function reading() {
  await until(async () => { try { return !!(await owner.ls()).find(r => r.name === name)?.reading; } catch { return false; } }, "session inbox has its reader again", 30000);
}

// ---- launch ----------------------------------------------------------------
let ownerEnv!: Record<string, string>;
function launch(extra: string[] = []) {
  output = "";
  proc = Bun.spawn([join(bin, `ab-${runtime}`), ...extra], {
    cwd, env: { ...ownerEnv, ...runtimeEnv, AGENT_BUS_NAME: name },
    terminal: { cols: 160, rows: 45, data(_t, b) { const s = Buffer.from(b).toString(); output += s; appendFileSync(join(out, "tui.log"), s); if (s.includes("\x1b[6n")) proc?.terminal?.write("\x1b[1;1R"); } },
  });
}
async function started() {
  let confirmed = false;
  await until(async () => {
    if (live && !confirmed && flat().includes("Iamusingthisforlocaldevelopment")) { confirmed = true; await Bun.sleep(500); proc!.terminal!.write("\r"); }
    const tui = runtime === "codex" ? output.includes("Ask Codex") : runtime === "opencode" ? output.includes("commands") : true;
    return tui && !!(await owner.ls()).find(r => r.name === name)?.reading;
  }, "native TUI and inbox reader ready", 90000);
}
async function type(text: string) {
  proc!.terminal!.write(text);
  await until(() => output.includes(text) || flat().includes(text.replace(/\s/g, "")), `typed ${text} rendered`);
  await Bun.sleep(300); // a same-write Enter would be a paste, not a submit
  proc!.terminal!.write("\r");
}
async function stopLauncher() {
  if (!proc) return;
  proc.kill();
  await Promise.race([proc.exited, Bun.sleep(5000)]);
  if (proc.exitCode === null) proc.kill(9);
  await proc.exited; proc.terminal?.close();
}

// Claude runs its MCP servers itself; the person restores one from /mcp.
// Keys go one at a time, and each is judged by what the TUI drew after it.
async function followClaudeRecovery() {
  const snap = (label: string) => writeFileSync(join(out, `claude-${label}.txt`), screen().slice(-3000));
  const key = async (k: string) => { const from = output.length; proc!.terminal!.write(k); await Bun.sleep(600); return screenOf(output.slice(from)).replace(/\s+/g, ""); };
  proc!.terminal!.write("/mcp");
  await until(() => flat().endsWith("/mcp") || screen().slice(-400).includes("/mcp"), "/mcp typed", 10000);
  await Bun.sleep(300);
  let drawn = await key("\r");
  await until(() => { drawn += screenOf(output.slice(-4000)).replace(/\s+/g, ""); return drawn.includes("ManageMCPservers"); }, "the MCP server list opens", 15000);
  snap("list");
  let selected = /❯[^A-Za-z0-9]{0,3}agent-bus/.test(drawn);
  for (let i = 0; i < 30 && !selected; i++) selected = /❯[^A-Za-z0-9]{0,3}agent-bus/.test(await key("\x1b[B"));
  snap("selected");
  check(selected, "agent-bus selected in /mcp");
  drawn = await key("\r");
  await until(() => { drawn += screenOf(output.slice(-3000)).replace(/\s+/g, ""); return /Reconnect/.test(drawn); }, "the agent-bus menu offers Reconnect", 15000);
  snap("menu");
  let onReconnect = /❯\d\.Reconnect/.test(drawn);
  for (let i = 0; i < 6 && !onReconnect; i++) onReconnect = /❯\d\.Reconnect/.test(await key("\x1b[B"));
  check(onReconnect, "Reconnect selected");
  await key("\r");
  snap("reconnected");
}

try {
  let token = "";
  await until(() => { try { token = Bun.spawnSync([join(bin, "agent-bus-token"), "owner@fixture"], { env: { ...process.env, AGENT_BUS_ADDR: join(out, `user-${userInfo().username}.sock`) } }).stdout.toString().trim(); return !!token; } catch { return false; } }, "disposable daemon ready");
  ownerEnv = { ...env, AGENT_BUS_ADDR: join(out, "bus.sock"), AGENT_BUS_NAME: "owner@fixture", AGENT_BUS_TOKEN: token };
  owner = new Bus(ownerEnv);
  await owner.register({ name: "#peer@fixture", kind: "agent", allow: ["@owner"] });
  await owner.register({ name: "#parked@fixture", kind: "agent" });
  peer = await owner.as("#peer@fixture");
  parked = await owner.as("#parked@fixture");
  await owner.register({ name, kind: "agent", allow: ["@owner"] });

  phase = "launch";
  launch();
  await started();
  if (!live) { await type("KEYBOARD one"); await until(() => output.includes("KEYBOARD-OK"), "native TUI answers keyboard input"); }
  // The person at the keyboard asks for the answers; a real model otherwise
  // may treat a peer's request as untrusted input and hold off.
  else { await type("I am running an agent-bus recovery test. Answer each agent-bus channel message from #peer@fixture with ab_reply, exactly as it asks. Reply OK now."); await until(() => transcript().includes('"type":"assistant"'), "the model answers the typed instruction", 120000); }
  await reading();
  check(true, "session registered and reading");
  await exchange(1);

  phase = "graceful-restart";
  const parkedID = (await owner.send({ to: "#parked@fixture", body: "PARKED-GRACEFUL" })).message_id;
  const before = daemon!.pid;
  daemon!.kill("SIGTERM");
  await Promise.race([daemon!.exited, Bun.sleep(10000)]);
  check(daemon!.exitCode !== null, "daemon stopped gracefully");
  check(proc!.exitCode === null, "runtime TUI stays open while the daemon is down");
  await Bun.sleep(1000);
  startDaemon();
  check(daemon!.pid !== before, "a new daemon process");
  await ready("graceful restart");
  const kept = await parked.consume({ wait: "1s" });
  check(kept?.body === "PARKED-GRACEFUL" && kept.message_id === parkedID, "a queued message survives the graceful stop (snapshot), under its message_id");
  check(await parked.consume({ wait: "1s" }) === null, "no duplicate after the graceful restart");
  await reading();
  await exchange(2);

  phase = "bus-crash";
  const busBefore = children(daemon!.pid);
  check(busBefore.length === 1, `one supervised bus child (${busBefore})`);
  await owner.send({ to: "#parked@fixture", body: "PARKED-CRASH" });
  process.kill(busBefore[0]!, "SIGKILL");
  await until(() => { const now = children(daemon!.pid); return now.length === 1 && now[0] !== busBefore[0]; }, "supervisor restarts the bus child", 10000);
  check(daemon!.exitCode === null, "supervisor survives the bus-child crash");
  check(proc!.exitCode === null, "runtime TUI stays open across the bus-child crash");
  await ready("bus-child crash");
  // PARKED-GRACEFUL was consumed after the last flush and PARKED-CRASH queued
  // after it. The crash may lose the second and may hand the first over again,
  // but only as the same message, so a receiver can drop the repeat
  // (docs/04-messaging.md#durability, Q115).
  const after: { body: string; id: string }[] = [];
  for (let m; after.length < 4 && (m = await parked.consume({ wait: "1s" }));) after.push({ body: m.body, id: m.message_id });
  const repeat = after.filter(m => m.body === "PARKED-GRACEFUL");
  check(repeat.length <= 1 && repeat.every(m => m.id === parkedID), `a message redelivered after the crash keeps its message_id (${JSON.stringify(after)} vs ${parkedID})`);
  check(after.filter(m => m.body === "PARKED-CRASH").length <= 1 && after.length === repeat.length + after.filter(m => m.body === "PARKED-CRASH").length, "a message queued at the crash arrives at most once, and nothing else does");
  await reading();
  await exchange(3);

  phase = "face-kill";
  const faceProcs = () => descendants(proc!.pid).filter(p => /^\S*bun \S*mcp\/server\.(js|ts) ?$/.test(cmdline(p)));
  const faces = faceProcs();
  check(faces.length === 1, `one MCP face under the runtime (${faces})`);
  let mark = output.length;
  // The launcher's own lines, whatever the TUI drew around them.
  const said = (text: string) => screenOf(output.slice(mark)).replace(/\s+/g, "").includes(text.replace(/\s+/g, ""));
  process.kill(faces[0]!, "SIGKILL");
  await until(() => said("the agent-bus MCP server of this session stopped"), "the session reports its bus integration inactive", 10000);
  check(true, "the session reports its bus integration inactive");
  check(proc!.exitCode === null, "runtime TUI stays open after its MCP face is killed");
  if (live) {
    check(said("to restore it, type /mcp in this session, select agent-bus and choose Reconnect"), "the report says how to restore it");
    check(!(await owner.ls()).find(r => r.name === name)?.reading, "the inbox has no reader while the face is down");
    await followClaudeRecovery();
  } else check(said(runtime === "codex" ? "reloading the agent-bus MCP server" : "reconnecting the agent-bus MCP server"), "the launcher restores the face through the runtime");
  await until(() => said("the agent-bus MCP server is back; bus delivery resumed"), "the launcher reports the face back", 60000);
  check(true, "the launcher reports bus delivery resumed");
  await reading();
  await exchange(4);
  const restored = faceProcs();
  check(restored.length === 1 && restored[0] !== faces[0], `a new MCP face (${restored})`);

  if (!live) {
    phase = "server-kill";
    const server = descendants(proc!.pid).find(p => (runtime === "codex" ? / app-server / : / serve --port /).test(cmdline(p)));
    check(!!server, `the runtime server runs under the launcher (${server})`);
    mark = output.length;
    process.kill(server!, "SIGKILL");
    await Promise.race([proc!.exited, Bun.sleep(15000)]);
    check(proc!.exitCode !== null, "the launcher ends the session when its runtime server dies");
    const hint = screenOf(output.slice(mark)).replace(/\s+/g, " ").match(new RegExp(`resume this session with \`ab-${runtime} ([^\`]+)\` in`));
    check(!!hint, `the launcher prints how to resume (${hint?.[1]})`);
    proc!.terminal?.close();
    launch(hint![1]!.trim().split(" "));
    await started();
    await reading();
    await exchange(5);
  }

  writeFileSync(join(out, "result.json"), JSON.stringify({ runtime, version: Bun.spawnSync([runtime, "--version"]).stdout.toString().trim(), checks, failed, results }, null, 2));
  console.log(`checks ${checks}, failed ${failed}`);
} catch (e) {
  failed++;
  console.log(`FAIL [${phase}] ${e}`);
} finally {
  writeFileSync(join(out, "tui.txt"), screen());
  await stopLauncher();
  if (daemon) { daemon.kill(); await Promise.race([daemon.exited, Bun.sleep(8000)]); if (daemon.exitCode === null) daemon.kill(9); }
  provider?.stop(true);
  console.log(`checks ${checks}, failed ${failed}`);
  process.exit(failed ? 1 : 0);
}
