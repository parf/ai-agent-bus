// Fresh-host installed-node exchange: an installed launcher, run by an actual
// mapped account with no address, token or name, finds the systemd node
// through that account's own socket, registers its session under the
// account's principal, and the runtime answers a peer's message itself.
// bun runtime-installed.ts INSTALLED_BIN NEW_EVIDENCE_DIR codex|opencode|claude [SIGNED_IN_CLAUDE_CONFIG_DIR]
//
// Codex and OpenCode take model decisions from a deterministic loopback
// fixture that never calls MCP or the bus; Claude uses the real model under a
// copied claude.ai login, because only such a profile has channels.
import { mkdirSync, readFileSync, readdirSync, writeFileSync, copyFileSync, chmodSync, existsSync, appendFileSync } from "node:fs";
import { resolve, join } from "node:path";
import { randomBytes } from "node:crypto";
import { Bus } from "../mcp/bus.ts";

const bin = resolve(process.argv[2]!);
const out = resolve(process.argv[3]!); mkdirSync(out); // never overwrite evidence
const runtime = process.argv[4] as "codex" | "opencode" | "claude";
if (!["codex", "opencode", "claude"].includes(runtime)) throw new Error("expected codex, opencode or claude");
const live = runtime === "claude";
if (live && !process.argv[5]) throw new Error("claude needs a signed-in configuration directory to copy the login from");
const home = join(out, "home"), cwd = join(out, "work with spaces"), config = join(out, "config");
for (const d of [home, cwd]) mkdirSync(d, { mode: 0o700 });
// The installed node's realm; the container is started with this host name.
const realm = process.env.GATE_REALM ?? "fresh";
const account = Bun.spawnSync(["id", "-nu"]).stdout.toString().trim();
const socket = `/run/agent-bus/user-${account}.sock`;
const say = (s: string) => { const line = `${new Date().toISOString()} ${s}`; console.log(line); appendFileSync(join(out, "phases.log"), line + "\n"); };

