// Native Claude channel exchange against a real claude.ai login.
// bun claude-channel-live.ts BUILT_PROGRAM_DIR NEW_EVIDENCE_DIR SIGNED_IN_CONFIG_DIR [OTHER_ACCOUNT]
//
// The fixture probe (claude-channel-probe.ts) cannot pass: channels need a
// signed-in account. This copies only that account's login into a disposable
// profile — never the person's projects, flags or transcripts — and runs the
// real model. It passes only when a peer's bus message reaches the session as
// a channel event and the model answers it with ab_reply, correlated and
// carrying a random value the peer alone knows.
import { mkdirSync, readdirSync, readFileSync, writeFileSync, openSync, closeSync, copyFileSync, chmodSync } from "node:fs";
import { resolve, join } from "node:path";
import { userInfo } from "node:os";
import { randomBytes } from "node:crypto";
import { Bus } from "../mcp/bus.ts";

const bin = resolve(process.argv[2]!);
const out = resolve(process.argv[3]!); mkdirSync(out);
const signedIn = resolve(process.argv[4]!);
const other = process.argv[5] ?? "nobody";
const home = join(out, "home"), config = join(out, "config"), cwd = join(out, "work with spaces");
for (const d of [home, config, cwd]) mkdirSync(d, { mode: 0o700 });
copyFileSync(join(signedIn, ".credentials.json"), join(config, ".credentials.json"));
chmodSync(join(config, ".credentials.json"), 0o600);
const account = JSON.parse(readFileSync(join(signedIn, ".claude.json"), "utf8"));
writeFileSync(join(config, ".claude.json"), JSON.stringify({
  hasCompletedOnboarding: true, theme: "dark", oauthAccount: account.oauthAccount, userID: account.userID,
  projects: { [cwd]: { hasTrustDialogAccepted: true } },
}), { mode: 0o600 });
const env = {
  PATH: process.env.PATH!, HOME: home, TERM: "xterm-256color", LANG: "C.UTF-8", CLAUDE_CONFIG_DIR: config,
  XDG_STATE_HOME: join(home, "state"), XDG_CACHE_HOME: join(home, "cache"),
};
const fd = openSync(join(out, "daemon.log"), "w");
const daemon = Bun.spawn([join(bin, "agent-busd"), "-addr", "127.0.0.1:0", "-socket", join(out, "bus.sock"), "-owner", "owner@fixture", "-db", join(out, "bus.db"), "-create", "-flush-every", "0"], { env, stdout: fd, stderr: fd }); closeSync(fd);
let proc: ReturnType<typeof Bun.spawn> | undefined, output = "";
const text = () => output.replace(/\x1b\[[0-9;?]*[a-zA-Z]/g, "").replace(/\x1b\][^\x07]*\x07/g, "");
let failed = 0;
const check = (ok: boolean, label: string) => { console.log(`${ok ? "ok" : "FAIL"} ${label}`); if (!ok) failed++; };
async function until(test: () => boolean | Promise<boolean>, label: string, ms = 30000) {
  const deadline = Date.now() + ms;
  while (Date.now() < deadline) {
    if (await test()) return;
    if (proc && proc.exitCode !== null) throw new Error(`launcher exited: ${label}`);
    await Bun.sleep(100);
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
  // The development-channel warning asks once; take its default "I am using
  // this for local development" answer.
  let confirmed = false;
  await until(async () => {
    if (!confirmed && text().replace(/\s/g, "").includes("Iamusingthisforlocaldevelopment")) { confirmed = true; await Bun.sleep(500); proc!.terminal!.write("\r"); }
    return (await owner.ls()).some(r => r.name === "#claude/test@fixture" && r.reading);
  }, "native MCP reader ready", 60000);
  check(true, "session registered and reading");
  // A second OS account can neither steer the session nor read its credential.
  const runs = join(home, "state/agent-bus/sessions/runs");
  const run = join(runs, readdirSync(runs)[0]!);
  const settings = JSON.parse(readFileSync(join(run, "bus-env.json"), "utf8"));
  const p = Bun.spawn(["sudo", "-n", "-u", other, "--", process.execPath, join(import.meta.dir, "runtime-isolation.ts"), "probe", "control", settings.AGENT_BUS_CONTROL_ADDR + "/rename", ""], { env, cwd: out, stdout: "pipe", stderr: "pipe" });
  const [probed, probeErr, probeCode] = await Promise.all([new Response(p.stdout).text(), new Response(p.stderr).text(), p.exited]);
  if (probeCode) throw new Error(`second-account probe failed: ${probeErr}`);
  const attack = JSON.parse(probed);
  check(attack.readStatus === 401 && attack.writeStatus === 401, `second account refused by the control endpoint (${attack.readStatus}/${attack.writeStatus})`);
  check(Bun.spawnSync(["sudo", "-n", "-u", other, "--", "test", "-r", join(run, "bus-env.json")]).exitCode !== 0, "credential file stays private from the second account");
  await Bun.sleep(3000);
  check(!text().replace(/\s/g, "").includes("Channelsarenotcurrentlyavailable"), "channels available to the signed-in account");
  await owner.register({ name: "#claude/test@fixture", kind: "agent", allow: ["#peer@fixture"] });
  const canary = "CANARY-" + randomBytes(6).toString("hex");
  const tag = "live-" + randomBytes(3).toString("hex");
  const sent = await peer.send({ to: "#claude/test@fixture", topic: "interactive", tag,
    body: `Acceptance test. Answer this message with ab_reply, body exactly: ${canary}. Do nothing else.` });
  let reply: any;
  await until(async () => {
    try { reply = await peer.consume({ wait: "5s" }); } catch { reply = undefined; }
    return !!reply;
  }, "correlated reply from the model", 180000);
  const r = reply;
  check(JSON.stringify(r).includes(canary), "reply carries the peer's random value");
  check(r?.from === "#claude/test@fixture", `reply comes from the session (${r?.from})`);
  check(r?.tag === tag, `reply is correlated by tag (${r?.tag})`);
  check(!!sent, "peer send accepted");
  // Nobody typed: the turn must be the channel event, answered with ab_reply.
  const transcripts = join(config, "projects");
  const read = () => readdirSync(transcripts).flatMap(d => readdirSync(join(transcripts, d)).filter(f => f.endsWith(".jsonl")).map(f => readFileSync(join(transcripts, d, f), "utf8"))).join("\n");
  const answered = /"type":"tool_use"[^}]*"name":"mcp__agent-bus__ab_reply"/;
  // The transcript is written after the tool returns; give it that moment.
  await until(() => answered.test(read()), "transcript records the reply", 20000).catch(() => {});
  const lines = read();
  check(lines.includes(`<channel source=\\"agent-bus\\"`) && lines.includes(`tag=\\"${tag}\\"`), "the message reached the model as an agent-bus channel event");
  check(answered.test(lines), "the model answered with ab_reply");
} catch (e) {
  check(false, String(e));
} finally {
  writeFileSync(join(out, "tui.log"), output);
  writeFileSync(join(out, "tui.txt"), text());
  proc?.kill();
  if (proc) { await Promise.race([proc.exited, Bun.sleep(3000)]); if (proc.exitCode === null) proc.kill(9); await proc.exited; proc.terminal?.close(); }
  daemon.kill(); await daemon.exited;
  console.log(`checks failed ${failed}`);
  process.exit(failed ? 1 : 0);
}
