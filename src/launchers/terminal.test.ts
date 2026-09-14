import { expect, test } from "bun:test";
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { terminalTitle } from "./terminal.ts";

test("terminal titles prefer names, shorten paths, abbreviate home and reject control characters", () => {
  expect(terminalTitle("codex", "/home/me", "home", "/home/me")).toBe("Codex(home)");
  expect(terminalTitle("claude", "/home/me", undefined, "/home/me")).toBe("Claude(~)");
  expect(terminalTitle("opencode", "/home/me/project", undefined, "/home/me")).toBe("OpenCode(~/project)");
  expect(terminalTitle("codex", "/home/me/src/project", undefined, "/home/me")).toBe("Codex(src/project)");
  expect(terminalTitle("codex", "/rd/vhosts/realty", undefined, "/home/me")).toBe("Codex(vhosts/realty)");
  expect(terminalTitle("codex", "/", "  ", "/home/me")).toBe("Codex(/)");
  expect(terminalTitle("claude", "/", "hello\x07\x1b]2;bad\x9c\n")).toBe("Claude(hello]2;bad)");
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
    expect(generic.output).toContain("\x1b]0;Codex(long/path)\x07");
    expect(generic.output).toContain("\x1b]0;Codex(Named session)\x07");
    expect(generic.output).toContain("\x1b]0;Codex(Renamed session)\x07");
    expect(generic.output.startsWith("\x1b[22;0t")).toBe(true);
    expect(generic.output.endsWith("\x1b]0;\x07\x1b[23;0t")).toBe(true);
    expect(generic.output.match(/\x1b\[23;0t/g)?.length).toBe(1);
    expect(generic.rows).toEqual([]);
    const colors: Record<string, string> = {};
    for (const runtime of ["claude", "codex", "opencode"]) {
      const kitty = await run(runtime, { KITTY_WINDOW_ID: "42" });
      expect(kitty.rows.every(r => r.includes("window_id:42"))).toBe(true);
      const color = kitty.rows.find(r => r.includes("set-tab-color"))!.find(a => a.startsWith("active_bg="))!;
      colors[runtime] = color;
      expect(kitty.rows.some(r => r.includes("set-tab-title") && r.at(-1) === "")).toBe(true);
      expect(kitty.rows.at(-1)).toEqual([join(dir, "kitty"), "@", "set-tab-color", "--match", "window_id:42", "active_fg=NONE", "active_bg=NONE", "inactive_fg=NONE", "inactive_bg=NONE"]);
      const next = await run(runtime, { KITTY_WINDOW_ID: "42" });
      expect(next.rows.find(r => r.includes("set-tab-color"))).not.toContain(color);
    }
    expect(new Set(Object.values(colors)).size).toBe(3);
    const rgb = colors.opencode!.split("#")[1]!.match(/../g)!.map(c => parseInt(c, 16));
    expect(rgb[2]!).toBeGreaterThan(rgb[0]!); // Violet, never a red palette.
    const konsole = await run("opencode", { KONSOLE_DBUS_SERVICE: "org.kde.konsole-123", KONSOLE_DBUS_SESSION: "/Sessions/7" });
    expect(konsole.rows.some(r => r.includes("org.kde.konsole.Session.setTitle") && r.includes("OpenCode(Named session)"))).toBe(true);
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
