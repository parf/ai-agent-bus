// H.9, H.9.1 and H.9.3 on live runtimes: launcher-owned native TUIs on PTYs,
// a disposable daemon, clean runtime profiles and a program dir built as it
// ships. Each phase group can run alone:
//   launch  (H.9)   continuation, fresh start, exact forwarded arguments, active integration
//   failure (H.9.1) missing runtime, no bus configuration, helper failure, runtime exit, enforced mode
//   names   (H.9.3) fallback, assigned and explicit names, collisions, live rename, restart
// bun runtime-launch.ts BUILT_PROGRAM_DIR NEW_EVIDENCE_DIR codex|opencode|claude [SIGNED_IN_CLAUDE_CONFIG_DIR] [launch,failure,names]
//
// Codex and OpenCode take model decisions from a deterministic loopback
// fixture that never calls MCP or the bus. Claude uses the real model under a
// copied claude.ai login, because only such a profile has channels.
import { mkdirSync, readFileSync, readdirSync, writeFileSync, openSync, closeSync, copyFileSync, chmodSync, existsSync, appendFileSync, symlinkSync, statSync } from "node:fs";
import { resolve, join } from "node:path";
import { userInfo } from "node:os";
import { randomBytes } from "node:crypto";
import { Database } from "bun:sqlite";
import { Bus, defaultName } from "../mcp/bus.ts";
import { Codex } from "../mcp/codex.ts";
import { Opencode } from "../mcp/opencode.ts";

const bin = resolve(process.argv[2]!);
const out = resolve(process.argv[3]!); mkdirSync(out); // never overwrite evidence
const runtime = process.argv[4] as "codex" | "opencode" | "claude";
if (!["codex", "opencode", "claude"].includes(runtime)) throw new Error("expected codex, opencode or claude");
const live = runtime === "claude";
if (live && !process.argv[5]) throw new Error("claude needs a signed-in configuration directory to copy the login from");
const signedIn = live ? resolve(process.argv[5]!) : "";
const phases = new Set((process.argv[live ? 6 : 5] ?? "launch,failure,names").split(","));
const home = join(out, "home"), config = join(out, "config");
mkdirSync(home, { mode: 0o700 });
const dirs: Record<string, string> = {};
for (const d of ["other dir", "work with spaces", "sibling dir", "plain dir", "helper dir", "exit dir", "mode dir", "fallback dir", "explicit dir", "twin a/same", "twin b/same"]) {
  dirs[d] = join(out, d); mkdirSync(dirs[d]!, { recursive: true });
}
const env: Record<string, string> = { PATH: process.env.PATH!, TERM: "xterm-256color", LANG: "C.UTF-8" };
const say = (s: string) => appendFileSync(join(out, "phases.log"), `${new Date().toISOString()} ${s}\n`);

