// H.9.5 native runtime co-exercise: real interactive TUI, real MCP, real bus and pusher;
// only model decisions are deterministic and served on loopback.
// bun runtime-interactive.ts BUILT_PROGRAM_DIR NEW_EVIDENCE_DIR codex|opencode [OTHER_ACCOUNT]
import { mkdirSync, readFileSync, readdirSync, writeFileSync, openSync, closeSync } from "node:fs";
import { resolve, join } from "node:path";
import { randomBytes } from "node:crypto";
import { Bus } from "../mcp/bus.ts";
import { RuntimeModelFixture } from "./runtime-model-fixture.ts";

const bin = resolve(process.argv[2]!);
const out = resolve(process.argv[3]!); mkdirSync(out); // no overwriting evidence
const runtime = process.argv[4];
if (runtime !== "codex" && runtime !== "opencode") throw new Error("expected codex or opencode");
const other = process.argv[5] || "agent-bus-runner";
const uid = Bun.spawnSync(["id", "-u", other]);
if (uid.exitCode || Number(uid.stdout.toString()) === process.getuid!()) throw new Error("second account must be distinct");
const env = { PATH: process.env.PATH!, TERM: "xterm-256color", LANG: "C.UTF-8" };
const slots = [0, 1].map(slot => {
  const dir = join(out, `session-${slot}`), home = join(dir, "home"), cwd = join(dir, "work with spaces");
  mkdirSync(dir); mkdirSync(home); mkdirSync(cwd);
  return { slot, dir, home, cwd, name: `${runtime}/test-${slot}@fixture`, output: "", model: new RuntimeModelFixture(slot, dir, `SERVICE-${slot}`, runtime), proc: undefined as ReturnType<typeof Bun.spawn> | undefined, requests: [] as any[] };
});
const errors: string[] = [];
const provider = Bun.serve({ hostname: "127.0.0.1", port: 0, async fetch(req) {
  const slot = Number(new URL(req.url).pathname.split("/")[1]);
  try { return await slots[slot]!.model.respond(req); }
  catch (e) { errors.push(String(e)); return new Response(String(e), { status: 500 }); }
} });
let checks = 0;
function check(ok: boolean, label: string) { console.log(`${ok ? "ok" : "FAIL"} ${label}`); if (!ok) throw new Error(label); checks++; }
async function until(test: () => boolean | Promise<boolean>, label: string, ms = 20000) {
  const deadline = Date.now() + ms;
  while (Date.now() < deadline) {
    if (errors.length) throw new Error(errors.join("\n"));
    if (await test()) return;
    if (slots.some(s => s.proc && s.proc.exitCode !== null)) throw new Error("launcher exited: " + label);
    await Bun.sleep(50);
  }
  throw new Error("timed out: " + label);
}
const fd = openSync(join(out, "daemon.log"), "w");
const daemon = Bun.spawn([join(bin, "agent-busd"), "-addr", "127.0.0.1:0", "-socket", join(out, "bus.sock"), "-owner", "owner@fixture", "-token-file", join(out, "tokens"), "-dump-file", join(out, "dump.json"), "-dump-every", "0"], { env, stdout: fd, stderr: fd }); closeSync(fd);
async function secondAccount(kind: string, url: string, target = "") {
  const p = Bun.spawn(["sudo", "-n", "-u", other, "--", process.execPath, join(import.meta.dir, "runtime-isolation.ts"), "probe", kind, url, target], { env, cwd: out, stdout: "pipe", stderr: "pipe" });
  const [text, stderr, code] = await Promise.all([new Response(p.stdout).text(), new Response(p.stderr).text(), p.exited]);
  if (code) throw new Error(`second-account probe failed: ${stderr}`);
  return JSON.parse(text);
}
try {
  let token = "";
  await until(() => { try { token = readFileSync(join(out, "tokens"), "utf8").split(/\s+/)[1]!; return !!token; } catch { return false; } }, "disposable daemon ready");
  const ownerEnv = { ...env, AGENT_BUS_ADDR: join(out, "bus.sock"), AGENT_BUS_NAME: "owner@fixture", AGENT_BUS_TOKEN: token };
  const owner = new Bus(ownerEnv);
  await owner.register({ name: "echo@fixture", allow: slots.map(s => s.name) });
  await owner.register({ name: "forbidden@fixture", allow: ["nobody@fixture"], no_master: true });
  const service = await owner.as("echo@fixture");
  const peers = [];
  for (const s of slots) {
    await owner.register({ name: `peer-${s.slot}@fixture`, allow: [s.name] });
    peers.push(await owner.as(`peer-${s.slot}@fixture`));
    if (runtime === "codex") writeFileSync(join(s.home, "config.toml"), `model="fixture"\nmodel_provider="fixture"\n[model_providers.fixture]\nname="fixture"\nbase_url="http://127.0.0.1:${provider.port}/${s.slot}/v1"\nwire_api="responses"\nrequires_openai_auth=false\n[projects.${JSON.stringify(s.cwd)}]\ntrust_level="trusted"\n`);
    else {
      const config = join(s.home, "config/opencode"); mkdirSync(config, { recursive: true });
      writeFileSync(join(config, "opencode.json"), JSON.stringify({ model: "fixture/test", small_model: "fixture/test", enabled_providers: ["fixture"], plugin: [], provider: { fixture: { npm: "@ai-sdk/openai-compatible", name: "Fixture", options: { baseURL: `http://127.0.0.1:${provider.port}/${s.slot}/v1`, apiKey: "fixture-only" }, models: { test: { name: "Fixture", limit: { context: 100000, output: 10000 } } } } } }));
    }
    s.proc = Bun.spawn([join(bin, `ab-${runtime}`)], {
      cwd: s.cwd, env: { ...ownerEnv, HOME: s.home, CODEX_HOME: s.home, XDG_CONFIG_HOME: join(s.home, "config"), XDG_DATA_HOME: join(s.home, "data"), OPENCODE_DISABLE_MODELS_FETCH: "1", XDG_STATE_HOME: join(s.home, "state"), XDG_CACHE_HOME: join(s.home, "cache"), AGENT_BUS_NAME: s.name },
      terminal: { cols: 150, rows: 40, data(_t, b) { const text = Buffer.from(b).toString(); s.output += text; if (text.includes("\x1b[6n")) s.proc?.terminal?.write("\x1b[1;1R"); } },
    });
  }
  await until(async () => slots.every(s => s.output.includes(runtime === "codex" ? "Ask Codex" : "commands")) && (await owner.ls()).filter(r => slots.some(s => s.name === r.name) && r.reading).length === 2, "both native TUIs and pushers ready");
  for (const s of slots) s.proc!.terminal!.write(`KEYBOARD-${s.slot}`);
  await until(() => slots.every(s => s.output.includes(`KEYBOARD-${s.slot}`)), "keyboard text rendered");
  // Separate typing from Enter: native paste-burst handling otherwise treats
  // the same-write Enter as pasted text rather than an intentional submit.
  await Bun.sleep(300);
  for (const s of slots) s.proc!.terminal!.write("\r");
  await until(() => slots.every(s => s.output.includes(`KEYBOARD-OK-${s.slot}`)), "both native TUIs answer keyboard input");
  check(true, "two native runtime TUIs answer their own keyboard input");
  for (const s of slots) {
    await owner.register({ name: s.name, kind: "agent", allow: [`peer-${s.slot}@fixture`, "echo@fixture"] });
    await peers[s.slot]!.send({ to: s.name, topic: "interactive", tag: `slot-${s.slot}`, body: `KICKOFF-${s.slot}` });
  }
  await until(() => slots.every(s => s.model.held), "two addressed pusher turns held at provider");
  for (const s of slots) {
    check(s.proc!.exitCode === null, `session ${s.slot} TUI still live during attack`);
    const remote = s.output.match(runtime === "codex" ? /shared app-server (ws:\/\/127\.0\.0\.1:\d+)/ : /opencode: attached to (http:\/\/127\.0\.0\.1:\d+)/)?.[1];
    check(!!remote, `session ${s.slot} exposes its tested endpoint location`);
    const attack = await secondAccount(runtime, remote! + (runtime === "codex" ? "" : "/session"));
    check(runtime === "codex" ? attack.connected === false : attack.readStatus === 401 && attack.writeStatus === 401, `session ${s.slot} refuses second account mid-exchange`);
    const runs = join(s.home, "state/agent-bus/sessions/runs");
    const run = join(runs, readdirSync(runs)[0]!);
    const settings = JSON.parse(readFileSync(join(run, "bus-env.json"), "utf8"));
    const controlAttack = await secondAccount("control", settings.AGENT_BUS_CONTROL_ADDR + "/rename");
    check(controlAttack.readStatus === 401 && controlAttack.writeStatus === 401, `session ${s.slot} refuses control attack mid-exchange`);
    check(Bun.spawnSync(["sudo", "-n", "-u", other, "--", "test", "-r", join(run, "bus-env.json")]).exitCode !== 0, `session ${s.slot} credential file stays private`);
  }
  slots.forEach(s => s.model.release());
  await until(() => slots.every(s => s.model.serviceSent && s.output.includes(`REQUEST-SENT-${s.slot}`)), "real MCP listing, refusal and service sends");
  const replies = new Map<number, string>();
  for (let i = 0; i < 2; i++) {
    const request = await service.consume({ wait: "5s" });
    check(!!request, "service received an actual MCP request");
    const s = slots.find(s => s.name === request!.from);
    check(!!s && request!.body === `QUESTION-${s.slot}` && request!.topic === "interactive" && request!.tag === `slot-${s.slot}`, "service request preserves sender and correlation");
    check(!replies.has(s!.slot), "service got one request per session");
    const answer = `SERVICE-${s!.slot}-${randomBytes(12).toString("hex")}`; replies.set(s!.slot, answer);
    await service.send({ to: request!.from, topic: request!.topic, tag: request!.tag, body: answer });
  }
  await until(() => slots.every(s => s.model.replied && s.output.includes(`COMPLETE-${s.slot}: ${replies.get(s.slot)}`)), "correlated replies execute through MCP and appear in native TUIs");
  for (const s of slots) {
    const reply = await peers[s.slot]!.consume({ wait: "5s", topic: "interactive", tag: `slot-${s.slot}` });
    check(reply?.from === s.name && reply.body === replies.get(s.slot), `session ${s.slot} returns the unique service answer, not just acceptance`);
    const sibling = slots[1 - s.slot]!;
    check(!s.output.includes(replies.get(sibling.slot)!) && !s.model.requests.some(b => JSON.stringify(b.input ?? b.messages).includes(replies.get(sibling.slot)!)), `session ${s.slot} never receives its sibling's answer`);
    check(s.model.listed && s.model.denied, `session ${s.slot} real MCP listing and forbidden-send controls passed`);
  }
  writeFileSync(join(out, "result.json"), JSON.stringify({ checks, runtime: Bun.spawnSync([runtime, "--version"]).stdout.toString().trim(), slots: slots.map(s => ({ name: s.name, providerRequests: s.model.requests.length })) }, null, 2));
  console.log(`checks ${checks}, failed 0`);
} finally {
  slots.forEach(s => { s.model.release(); writeFileSync(join(s.dir, "tui.log"), s.output); s.proc?.kill(); });
  await Promise.all(slots.map(async s => { if (s.proc) { await Promise.race([s.proc.exited, Bun.sleep(3000)]); if (s.proc.exitCode === null) s.proc.kill(9); await s.proc.exited; s.proc.terminal?.close(); } }));
  daemon.kill(); await daemon.exited; provider.stop(true);
}