// ---- model fixture (codex, opencode) ---------------------------------------
const errors: string[] = [];
const pongs: { sent: boolean; failure?: string }[] = [];
let sequence = 0;
function userTexts(body: any): string[] {
  if (runtime === "codex") return (body.input ?? []).filter((x: any) => x.role === "user").map((x: any) => JSON.stringify(x.content));
  return (body.messages ?? []).filter((m: any) => m.role === "user").map((m: any) => JSON.stringify(m.content));
}
function toolOutput(body: any, id: string): string | undefined {
  if (runtime === "codex") { const o = (body.input ?? []).findLast((x: any) => x.type === "function_call_output" && x.call_id === id); return o && JSON.stringify(o.output); }
  const o = (body.messages ?? []).findLast((m: any) => m.role === "tool" && m.tool_call_id === id); return o && JSON.stringify(o.content);
}
type Item = { text?: string; call?: { id: string; name: string; args: unknown } };
function decide(body: any): Item {
  if (runtime === "opencode" && JSON.stringify(body.messages?.[0]?.content ?? "").includes("You are a title generator")) return { text: "Installed session" };
  const last = userTexts(body).at(-1) ?? "";
  if (last.includes("KEYBOARD")) return { text: "KEYBOARD-OK" };
  const ping = last.match(/PING-([a-f0-9]+)/);
  if (!ping) throw new Error("unexpected model input: " + last.slice(0, 300));
  const id = `pong-${ping[1]}`, output = toolOutput(body, id);
  if (output === undefined) {
    // The route the pushed message spells out, not one this fixture knows.
    const route = JSON.parse(last.match(/Reply using ab_send with (\{.*?\}) and text/)?.[1]?.replace(/\\"/g, '"') ?? "null");
    if (!route?.to) throw new Error("pushed message spelled out no reply route: " + last.slice(0, 300));
    return { call: { id, name: "ab_send", args: { ...route, text: `PONG-${ping[1]}` } } };
  }
  pongs.push(output.includes("the bus accepted") ? { sent: true } : { sent: false, failure: output });
  return { text: output.includes("the bus accepted") ? "PONG-SENT" : `PONG-FAILED: ${output.slice(0, 200)}` };
}
function stream(item: Item, model: string): Response {
  const seq = ++sequence;
  if (runtime === "opencode") {
    const id = `chatcmpl-${seq}`;
    const chunk = (delta: unknown, finish_reason: string | null = null) => ({ id, object: "chat.completion.chunk", created: Math.floor(Date.now() / 1000), model, choices: [{ index: 0, delta, finish_reason }] });
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
const provider = live ? undefined : Bun.serve({ hostname: "127.0.0.1", port: 0, idleTimeout: 120, async fetch(req) {
  try { const body = await req.json() as any; appendFileSync(join(out, "provider.jsonl"), JSON.stringify(body) + "\n"); return stream(decide(body), body.model); }
  catch (e) { errors.push(String(e)); return new Response(String(e), { status: 500 }); }
} });

// ---- checks and bounded waits ---------------------------------------------
let checks = 0, failed = 0;
class CheckFailed extends Error {}
function check(ok: boolean, label: string) {
  console.log(`${ok ? "ok" : "FAIL"} ${label}`);
  if (ok) checks++; else { failed++; throw new CheckFailed(label); }
}
let proc: ReturnType<typeof Bun.spawn> | undefined, output = "";
const screenOf = (o: string) => o.replace(/\x1b\[[0-9;?<>=]*[a-zA-Z~]/g, "").replace(/\x1b\][^\x07\x1b]*(\x07|\x1b\\)/g, "").replace(/\x1b[()][0-9A-Za-z]/g, "");
const said = (t: string) => screenOf(output).replace(/\s+/g, "").includes(t.replace(/\s+/g, ""));
async function until(test: () => boolean | Promise<boolean>, label: string, ms = 30000) {
  const deadline = Date.now() + ms;
  while (Date.now() < deadline) {
    if (errors.length) throw new Error(errors.join("\n"));
    if (await test()) return;
    if (proc && proc.exitCode !== null) throw new Error(`launcher exited (${proc.exitCode}): ${label}`);
    await Bun.sleep(100);
  }
  throw new Error(`timed out: ${label}`);
}
const profileProcs = () => readdirSync("/proc").filter(p => /^\d+$/.test(p)).map(Number).filter(p => {
  try { return readFileSync(`/proc/${p}/environ`, "utf8").split("\0").includes(`HOME=${home}`); } catch { return false; }
});

// ---- a clean runtime profile; no bus address, token or name -----------------
const env: Record<string, string> = { PATH: process.env.PATH!, TERM: "xterm-256color", LANG: "C.UTF-8", HOME: home,
  XDG_CONFIG_HOME: join(home, "config"), XDG_DATA_HOME: join(home, "data"), XDG_STATE_HOME: join(home, "state"), XDG_CACHE_HOME: join(home, "cache") };
if (live) {
  mkdirSync(config, { mode: 0o700 });
  copyFileSync(join(process.argv[5]!, ".credentials.json"), join(config, ".credentials.json"));
  chmodSync(join(config, ".credentials.json"), 0o600);
  const account = JSON.parse(readFileSync(join(process.argv[5]!, ".claude.json"), "utf8"));
  writeFileSync(join(config, ".claude.json"), JSON.stringify({ hasCompletedOnboarding: true, theme: "dark", oauthAccount: account.oauthAccount, userID: account.userID, projects: { [cwd]: { hasTrustDialogAccepted: true } } }), { mode: 0o600 });
  env.CLAUDE_CONFIG_DIR = config;
} else if (runtime === "codex") {
  env.CODEX_HOME = home;
  writeFileSync(join(home, "config.toml"), `model="fixture"\nmodel_provider="fixture"\n[model_providers.fixture]\nname="fixture"\nbase_url="http://127.0.0.1:${provider!.port}/v1"\nwire_api="responses"\nrequires_openai_auth=false\n[projects.${JSON.stringify(cwd)}]\ntrust_level="trusted"\n`);
} else {
  env.OPENCODE_DISABLE_MODELS_FETCH = "1";
  const dir = join(home, "config/opencode"); mkdirSync(dir, { recursive: true });
  writeFileSync(join(dir, "opencode.json"), JSON.stringify({ model: "fixture/test", small_model: "fixture/test", enabled_providers: ["fixture"], plugin: [], provider: { fixture: { npm: "@ai-sdk/openai-compatible", name: "Fixture", options: { baseURL: `http://127.0.0.1:${provider!.port}/v1`, apiKey: "fixture-only" }, models: { test: { name: "Fixture", limit: { context: 100000, output: 10000 } } } } } }));
}
check(!Object.keys(env).some(k => k.startsWith("AGENT_BUS_")) && !process.env.XDG_RUNTIME_DIR, "the session environment carries no bus address, token, name or login runtime directory");

try {
  // The account's own view of the installed node, through the socket the launcher must discover.
  const me = new Bus({ AGENT_BUS_ADDR: socket }, true);
  const you = (await me.status()).you;
  check(you === `${account}@${realm}`, `the account's own socket authenticates it as its mapped principal (${you})`);
  const before = new Set((await me.ls()).map(r => r.name));

  proc = Bun.spawn([join(bin, `ab-${runtime}`)], { cwd, env, terminal: { cols: 160, rows: 45, data(_t, b) {
    const t = Buffer.from(b).toString(); output += t; appendFileSync(join(out, "tui.log"), t);
    if (t.includes("\x1b[6n")) proc?.terminal?.write("\x1b[1;1R");
  } } });
  let confirmed = !live;
  await until(async () => {
    if (!confirmed && said("I am using this for local development")) { confirmed = true; await Bun.sleep(500); proc!.terminal!.write("\r"); await Bun.sleep(1000); }
    return confirmed && (runtime === "codex" ? said("Ask Codex") : runtime === "opencode" ? output.includes("commands") : said("for shortcuts") || said("mode on"));
  }, "native TUI ready", 90000);
  check(true, "the installed launcher starts the native TUI");
  const session = async () => (await me.ls()).find(r => !before.has(r.name) && r.name.startsWith(`#${runtime}/`));
  if (runtime === "codex") { // a fresh Codex TUI creates its thread, and binds its pusher, with its first turn
    await Bun.sleep(3000);
    if (!(await session())?.reading) {
      proc.terminal!.write("KEYBOARD warmup"); await until(() => said("KEYBOARD warmup"), "typed text rendered", 15000);
      await Bun.sleep(300); proc.terminal!.write("\r");
      await until(() => said("KEYBOARD-OK"), "the typed turn is answered", 30000);
    }
  }
  await until(async () => !!(await session())?.reading, "the session's inbox has its reader on the installed node", 60000);
  const record = (await session())!;
  say(`session record ${JSON.stringify(record)}`);
  check(record.owner === you, `the session is registered under the account's own principal (${record.name}, owner ${record.owner})`);
  check(record.name.endsWith(`@${realm}`), `the session's derived name takes this host's realm (${record.name})`);

  const peerName = `#peer-${runtime}@${realm}`;
  await me.register({ name: peerName, kind: "agent", allow: [record.name] });
  // An account socket always speaks as its account; a peer uses its own token on the shared socket, as the launcher does.
  const peer = new Bus({ AGENT_BUS_ADDR: "/run/agent-bus/bus.sock", AGENT_BUS_NAME: peerName, AGENT_BUS_TOKEN: await me.token(peerName) });
  check((await peer.status()).you === peerName, `the peer speaks as itself on the shared socket`);
  const value = randomBytes(8).toString("hex"), tag = "installed-" + randomBytes(3).toString("hex");
  await peer.send({ to: record.name, topic: "installed", tag, body: live
    ? `Acceptance test. Answer this message with ab_reply, body exactly: PONG-${value}. Do nothing else.`
    : `PING-${value}` });
  let reply: any;
  await until(async () => { try { reply = await peer.consume({ wait: "5s" }); } catch { reply = undefined; } return !!reply; }, "a correlated reply from the session", live ? 180000 : 60000);
  say(`reply ${JSON.stringify(reply)}`);
  check(reply.from === record.name && reply.to === peerName && reply.topic === "installed" && reply.tag === tag, `the reply comes from the session and is correlated (${reply.from} ${reply.topic}/${reply.tag})`);
  check(reply.body.includes(`PONG-${value}`), "the reply carries the peer's random value");
  if (!live) await until(() => pongs.length > 0, "the model sees its own ab_send result", 20000);
  if (!live) check(pongs.length === 1 && pongs[0]!.sent, `the runtime's own MCP ab_send carried the reply, once (${JSON.stringify(pongs)})`);
  else {
    const projects = join(config, "projects");
    const read = () => !existsSync(projects) ? "" : readdirSync(projects).flatMap(d => readdirSync(join(projects, d)).filter(f => f.endsWith(".jsonl")).map(f => readFileSync(join(projects, d, f), "utf8"))).join("\n");
    await until(() => /"name":"mcp__agent-bus__ab_reply"/.test(read()), "transcript records the reply", 20000).catch(() => {});
    check(read().includes(`<channel source=\\"agent-bus\\"`) && read().includes(`tag=\\"${tag}\\"`), "the message reached the model as an agent-bus channel event");
    check(/"name":"mcp__agent-bus__ab_reply"/.test(read()), "the model answered with ab_reply");
  }
  check(await peer.consume({ wait: "2s" }) === null, "nothing else reached the peer");

  proc.kill("SIGTERM");
  await Promise.race([proc.exited, Bun.sleep(10000)]);
  const code = proc.exitCode; if (code === null) proc.kill(9);
  await proc.exited; proc.terminal?.close(); proc = undefined;
  check(code !== null, `the launcher exits on SIGTERM (${code})`);
  await until(() => profileProcs().length === 0, "every process the session started has exited", 10000);
  check(true, "every process the session started has exited");
  await until(async () => !(await me.ls()).find(r => r.name === record.name)?.reading, "the session's reader is gone", 15000);
  check(true, "the session's reader is gone from the installed node");
  writeFileSync(join(out, "result.json"), JSON.stringify({ checks, runtime: Bun.spawnSync([runtime, "--version"]).stdout.toString().trim(), session: record.name }, null, 2));
} catch (e) {
  if (!(e instanceof CheckFailed)) { failed++; console.log(`FAIL ${e}`); }
} finally {
  if (proc) { proc.kill(); await Promise.race([proc.exited, Bun.sleep(3000)]); if (proc.exitCode === null) proc.kill(9); await proc.exited; proc.terminal?.close(); }
  writeFileSync(join(out, "tui.txt"), screenOf(output));
  provider?.stop(true);
  console.log(`checks ${checks}, failed ${failed}`);
  process.exit(failed ? 1 : 0);
}