// ---- model fixture (codex, opencode) ---------------------------------------
const errors: string[] = [];
const requests: any[] = [];
const renames: string[] = [];
const pongs = new Map<string, { sent: boolean; failure?: string }>();
const listed: string[] = [];
const shells: string[] = [];
let sequence = 0;
function userTexts(body: any): string[] {
  if (runtime === "codex") return (body.input ?? []).filter((x: any) => x.role === "user").map((x: any) => JSON.stringify(x.content));
  return (body.messages ?? []).filter((m: any) => m.role === "user").map((m: any) => JSON.stringify(m.content));
}
function toolOutput(body: any, id: string): string | undefined {
  if (runtime === "codex") { const o = (body.input ?? []).findLast((x: any) => x.type === "function_call_output" && x.call_id === id); return o && JSON.stringify(o.output); }
  const o = (body.messages ?? []).findLast((m: any) => m.role === "tool" && m.tool_call_id === id); return o && JSON.stringify(o.content);
}
type Item = { text?: string; call?: { id: string; name: string; args: unknown; native?: boolean } };
async function decide(body: any): Promise<Item> {
  if (runtime === "opencode" && JSON.stringify(body.messages?.[0]?.content ?? "").includes("You are a title generator")) return { text: "Fixture title" };
  const users = userTexts(body), last = users.at(-1) ?? "";
  const keyboard = last.match(/KEYBOARD ([a-z0-9-]+)/);
  if (keyboard) return { text: `KEYBOARD-OK ${keyboard[1]}` };
  const shell = last.match(/SHELL (.+?ENFORCED-[a-f0-9]+)/);
  if (shell) {
    const id = `shell-${shell[1]!.slice(-8)}`;
    const done = toolOutput(body, id);
    if (done !== undefined) { shells.push(done); return { text: `SHELL-DONE ${done.slice(0, 120)}` }; }
    const cmd = `touch ${JSON.stringify(shell[1])}`;
    return { call: runtime === "codex" ? { id, name: "exec_command", args: { cmd }, native: true } : { id, name: "bash", args: { command: cmd, description: "Create the marker file" }, native: true } };
  }
  if (last.includes("LIST-BUS")) {
    const done = toolOutput(body, "list-bus");
    if (done === undefined) return { call: { id: "list-bus", name: "ab_ls", args: {} } };
    listed.push(done); return { text: "LISTED" };
  }
  const ping = last.match(/PING-([a-z0-9]+)-([a-f0-9]+)/);
  if (!ping) throw new Error("unexpected model input: " + last.slice(0, 300));
  const id = `pong-${ping[1]}`;
  const record = pongs.get(ping[1]!) ?? { sent: false }; pongs.set(ping[1]!, record);
  const output = toolOutput(body, id);
  // Asked to rename first: the reply stays pending across the session's own ab_rename.
  const wanted = last.match(/RENAME-TO ([A-Za-z ]+?) THEN/)?.[1];
  if (wanted && toolOutput(body, `rename-${ping[1]}`) === undefined) return { call: { id: `rename-${ping[1]}`, name: "ab_rename", args: { name: wanted } } };
  if (wanted && output === undefined) renames.push(toolOutput(body, `rename-${ping[1]}`)!);
  if (output === undefined) {
    // The route the pushed message spells out, not one this fixture knows.
    const route = JSON.parse(last.match(/Reply using ab_send with (\{.*?\}) and text/)?.[1]?.replace(/\\"/g, '"') ?? "null");
    if (!route?.to) throw new Error("pushed message spelled out no reply route: " + last.slice(0, 300));
    return { call: { id, name: "ab_send", args: { ...route, text: `PONG-${ping[2]}` } } };
  }
  if (output.includes("the bus accepted")) { record.sent = true; return { text: `PONG-SENT ${ping[1]}` }; }
  record.failure = output; return { text: `PONG-FAILED ${ping[1]}: ${output.slice(0, 200)}` };
}
function stream(item: Item, model: string): Response {
  const seq = ++sequence;
  if (runtime === "opencode") {
    const id = `chatcmpl-${seq}`;
    const chunk = (delta: unknown, finish_reason: string | null = null) => ({ id, object: "chat.completion.chunk", created: Math.floor(Date.now() / 1000), model, choices: [{ index: 0, delta, finish_reason }] });
    const delta = item.call ? { role: "assistant", tool_calls: [{ index: 0, id: item.call.id, type: "function", function: { name: (item.call.native ? "" : "agent-bus_") + item.call.name, arguments: JSON.stringify(item.call.args) } }] } : { role: "assistant", content: item.text };
    return new Response([chunk(delta), chunk({}, item.call ? "tool_calls" : "stop")].map(e => `data: ${JSON.stringify(e)}\n\n`).join("") + "data: [DONE]\n\n", { headers: { "content-type": "text/event-stream" } });
  }
  const it: any = item.call
    ? { type: "function_call", id: `fc-${seq}`, call_id: item.call.id, ...(item.call.native ? {} : { namespace: "mcp__agent_bus" }), name: item.call.name, arguments: JSON.stringify(item.call.args), status: "completed" }
    : { type: "message", id: `msg-${seq}`, role: "assistant", status: "completed", content: [{ type: "output_text", text: item.text, annotations: [] }] };
  const response = { id: `resp-${seq}`, object: "response", created_at: Math.floor(Date.now() / 1000), status: "completed", output: [it], usage: { input_tokens: 1, output_tokens: 1, total_tokens: 2 } };
  const events: any[] = [{ type: "response.created", response: { ...response, status: "in_progress", output: [] } }, { type: "response.output_item.added", output_index: 0, item: { ...it, ...(item.call ? { arguments: "" } : { content: [] }), status: "in_progress" } }];
  if (item.call) events.push({ type: "response.function_call_arguments.delta", output_index: 0, item_id: it.id, delta: it.arguments });
  else events.push({ type: "response.content_part.added", output_index: 0, content_index: 0, item_id: it.id, part: { type: "output_text", text: "", annotations: [] } }, { type: "response.output_text.delta", output_index: 0, content_index: 0, item_id: it.id, delta: item.text });
  events.push({ type: "response.output_item.done", output_index: 0, item: it }, { type: "response.completed", response });
  return new Response(events.map((event, sequence_number) => `event: ${event.type}\ndata: ${JSON.stringify({ ...event, sequence_number })}\n\n`).join(""), { headers: { "content-type": "text/event-stream" } });
}
const provider = live ? undefined : Bun.serve({ hostname: "127.0.0.1", port: 0, idleTimeout: 120, async fetch(req) {
  try { const body = await req.json() as any; requests.push(body); appendFileSync(join(out, "provider.jsonl"), JSON.stringify(body) + "\n"); return stream(await decide(body), body.model); }
  catch (e) { errors.push(String(e)); return new Response(String(e), { status: 500 }); }
} });

// ---- checks and bounded waits ---------------------------------------------
let checks = 0, failed = 0, phase = "start";
function check(ok: boolean, label: string) {
  console.log(`${ok ? "ok" : "FAIL"} [${phase}] ${label}`);
  if (ok) checks++; else { failed++; throw new Error(`[${phase}] ${label}`); }
}
async function until(test: () => boolean | Promise<boolean>, label: string, ms = 30000, watch: Session[] = []) {
  const deadline = Date.now() + ms;
  while (Date.now() < deadline) {
    if (errors.length) throw new Error(errors.join("\n"));
    if (await test()) return;
    for (const s of watch) if (s.proc.exitCode !== null) throw new Error(`launcher ${s.label} exited (${s.proc.exitCode}): ${label}`);
    await Bun.sleep(100);
  }
  throw new Error(`timed out: ${label}`);
}
const screenOf = (o: string) => o.replace(/\x1b\[[0-9;?<>=]*[a-zA-Z~]/g, "").replace(/\x1b\][^\x07\x1b]*(\x07|\x1b\\)/g, "").replace(/\x1b[()][0-9A-Za-z]/g, "");
const flat = (o: string) => screenOf(o).replace(/\s+/g, "");
const children = (pid: number) => Bun.spawnSync(["pgrep", "-P", String(pid)]).stdout.toString().split(/\s+/).filter(Boolean).map(Number);
function descendants(pid: number): number[] { return children(pid).flatMap(c => [c, ...descendants(c)]); }
const argv = (pid: number) => { try { return readFileSync(`/proc/${pid}/cmdline`, "utf8").split("\0").slice(0, -1); } catch { return []; } };
// Every process this gate's runtime profile started, found by its private HOME.
function profileProcs(): number[] {
  return readdirSync("/proc").filter(p => /^\d+$/.test(p)).map(Number).filter(p => {
    try { return readFileSync(`/proc/${p}/environ`, "utf8").split("\0").includes(`HOME=${home}`); } catch { return false; }
  });
}
const aliveNow = (pids: number[]) => pids.filter(p => { try { process.kill(p, 0); return !readFileSync(`/proc/${p}/stat`, "utf8").split(") ")[1]!.startsWith("Z"); } catch { return false; } });

// ---- disposable daemon and profile ------------------------------------------
const fd = openSync(join(out, "daemon.log"), "w");
const daemon = Bun.spawn([join(bin, "agent-busd"), "-addr", "127.0.0.1:0", "-socket", join(out, "bus.sock"), "-owner", "owner@fixture", "-db", join(out, "bus.db"), "-create", "-flush-every", "0"], { env, stdout: fd, stderr: fd });
closeSync(fd);
const runtimeEnv: Record<string, string> = { HOME: home, XDG_CONFIG_HOME: join(home, "config"), XDG_DATA_HOME: join(home, "data"), XDG_STATE_HOME: join(home, "state"), XDG_CACHE_HOME: join(home, "cache"), AGENT_BUS_REALM: "fixture" };
if (live) {
  mkdirSync(config, { mode: 0o700 });
  copyFileSync(join(signedIn, ".credentials.json"), join(config, ".credentials.json"));
  chmodSync(join(config, ".credentials.json"), 0o600);
  const account = JSON.parse(readFileSync(join(signedIn, ".claude.json"), "utf8"));
  writeFileSync(join(config, ".claude.json"), JSON.stringify({ hasCompletedOnboarding: true, theme: "dark", oauthAccount: account.oauthAccount, userID: account.userID, projects: Object.fromEntries(Object.values(dirs).map(d => [d, { hasTrustDialogAccepted: true }])) }), { mode: 0o600 });
  // The account's default model: auto mode, which the launcher enforces, is not offered on every model.
  runtimeEnv.CLAUDE_CONFIG_DIR = config;
} else if (runtime === "codex") {
  runtimeEnv.CODEX_HOME = home;
  writeFileSync(join(home, "config.toml"), `model="fixture"\nmodel_provider="fixture"\n[model_providers.fixture]\nname="fixture"\nbase_url="http://127.0.0.1:${provider!.port}/v1"\nwire_api="responses"\nrequires_openai_auth=false\n` + Object.values(dirs).map(d => `[projects.${JSON.stringify(d)}]\ntrust_level="trusted"\n`).join(""));
} else {
  runtimeEnv.OPENCODE_DISABLE_MODELS_FETCH = "1";
  const dir = join(home, "config/opencode"); mkdirSync(dir, { recursive: true });
  // A contrary profile: this user's own config asks before every tool. The
  // launcher's enforced mode must still apply (H.9.1).
  writeFileSync(join(dir, "opencode.json"), JSON.stringify({ model: "fixture/test", small_model: "fixture/test", enabled_providers: ["fixture"], plugin: [], permission: { bash: "ask", edit: "ask", external_directory: "ask" }, provider: { fixture: { npm: "@ai-sdk/openai-compatible", name: "Fixture", options: { baseURL: `http://127.0.0.1:${provider!.port}/v1`, apiKey: "fixture-only" }, models: { test: { name: "Fixture", limit: { context: 100000, output: 10000 } } } } } }));
}
const derived = (where: string) => defaultName({ AGENT_BUS_RUNTIME: runtime, AGENT_BUS_CWD: where, AGENT_BUS_REALM: "fixture" }, "/");

// ---- sessions -----------------------------------------------------------------
type Session = { label: string; cwd: string; proc: ReturnType<typeof Bun.spawn>; output: string; said: (t: string) => boolean; stderrText?: string };
let ownerEnv!: Record<string, string>, owner!: Bus, peer!: Bus;
function launch(label: string, cwd: string, args: string[] = [], extra: Record<string, string | undefined> = {}, wrap: string[] = []): Session {
  const s = { label, cwd, output: "" } as Session;
  const e: Record<string, string> = { ...ownerEnv, ...runtimeEnv };
  for (const [k, v] of Object.entries(extra)) { if (v === undefined) delete e[k]; else e[k] = v; }
  s.proc = Bun.spawn([...wrap, join(bin, `ab-${runtime}`), ...args], {
    cwd, env: e,
    terminal: { cols: 160, rows: 45, data(_t, b) { const t = Buffer.from(b).toString(); s.output += t; appendFileSync(join(out, `tui-${label}.log`), t); if (t.includes("\x1b[6n")) s.proc?.terminal?.write("\x1b[1;1R"); } },
  });
  s.said = (t: string) => flat(s.output).includes(t.replace(/\s+/g, ""));
  say(`launch ${label} in ${cwd}: ${JSON.stringify(args)}`);
  return s;
}
async function started(s: Session, name?: string, channel = true) {
  // A launcher-started Claude asks once about development channels, sometimes
  // after its prompt is drawn; typing before that answer would go to the dialog.
  let confirmed = !(live && channel);
  await until(async () => {
    if (!confirmed && s.said("I am using this for local development")) { confirmed = true; await Bun.sleep(500); s.proc.terminal!.write("\r"); await Bun.sleep(1000); }
    const tui = runtime === "codex" ? s.output.includes("Ask Codex") || s.said("Ask Codex") : runtime === "opencode" ? s.output.includes("commands") : s.said("for shortcuts") || s.said("mode on");
    return tui && confirmed;
  }, `${s.label}: native TUI ready`, 90000, [s]);
  if (!name) return;
  const reading = async () => !!(await owner.ls()).find(r => r.name === name)?.reading;
  // A fresh Codex TUI creates its thread with its first turn; its pusher attaches then.
  if (runtime === "codex") { await Bun.sleep(3000); if (!await reading()) await keyboard(s, `warmup-${s.label}`); }
  await until(reading, `${s.label}: inbox reader for ${name} ready`, 30000, [s]);
}
// Codex binds the conversation it created at its next metadata poll; a person
// quitting before that would restart unbound. Wait for the reader it brings.
async function bound(s: Session, name: string) {
  await until(async () => !!(await record(name))?.reading, `${s.label}: the conversation is bound to ${name}`, 30000, [s]);
}
async function type(s: Session, text: string) {
  s.proc.terminal!.write(text);
  await until(() => s.said(text), `${s.label}: typed text rendered`, 15000, [s]);
  await Bun.sleep(300); // a same-write Enter would be a paste, not a submit
  s.proc.terminal!.write("\r");
}
async function stop(s: Session): Promise<number> {
  s.proc.kill("SIGTERM");
  await Promise.race([s.proc.exited, Bun.sleep(8000)]);
  if (s.proc.exitCode === null) s.proc.kill(9);
  const code = await s.proc.exited; s.proc.terminal?.close();
  writeFileSync(join(out, `tui-${s.label}.txt`), screenOf(s.output));
  return code;
}
// Claude's transcripts, by directory.
function transcriptDir(cwd: string) { return join(config, "projects", cwd.replace(/[^a-zA-Z0-9]/g, "-")); }
function transcript(cwd: string): string {
  const d = transcriptDir(cwd);
  return existsSync(d) ? readdirSync(d).filter(f => f.endsWith(".jsonl")).map(f => readFileSync(join(d, f), "utf8")).join("\n") : "";
}
/** Runtime conversations saved for one directory. */
function conversations(cwd: string): string[] {
  if (runtime === "codex") {
    const found: string[] = [];
    const walk = (d: string) => { if (!existsSync(d)) return; for (const e of readdirSync(d, { withFileTypes: true })) {
      if (e.isDirectory()) walk(join(d, e.name));
      else if (e.name.startsWith("rollout-")) { const meta = JSON.parse(readFileSync(join(d, e.name), "utf8").split("\n")[0]!); if (meta.payload?.cwd === cwd) found.push(meta.payload.id); }
    } };
    walk(join(home, "sessions"));
    return found.sort();
  }
  if (runtime === "opencode") {
    const db = join(home, "data/opencode/opencode.db");
    if (!existsSync(db)) return [];
    const h = new Database(db, { readonly: true });
    try { return (h.query("select id from session where parent_id is null and directory = ?").all(cwd) as { id: string }[]).map(r => r.id).sort(); } finally { h.close(); }
  }
  const d = transcriptDir(cwd);
  return existsSync(d) ? readdirSync(d).filter(f => /^[0-9a-f-]{36}\.jsonl$/.test(f)).map(f => f.slice(0, -6)).sort() : [];
}
/** A title the runtime gave the conversation itself (OpenCode generates one from the first turn). */
function titleOf(cwd: string): string | undefined {
  if (runtime !== "opencode") return undefined;
  const h = new Database(join(home, "data/opencode/opencode.db"), { readonly: true });
  try { return (h.query("select title from session where parent_id is null and directory = ?").get(cwd) as { title: string } | null)?.title; } finally { h.close(); }
}
const bindingsState = () => { const d = join(home, "state/agent-bus/sessions"); return existsSync(d) ? readdirSync(d) : []; };
const runs = () => { const d = join(home, "state/agent-bus/sessions/runs"); return existsSync(d) ? readdirSync(d) : []; };
function runSettings(s: Session): Record<string, string> {
  // Each launch's private run directory holds its session file; pick the one whose face env names this session's control.
  const d = join(home, "state/agent-bus/sessions/runs");
  for (const r of readdirSync(d)) {
    const f = join(d, r, "bus-env.json");
    if (!existsSync(f)) continue;
    const j = JSON.parse(readFileSync(f, "utf8"));
    // Its App Server or TUI names the run directory in argv; OpenCode's face has it in its environment.
    const environ = (p: number) => { try { return readFileSync(`/proc/${p}/environ`, "utf8"); } catch { return ""; } };
    if (descendants(s.proc.pid).some(p => argv(p).some(a => a.includes(join(d, r))) || environ(p).includes(join(d, r)))) return j;
  }
  throw new Error(`${s.label}: no run directory`);
}
const record = async (name: string) => (await owner.ls()).find(r => r.name === name);
const rows = async () => (await owner.ls()).filter(r => r.name.startsWith(`#${runtime}/`));

// One correlated exchange with a session: the peer sends, the session's model
// answers through the runtime's own tools, and only that answer counts.
async function exchange(s: Session, to: string, opts: { expectFrom?: string; rename?: string } = {}) {
  const id = randomBytes(3).toString("hex");
  const canary = randomBytes(8).toString("hex");
  const topic = "launch", tag = `t-${id}`;
  const body = live
    ? `Acceptance test. ${opts.rename ? `First call the agent-bus ab_rename tool with name exactly: ${opts.rename}. Then a` : "A"}nswer this message with ab_reply, body exactly: PONG-${canary}. Do nothing else.`
    : `${opts.rename ? `RENAME-TO ${opts.rename} THEN ` : ""}PING-${id}-${canary}`;
  await peer.send({ to, topic, tag, body });
  let reply: any;
  const deadline = Date.now() + (live ? 180000 : 45000);
  while (!reply && Date.now() < deadline) {
    if (errors.length) throw new Error(errors.join("\n"));
    if (s.proc.exitCode !== null) throw new Error(`${s.label} exited during an exchange`);
    try { reply = await peer.consume({ topic, tag, wait: "2s" }); } catch { await Bun.sleep(200); }
  }
  check(!!reply, `${s.label}: a correlated reply to ${to} arrives`);
  check(reply.from === (opts.expectFrom ?? to), `${s.label}: the reply comes from ${opts.expectFrom ?? to} (${reply.from})`);
  check(reply.body.includes(`PONG-${canary}`), `${s.label}: the reply carries this exchange's random value`);
  check(await peer.consume({ wait: "2s" }) === null, `${s.label}: exactly one reply, nothing else in the peer inbox`);
  if (live) check(transcript(s.cwd).split("\n").filter(l => l.includes('<channel source=\\"agent-bus\\"') && l.includes(`tag=\\"${tag}\\"`) && (l.includes('"type":"user"') || l.includes('"queued_command"'))).length === 1, `${s.label}: delivered once, as a channel event in this session (a busy session queues it as a command)`);
  else {
    // The tool result reaches the model on its next turn, just after the peer has the reply.
    await until(() => !!pongs.get(id)?.sent || !!pongs.get(id)?.failure, `${s.label}: the runtime returns the ab_send result`, 15000, [s]);
    check(!!pongs.get(id)?.sent, `${s.label}: the runtime's own MCP ab_send carried the reply`);
  }
}
async function keyboard(s: Session, word: string) {
  if (live) await type(s, `Reply with exactly this one token and nothing else: KEYBOARD-OK-${word}`);
  else await type(s, `KEYBOARD ${word}`);
  await until(() => answered(s, word), `${s.label}: the native TUI answers typed input`, live ? 120000 : 30000, [s]);
}
// The model's own answer: Claude's screen also shows the typed request, which contains the words.
function answered(s: Session, word: string) {
  return live ? transcript(s.cwd).split("\n").some(l => l.includes('"type":"assistant"') && l.includes(`KEYBOARD-OK-${word}`)) : s.said(`KEYBOARD-OK ${word}`);
}
async function listBus(s: Session) {
  if (live) {
    await type(s, "Call the agent-bus ab_ls tool once, then reply with exactly LISTED and nothing else.");
    await until(() => /"type":"tool_result"[^\n]*#peer@fixture/.test(transcript(s.cwd)), `${s.label}: the runtime's ab_ls result lists the bus`, 120000, [s]);
  } else {
    const before = listed.length;
    await type(s, "LIST-BUS");
    await until(() => listed.length > before, `${s.label}: the runtime executed ab_ls`, 30000, [s]);
    check(listed.at(-1)!.includes("#peer@fixture"), `${s.label}: the runtime's ab_ls result lists the bus`);
  }
  check(true, `${s.label}: bus tools are active`);
}
// Claude's model may treat a peer's channel message as untrusted; the person at the keyboard asks first.
async function prime(s: Session) {
  if (!live) return;
  await type(s, "I am running an agent-bus launcher test. Answer each agent-bus channel message from #peer@fixture with ab_reply, exactly as it asks. Reply OK now.");
  await until(() => transcript(s.cwd).includes('"type":"assistant"'), `${s.label}: the model answers the typed instruction`, 120000, [s]);
}
/** The runtime TUI process under a launcher, and its exact argv. */
function tuiArgv(s: Session): string[] {
  const marker = runtime === "codex" ? "--remote" : runtime === "opencode" ? "attach" : "--mcp-config";
  const found = descendants(s.proc.pid).map(argv).filter(a => a.includes(marker));
  if (found.length !== 1) throw new Error(`${s.label}: expected one runtime TUI, found ${found.length}`);
  return found[0]!;
}
const contains = (hay: string[], needle: string[]) => hay.some((_, i) => needle.every((n, j) => hay[i + j] === n));
const count = (hay: string[], v: string) => hay.filter(a => a === v).length;
// Offline, as a person would in the runtime's own UI: give a saved conversation a title.
async function setTitle(cwd: string, id: string, title: string) {
  if (runtime === "claude") { appendFileSync(join(transcriptDir(cwd), `${id}.jsonl`), JSON.stringify({ type: "custom-title", customTitle: title, sessionId: id }) + "\n"); return; }
  const port = (() => { const l = Bun.listen({ hostname: "127.0.0.1", port: 0, socket: { data() {} } }); const p = l.port; l.stop(true); return p; })();
  const e = { ...env, ...runtimeEnv };
  const server = runtime === "codex"
    ? Bun.spawn(["codex", "app-server", "--listen", `ws://127.0.0.1:${port}`], { cwd, env: e, stdout: "ignore", stderr: "ignore" })
    : Bun.spawn(["opencode", "serve", "--hostname", "127.0.0.1", "--port", String(port)], { cwd, env: { ...e, OPENCODE_SERVER_PASSWORD: "offline-title" }, stdout: "ignore", stderr: "ignore" });
  try {
    if (runtime === "codex") {
      await until(async () => { try { await fetch(`http://127.0.0.1:${port}`, { signal: AbortSignal.timeout(200) }); return true; } catch { return false; } }, "offline App Server ready");
      const c = new Codex(cwd, () => {}, `ws://127.0.0.1:${port}`, "");
      try { await c.start(); await c.openThread(id); await c.rename(title); check(await c.threadName() === title, `offline title "${title}" saved`); } finally { c.stop(); }
    } else {
      const o = new Opencode(cwd, () => {}, `http://127.0.0.1:${port}`, "offline-title");
      await until(() => o.probe(), "offline opencode server ready");
      try { await o.start(); o.bind(id); await o.rename(title); check(await o.title() === title, `offline title "${title}" saved`); } finally { o.stop(); }
    }
  } finally { server.kill(); await Promise.race([server.exited, Bun.sleep(3000)]); if (server.exitCode === null) server.kill(9); }
}
/** The session file the face reads must authenticate as exactly this name. */
async function identity(s: Session, name: string) {
  const settings = runSettings(s);
  check(settings.AGENT_BUS_NAME === name, `${s.label}: the face's session file names ${name} (${settings.AGENT_BUS_NAME})`);
  const you = (await new Bus({ ...env, AGENT_BUS_ADDR: settings.AGENT_BUS_ADDR ?? ownerEnv.AGENT_BUS_ADDR, AGENT_BUS_NAME: name, AGENT_BUS_TOKEN: settings.AGENT_BUS_TOKEN }).status()).you;
  check(you === name, `${s.label}: the face's credential authenticates as ${name} (${you})`);
}
async function cleanedUp(label: string, before: { runs: string[]; locks: string[] }, pids: number[]) {
  await until(() => aliveNow(pids).length === 0, `${label}: every process it started has exited`, 10000);
  check(true, `${label}: every process it started has exited`);
  check(JSON.stringify(runs()) === JSON.stringify(before.runs), `${label}: its private run directory is removed`);
  check(JSON.stringify(bindingsState().filter(f => f.endsWith(".lock"))) === JSON.stringify(before.locks), `${label}: its session locks are released`);
}
const snapshot = () => ({ runs: runs(), locks: bindingsState().filter(f => f.endsWith(".lock")) });

const open: Session[] = [];
try {
  let token = "";
  await until(() => { try { token = Bun.spawnSync([join(bin, "agent-bus-token"), "owner@fixture"], { env: { ...process.env, AGENT_BUS_ADDR: join(out, `user-${userInfo().username}.sock`) } }).stdout.toString().trim(); return !!token; } catch { return false; } }, "disposable daemon ready");
  ownerEnv = { ...env, AGENT_BUS_ADDR: join(out, "bus.sock"), AGENT_BUS_NAME: "owner@fixture", AGENT_BUS_TOKEN: token };
  owner = new Bus(ownerEnv);
  await owner.register({ name: "#peer@fixture", kind: "agent", allow: ["@owner"] });
  peer = await owner.as("#peer@fixture");
  const derivedOnly = { AGENT_BUS_NAME: undefined }; // let the launcher name the session

  if (phases.has("launch")) {
    phase = "other-directory";
    const other = launch("other", dirs["other dir"]!, [], derivedOnly); open.push(other);
    await started(other);
    await keyboard(other, "other");
    const otherIDs = conversations(dirs["other dir"]!);
    check(otherIDs.length === 1, `another directory holds one conversation ${otherIDs}`);
    await stop(other); open.pop();

    phase = "fresh";
    const cwd = dirs["work with spaces"]!;
    const forwarded = runtime === "codex" ? ["-m", "fixture-forwarded", "KEYBOARD forwarded"]
      : runtime === "opencode" ? ["--log-level", "WARN", "--pure"]
      : ["--model", "sonnet", "Reply with exactly this one token and nothing else: KEYBOARD-OK-forwarded"];
    check(conversations(cwd).length === 0, "the launch directory has no history");
    const fresh = launch("fresh", cwd, forwarded, derivedOnly); open.push(fresh);
    await started(fresh);
    const a = tuiArgv(fresh);
    say(`fresh runtime argv ${JSON.stringify(a)}`);
    check(contains(a, forwarded) && forwarded.every(f => count(a, f) === 1), `the runtime receives the forwarded arguments exactly once, in order: ${JSON.stringify(forwarded)}`);
    if (runtime === "opencode") await keyboard(fresh, "forwarded");
    else await until(() => answered(fresh, "forwarded"), "the forwarded prompt is answered", live ? 120000 : 30000, [fresh]);
    if (runtime === "codex") check(requests.some(r => r.model === "fixture-forwarded" && userTexts(r).at(-1)?.includes("KEYBOARD forwarded")), "the forwarded model and prompt reach the provider");
    if (live) check(/"model":"claude-sonnet[^"]*"/.test(transcript(cwd)) && transcript(cwd).includes("Reply with exactly this one token and nothing else: KEYBOARD-OK-forwarded"), "the forwarded model and prompt reach the session");
    const freshIDs = conversations(cwd);
    check(freshIDs.length === 1 && !otherIDs.includes(freshIDs[0]!), `a fresh conversation, not another directory's (${freshIDs} vs ${otherIDs})`);
    check(conversations(dirs["other dir"]!).join() === otherIDs.join(), "the other directory's conversation is untouched");
    if (!live) check(!requests.filter(r => userTexts(r).some(u => u.includes("KEYBOARD forwarded"))).some(r => userTexts(r).some(u => u.includes("KEYBOARD other"))), "the fresh turn carries no other conversation's history");
    const name = derived(cwd);
    await until(async () => !!(await record(name))?.reading, "the session's inbox has its reader", 30000, [fresh]);
    check(true, `the session is registered and reading as ${name}`);
    await listBus(fresh);
    await prime(fresh);
    await exchange(fresh, name);
    const binding = bindingsState().filter(f => f.endsWith(".json")).map(f => readFileSync(join(home, "state/agent-bus/sessions", f), "utf8")).join();
    await stop(fresh); open.pop();

    phase = "history";
    const title = titleOf(cwd);
    const againName = title ? derived(title) : name;
    say(`history: conversation title ${title}, expected address ${againName}`);
    const again = launch("again", cwd, [], derivedOnly); open.push(again);
    await started(again, againName);
    await keyboard(again, "again");
    check(conversations(cwd).join() === freshIDs.join(), `the same conversation continues (${conversations(cwd)})`);
    if (!live) {
      const turn = requests.filter(r => userTexts(r).at(-1)?.includes("KEYBOARD again")).at(-1)!;
      check(userTexts(turn).some(u => u.includes("KEYBOARD forwarded")) && !userTexts(turn).some(u => u.includes("KEYBOARD other")), "the continued turn carries this conversation's history, and no other");
    } else check(answered(again, "forwarded") && answered(again, "again") && !transcript(cwd).includes("KEYBOARD-OK-other"), "one transcript holds this conversation's turns, and no other's");
    const nowBound = bindingsState().filter(f => f.endsWith(".json")).map(f => readFileSync(join(home, "state/agent-bus/sessions", f), "utf8")).join();
    if (title) check(nowBound.includes(`"name":${JSON.stringify(againName)}`) && !nowBound.includes(`"name":${JSON.stringify(name)}`), "the saved binding now names the address its title derives");
    else check(nowBound === binding, "the saved binding is unchanged");
    check((await rows()).filter(r => r.reading).map(r => r.name).join() === againName, `one session address is reading, ${title ? "the one its title now derives" : "the same one"} (${againName})`);
    await exchange(again, againName);
    await stop(again); open.pop();
  }

  if (phases.has("failure")) {
    phase = "sibling";
    const sibling = launch("sibling", dirs["sibling dir"]!, [], { AGENT_BUS_NAME: `#${runtime}/sibling@fixture` }); open.push(sibling);
    await started(sibling, `#${runtime}/sibling@fixture`);
    await prime(sibling);
    // The processes that make the session: launcher, runtime TUI, its server and the MCP face.
    // Runtimes also start short-lived helpers of their own, which may come and go.
    const role = (p: number) => argv(p).join(" ");
    const siblingProcs = [sibling.proc.pid, ...descendants(sibling.proc.pid).filter(p => /--remote|attach|--mcp-config| app-server | serve |mcp\/server\.(js|ts)$/.test(role(p)))];
    say(`sibling core processes ${siblingProcs.map(p => `${p}: ${role(p).slice(0, 80)}`).join("; ")}`);
    const siblingAlive = async (what: string) => {
      check(sibling.proc.exitCode === null && aliveNow(siblingProcs).length === siblingProcs.length, `the separate session keeps its launcher, runtime, server and face after ${what} (${siblingProcs.length})`);
      check(!!(await record(`#${runtime}/sibling@fixture`))?.reading, `the separate session keeps reading after ${what}`);
    };

    phase = "missing-runtime";
    {
      const before = snapshot(), rowsBefore = (await rows()).map(r => r.name).join();
      const p = Bun.spawn([join(bin, `ab-${runtime}`)], { cwd: dirs["exit dir"]!, env: { ...ownerEnv, ...runtimeEnv, PATH: "/usr/bin:/bin", AGENT_BUS_NAME: `#${runtime}/missing@fixture` }, stdout: "pipe", stderr: "pipe" });
      const code = await Promise.race([p.exited, Bun.sleep(20000).then(() => -1)]);
      if (code === -1) p.kill(9);
      const stderr = await new Response(p.stderr).text();
      check(code !== 0 && code !== -1, `the launcher exits non-zero (${code})`);
      check(stderr.includes(`${runtime} executable not found; install the runtime or set ${runtime.toUpperCase()}_BIN`), `the launcher names the missing runtime and its override: ${stderr.trim()}`);
      check((await rows()).map(r => r.name).join() === rowsBefore, "nothing is registered for it");
      check(JSON.stringify(snapshot()) === JSON.stringify(before), "no run directory or lock is left");
      await siblingAlive("a missing runtime");
    }

    phase = "no-bus";
    {
      // Hide this host's installed daemon sockets: the launcher must find no
      // bus, and must never reach the live one (docs/08-runner-role.md#smart-launchers).
      const empty = join(out, "no-sockets"); mkdirSync(empty);
      const runtimeDir = join(out, "no-runtime-dir"); mkdirSync(runtimeDir, { mode: 0o700 });
      const wrap = ["unshare", "-r", "--mount", "sh", "-c", `mount --bind "$0" /run/agent-bus && exec unshare --user --map-user=${process.getuid!()} --map-group=${process.getgid!()} "$@"`, empty];
      const unconfigured = Object.fromEntries(Object.keys(ownerEnv).filter(k => k.startsWith("AGENT_BUS_")).map(k => [k, undefined])) as Record<string, undefined>;
      const plain = launch("plain", dirs["plain dir"]!, [], { ...unconfigured, AGENT_BUS_REALM: undefined, XDG_RUNTIME_DIR: runtimeDir }, wrap); open.push(plain);
      await until(() => plain.said("bus is not configured; starting a plain runtime session"), "the launcher says the bus is not configured and starts a plain session", 30000, [plain]);
      check(true, "the launcher says the bus is not configured and starts a plain session");
      await started(plain, undefined, false);
      await keyboard(plain, "plain");
      const procs = [plain.proc.pid, ...descendants(plain.proc.pid)];
      check(!procs.some(p => argv(p).some(a => /mcp\/server\.(js|ts)$/.test(a))), "a plain session runs no agent-bus MCP face");
      check(!plain.said("→ #"), "a plain session reports no bus identity");
      const code = await stop(plain); open.pop();
      check(code === 143, `a terminated plain session exits 143 (${code})`);
      await until(() => aliveNow(procs).length === 0, "the plain session leaves no process", 10000);
      check(true, "the plain session leaves no process");
      await siblingAlive("a plain session");
    }

    phase = "helper-failure";
    {
      const before = snapshot(), rowsBefore = (await rows()).map(r => r.name).join();
      const known = profileProcs();
      let args: string[] = [], expected = "", reason = "", extra: Record<string, string> = {};
      // Codex refuses a mistyped setting the person passed; OpenCode cannot open its data directory.
      if (runtime === "codex") { args = ["-c", "model_providers=5"]; expected = "App Server exited during startup"; reason = "invalid type: integer `5`, expected a map"; }
      else if (runtime === "opencode") { const ro = join(out, "read-only data"); mkdirSync(ro, { mode: 0o500 }); extra = { XDG_DATA_HOME: ro }; expected = "the opencode server exited during startup"; reason = "EACCES"; }
      else { // Claude replaces its file by rename, so only a read-only directory stops the write.
        chmodSync(join(config, ".claude.json"), 0o400); chmodSync(config, 0o500); expected = "could not register the channel server; this session has tools but no channel"; }
      const helper = launch("helper", dirs["helper dir"]!, args, { ...extra, AGENT_BUS_NAME: `#${runtime}/helper@fixture` }); open.push(helper);
      if (live) {
        // Claude's helper is the channel registration; its failure is reported and the session keeps its tools.
        await until(() => helper.said(expected), "the launcher reports the failed channel registration", 60000, [helper]);
        check(true, `the launcher reports the failed helper: ${expected}`);
        chmodSync(config, 0o700); chmodSync(join(config, ".claude.json"), 0o600);
        await started(helper, `#${runtime}/helper@fixture`);
        const code = await stop(helper); open.pop();
        check(code === 143, `a terminated session exits 143 (${code})`);
      } else {
        const code = await Promise.race([helper.proc.exited, Bun.sleep(30000).then(() => -1)]);
        check(code !== -1, "the launcher gives up rather than waiting on a dead helper");
        open.pop(); writeFileSync(join(out, "tui-helper.txt"), screenOf(helper.output));
        check(code !== 0, `the launcher exits non-zero (${code})`);
        check(helper.said(expected), `the launcher reports the failed helper: ${expected}`);
        check(helper.said(`${expected}:`) && helper.said(reason), `the diagnostic carries the helper's own reason: ${reason}`);
        check(!helper.said("→ #"), "no session was reported ready");
        const started = profileProcs().filter(p => !known.includes(p));
        check(aliveNow(started).length === 0, `the failed launch leaves no process (${started})`);
        check((await rows()).filter(r => r.reading).map(r => r.name).join() === `#${runtime}/sibling@fixture`, "no inbox reader is left behind");
      }
      check(JSON.stringify(snapshot()) === JSON.stringify(before), "no run directory or lock is left");
      if (!live) check((await rows()).map(r => r.name).join() === rowsBefore, "nothing is registered for the failed launch");
      await siblingAlive("a helper failure");
    }

    phase = "runtime-exit";
    {
      // The runtime refuses an option and exits on its own; the launcher must
      // hand back that same status and clean up.
      const direct = Bun.spawnSync(runtime === "opencode" ? ["opencode", "attach", "http://127.0.0.1:9", "--no-such-option"] : [runtime, "--no-such-option"], { cwd: dirs["exit dir"]!, env: { ...env, ...runtimeEnv }, stdout: "ignore", stderr: "ignore", timeout: 20000 });
      say(`direct ${runtime} --no-such-option exit ${direct.exitCode}`);
      const before = snapshot();
      const known = profileProcs();
      const ex = launch("exit", dirs["exit dir"]!, ["--no-such-option"], { AGENT_BUS_NAME: `#${runtime}/exit@fixture` }); open.push(ex);
      let pids: number[] = [];
      const code = await Promise.race([ex.proc.exited, (async () => { for (;;) { pids = [...new Set([...pids, ...profileProcs().filter(p => !known.includes(p))])]; await Bun.sleep(50); if (ex.proc.exitCode !== null) return ex.proc.exitCode; } })(), Bun.sleep(60000).then(() => -1)]);
      open.pop(); writeFileSync(join(out, "tui-exit.txt"), screenOf(ex.output));
      check(code !== -1, "the launcher ends when its runtime exits");
      check(code !== 0 && code === direct.exitCode, `the launcher returns the runtime's own exit status (${code}, runtime alone ${direct.exitCode})`);
      await cleanedUp("the exited launch", before, pids);
      check(!(await record(`#${runtime}/exit@fixture`))?.reading, "the exited session's inbox has no reader");
      await siblingAlive("a runtime exit");
      await exchange(sibling, `#${runtime}/sibling@fixture`);
    }

    phase = "enforced-mode";
    {
      const contrary = runtime === "codex" ? ["-a", "on-request", "-s", "read-only"] : runtime === "claude" ? ["--permission-mode", "plan"] : [];
      const mode = launch("mode", dirs["mode dir"]!, contrary, { AGENT_BUS_NAME: `#${runtime}/mode@fixture` }); open.push(mode);
      await started(mode, `#${runtime}/mode@fixture`);
      if (live) {
        const a = tuiArgv(mode);
        check(a.includes("--enable-auto-mode") && !a.includes("plan") && !a.includes("--permission-mode"), `the runtime gets the enforced mode, not the contrary one (${a.filter(x => /mode/.test(x))})`);
        await Bun.sleep(2000);
        check(mode.said("auto mode on") && !mode.said("plan mode on"), "the session runs in auto mode despite --permission-mode plan");
      } else {
        // Outside the working directory: Codex's TUI starts its thread with workspace-write,
        // which allows the working directory, so only full access writes here without a prompt.
        const marker = join(out, `ENFORCED-${randomBytes(4).toString("hex")}`);
        const before = shells.length;
        await type(mode, `SHELL ${marker}`);
        await until(() => existsSync(marker), `a shell command writes outside the working directory with no prompt, despite ${runtime === "codex" ? "-a on-request -s read-only" : "a profile that asks first"}`, 30000, [mode]);
        check(true, `a shell command writes outside the working directory with no prompt, despite ${runtime === "codex" ? "-a on-request -s read-only" : "a profile that asks first"}`);
        await until(() => shells.length > before, "the shell result returns to the model", 15000, [mode]);
      }
      await stop(mode); open.pop();
    }
    const siblingPids = [sibling.proc.pid, ...descendants(sibling.proc.pid)];
    const code = await stop(sibling); open.pop();
    check(code === 143, `the separate session exits 143 when terminated (${code})`);
    await until(() => aliveNow(siblingPids).length === 0, "the separate session leaves no process", 10000);
  }

  if (phases.has("names")) {
    phase = "fallback";
    const fb = dirs["fallback dir"]!;
    const fallback = launch("fallback", fb, [], derivedOnly); open.push(fallback);
    await started(fallback);
    const fbName = derived(fb);
    // The launcher's own line: OpenCode titles a new session within seconds, which then relabels it.
    check(fallback.said(`${runtime}(${fb}) → ${fbName}`), `an unnamed session is labelled by runtime and directory: ${runtime}(${fb}) → ${fbName}`);
    check(!!(await record(fbName)), "the fallback address is registered");
    await keyboard(fallback, "fallback");
    await bound(fallback, fbName);
    check(fbName.startsWith(`#${runtime}/`) && fbName.endsWith("-fallback-dir@fixture"), `an unnamed session's address comes from its directory (${fbName})`);
    await stop(fallback); open.pop();

    phase = "assigned";
    await setTitle(fb, conversations(fb)[0]!, "Assigned Title");
    const assigned = launch("assigned", fb, [], derivedOnly); open.push(assigned);
    const asName = `#${runtime}/assigned-title@fixture`;
    await started(assigned, asName);
    check((await record(asName))!.descr === "Assigned Title", `the assigned name labels the session (${(await record(asName))!.descr})`);
    check(!(await record(fbName)), "the idle fallback address is released");
    await prime(assigned);
    await exchange(assigned, asName);

    phase = "rename-pending";
    // The session renames itself with its own ab_rename while the reply to the
    // message it holds is still pending: the reply must go out under the new name.
    const renamed = `#${runtime}/renamed-live@fixture`;
    await exchange(assigned, asName, { expectFrom: renamed, rename: "Renamed Live" });
    const said = live ? transcript(fb).split("\n").filter(l => l.includes('"tool_result"') && l.includes("registered as")).at(-1) ?? "" : renames.at(-1) ?? "";
    check(said.includes(`registered as ${renamed}`) && said.includes(`released ${asName}`), `the rename moves the address and releases the old idle one (${said.slice(0, 300)})`);
    await identity(assigned, renamed);
    check(!(await record(asName)), "the old address is gone");
    let refused = ""; try { await peer.send({ to: asName, body: "to the old address" }); } catch (e) { refused = String(e); }
    check(/no such receiver/.test(refused), `the old address takes no more messages (${refused})`);
    await exchange(assigned, renamed);
    await stop(assigned); open.pop();

    phase = "restart";
    // While it is down, a message waits at its address; then its title changes.
    const kept = await peer.send({ to: renamed, topic: "queued", tag: "kept", body: "QUEUED-FOR-OLD" });
    await setTitle(fb, conversations(fb)[0]!, "After Restart");
    const after = launch("after", fb, [], derivedOnly); open.push(after);
    const afterName = `#${runtime}/after-restart@fixture`;
    await started(after, afterName);
    check(after.said(`old address ${renamed} retained`), "the launcher reports the retained old address");
    const old = await record(renamed);
    check(!!old && old.queued === 1 && !old.reading, `the old address keeps its one queued message, unread (${JSON.stringify(old && { queued: old.queued, reading: old.reading })})`);
    const oldInbox = await owner.as(renamed);
    const got = await oldInbox.consume({ wait: "1s" });
    check(got?.message_id === kept.message_id && got.body === "QUEUED-FOR-OLD", "the queued message is still in the old inbox, not moved");
    await identity(after, afterName);
    await prime(after);
    await exchange(after, afterName);
    await stop(after); open.pop();

    phase = "explicit";
    const ex = dirs["explicit dir"]!, pinned = `#${runtime}/pinned@fixture`;
    const first = launch("pinned", ex, [], { AGENT_BUS_NAME: pinned }); open.push(first);
    await started(first, pinned);
    await keyboard(first, "pinned");
    await stop(first); open.pop();
    await setTitle(ex, conversations(ex)[0]!, "Pinned Title");
    const second = launch("pinned-again", ex, [], { AGENT_BUS_NAME: pinned }); open.push(second);
    await started(second, pinned);
    check((await rows()).filter(r => r.reading).map(r => r.name).join() === pinned, "an explicit address stays fixed when the title changes");
    check((await record(pinned))!.descr === "Pinned Title", `the title still labels it (${(await record(pinned))!.descr})`);
    const renameRefused = await fetch(runSettings(second).AGENT_BUS_CONTROL_ADDR + "/rename", { method: "POST", headers: { authorization: `Bearer ${runSettings(second).AGENT_BUS_CONTROL_TOKEN}` }, body: JSON.stringify({ name: "Moved" }) });
    check(renameRefused.status === 400 && (await renameRefused.text()).includes("AGENT_BUS_NAME pins this address"), "an explicit address refuses a live rename");
    await identity(second, pinned);
    await stop(second); open.pop();

    phase = "collision";
    const twins = ["twin a/same", "twin b/same"].map(d => dirs[d]!);
    for (const [i, d] of twins.entries()) {
      const t = launch(`twin-${i}`, d, [], derivedOnly); open.push(t);
      await started(t); await keyboard(t, `twin-${i}`); await bound(t, derived(d)); await stop(t); open.pop();
      await setTitle(d, conversations(d)[0]!, "Same Title");
    }
    const pair = twins.map((d, i) => launch(`same-${i}`, d, [], derivedOnly));
    open.push(...pair);
    const names = [`#${runtime}/same-title@fixture`, `#${runtime}/same-title.2@fixture`];
    for (const s of pair) await started(s);
    await until(async () => (await Promise.all(names.map(record))).every(r => r?.reading), "both same-titled sessions read their own address", 60000, pair);
    const labels = (await Promise.all(names.map(record))).map(r => r!.descr).sort();
    check(JSON.stringify(labels) === JSON.stringify(["Same Title", "Same Title #2"]), `colliding titles get distinct labels ${labels}`);
    // Which session holds which name is the daemon's arbitration; learn it from each session file.
    const owners = pair.map(s => runSettings(s).AGENT_BUS_NAME!);
    check(new Set(owners).size === 2 && owners.every(n => names.includes(n)), `each session holds its own address ${owners}`);
    for (const [i, s] of pair.entries()) { await prime(s); await exchange(s, owners[i]!); }
    for (const s of pair) await stop(s);
    open.length = 0;
  }
  writeFileSync(join(out, "result.json"), JSON.stringify({ runtime, version: Bun.spawnSync([runtime, "--version"]).stdout.toString().trim(), phases: [...phases], checks, failed }, null, 2));
} catch (e) {
  if (!String(e).startsWith(`Error: [${phase}]`)) failed++;
  console.log(`FAIL [${phase}] ${e}`);
} finally {
  for (const s of open) await stop(s).catch(() => {});
  daemon.kill(); await Promise.race([daemon.exited, Bun.sleep(8000)]); if (daemon.exitCode === null) daemon.kill(9);
  provider?.stop(true);
  console.log(`checks ${checks}, failed ${failed}`);
  process.exit(failed ? 1 : 0);
}
