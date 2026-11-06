import { expect, test } from "bun:test";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync, writeFileSync, openSync, closeSync, statSync } from "node:fs";
import { join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { accountOf, busLabel, cleanLabel, glyph, terminalTitle } from "./terminal.ts";

test("titles lead with the runtime and account glyph, then the label, with no runtime word", () => {
  expect(terminalTitle("claude", "CEO")).toBe("✴️ CEO");
  expect(terminalTitle("claude", "DIR", 2)).toBe("🔶 DIR");
  expect(terminalTitle("claude", "x", 3)).toBe("🌟 x");
  expect(terminalTitle("claude", "x", 4)).toBe("🔷 x");
  expect(terminalTitle("codex", "MDH-CEO")).toBe("❇️ MDH-CEO");
  expect(terminalTitle("opencode", "y")).toBe("👾 y");
  expect(glyph("opencode1")).toBe("👾");
  expect(accountOf({ CLAUDE_CONFIG_DIR: "/home/me/.claude2" })).toBe(2);
  expect(accountOf({ CLAUDE_CONFIG_DIR: "/home/me/.claude4/" })).toBe(4);
  expect(accountOf({ CLAUDE_CONFIG_DIR: "/home/me/cfg2" })).toBe(1);
  expect(accountOf({})).toBe(1);
  expect(busLabel("#ab-w1b")).toBe("ab-w1b");
  expect(busLabel("#claude/home@node.example")).toBe("home");
  expect(busLabel("#mdhouse-ceo@parf.us")).toBe("mdhouse-ceo");
  expect(busLabel(undefined)).toBeUndefined();
});

test("labels lose every control and bidi override and are capped by graphemes", () => {
  expect(cleanLabel("hello\x07\x1b]2;bad\x9c\n")).toBe("hello]2;bad");
  expect(cleanLabel("a\x1b[31mred\x9b0m\u202eevil\u2066x\u200f")).toBe("a[31mred0mevilx");
  expect(cleanLabel("  W1B\t\tT-35  ")).toBe("W1B T-35");
  expect(cleanLabel("\x1b\x07  ")).toBeUndefined();
  const coder = "👩‍💻";
  expect(cleanLabel(coder.repeat(30))).toBe(coder.repeat(19)); // five UTF-16 units each, 95 of 96
  expect(cleanLabel("🔶".repeat(30))).toBe("🔶".repeat(24));
  expect(cleanLabel("x".repeat(30))).toBe("x".repeat(24));
  // One grapheme with 3000 combining marks is still bounded in UTF-16 units.
  const heavy = cleanLabel("ab" + String.fromCharCode(0x301).repeat(3000))!;
  expect(heavy).toBe("a");
  expect(cleanLabel("é".repeat(10) + "a" + String.fromCharCode(0x301).repeat(90))).toBe("é".repeat(10));
});

test("a hostile launch directory is cleaned like any other label", async () => {
  const { Terminal } = await import("./terminal.ts");
  const saved = process.env.AB_TAB_TITLE;
  delete process.env.AB_TAB_TITLE;
  try {
    const cwd = "/work/x\x1b]0;pwn\x07y" + String.fromCharCode(0x202e) + "z" + "q".repeat(80);
    expect(new Terminal("claude", cwd, 1, "/home/me").recommended()).toBe("x]0;pwnyz" + "q".repeat(15));
    expect(new Terminal("claude", "/", 1, "/home/me").recommended()).toBe("/");
  } finally { if (saved !== undefined) process.env.AB_TAB_TITLE = saved; }
});

test("PTY titles restore once; palettes rotate, target this tab and reset; missing helpers and pipes are harmless", async () => {
  const root = resolve(import.meta.dir, "../../tmp/terminal-tests");
  mkdirSync(root, { recursive: true });
  const dir = mkdtempSync(join(root, "run-"));
  const calls = join(dir, "calls");
  const env = { ...process.env, TERM: "xterm-256color", KITTY_WINDOW_ID: "", KONSOLE_DBUS_SERVICE: "", KONSOLE_DBUS_SESSION: "", TMUX: "", PATH: dir, XDG_CACHE_HOME: dir, TEST_CALLS: calls };
  const helper = `#!${process.execPath}\nimport { appendFileSync } from 'node:fs';\nappendFileSync(process.env.TEST_CALLS, JSON.stringify(process.argv.slice(1))+'\\n');\nif(process.argv.includes('org.kde.konsole.Session.tabTitleFormat')) console.log(process.argv.at(-1)==='0'?'%d : %n':'%u@%h');\n`;
  for (const bin of ["kitty", "konsoleprofile", "qdbus6"]) writeFileSync(join(dir, bin), helper, { mode: 0o700 });
  const entry = join(import.meta.dir, "terminal.ts");
  async function run(runtime: string, extra: Record<string, string> = {}, tty = true) {
    writeFileSync(calls, "");
    let output = "";
    const code = `import {Terminal} from ${JSON.stringify(entry)}; const t=new Terminal(${JSON.stringify(runtime)},'/some/long/path'); t.set('Named session'); t.set('Renamed session'); t.restore(); t.restore();`;
    const p = Bun.spawn([process.execPath, "-e", code], { env: { ...env, ...extra },
      ...(tty ? { terminal: { data(_t: Bun.Terminal, data: Uint8Array) { output += Buffer.from(data).toString(); } } } : { stdout: "pipe" as const, stderr: "pipe" as const }),
    });
    try {
      expect(await p.exited).toBe(0);
      if (!tty) output = await new Response(p.stdout).text();
      // Drain the PTY's final output event after process exit.
      await Bun.sleep(20);
      return { output, rows: readFileSync(calls, "utf8").trim().split("\n").filter(Boolean).map(s => JSON.parse(s) as string[]) };
    } finally { p.terminal?.close(); }
  }
  try {
    const generic = await run("codex");
    expect(generic.output).toContain("\x1b]0;❇️ path\x07");
    expect(generic.output).toContain("\x1b]0;❇️ Named session\x07");
    expect(generic.output).toContain("\x1b]0;❇️ Renamed session\x07");
    expect(generic.output.startsWith("\x1b[22;0t")).toBe(true);
    expect(generic.output.endsWith("\x1b]0;\x07\x1b[23;0t")).toBe(true);
    expect(generic.output.match(/\x1b\[23;0t/g)?.length).toBe(1);
    expect(generic.rows).toEqual([]);
    const colors: Record<string, string> = {};
    for (const runtime of ["claude", "codex", "opencode"]) {
      const kitty = await run(runtime, { KITTY_WINDOW_ID: "42" });
      // Every change targets this tab; only the listing reads the whole of kitty.
      expect(kitty.rows.filter(r => r[2] !== "ls").every(r => r.includes("window_id:42"))).toBe(true);
      expect(kitty.rows.filter(r => r[2] === "ls").map(r => r.slice(1))).toEqual([["@", "ls"]]);
      const color = kitty.rows.find(r => r.includes("set-tab-color"))!.find(a => a.startsWith("active_bg="))!;
      colors[runtime] = color;
      expect(kitty.rows.some(r => r.includes("set-tab-title") && r.at(-1) === "")).toBe(true);
      expect(kitty.rows.at(-1)).toEqual([join(dir, "kitty"), "@", "set-tab-color", "--match", "window_id:42", "active_fg=NONE", "active_bg=NONE", "inactive_fg=NONE", "inactive_bg=NONE"]);
      const counter = join(dir, "agent-bus/terminal", runtime);
      const previous = readFileSync(counter, "utf8"), snapshot = openSync(counter, "r");
      try {
        const next = await run(runtime, { KITTY_WINDOW_ID: "42" });
        expect(next.rows.find(r => r.includes("set-tab-color"))).not.toContain(color);
        expect(readFileSync(snapshot, "utf8")).toBe(previous);
        expect(statSync(counter).mode & 0o777).toBe(0o600);
      } finally { closeSync(snapshot); }
    }
    expect(new Set(Object.values(colors)).size).toBe(3);
    const rgb = colors.opencode!.split("#")[1]!.match(/../g)!.map(c => parseInt(c, 16));
    expect(rgb[2]!).toBeGreaterThan(rgb[0]!); // Violet, never a red palette.
    const konsole = await run("opencode", { KONSOLE_DBUS_SERVICE: "org.kde.konsole-123", KONSOLE_DBUS_SESSION: "/Sessions/7" });
    expect(konsole.rows.some(r => r.includes("org.kde.konsole.Session.setTitle") && r.includes("👾 Named session"))).toBe(true);
    expect(konsole.rows.filter(r => r[0]?.endsWith("qdbus6")).every(r => r[2] === "/Sessions/7")).toBe(true);
    expect(konsole.rows.some(r => r.includes("TabColor="))).toBe(true);
    expect(konsole.rows.slice(-2).map(r => r.slice(-3))).toEqual([
      ["org.kde.konsole.Session.setTabTitleFormat", "0", "%d : %n"],
      ["org.kde.konsole.Session.setTabTitleFormat", "1", "%u@%h"],
    ]);
    expect((await run("claude", { PATH: "/nonexistent", KITTY_WINDOW_ID: "42" })).output).toContain("\x1b[23;0t");
    expect(await run("codex", { KITTY_WINDOW_ID: "42" }, false)).toEqual({ output: "", rows: [] });
    expect(await run("codex", { TERM: "dumb", KITTY_WINDOW_ID: "42" })).toEqual({ output: "", rows: [] });
  } finally { rmSync(dir, { recursive: true, force: true }); }
}, 15000);

test("labels: the session's choice, then AB_TAB_TITLE, then the bus name, then the session; never the bus name itself", async () => {
  const { Terminal } = await import("./terminal.ts");
  const saved = process.env.AB_TAB_TITLE;
  try {
    delete process.env.AB_TAB_TITLE;
    const plain = new Terminal("claude", "/home/me/src/project", 1, "/home/me");
    expect(plain.label()).toBe("project");
    plain.set("Named session");
    expect(plain.label()).toBe("Named session");
    plain.setBus("#claude/home@node");
    expect(plain.label()).toBe("home");
    process.env.AB_TAB_TITLE = "CEO\x1b]0;x\x07";
    const t = new Terminal("codex", "/x");
    t.setBus("#ab-w1b@parf.us");
    expect(t.setLabel()).toEqual({ current: "CEO]0;x", recommended: "CEO]0;x" });
    expect(t.setLabel("W1B T-35")).toEqual({ current: "W1B T-35", recommended: "CEO]0;x" });
    expect(() => t.setLabel("\x1b\x07")).toThrow("printable");
    expect(t.label()).toBe("W1B T-35");
  } finally { if (saved === undefined) delete process.env.AB_TAB_TITLE; else process.env.AB_TAB_TITLE = saved; }
});

test("kitty gets its own tab title back, the label reaches it clean, and tmux window names are claimed, restored and shared", async () => {
  const tmux = Bun.which("tmux");
  const root = resolve(import.meta.dir, "../../tmp/terminal-tests");
  mkdirSync(root, { recursive: true });
  const dir = mkdtempSync(join(root, "tab-"));
  const calls = join(dir, "calls"), seen = join(dir, "seen");
  writeFileSync(join(dir, "kitty"), `#!${process.execPath}\nimport { appendFileSync } from 'node:fs';\nappendFileSync(process.env.TEST_CALLS, JSON.stringify(process.argv.slice(1))+'\\n');\nif(process.argv.includes('ls')) console.log(process.env.TEST_KITTY_LS||'[]');\n`, { mode: 0o700 });
  if (tmux) symlinkSync(tmux, join(dir, "tmux"));
  const env = { ...process.env, TERM: "xterm-256color", KITTY_WINDOW_ID: "", KONSOLE_DBUS_SERVICE: "", KONSOLE_DBUS_SESSION: "", TMUX: "", TMUX_PANE: "", AB_TAB_TITLE: "", AGENT_BUS_TITLE_OWNER: "", PATH: dir, XDG_CACHE_HOME: dir, TEST_CALLS: calls, TEST_SEEN: seen };
  const entry = join(import.meta.dir, "terminal.ts");
  async function run(extra: Record<string, string>, restore = true) {
    writeFileSync(calls, "");
    writeFileSync(seen, "");
    let output = "";
    const code = `import {Terminal} from ${JSON.stringify(entry)}; import {spawnSync} from 'node:child_process'; import {writeFileSync} from 'node:fs';
const t=new Terminal('claude','/some/path');
if(process.env.TMUX_PANE) writeFileSync(process.env.TEST_SEEN, spawnSync('tmux',['display-message','-p','-t',process.env.TMUX_PANE,'#{window_name}'],{encoding:'utf8'}).stdout.trim());
${restore ? "t.restore();" : ""}`;
    const p = Bun.spawn([process.execPath, "-e", code], { env: { ...env, ...extra }, terminal: { data(_t: Bun.Terminal, data: Uint8Array) { output += Buffer.from(data).toString(); } } });
    try {
      expect(await p.exited).toBe(0);
      await Bun.sleep(20);
      return { output, seen: readFileSync(seen, "utf8"), rows: readFileSync(calls, "utf8").trim().split("\n").filter(Boolean).map(s => JSON.parse(s) as string[]) };
    } finally { p.terminal?.close(); }
  }
  const titles = (rows: string[][]) => rows.filter(r => r.includes("set-tab-title")).map(r => r.at(-1));
  let outer: ReturnType<typeof Bun.spawn> | undefined;
  const sock = join(dir, "sock");
  const t = (...a: string[]) => spawnSync(tmux!, ["-S", sock, ...a], { encoding: "utf8" }).stdout.trim();
  try {
    // A tab title given on purpose comes back; one that only followed its window is let go.
    const kittyLs = (tab: string, window: string) => JSON.stringify([{ tabs: [{ title: "other", windows: [{ id: 7, title: "zsh" }] }, { title: tab, windows: [{ id: 42, title: window }] }] }]);
    expect(titles((await run({ KITTY_WINDOW_ID: "42", TEST_KITTY_LS: kittyLs("CEO", "zsh") })).rows)).toEqual(["✴️ path", "CEO"]);
    expect(titles((await run({ KITTY_WINDOW_ID: "42", TEST_KITTY_LS: kittyLs("zsh", "zsh") })).rows)).toEqual(["✴️ path", ""]);
    // An escape, a bell, a CSI and a bidi override never reach the terminal or kitty.
    const hostile = await run({ KITTY_WINDOW_ID: "42", AB_TAB_TITLE: "a\x1b]0;pwn\x07b\x9b2J\u202ec" });
    expect(hostile.output).toContain("\x1b]0;✴️ a]0;pwnb2Jc\x07");
    expect(hostile.output).not.toContain("\x1b]0;pwn");
    expect(hostile.output).not.toContain("\u202e");
    expect(titles(hostile.rows)[0]).toBe("✴️ a]0;pwnb2Jc");

    if (!tmux) return;
    t("-f", "/dev/null", "new-session", "-d", "-s", "t", "-x", "80", "-y", "24");
    t("rename-window", "-t", "t", "w##(x)"); // tmux would run #(x)
    expect(t("display-message", "-p", "-t", "t", "#{window_name}")).toBe("w#(x)");
    const pane = t("display-message", "-p", "-t", "t", "#{pane_id}");
    const inTmux = { TMUX: `${sock},1,0`, TMUX_PANE: pane };
    const name = () => t("display-message", "-p", "-t", "t", "#{window_name}");
    // Claimed with the glyph, given back exactly, a # never run as a format.
    expect((await run(inTmux)).seen).toBe("w#(x) ✴️");
    expect(name()).toBe("w#(x)");
    expect(t("show-options", "-wq", "-t", "t", "automatic-rename")).toBe("automatic-rename off"); // the window's own value, put back
    expect(t("show-options", "-wqv", "-t", "t", "@ab_title_owner")).toBe("");
    // A launcher that died without restoring: the next one starts from the saved name.
    await run(inTmux, false);
    expect(name()).toBe("w#(x) ✴️");
    expect((await run(inTmux)).seen).toBe("w#(x) ✴️");
    expect(name()).toBe("w#(x)");
    // Another live launcher owns the window: its name stays, and a launcher it started stays off the tab.
    outer = Bun.spawn([process.execPath, "-e", "setTimeout(() => {}, 60000)", "launcher"]);
    t("rename-window", "-t", "t", "outer ✴️");
    t("set-option", "-w", "-t", "t", "@ab_title_owner", `${outer.pid}:%99`);
    expect((await run(inTmux)).seen).toBe("outer ✴️");
    expect(name()).toBe("outer ✴️");
    const nested = await run({ ...inTmux, KITTY_WINDOW_ID: "42", AGENT_BUS_TITLE_OWNER: String(outer.pid) });
    expect(nested.output).toBe("");
    expect(nested.rows).toEqual([]);
    expect(name()).toBe("outer ✴️");
  } finally {
    outer?.kill();
    if (tmux && existsSync(sock)) t("kill-server");
    rmSync(dir, { recursive: true, force: true });
  }
}, 20000);
