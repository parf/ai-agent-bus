// Installed endpoint checks, not a substitute for H.9's interactive delivery gate.
// bun runtime-isolation.ts NEW_EVIDENCE_DIR [OTHER_OS_ACCOUNT]
// Requires installed Codex/OpenCode, sudo and a distinct existing OS account.
// Fresh runtime homes, no model calls, no access to any live session.
import { mkdirSync, readFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { randomUUID } from "node:crypto";
import { Codex } from "../mcp/codex.ts";
import { Opencode } from "../mcp/opencode.ts";
import { serveControl } from "../launchers/control.ts";
import { codexAuth } from "../launchers/runtime-auth.ts";

// Probes run in an actual second account. Only public addresses and disposable
// thread IDs appear in argv; credentials are never passed to that account.
if (process.argv[2] === "probe") {
  const [kind, url, target = ""] = process.argv.slice(3);
  if (kind === "codex") {
    const ws = new WebSocket(url!);
    let connected = false;
    const pending = new Map<number, { resolve: (v: any) => void; reject: (e: Error) => void }>();
    let sequence = 0;
    const request = (method: string, params: unknown) => new Promise<any>((resolve, reject) => {
      const id = ++sequence; pending.set(id, { resolve, reject });
      ws.send(JSON.stringify({ id, method, params }));
    });
    const timer = setTimeout(() => { console.error("probe timed out"); process.exit(2); }, 5000);
    try {
      await new Promise<void>((resolve, reject) => {
        ws.onopen = () => { connected = true; resolve(); };
        ws.onerror = () => reject(new Error("handshake refused"));
      });
      ws.onmessage = e => {
        const m = JSON.parse(String(e.data)); const p = pending.get(m.id);
        if (p) { pending.delete(m.id); m.error ? p.reject(new Error(m.error.message)) : p.resolve(m.result); }
      };
      await request("initialize", { clientInfo: { name: "cross-account-probe", version: "1" } });
      ws.send(JSON.stringify({ method: "initialized" }));
      const read = await request("thread/read", { threadId: target });
      await request("thread/name/set", { threadId: target, name: "cross-account-write" });
      console.log(JSON.stringify({ connected, read: read.thread.id === target, changed: true }));
    } catch (e) { console.log(JSON.stringify({ connected, error: String(e) })); }
    finally { clearTimeout(timer); ws.close(); }
  } else {
    const headers = { "content-type": "application/json" };
    const get = await fetch(url!, { signal: AbortSignal.timeout(5000) });
    const body = await get.text();
    const changed = await fetch(kind === "control" ? url! : url! + "/" + target, {
      method: kind === "control" ? "POST" : "PATCH", headers,
      body: JSON.stringify({ name: "cross-account-write", title: "cross-account-write" }),
      signal: AbortSignal.timeout(5000),
    });
    console.log(JSON.stringify({ readStatus: get.status, writeStatus: changed.status, read: body.includes(target) }));
  }
  process.exit(0);
}
const out = resolve(process.argv[2]!);
mkdirSync(out, { mode: 0o755 }); // never overwrite earlier evidence
const other = process.argv[3] || "agent-bus-runner";
const otherID = Bun.spawnSync(["id", "-u", other]);
if (otherID.exitCode || Number(otherID.stdout.toString().trim()) === process.getuid!()) throw new Error("a distinct existing OS account is required");
const privateDir = join(out, "private");
mkdirSync(privateDir, { mode: 0o700 });
const baseEnv: Record<string, string> = { PATH: process.env.PATH!, LANG: "C.UTF-8" };
let failures = 0, checks = 0;
function check(ok: boolean, label: string) { checks++; console.log(`${ok ? "ok" : "FAIL"} ${label}`); if (!ok) failures++; }
function port() {
  const s = Bun.listen({ hostname: "127.0.0.1", port: 0, socket: { data() {} } });
  const p = s.port; s.stop(true); return p;
}
async function probe(kind: string, url: string, target: string) {
  const p = Bun.spawn(["sudo", "-n", "-u", other, "--", process.execPath, import.meta.path, "probe", kind, url, target], {
    cwd: out, env: baseEnv, stdout: "pipe", stderr: "pipe",
  });
  const [text, err, status] = await Promise.all([new Response(p.stdout).text(), new Response(p.stderr).text(), p.exited]);
  if (status !== 0) throw new Error(`second-account probe failed (${status}): ${err}`);
  return JSON.parse(text.trim());
}
function cannotRead(path: string) {
  return Bun.spawnSync(["sudo", "-n", "-u", other, "--", "test", "-r", path], { env: baseEnv }).exitCode !== 0;
}
async function waitReady(fn: () => Promise<boolean>, proc: ReturnType<typeof Bun.spawn>) {
  for (let i = 0; i < 150; i++) {
    if (proc.exitCode !== null) throw new Error("runtime exited before ready; inspect evidence log");
    if (await fn()) return;
    await Bun.sleep(100);
  }
  throw new Error("runtime readiness timed out");
}
async function stop(proc: ReturnType<typeof Bun.spawn>) {
  proc.kill(); await Promise.race([proc.exited, Bun.sleep(2000)]);
  if (proc.exitCode === null) proc.kill(9);
  await proc.exited;
}
// Read and write on a deliberately unprotected disposable server prove the
// probes work. The same actions must be refused by the protected server.
for (const protectedMode of [false, true]) {
  const label = protectedMode ? "protected" : "unprotected-control";
  const home = join(privateDir, `codex-${label}`); mkdirSync(home);
  const cwd = join(out, `codex-work-${label}`); mkdirSync(cwd);
  const { token, file, args: authArgs } = codexAuth(home);
  const url = `ws://127.0.0.1:${port()}`;
  const args = ["codex", "app-server", "--listen", url];
  if (protectedMode) args.push(...authArgs);
  const proc = Bun.spawn(args, { cwd, env: { ...baseEnv, HOME: home, CODEX_HOME: home }, stdout: Bun.file(join(out, `codex-${label}.log`)), stderr: Bun.file(join(out, `${label}-${args[0]}.stderr`)) });
  const client = new Codex(cwd, () => {}, url, protectedMode ? token : "");
  try {
    await waitReady(async () => { try { await fetch(url.replace("ws:", "http:"), { signal: AbortSignal.timeout(200) }); return true; } catch { return false; } }, proc);
    await client.start();
    const thread = await client.openThread();
    await client.rename("owner-canary");
    check(await client.threadName() === "owner-canary", `Codex ${label}: intended client reads and changes its thread`);
    // The probe must use the same cwd filter as this disposable thread.
    const original = process.cwd(); process.chdir(cwd);
    let result: any;
    try {
      const p = Bun.spawn(["sudo", "-n", "-u", other, "--", process.execPath, import.meta.path, "probe", "codex", url, thread.id], { cwd, env: baseEnv, stdout: "pipe", stderr: "pipe" });
      const [text, err, status] = await Promise.all([new Response(p.stdout).text(), new Response(p.stderr).text(), p.exited]);
      if (status) throw new Error(err); result = JSON.parse(text);
    } finally { process.chdir(original); }
    check(protectedMode ? !result.connected : result.read && result.changed,
      `Codex ${label}: second-account read and rename ${protectedMode ? "refused" : "succeed when boundary removed"}`);
    check(await client.threadName() === (protectedMode ? "owner-canary" : "cross-account-write"), `Codex ${label}: owner observes expected thread state`);
    if (protectedMode) {
      check(cannotRead(file), "Codex capability file unreadable by second account");
      check(cannotRead(`/proc/${proc.pid}/environ`), "Codex runtime environment unreadable by second account");
      check(!readFileSync(`/proc/${proc.pid}/cmdline`, "utf8").includes(token), "Codex capability absent from command line");
      const wrong = new Codex(cwd, () => {}, url, "wrong-capability");
      let refused = false; try { await wrong.start(); } catch { refused = true; } finally { wrong.stop(); }
      check(refused, "Codex incorrect capability refused");
    }
  } finally { client.stop(); await stop(proc); }
}
for (const protectedMode of [false, true]) {
  const label = protectedMode ? "protected" : "unprotected-control";
  const home = join(privateDir, `opencode-${label}`); mkdirSync(home);
  const cwd = join(out, `opencode-work-${label}`); mkdirSync(cwd);
  const password = randomUUID(); const number = port(); const url = `http://127.0.0.1:${number}`;
  const env = { ...baseEnv, HOME: home, XDG_CONFIG_HOME: join(home, "config"), XDG_DATA_HOME: join(home, "data"), XDG_CACHE_HOME: join(home, "cache"), XDG_STATE_HOME: join(home, "state"), OPENCODE_DISABLE_EXTERNAL_SKILLS: "1", OPENCODE_CONFIG_CONTENT: JSON.stringify({ mcp: {}, plugin: [] }), ...(protectedMode ? { OPENCODE_SERVER_PASSWORD: password } : {}) };
  const proc = Bun.spawn(["opencode", "serve", "--hostname", "127.0.0.1", "--port", String(number)], { cwd, env, stdout: Bun.file(join(out, `opencode-${label}.log`)), stderr: Bun.file(join(out, `opencode-${label}.stderr`)) });
  const client = new Opencode(cwd, () => {}, url, protectedMode ? password : undefined);
  try {
    await waitReady(() => client.probe(), proc);
    await client.start();
    const session = await client.openSession(); client.bind(session.id);
    await client.rename("owner-canary");
    check((await client.sessions()).some(s => s.id === session.id && s.title === "owner-canary"), `OpenCode ${label}: intended client reads and changes its session`);
    const r = await probe("opencode", url + "/session", session.id);
    check(protectedMode ? r.readStatus === 401 && r.writeStatus === 401 : r.readStatus === 200 && r.read && r.writeStatus === 200,
      `OpenCode ${label}: second-account read and rename ${protectedMode ? "refused" : "succeed when boundary removed"}`);
    check((await client.sessions()).some(s => s.id === session.id && s.title === (protectedMode ? "owner-canary" : "cross-account-write")), `OpenCode ${label}: owner observes expected session state`);
    if (protectedMode) {
      check(cannotRead(`/proc/${proc.pid}/environ`), "OpenCode password environment unreadable by second account");
      check(!readFileSync(`/proc/${proc.pid}/cmdline`, "utf8").includes(password), "OpenCode password absent from command line");
    }
  } finally { client.stop(); await stop(proc); }
}
let renamed = "unchanged";
const control = serveControl(async title => renamed = title || "default");
try {
  const url = control.env.AGENT_BUS_CONTROL_ADDR + "/rename";
  const r = await probe("control", url, "unused");
  check(r.readStatus === 401 && r.writeStatus === 401 && renamed === "unchanged", "launcher control refuses another account without renaming");
  const positive = await fetch(url, { method: "POST", headers: { authorization: `Bearer ${control.env.AGENT_BUS_CONTROL_TOKEN}` }, body: JSON.stringify({ name: "owner-canary" }) });
  check(positive.status === 200 && renamed === "owner-canary", "intended launcher-control caller renames successfully");
} finally { control.stop(); }
console.log(`checks ${checks}, failed ${failures}`);
process.exit(failures ? 1 : 0);
