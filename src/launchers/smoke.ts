import { restartChecks } from "./restart-smoke.ts";
import { existsSync, mkdirSync, mkdtempSync, writeFileSync, readFileSync, readdirSync, rmSync, symlinkSync } from "node:fs";
import { basename, dirname, join, resolve } from "node:path";
import { Bus, defaultName } from "../mcp/bus.ts";
import { hash } from "./sessions.ts";
import { version } from "../mcp/version.ts";

const root = resolve(import.meta.dir, "../../tmp/launcher-smoke");
mkdirSync(root, { recursive: true });
const dir = mkdtempSync(join(root, "run-"));
const owner = new Bus();
const peerName = `#launcher-peer-${process.pid}@srv1`;
await owner.register({ name: peerName, kind: "agent", allow: ["*"] });
const deniedName = `#launcher-denied-${process.pid}@srv1`;
await owner.register({ name: deniedName, kind: "agent", allow: ["nobody@srv1"] });
const peer = await owner.as(peerName);
const stop = new AbortController();
const serve = (async () => {
  while (!stop.signal.aborted) {
    const e = await peer.consume({ wait: "1s" }, stop.signal);
    if (e) await peer.send({ to: e.from, body: "launcher-answer", topic: e.topic, tag: e.tag });
  }
})().catch(e => { if (!stop.signal.aborted) throw e; });
let failures = 0;
function check(label: string, pass: boolean) { console.log(`${pass ? "ok" : "FAIL"} ${label}`); if (!pass) failures++; }
// A terminal's last bytes arrive after the process has exited, later under a
// loaded parallel run: wait for the title restore, bounded, rather than a
// fixed sleep that races it. Absent, the check still fails at the deadline.
const RESTORE = "\x1b]0;\x07\x1b[23;0t";
const settled = async (output: () => string) => { for (let i = 0; i < 150 && !output().includes(RESTORE); i++) await Bun.sleep(20); };
// Saved first, restored, and no title set or saved after the restore. A log
// line after it is allowed: the face, the runtime's child, can say it stopped
// a moment after the launcher restored the title, on the same terminal.
const restoredTitle = (out: string) => {
  const at = out.lastIndexOf(RESTORE);
  const after = at < 0 ? "" : out.slice(at + RESTORE.length);
  return out.startsWith("\x1b[22;0t") && at >= 0 && !after.includes("\x1b]0;") && !after.includes("\x1b[22;");
};
const rowsIn = (file: string): any[] => existsSync(file) ? readFileSync(file, "utf8").split("\n").slice(0, -1).filter(Boolean).map(s => JSON.parse(s)) : [];
const processes: ReturnType<typeof Bun.spawn>[] = [];
async function collisions(launcher: string, cwd: string, env: NodeJS.ProcessEnv) {
  const title = `Collision-${process.pid}`;
  const attach = join(dir, "collision-attach"), release = join(dir, "collision-release");
  const concurrent: { proc: ReturnType<typeof Bun.spawn>; events: string; isolatedState: string; err: Promise<string> }[] = [];
  const until = async (ready: () => boolean) => {
    const end = performance.now() + 10000;
    while (!ready() && performance.now() < end) await Bun.sleep(10);
    return ready();
  };
  // Each later process starts after its predecessor has registered, while
  // the fixture gates MCP attachment. A process-start timestamp cannot tell
  // this live startup from a dormant record.
  for (const n of [1, 2, 3]) {
    const events = join(dir, `collision-${n}`), isolatedState = join(dir, `collision-state-${n}`);
    // A fresh runtime session has no older transcript title for metadata
    // refresh to restore while the fixture holds its MCP startup.
    const isolatedHome = join(dir, `collision-home-${n}`);
    mkdirSync(isolatedHome);
    const proc = Bun.spawn([launcher, "--name", title], { cwd,
      env: { ...env, CLAUDE_CONFIG_DIR: isolatedHome, XDG_STATE_HOME: isolatedState, TEST_EVENTS: events, TEST_ATTACH_GATE: attach, TEST_RELEASE_GATE: release, TEST_TAG: String(n) }, stdout: "ignore", stderr: "pipe" });
    processes.push(proc);
    concurrent.push({ proc, events, isolatedState, err: new Response(proc.stderr).text() });
    check(`collision session ${n} registers before the next process starts`, await until(() => rowsIn(events).some(r => r.kind === "identity")));
  }
  writeFileSync(attach, "ready");
  const answered = await until(() => concurrent.every(c => rowsIn(c.events).some(r => r.kind === "delivered" && r.data.includes("launcher-answer"))));
  // Keep all claimants alive until the assertion observed every answer; a
  // fixed hold can end before a slow sibling even starts under load.
  writeFileSync(release, "ready");
  const codes = await Promise.all(concurrent.map(async c => { const code = await c.proc.exited; const err = await c.err; if (code !== 17) console.log(`collision exit ${code}: ${err}`); return code; }));
  check("concurrent sessions all receive their answers", answered && codes.every(c => c === 17));
  const identities = concurrent.map(c => rowsIn(c.events).find(r => r.kind === "identity")?.data);
  check("duplicate sessions retain their runtime cwd descriptions", identities.every(i => i?.label === `claude ${cwd}`));
  check("duplicate names have separate bus inboxes", new Set(identities.map(i => i?.name)).size === 3);
  check("concurrent cleanup releases every lock", concurrent.every(c => !readdirSync(join(c.isolatedState, "agent-bus/sessions")).some(f => f.endsWith(".lock"))));
}
async function explicitReservation(launcher: string, cwd: string, env: NodeJS.ProcessEnv) {
  const name = `#claude/pinned-${process.pid}@srv1`;
  await owner.register({ name, kind: "agent", descr: "Dormant pin", allow: ["@owner"] });
  const run = async (suffix: string) => {
    const home = join(dir, `pin-home-${suffix}`), events = join(dir, `pin-${suffix}.events`);
    mkdirSync(home);
    const proc = Bun.spawn([launcher], { cwd, env: { ...env, AGENT_BUS_NAME: name,
      CLAUDE_CONFIG_DIR: home, XDG_STATE_HOME: join(dir, `pin-state-${suffix}`), TEST_EVENTS: events }, stdout: "ignore", stderr: "pipe" });
    processes.push(proc);
    const diagnostic = new Response(proc.stderr).text();
    return { code: await proc.exited, diagnostic: await diagnostic, rows: rowsIn(events) };
  };
  const dormant = await run("dormant");
  check("a fresh session with an explicit dormant name keeps exactly that name", dormant.code === 17 && dormant.rows.some(r => r.kind === "identity" && r.data.name === name));
  check("resuming without --descr preserves the dormant purpose", (await owner.ls()).find(r => r.name === name)?.descr === "Dormant pin");
  await owner.post("/manage", { name, descr: "Web-edited purpose" });
  const resumed = await run("web-purpose");
  check("resume keeps a web-edited purpose and the same identity", resumed.code === 17 && resumed.rows.some(r => r.kind === "identity" && r.data.name === name) && (await owner.ls()).find(r => r.name === name)?.descr === "Web-edited purpose");
  const original = (await owner.ls()).find(r => r.name === name);
  const bus = await owner.as(name), cancel = new AbortController();
  const reader = (async () => { while (!cancel.signal.aborted) await bus.consume({ wait: "5s" }, cancel.signal); })().catch(e => { if (!cancel.signal.aborted) throw e; });
  try {
    for (let i = 0; i < 100 && !(await owner.ls()).some(r => r.name === name && r.readers); i++) await Bun.sleep(20);
    check("explicit-name fixture has a live reader", (await owner.ls()).some(r => r.name === name && !!r.readers));
    const live = await run("live");
    check("an explicit live name is refused by name without starting the runtime", live.code !== 0 && live.diagnostic.includes(`${name} is already live`) && !live.rows.some(r => r.kind === "identity"));
    const after = (await owner.ls()).find(r => r.name === name);
    check("refused explicit claim preserves the live record", after?.descr === original?.descr && JSON.stringify(after?.allow) === JSON.stringify(original?.allow));
  } finally { cancel.abort(); await reader; }
}
try {
  for (const runtime of (["claude", "codex", "opencode", "opencode1"] as const).filter(r => !process.env.TEST_RUNTIME_ONLY || r === process.env.TEST_RUNTIME_ONLY)) {
    const cwd = join(dir, `${runtime} path with spaces`);
    const home = join(dir, runtime + "-home");
    mkdirSync(cwd); mkdirSync(home);
    const id = "11111111-1111-4111-8111-111111111111";
    const proj = join(home, "projects", cwd.replace(/[^a-zA-Z0-9]/g, "-"));
    mkdirSync(proj, { recursive: true });
    writeFileSync(join(proj, id + ".jsonl"), JSON.stringify({ type: "custom-title", customTitle: "Named session" }) + "\n");
    const executable = join(dir, runtime);
    // Fixture imports use the existing MCP dependency tree.
    const fixture = resolve(import.meta.dir, "../mcp/launcher-runtime-fixture.ts");
    writeFileSync(executable, `#!/bin/sh\nulimit -c 0\nexec '${process.execPath}' '${fixture}' "$@"\n`, { mode: 0o700 });
    const events = join(dir, runtime + ".events");
    const env = { ...process.env, AGENT_BUS_NAME: "", TEST_RUNTIME: runtime,
      CLAUDE_BIN: executable, CODEX_BIN: executable, OPENCODE_BIN: executable, CLAUDE_CONFIG_DIR: home,
      XDG_STATE_HOME: join(dir, "state"), TEST_EVENTS: events, TEST_PEER: peerName,
      TEST_TITLE: "Named session", TEST_EXIT: "17", TEST_DENIED: deniedName,
      AGENT_BUS_OWNER_TOKEN: "must-not-reach-the-runtime",
      TEST_OWNER_TOKEN_SHA: process.env.AGENT_BUS_TOKEN ? new Bun.CryptoHasher("sha256").update(process.env.AGENT_BUS_TOKEN).digest("hex") : "" };
    const launcher = resolve(process.env.LAUNCHER_BUILD || import.meta.dir, `ab-${runtime}`);
    if (runtime !== "opencode1") await restartChecks(launcher, cwd, env, check);
    if (process.env.TEST_RESTART_ONLY) continue;
    if (process.env.TEST_COLLISIONS_ONLY) {
      if (runtime === "claude") { await collisions(launcher, cwd, env); await explicitReservation(launcher, cwd, env); }
      break;
    }
    const state = join(env.XDG_STATE_HOME, "agent-bus/sessions");
    mkdirSync(state, { recursive: true });
    const legacy = defaultName({ ...env, AGENT_BUS_RUNTIME: runtime, AGENT_BUS_CWD: "Named session" });
    await owner.register({ name: legacy, kind: "agent", descr: "Named session" });
    const legacyID = runtime === "claude" ? id : `${runtime === "codex" ? "codex" : "opencode"}-session`;
    writeFileSync(join(state, `${hash(`${runtime}:${legacyID}`)}.json`), JSON.stringify({ name: legacy, ...(runtime === "claude" ? {} : { base: legacy }) }), { mode: 0o600 });
    const overrides = runtime === "claude" ? ["--permission-mode", "plan"]
      : runtime === "codex" ? ["-C", "codex path with spaces", "-a", "on-request", "-s", "read-only"]
      : ["--dir", `${runtime} path with spaces`];
    let terminalOutput = "";
    const proc = Bun.spawn([launcher, ...overrides, "a prompt with spaces"], { cwd: runtime === "claude" ? cwd : dir,
      env: { ...env, TERM: "xterm-256color", KITTY_WINDOW_ID: "", KONSOLE_DBUS_SERVICE: "", TMUX: "" },
      terminal: { data(_terminal, data) { terminalOutput += Buffer.from(data).toString(); } },
    });
    processes.push(proc);
    const code = await proc.exited;
    await settled(() => terminalOutput);
    proc.terminal?.close();
    const diagnostic = terminalOutput;
    const titleRuntime = { claude: "Claude", codex: "Codex", opencode: "OpenCode", opencode1: "OpenCode1" }[runtime];
    check(`${runtime} sets its named terminal title and restores it after TUI exit`, restoredTitle(terminalOutput) && terminalOutput.includes(`\x1b]0;${titleRuntime}(Named session`));
    if (code !== 17) console.log(diagnostic);
    check(`${runtime} propagates the runtime exit status`, code === 17);
    const rows = readFileSync(events, "utf8").trim().split("\n").map(s => JSON.parse(s));
    const allArgs = rows.filter(r => r.kind === "argv").map(r => r.data);
    if(runtime === "claude") check("Claude bootstraps the local plugin before starting its TUI", existsSync(join(home,"plugins/installed_plugins.json")));
    check(`${runtime} prevents the runtime from replacing its terminal title`, runtime === "codex" ? allArgs.some(a => a.includes("tui.terminal_title=[]")) : rows.some(r => r.kind === "terminal-env" && r.data === "1"));
    check(`${runtime} preserves the prompt argument`, allArgs.some(a => a.includes("a prompt with spaces")));
    if (runtime === "codex") check("Codex resolves a relative directory exactly once", allArgs.some(a => a.filter((v: string) => v === "-C").length === 1 && a[a.indexOf("-C") + 1] === cwd));
    if (runtime === "codex") {
      check("Codex rejects an unauthenticated runtime client", rows.some(r => r.kind === "runtime-denied" && r.data === 401));
      check("Codex TUI authenticates to its private server", rows.some(r => r.kind === "tui-authenticated"));
    }
    if (runtime === "opencode1") check("opencode1 resolves a relative directory exactly once", allArgs.some(a => a.filter((v: string) => v === "--dir").length === 1 && a[a.indexOf("--dir") + 1] === cwd));
    if (runtime === "opencode") check("opencode resolves its directory once and hands the TUI a server, not a flag", allArgs.some(a => a.includes("--server")) && allArgs.every(a => !a.includes("--dir"))
      && rows.some(r => r.kind === "http" && r.data.method === "GET" && r.data.path === "/api/session" && r.data.query?.includes(encodeURIComponent(cwd))));
    check(`${runtime} enforces automatic execution`, runtime === "claude" ? allArgs.some(a => a.includes("--enable-auto-mode") && !a.includes("--permission-mode")) : runtime !== "codex" ? rows.filter(r => r.kind === "permission").at(-1)?.data === "allow" : rows.filter(r => r.kind === "approval").at(-1)?.data === "never" && rows.filter(r => r.kind === "sandbox").at(-1)?.data === "danger-full-access" && !rows.some(r => /^(thread|turn)\//.test(r.kind) && r.data?.approvalPolicy === "on-request"));
    check(`${runtime} continues its session`, runtime === "claude" ? allArgs.some(a => a.includes("--continue"))
      : runtime === "opencode" ? allArgs.some(a => a.includes("--server") && a.includes("--session") && a.includes("opencode-session"))
      : runtime === "opencode1" ? allArgs.some(a => a.includes("attach") && a.includes("--session") && a.includes("opencode-session"))
      : allArgs.some(a => a.includes("resume") && a.includes("codex-session")));
    check(`${runtime} loads bus tools`, rows.some(r => r.kind === "tools" && r.data.includes("ab_ls") && r.data.includes("ab_send")));
    check(`${runtime} lists the real bus`, rows.some(r => r.kind === "listed" && JSON.stringify(r.data).includes(peerName)));
    check(`${runtime} receives the agent's answer`, rows.some(r => r.kind === "delivered" && r.data.includes("launcher-answer")));
    check(`${runtime} refuses a forbidden agent`, rows.some(r => r.kind === "denied" && r.data.isError));
    check(`${runtime} hides forbidden agents`, rows.some(r => r.kind === "listed" && !JSON.stringify(r.data).includes(deniedName)));
    check(`${runtime} does not pass owner credentials to the runtime`, rows.some(r => r.kind === "identity" && (r.data.ownerChecked || !process.env.AGENT_BUS_TOKEN) && !r.data.ownerLeaked));
    const identity = rows.find(r => r.kind === "identity")?.data.name;
    const registered = (await owner.ls()).find(r => r.name === identity);
    check(`${runtime} uses its assigned title`, !!registered && registered.name.includes("named-session") && registered.descr === `${runtime === "opencode1" ? "opencode" : runtime} ${cwd}`);
    if (!registered?.allow?.includes("@owner")) console.log(`${runtime} registered ACL: ${JSON.stringify(registered?.allow)}`);
    check(`${runtime} registers with the runtime @owner ACL term`, registered?.allow?.includes("@owner") === true);
    // The claim is about the address. A label suffix here only means an
    // earlier runtime in this run still holds the same label, which the
    // collision checks below own.
    check(`${runtime} migrates a saved dot address to template/instance`, !!registered && registered.name.startsWith(`#${runtime}/named-session@`) && registered.descr === `${runtime === "opencode1" ? "opencode" : runtime} ${cwd}` && !(await owner.ls()).some(r => r.name === legacy));
    check(`${runtime} releases its session locks`, !readdirSync(state).some(f => f.endsWith(".lock")));
    const serverPID = rows.find(r => r.kind === "server-ready")?.pid;
    let alive = false;
    if (serverPID) try { process.kill(serverPID, 0); alive = true; } catch {}
    check(`${runtime} cleans up its ${runtime === "opencode" || runtime === "opencode1" ? "server" : "App Server"}`, !alive);
    // The fixture answers 401 before it records anything, so a recorded
    // request is proof the launcher authenticated to its own server.
    if (runtime === "opencode") check("opencode authenticates to its own v2 API", rows.some(r => r.kind === "http" && r.data.path === "/api/config"));
    if (runtime === "opencode1") check("opencode1 authenticates to its own 1.x API", rows.some(r => r.kind === "http" && r.data.path === "/config"));

    const unavailable = join(dir, "missing-bus.sock");
    writeFileSync(events, "");
    let failureOutput = "";
    const missingBus = Bun.spawn([launcher], { cwd, env: { ...env, AGENT_BUS_TOKEN: "", AGENT_BUS_ADDR: unavailable, TERM: "xterm-256color", KITTY_WINDOW_ID: "", KONSOLE_DBUS_SERVICE: "", TMUX: "" },
      terminal: { data(_terminal, data) { failureOutput += Buffer.from(data).toString(); } },
    });
    processes.push(missingBus);
    const missingCode = await missingBus.exited;
    await settled(() => failureOutput);
    missingBus.terminal?.close();
    check(`${runtime} diagnoses an explicit unavailable bus without fallback or runtime startup`, missingCode !== 0 && failureOutput.includes(unavailable) && rowsIn(events).length === 0);
    check(`${runtime} restores its terminal title on startup failure`, restoredTitle(failureOutput));
    const ver = Bun.spawn([launcher, "--version"], { env, stdout: "pipe", stderr: "pipe" });
    check(`${runtime} shares the release version`, (await new Response(ver.stdout).text()).trim() === version && await ver.exited === 0);

    // Renaming takes effect at the next launch, including old name-only bindings.
    const previous = rows.find(r => r.kind === "identity")?.data.name;
    for (const f of readdirSync(state).filter(f => f.endsWith(".json"))) {
      const path = join(state, f), binding = JSON.parse(readFileSync(path, "utf8"));
      if (binding.name === previous) writeFileSync(path, JSON.stringify({ name: previous }));
    }
    if (runtime !== "claude") await owner.send({ to: previous, body: "keep old backlog" });
    writeFileSync(join(proj, id + ".jsonl"), JSON.stringify({ type: "custom-title", customTitle: "Renamed session" }) + "\n");
    writeFileSync(events, "");
    const resumed = Bun.spawn([launcher], { cwd, env: { ...env, TEST_TITLE: "Renamed session" }, stdout: "pipe", stderr: "pipe" });
    processes.push(resumed);
    const resumedErr = new Response(resumed.stderr).text();
    check(`${runtime} resumes after renaming`, await resumed.exited === 17);
    const again = readFileSync(events, "utf8").trim().split("\n").map(s => JSON.parse(s));
    const renamed = again.find(r => r.kind === "identity")?.data.name;
    check(`${runtime} changes its address after a rename on restart`, !!renamed && renamed !== previous && renamed.startsWith(`#${runtime}/renamed-session@`) && again.some(r => r.kind === "identity" && r.data.label === `${runtime === "opencode1" ? "opencode" : runtime} ${cwd}`));
    const renameDiagnostic = await resumedErr;
    const old = (await owner.ls()).find(r => r.name === previous);
    if (runtime !== "claude") {
      check(`${runtime} rename retains and reports the old queued inbox`, old?.queued === 1 && renameDiagnostic.includes(`old address ${previous} retained`));
      const previousBus = await owner.as(previous);
      check("rename preserves the old queued message", (await previousBus.consume({ wait: "0s" }))?.body === "keep old backlog");
    } else check("rename removes the old idle registry entry", !old);

    writeFileSync(events, "");
    const stable = Bun.spawn([launcher, "--descr", "Review billing"], { cwd, env: { ...env, TEST_TITLE: "Renamed session" }, stdout: "pipe", stderr: "pipe" });
    processes.push(stable);
    const stableErr = new Response(stable.stderr).text();
    check(`${runtime} reuses an unchanged derived address`, await stable.exited === 17 && rowsIn(events).some(r => r.kind === "identity" && r.data.name === renamed && r.data.label === "Review billing"));
    await stableErr;

    writeFileSync(join(proj, id + ".jsonl"), JSON.stringify({ type: "custom-title", customTitle: "Pinned title" }) + "\n");
    writeFileSync(events, "");
    const pinned = Bun.spawn([launcher], { cwd, env: { ...env, TEST_TITLE: "Pinned title", AGENT_BUS_NAME: renamed }, stdout: "pipe", stderr: "pipe" });
    processes.push(pinned);
    const pinnedErr = new Response(pinned.stderr).text();
    check(`${runtime} keeps an explicit address and purpose across a rename`, await pinned.exited === 17 && rowsIn(events).some(r => r.kind === "identity" && r.data.name === renamed && r.data.label === "Review billing"));
    check(`${runtime} preserves the record purpose on resume`, (await owner.ls()).find(r => r.name === renamed)?.descr === "Review billing");
    await pinnedErr;

    // First session: continue must not prevent startup when nothing exists.
    rmSync(join(proj, id + ".jsonl"));
    writeFileSync(events, "");
    // A delivery recorded by an earlier sub-run would answer for this one.
    rmSync(events + ".delivered", { force: true });
    rmSync(events + ".tui-ready", { force: true });
    const fresh = Bun.spawn([launcher], { cwd, env: { ...env, TEST_NO_HISTORY: "1", TEST_SESSION_ID: "fresh-codex", TEST_TITLE: "" }, stdout: "pipe", stderr: "pipe" });
    processes.push(fresh);
    const freshErr = new Response(fresh.stderr).text();
    check(`${runtime} starts when there is no conversation to continue`, await fresh.exited === 17);
    await freshErr;
    const freshRows = readFileSync(events, "utf8").trim().split("\n").map(s => JSON.parse(s));
    check(`${runtime} describes its runtime and launch directory`, freshRows.some(r => r.kind === "identity" && r.data.label === `${runtime === "opencode1" ? "opencode" : runtime} ${cwd}`));
    if (runtime === "claude") check("Claude installs the local channel plugin once per home", freshRows.filter(r => r.kind === "plugin-installed").length === 0 && readFileSync(join(home,"plugins/installed_plugins.json"),"utf8").includes("agent-bus@agent-bus-local"));
    if (runtime === "codex") check("Codex lets the fresh TUI create its own thread", !freshRows.some(r => r.kind === "thread/start") && freshRows.some(r => r.kind === "argv" && r.data.includes("--remote") && !r.data.includes("resume")));
    if (runtime === "codex") check("Codex fresh TUI keeps automatic execution", freshRows.some(r => r.kind === "argv" && r.data.includes("--remote") && r.data.includes('approval_policy="never"') && r.data.includes('sandbox_mode="danger-full-access"')));
    if (runtime === "opencode") check("opencode attaches its fresh TUI and pusher to the created session without a selection event", freshRows.some(r => r.kind === "http" && r.data.method === "POST" && r.data.path === "/api/session") && freshRows.some(r => r.kind === "argv" && r.data.includes("--server") && r.data[r.data.indexOf("--session") + 1] === "fresh-codex") && freshRows.some(r => r.kind === "delivered" && r.data.includes("launcher-answer")));
    if (runtime === "opencode1") check("opencode1 attaches its fresh TUI and pusher to the created session without a selection event", freshRows.some(r => r.kind === "http" && r.data.method === "POST" && r.data.path === "/session") && freshRows.some(r => r.kind === "argv" && r.data[0] === "attach" && r.data[r.data.indexOf("--session") + 1] === "fresh-codex") && freshRows.some(r => r.kind === "delivered" && r.data.includes("launcher-answer")));

    if (runtime === "claude") {
      // Several accounts on one machine: the flag moves the whole configuration
      // home, so the default account's conversation is not this account's.
      writeFileSync(join(proj, id + ".jsonl"), JSON.stringify({ type: "custom-title", customTitle: "Default account session" }) + "\n");
      writeFileSync(events, "");
      const second = Bun.spawn([launcher, "-2"], { cwd, env: { ...env, TEST_TITLE: "Default account session" }, stdout: "pipe", stderr: "pipe" });
      processes.push(second);
      const secondOut = new Response(second.stdout).text();
      check("ab-claude -2 starts", await second.exited === 17);
      const secondRows = rowsIn(events);
      const secondArgs = secondRows.filter(r => r.kind === "argv").map(r => r.data as string[]);
      check("ab-claude -2 runs the runtime in the sibling account home",
        secondRows.some(r => r.kind === "config-dir" && r.data === home + "2") && existsSync(home + "2"));
      check("ab-claude -2 keeps its own flag out of the runtime arguments", secondArgs.every(a => !a.includes("-2")));
      check("ab-claude -2 continues no session from the default account",
        secondArgs.some(a => a.includes("--session-id")) && secondArgs.every(a => !a.includes("--continue")));
      check("ab-claude -2 names the account it opened", (await secondOut).includes(home + "2"));
      // The same launcher without the flag still finds the default account's conversation.
      writeFileSync(events, "");
      const first = Bun.spawn([launcher], { cwd, env: { ...env, TEST_TITLE: "Default account session" }, stdout: "pipe", stderr: "pipe" });
      processes.push(first);
      const firstErr = new Response(first.stderr).text();
      check("ab-claude without a flag keeps the default account", await first.exited === 17
        && rowsIn(events).some(r => r.kind === "config-dir" && r.data === home)
        && rowsIn(events).filter(r => r.kind === "argv").some(a => a.data.includes("--continue")));
      await firstErr;
      rmSync(join(proj, id + ".jsonl"), { force: true });
    }

    if (runtime === "codex" || runtime === "opencode" || runtime === "opencode1") {
      const broken = Bun.spawn([launcher], { cwd, env: { ...env, TEST_FAIL_START: "1" }, stdout: "pipe", stderr: "pipe" });
      processes.push(broken);
      // The helper's own reason, from a log that cleanup then removes.
      check(`${runtime} startup failure is reported with the helper's own reason`, await broken.exited !== 0 && /exited during startup: .*fixture server refuses to start: TEST_FAIL_START/.test(await new Response(broken.stderr).text()));
      check(`${runtime} failed startup releases locks`, !readdirSync(state).some(f => f.endsWith(".lock")));
    }
    if (process.env.TEST_MAPPED_SOCKET) {
      writeFileSync(events, "");
      const local = Bun.spawn([launcher], { cwd, env: { ...env, AGENT_BUS_TOKEN: "", AGENT_BUS_ADDR: process.env.TEST_MAPPED_SOCKET }, stdout: "pipe", stderr: "pipe" });
      processes.push(local);
      const localErr = new Response(local.stderr).text();
      const code = await local.exited;
      const diagnostic = await localErr;
      if (code !== 17) console.log(diagnostic);
      check(`${runtime} acquires a session token through the mapped socket`, code === 17);
      const login = join(dir, runtime + "-runtime");
      mkdirSync(login);
      symlinkSync(dirname(process.env.TEST_MAPPED_SOCKET), join(login, "agent-bus"));
      writeFileSync(events, "");
      const discovered = Bun.spawn([launcher], { cwd, env: { ...env, AGENT_BUS_ADDR: "", AGENT_BUS_TOKEN: "", XDG_RUNTIME_DIR: login }, stdout: "pipe", stderr: "pipe" });
      processes.push(discovered);
      const discoveredErr = new Response(discovered.stderr).text();
      const discoveredCode = await discovered.exited;
      const discoveredDiagnostic = await discoveredErr;
      if (discoveredCode !== 17) console.log(discoveredDiagnostic);
      check(`${runtime} discovers its local socket and receives the agent's response without AGENT_BUS_ADDR`, discoveredCode === 17 && rowsIn(events).some(r => r.kind === "delivered" && r.data.includes("launcher-answer")));
      // Only the account socket, no shared one beside it — as on a host it
      // was forwarded to: the session stays on it with its own token
      // (docs/02-access.md#local-socket).
      const onesock = join(dir, runtime + "-onesock");
      mkdirSync(onesock);
      symlinkSync(process.env.TEST_MAPPED_SOCKET, join(onesock, basename(process.env.TEST_MAPPED_SOCKET)));
      writeFileSync(events, "");
      const alone = Bun.spawn([launcher], { cwd, env: { ...env, AGENT_BUS_TOKEN: "", AGENT_BUS_ADDR: join(onesock, basename(process.env.TEST_MAPPED_SOCKET)) }, stdout: "pipe", stderr: "pipe" });
      processes.push(alone);
      const aloneErr = new Response(alone.stderr).text();
      const aloneCode = await alone.exited;
      const aloneDiagnostic = await aloneErr;
      if (aloneCode !== 17) console.log(aloneDiagnostic);
      check(`${runtime} runs on the account socket alone, with no shared socket beside it`, aloneCode === 17 && rowsIn(events).some(r => r.kind === "delivered" && r.data.includes("launcher-answer")));
    }
    const missing = Bun.spawn([launcher], { cwd, env: { ...env, CLAUDE_BIN: join(dir, "missing"), CODEX_BIN: join(dir, "missing"), OPENCODE_BIN: join(dir, "missing") }, stdout: "pipe", stderr: "pipe" });
    check(`${runtime} diagnoses a missing executable`, await missing.exited !== 0 && (await new Response(missing.stderr).text()).includes("executable not found"));
    const held = [1, 2].map(n => {
      const events = join(dir, `${runtime}-held-${n}`);
      let output = "";
      const proc = Bun.spawn([launcher], { cwd, env: { ...env, TEST_EVENTS: events, TEST_SESSION_ID: `held-${n}`, TEST_NO_HISTORY: "1", TEST_HOLD: "30000", TERM: "xterm-256color", KITTY_WINDOW_ID: "", KONSOLE_DBUS_SERVICE: "", TMUX: "" },
        terminal: { data(_terminal, data) { output += Buffer.from(data).toString(); } },
      });
      processes.push(proc);
      return { proc, events, output: () => output };
    });
    for (let i = 0; i < 100 && !held.every(h => rowsIn(h.events).some(r => r.kind === "delivered")); i++) await Bun.sleep(50);
    for (const h of held) {
      const rows = rowsIn(h.events);
      if (rows.some(r => r.kind === "delivered" && r.data.includes("launcher-answer"))) continue;
      // The PTY holds stderr too; preserve it before cleanup removes the fixture.
      console.log(`${runtime} independent session failed: exit=${h.proc.exitCode}, events=${h.events}`);
      console.log(h.output());
      console.log(JSON.stringify(rows.map(r => ({ kind: r.kind, ...(r.kind === "identity" ? { data: r.data } : {}) }))));
    }
    check(`${runtime} starts both independent sessions`, held.every(h => rowsIn(h.events).some(r => r.kind === "delivered" && r.data.includes("launcher-answer"))));
    if (runtime === "claude") {
      const argv = rowsIn(held[0]!.events).find(r => r.kind === "argv").data as string[];
      const liveID = argv[argv.indexOf("--session-id") + 1]!;
      const liveTitle = `Live tab ${process.pid}`; // one run's title, never a parallel run's
      writeFileSync(join(proj, liveID + ".jsonl"), JSON.stringify({ type: "custom-title", customTitle: liveTitle }) + "\n");
      for (let i = 0; i < 70 && !held.some(h => h.output().includes(`Claude(${liveTitle})`)); i++) await Bun.sleep(50);
      check("Claude updates its terminal title after a live session rename", held.some(h => h.output().includes(`\x1b]0;Claude(${liveTitle})\x07`)));
    }
    held[0]!.proc.kill("SIGTERM");
    check(`${runtime} propagates termination`, await held[0]!.proc.exited === 143);
    check(`${runtime} termination leaves its sibling running`, held[1]!.proc.exitCode === null);
    held[1]!.proc.kill("SIGTERM");
    await held[1]!.proc.exited;
    for (const h of held) {
      await settled(h.output);
      h.proc.terminal?.close();
      const restored = restoredTitle(h.output());
      // Under the gate's load this has failed now and then: say what the terminal got.
      if (!restored) console.log(`  title output starts ${JSON.stringify(h.output().slice(0, 60))} and ends ${JSON.stringify(h.output().slice(-160))}`);
      check(`${runtime} restores its terminal title on termination`, restored);
      const pids = rowsIn(h.events).filter(r => r.kind === "argv" || r.kind === "mcp-pid").map(r => r.kind === "mcp-pid" ? r.data : r.pid);
      const alive = () => pids.some(pid => { try { process.kill(pid, 0); return true; } catch { return false; } });
      for (let i = 0; i < 30 && alive(); i++) await Bun.sleep(50);
      check(`${runtime} termination leaves no runtime or MCP child`, !alive());
      // Reclaim only fixture-recorded children even when testing a broken cleanup.
      for (const pid of pids) try { process.kill(pid, "SIGKILL"); } catch {}
    }
    if (runtime === "claude") { await collisions(launcher, cwd, env); await explicitReservation(launcher, cwd, env); }

  }
} finally {
  stop.abort(); await serve;
  for (const p of processes) if (p.exitCode === null) { p.kill(); await p.exited; }
  rmSync(dir, { recursive: true, force: true });
}
process.exit(failures ? 1 : 0);
