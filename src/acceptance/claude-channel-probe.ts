// Native Claude channel prerequisite probe. A channel-unavailable run FAILS;
// it is not interactive isolation acceptance. No real provider credentials.
// bun claude-channel-probe.ts BUILT_PROGRAM_DIR NEW_EVIDENCE_DIR
import { mkdirSync, readFileSync, writeFileSync, appendFileSync, openSync, closeSync } from "node:fs";
import { resolve, join } from "node:path";
import { userInfo } from "node:os";
import { Bus } from "../mcp/bus.ts";

const bin = resolve(process.argv[2]!);
const out = resolve(process.argv[3]!); mkdirSync(out);
const home = join(out, "home"), cwd = join(out, "work with spaces");
mkdirSync(home); mkdirSync(cwd);
let calls = 0;
const provider = Bun.serve({ hostname: "127.0.0.1", port: 0, async fetch(req) {
  const body = await req.text();
  appendFileSync(join(out, "provider.jsonl"), JSON.stringify({ url: req.url, body }) + "\n");
  if (new URL(req.url).pathname.endsWith("/count_tokens")) return Response.json({ input_tokens: 10 });
  if (!new URL(req.url).pathname.endsWith("/messages")) return new Response("fixture route unavailable", { status: 404 });
  calls++;
  const events = [
    { type: "message_start", message: { id: `msg-${calls}`, type: "message", role: "assistant", model: "claude-sonnet-4-6", content: [], stop_reason: null, stop_sequence: null, usage: { input_tokens: 1, output_tokens: 1 } } },
    { type: "content_block_start", index: 0, content_block: { type: "text", text: "" } },
    { type: "content_block_delta", index: 0, delta: { type: "text_delta", text: "LOCAL-TURN-DONE" } },
    { type: "content_block_stop", index: 0 },
    { type: "message_delta", delta: { stop_reason: "end_turn", stop_sequence: null }, usage: { output_tokens: 5 } },
    { type: "message_stop" },
  ];
  return new Response(events.map(e => `event: ${e.type}\ndata: ${JSON.stringify(e)}\n\n`).join(""), { headers: { "content-type": "text/event-stream" } });
} });
// Only skip first-run onboarding and approval of this literal dummy key.
// Do not seed feature flags, authentication state or channel availability.
writeFileSync(join(home, ".claude.json"), JSON.stringify({ hasCompletedOnboarding: true, theme: "dark", customApiKeyResponses: { approved: ["fixture-only"], rejected: [] }, projects: { [cwd]: { hasTrustDialogAccepted: true } } }));
const env = {
  PATH: process.env.PATH!, HOME: home, TERM: "xterm-256color", LANG: "C.UTF-8",
  XDG_STATE_HOME: join(home, "state"), XDG_CACHE_HOME: join(home, "cache"),
  ANTHROPIC_BASE_URL: `http://127.0.0.1:${provider.port}`, ANTHROPIC_API_KEY: "fixture-only",
  ANTHROPIC_MODEL: "claude-sonnet-4-6", CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC: "1",
};
const fd = openSync(join(out, "daemon.log"), "w");
const daemon = Bun.spawn([join(bin, "agent-busd"), "-addr", "127.0.0.1:0", "-socket", join(out, "bus.sock"), "-owner", "owner@fixture", "-db", join(out, "bus.db"), "-create", "-flush-every", "0"], { env, stdout: fd, stderr: fd }); closeSync(fd);
let proc: ReturnType<typeof Bun.spawn> | undefined, output = "";
const text = () => output.replace(/\x1b\[[0-9;?]*[a-zA-Z]/g, "");
async function until(test: () => boolean | Promise<boolean>, label: string) {
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline) {
    if (await test()) return;
    if (proc && proc.exitCode !== null) throw new Error(`launcher exited: ${label}`);
    await Bun.sleep(50);
  }
  throw new Error(`timed out: ${label}`);
}
try {
  let token = "";
  await until(() => { try { token = Bun.spawnSync([join(bin, "agent-bus-token"), "owner@fixture"], { env: { ...process.env, AGENT_BUS_ADDR: join(out, `user-${userInfo().username}.sock`) } }).stdout.toString().trim(); return !!token; } catch { return false; } }, "daemon ready");
  const ownerEnv = { ...env, AGENT_BUS_ADDR: join(out, "bus.sock"), AGENT_BUS_NAME: "owner@fixture", AGENT_BUS_TOKEN: token };
  const owner = new Bus(ownerEnv);
  await owner.register({ name: "#peer@fixture", kind: "agent", allow: ["#claude/test@fixture"] });
  const peer = await owner.as("#peer@fixture");
  proc = Bun.spawn([join(bin, "ab-claude")], { cwd, env: { ...ownerEnv, AGENT_BUS_NAME: "#claude/test@fixture" }, terminal: { cols: 150, rows: 40, data(_t, b) {
    const s = Buffer.from(b).toString(); output += s;
    if (s.includes("\x1b[6n")) proc?.terminal?.write("\x1b[1;1R");
  } } });
  await until(async () => (await owner.ls()).some(r => r.name === "#claude/test@fixture" && r.reading), "native MCP reader ready");
  proc.terminal!.write("KEYBOARD-PROBE"); await Bun.sleep(300); proc.terminal!.write("\r");
  await until(() => text().includes("LOCAL-TURN-DONE") && calls > 0, "native keyboard turn answered by local provider");
  console.log("ok native keyboard turn with local provider");
  await owner.register({ name: "#claude/test@fixture", kind: "agent", allow: ["#peer@fixture"] });
  const before = calls;
  await peer.send({ to: "#claude/test@fixture", topic: "interactive", tag: "probe", body: "INCOMING-INTERACTIVE-CANARY" });
  if (text().includes("Channels are not currently available")) throw new Error("Claude reports Channels unavailable; interactive channel acceptance remains open");
  await until(() => calls > before && readFileSync(join(out, "provider.jsonl"), "utf8").includes("INCOMING-INTERACTIVE-CANARY"), "bus event reaches native channel input");
  console.log("ok native channel reaches local provider; full co-exercise still required");
} finally {
  writeFileSync(join(out, "tui.log"), output);
  writeFileSync(join(out, "result.json"), JSON.stringify({ calls, channelUnavailable: text().includes("Channels are not currently available") }));
  proc?.kill();
  if (proc) { await Promise.race([proc.exited, Bun.sleep(3000)]); if (proc.exitCode === null) proc.kill(9); await proc.exited; proc.terminal?.close(); }
  daemon.kill(); await daemon.exited; provider.stop(true);
}
