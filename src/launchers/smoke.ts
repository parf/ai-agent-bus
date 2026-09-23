import { existsSync, mkdirSync, mkdtempSync, writeFileSync, readFileSync, readdirSync, rmSync, symlinkSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
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
const rowsIn = (file: string): any[] => existsSync(file) ? readFileSync(file, "utf8").split("\n").filter(Boolean).map(s => JSON.parse(s)) : [];
const processes: ReturnType<typeof Bun.spawn>[] = [];
try {
  for (const runtime of ["claude", "codex", "opencode"]) {
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
    writeFileSync(executable, `#!/bin/sh\nexec '${process.execPath}' '${fixture}' "$@"\n`, { mode: 0o700 });
    const events = join(dir, runtime + ".events");
    const env = { ...process.env, AGENT_BUS_NAME: "", TEST_RUNTIME: runtime,
      CLAUDE_BIN: executable, CODEX_BIN: executable, OPENCODE_BIN: executable, CLAUDE_CONFIG_DIR: home,
      XDG_STATE_HOME: join(dir, "state"), TEST_EVENTS: events, TEST_PEER: peerName,
      TEST_TITLE: "Named session", TEST_EXIT: "17", TEST_DENIED: deniedName,
      AGENT_BUS_OWNER_TOKEN: "must-not-reach-the-runtime" };
    const launcher = resolve(process.env.LAUNCHER_BUILD || import.meta.dir, `ab-${runtime}`);
    const state = join(env.XDG_STATE_HOME, "agent-bus/sessions");
    mkdirSync(state, { recursive: true });
    const legacy = defaultName({ ...env, AGENT_BUS_RUNTIME: runtime, AGENT_BUS_CWD: "Named session" });
    await owner.register({ name: legacy, kind: "agent", descr: "Named session" });
    const legacyID = runtime === "claude" ? id : `${runtime === "codex" ? "codex" : "opencode"}-session`;
    writeFileSync(join(state, `${hash(`${runtime}:${legacyID}`)}.json`), JSON.stringify({ name: legacy, ...(runtime === "claude" ? {} : { base: legacy }) }), { mode: 0o600 });
    const overrides = runtime === "claude" ? ["--permission-mode", "plan"]
      : runtime === "opencode" ? ["--dir", "opencode path with spaces"]
      : ["-C", "codex path with spaces", "-a", "on-request", "-s", "read-only"];
    let terminalOutput = "";
    const proc = Bun.spawn([launcher, ...overrides, "a prompt with spaces"], { cwd: runtime === "claude" ? cwd : dir,
      env: { ...env, TERM: "xterm-256color", KITTY_WINDOW_ID: "", KONSOLE_DBUS_SERVICE: "", TMUX: "" },
      terminal: { data(_terminal, data) { terminalOutput += Buffer.from(data).toString(); } },
    });
    processes.push(proc);
    const code = await proc.exited;
    await Bun.sleep(20);
    proc.terminal?.close();
    const diagnostic = terminalOutput;
    const titleRuntime = { claude: "Claude", codex: "Codex", opencode: "OpenCode" }[runtime];
    check(`${runtime} sets its named terminal title and restores it after TUI exit`, terminalOutput.startsWith("\x1b[22;0t") && terminalOutput.includes(`\x1b]0;${titleRuntime}(Named session`) && terminalOutput.endsWith("\x1b]0;\x07\x1b[23;0t"));
    if (code !== 17) console.log(diagnostic);
    check(`${runtime} propagates the runtime exit status`, code === 17);
    const rows = readFileSync(events, "utf8").trim().split("\n").map(s => JSON.parse(s));
    const allArgs = rows.filter(r => r.kind === "argv").map(r => r.data);
    check(`${runtime} prevents the runtime from replacing its terminal title`, runtime === "codex" ? allArgs.some(a => a.includes("tui.terminal_title=[]")) : rows.some(r => r.kind === "terminal-env" && r.data === "1"));
    check(`${runtime} preserves the prompt argument`, allArgs.some(a => a.includes("a prompt with spaces")));
    if (runtime === "codex") check("Codex resolves a relative directory exactly once", allArgs.some(a => a.filter((v: string) => v === "-C").length === 1 && a[a.indexOf("-C") + 1] === cwd));
    if (runtime === "codex") {
      check("Codex rejects an unauthenticated runtime client", rows.some(r => r.kind === "runtime-denied" && r.data === 401));
      check("Codex TUI authenticates to its private server", rows.some(r => r.kind === "tui-authenticated"));
    }
    if (runtime === "opencode") check("opencode resolves a relative directory exactly once", allArgs.some(a => a.filter((v: string) => v === "--dir").length === 1 && a[a.indexOf("--dir") + 1] === cwd));
    check(`${runtime} enforces automatic execution`, runtime === "claude" ? allArgs.some(a => a.includes("--enable-auto-mode") && !a.includes("--permission-mode")) : runtime === "opencode" ? rows.filter(r => r.kind === "permission").at(-1)?.data === "allow" : rows.filter(r => r.kind === "approval").at(-1)?.data === "never" && rows.filter(r => r.kind === "sandbox").at(-1)?.data === "danger-full-access" && !rows.some(r => /^(thread|turn)\//.test(r.kind) && r.data?.approvalPolicy === "on-request"));
    check(`${runtime} continues its session`, runtime === "claude" ? allArgs.some(a => a.includes("--continue"))
      : runtime === "opencode" ? allArgs.some(a => a.includes("attach") && a.includes("--session") && a.includes("opencode-session"))
      : allArgs.some(a => a.includes("resume") && a.includes("codex-session")));
    check(`${runtime} loads bus tools`, rows.some(r => r.kind === "tools" && r.data.includes("ab_ls") && r.data.includes("ab_send")));
    check(`${runtime} lists the real bus`, rows.some(r => r.kind === "listed" && JSON.stringify(r.data).includes(peerName)));
    check(`${runtime} receives the service answer`, rows.some(r => r.kind === "delivered" && r.data.includes("launcher-answer")));
    check(`${runtime} refuses a forbidden service`, rows.some(r => r.kind === "denied" && r.data.isError));
    check(`${runtime} hides forbidden services`, rows.some(r => r.kind === "listed" && !JSON.stringify(r.data).includes(deniedName)));
    check(`${runtime} does not pass owner credentials to the runtime`, rows.some(r => r.kind === "identity" && !r.data.ownerLeaked));
    const identity = rows.find(r => r.kind === "identity")?.data.name;
    const registered = (await owner.ls()).find(r => r.name === identity);
    check(`${runtime} uses its assigned title`, !!registered && registered.name.includes("named-session") && !!registered.descr?.startsWith("Named session"));
    if (!registered?.allow?.includes("@owner")) console.log(`${runtime} registered ACL: ${JSON.stringify(registered?.allow)}`);
    check(`${runtime} registers with the runtime @owner ACL term`, registered?.allow?.includes("@owner") === true);
    // The claim is about the address. A label suffix here only means an
    // earlier runtime in this run still holds the same label, which the
    // collision checks below own.
    check(`${runtime} migrates a saved dot address to template/instance`, !!registered && registered.name.startsWith(`#${runtime}/named-session@`) && !!registered.descr?.startsWith("Named session") && !(await owner.ls()).some(r => r.name === legacy));
    check(`${runtime} releases its session locks`, !readdirSync(state).some(f => f.endsWith(".lock")));
    const serverPID = rows.find(r => r.kind === "server-ready")?.pid;
    let alive = false;
    if (serverPID) try { process.kill(serverPID, 0); alive = true; } catch {}
    check(`${runtime} cleans up its ${runtime === "opencode" ? "server" : "App Server"}`, !alive);
    // The fixture answers 401 before it records anything, so a recorded
    // request is proof the launcher authenticated to its own server.
    if (runtime === "opencode") check("opencode authenticates to its own server", rows.some(r => r.kind === "http" && r.data.path === "/config"));

    const unavailable = join(dir, "missing-bus.sock");
    writeFileSync(events, "");
    let failureOutput = "";
    const missingBus = Bun.spawn([launcher], { cwd, env: { ...env, AGENT_BUS_TOKEN: "", AGENT_BUS_ADDR: unavailable, TERM: "xterm-256color", KITTY_WINDOW_ID: "", KONSOLE_DBUS_SERVICE: "", TMUX: "" },
      terminal: { data(_terminal, data) { failureOutput += Buffer.from(data).toString(); } },
    });
    processes.push(missingBus);
    const missingCode = await missingBus.exited;
    await Bun.sleep(20);
    missingBus.terminal?.close();
    check(`${runtime} diagnoses an explicit unavailable bus without fallback or runtime startup`, missingCode !== 0 && failureOutput.includes(unavailable) && rowsIn(events).length === 0);
    check(`${runtime} restores its terminal title on startup failure`, failureOutput.startsWith("\x1b[22;0t") && failureOutput.endsWith("\x1b]0;\x07\x1b[23;0t"));
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
    check(`${runtime} changes its address after a rename on restart`, !!renamed && renamed !== previous && renamed.startsWith(`#${runtime}/renamed-session@`) && again.some(r => r.kind === "identity" && r.data.label.startsWith("Renamed session")));
    const renameDiagnostic = await resumedErr;
    const old = (await owner.ls()).find(r => r.name === previous);
    if (runtime !== "claude") {
      check(`${runtime} rename retains and reports the old queued inbox`, old?.queued === 1 && renameDiagnostic.includes(`old address ${previous} retained`));
      const previousBus = await owner.as(previous);
      check("rename preserves the old queued message", (await previousBus.consume({ wait: "0s" }))?.body === "keep old backlog");
    } else check("rename removes the old idle registry entry", !old);

    writeFileSync(events, "");
    const stable = Bun.spawn([launcher], { cwd, env: { ...env, TEST_TITLE: "Renamed session" }, stdout: "pipe", stderr: "pipe" });
    processes.push(stable);
    const stableErr = new Response(stable.stderr).text();
    check(`${runtime} reuses an unchanged derived address`, await stable.exited === 17 && rowsIn(events).some(r => r.kind === "identity" && r.data.name === renamed));
    await stableErr;

    writeFileSync(join(proj, id + ".jsonl"), JSON.stringify({ type: "custom-title", customTitle: "Pinned title" }) + "\n");
    writeFileSync(events, "");
    const pinned = Bun.spawn([launcher], { cwd, env: { ...env, TEST_TITLE: "Pinned title", AGENT_BUS_NAME: renamed }, stdout: "pipe", stderr: "pipe" });
    processes.push(pinned);
    const pinnedErr = new Response(pinned.stderr).text();
    check(`${runtime} keeps an explicit address across a rename`, await pinned.exited === 17 && rowsIn(events).some(r => r.kind === "identity" && r.data.name === renamed && r.data.label.startsWith("Pinned title")));
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
    check(`${runtime} falls back to the launch directory`, freshRows.some(r => r.kind === "identity" && r.data.label.startsWith(`${runtime}(${cwd})`)));
    if (runtime === "codex") check("Codex lets the fresh TUI create its own thread", !freshRows.some(r => r.kind === "thread/start") && freshRows.some(r => r.kind === "argv" && r.data.includes("--remote") && !r.data.includes("resume")));
    if (runtime === "opencode") check("opencode attaches its fresh TUI and pusher to the created session without a selection event", freshRows.some(r => r.kind === "http" && r.data.method === "POST" && r.data.path === "/session") && freshRows.some(r => r.kind === "argv" && r.data[0] === "attach" && r.data[r.data.indexOf("--session") + 1] === "fresh-codex") && freshRows.some(r => r.kind === "delivered" && r.data.includes("launcher-answer")));

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

    if (runtime === "codex" || runtime === "opencode") {
      const broken = Bun.spawn([launcher], { cwd, env: { ...env, TEST_FAIL_START: "1" }, stdout: "pipe", stderr: "pipe" });
      processes.push(broken);
      check(`${runtime} startup failure is reported`, await broken.exited !== 0 && (await new Response(broken.stderr).text()).includes("startup"));
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
      check(`${runtime} discovers its local socket and receives the service response without AGENT_BUS_ADDR`, discoveredCode === 17 && rowsIn(events).some(r => r.kind === "delivered" && r.data.includes("launcher-answer")));
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
    check(`${runtime} starts both independent sessions`, held.every(h => rowsIn(h.events).some(r => r.kind === "delivered" && r.data.includes("launcher-answer"))));
    if (runtime === "claude") {
      const argv = rowsIn(held[0]!.events).find(r => r.kind === "argv").data as string[];
      const liveID = argv[argv.indexOf("--session-id") + 1]!;
      writeFileSync(join(proj, liveID + ".jsonl"), JSON.stringify({ type: "custom-title", customTitle: "Live tab name" }) + "\n");
      for (let i = 0; i < 70 && !held.some(h => h.output().includes("Claude(Live tab name)")); i++) await Bun.sleep(50);
      check("Claude updates its terminal title after a live session rename", held.some(h => h.output().includes("\x1b]0;Claude(Live tab name)\x07")));
    }
    held[0]!.proc.kill("SIGTERM");
    check(`${runtime} propagates termination`, await held[0]!.proc.exited === 143);
    check(`${runtime} termination leaves its sibling running`, held[1]!.proc.exitCode === null);
    held[1]!.proc.kill("SIGTERM");
    await held[1]!.proc.exited;
    for (const h of held) {
      await Bun.sleep(20);
      h.proc.terminal?.close();
      check(`${runtime} restores its terminal title on termination`, h.output().startsWith("\x1b[22;0t") && h.output().endsWith("\x1b]0;\x07\x1b[23;0t"));
      const pids = rowsIn(h.events).filter(r => r.kind === "argv" || r.kind === "mcp-pid").map(r => r.kind === "mcp-pid" ? r.data : r.pid);
      const alive = () => pids.some(pid => { try { process.kill(pid, 0); return true; } catch { return false; } });
      for (let i = 0; i < 30 && alive(); i++) await Bun.sleep(50);
      check(`${runtime} termination leaves no runtime or MCP child`, !alive());
      // Reclaim only fixture-recorded children even when testing a broken cleanup.
      for (const pid of pids) try { process.kill(pid, "SIGKILL"); } catch {}
    }
    if (runtime === "claude") {
      const title = `Collision-${process.pid}`;
      const concurrent = [1, 2, 3].map(n => {
        const events = join(dir, `collision-${n}`);
        const isolatedState = join(dir, `collision-state-${n}`);
        const proc = Bun.spawn([launcher, "--name", title], { cwd,
          env: { ...env, XDG_STATE_HOME: isolatedState, TEST_EVENTS: events, TEST_HOLD: "1500", TEST_TAG: String(n) }, stdout: "pipe", stderr: "pipe" });
        processes.push(proc);
        return { proc, events, isolatedState, err: new Response(proc.stderr).text() };
      });
      const codes = await Promise.all(concurrent.map(async c => { const code = await c.proc.exited; await c.err; return code; }));
      check("concurrent sessions all receive their answers", codes.every(c => c === 17));
      const identities = concurrent.map(c => readFileSync(c.events, "utf8").trim().split("\n").map(s => JSON.parse(s)).find(r => r.kind === "identity")?.data);
      check("duplicate labels receive #2 and #3", new Set(identities.map(i => i?.label)).size === 3 && [title, `${title} #2`, `${title} #3`].every(t => identities.some(i => i?.label === t)));
      check("duplicate names have separate bus inboxes", new Set(identities.map(i => i?.name)).size === 3);
      check("concurrent cleanup releases every lock", concurrent.every(c => !readdirSync(join(c.isolatedState, "agent-bus/sessions")).some(f => f.endsWith(".lock"))));
    }
  }
} finally {
  stop.abort(); await serve;
  for (const p of processes) if (p.exitCode === null) { p.kill(); await p.exited; }
  rmSync(dir, { recursive: true, force: true });
}
process.exit(failures ? 1 : 0);
