import { writeState } from "./state.ts";
import { spawnSync } from "node:child_process";
import { mkdirSync, readFileSync } from "node:fs";
import { homedir } from "node:os";
import { basename, join } from "node:path";

type Runtime = "claude" | "codex" | "opencode" | "opencode1";
// The glyph leads the title, so a narrow tab still shows which runtime and
// account it is; Claude has one per account (-2, -3, -4).
const glyphs = { claude: ["✴️", "🔶", "🌟", "🔷"], codex: ["❇️"], opencode: ["👾"], opencode1: ["👾"] };
const palettes = {
  claude: ["#0d47a1", "#00acc1", "#4fc3f7"],
  codex: ["#2e7d32", "#00a86b", "#66bb6a"],
  opencode: ["#512da8", "#7e57c2", "#9575cd"],
  opencode1: ["#512da8", "#7e57c2", "#9575cd"],
};
export const LABEL_GRAPHEMES = 24;
const graphemes = new Intl.Segmenter(undefined, { granularity: "grapheme" });

// A label reaches an escape sequence, kitty and tmux, and may come from a
// model: no C0/C1 control (ESC, BEL, CSI, ST), no bidi override, capped by
// graphemes so an emoji is never cut in half.
export function cleanLabel(s?: string): string | undefined {
  if (s === undefined) return undefined;
  const text = s.replace(/\s+/g, " ").replace(/[\x00-\x1f\x7f-\x9f؜‎‏‪-‮⁦-⁩]/g, "").trim();
  const parts = [...graphemes.segment(text)];
  return (parts.length > LABEL_GRAPHEMES ? parts.slice(0, LABEL_GRAPHEMES).map(p => p.segment).join("").trimEnd() : text) || undefined;
}

// The part of a bus name that tells sessions apart: #ab-w1b → ab-w1b,
// #claude/home@node → home.
export function busLabel(name?: string): string | undefined {
  return name ? cleanLabel(name.replace(/^#/, "").replace(/@.*$/, "").replace(/^[^/]*\//, "")) : undefined;
}

// -2..-4 suffix the Claude home (.claude2); anything else is account 1.
export function accountOf(env: NodeJS.ProcessEnv): number {
  const m = /^\.claude([2-4])$/.exec(basename((env.CLAUDE_CONFIG_DIR || "").replace(/\/+$/, "")));
  return m ? Number(m[1]) : 1;
}

export function glyph(runtime: Runtime, account = 1): string {
  const set = glyphs[runtime];
  return set[account - 1] ?? set[0]!;
}

export function terminalTitle(runtime: Runtime, label: string, account = 1): string {
  return `${glyph(runtime, account)} ${label}`;
}

function directoryLabel(cwd: string, home: string): string {
  return cwd === home ? "~" : cwd.split("/").filter(Boolean).at(-1) || "/";
}

// tmux expands formats in a window name, and #(…) runs a shell command; ## is
// a literal #.
const tmuxName = (s: string) => s.replace(/#/g, "##");

// Alive and an ab-* launcher; without /proc, alive is enough.
function liveLauncher(pid: number): boolean {
  if (!Number.isSafeInteger(pid) || pid <= 0 || pid === process.pid) return false;
  try { process.kill(pid, 0); } catch { return false; }
  try { return readFileSync(`/proc/${pid}/cmdline`, "utf8").includes("launcher"); } catch { return true; }
}

// Internal launcher helper, bundled with launcher.js; no command added to PATH.
export class Terminal {
  // A launcher started inside another launcher's session leaves the outer
  // launcher's tab, title and window name alone.
  readonly active = !!process.stdout.isTTY && process.env.TERM !== "dumb" && !liveLauncher(Number(process.env.AGENT_BUS_TITLE_OWNER));
  private closed = false;
  private title = "";
  private kitty = this.active && /^\d+$/.test(process.env.KITTY_WINDOW_ID || "") ? process.env.KITTY_WINDOW_ID : undefined;
  private kittyTitle = "";
  private konsole = this.active && !!process.env.KONSOLE_DBUS_SERVICE;
  private dbus = this.konsole && process.env.KONSOLE_DBUS_SESSION
    ? ["qdbus6", "qdbus", "qdbus-qt5"].find(bin => Bun.which(bin)) : undefined;
  private formats: string[] = [];
  private tmux?: { window: string; name: string; auto: string };
  private env = cleanLabel(process.env.AB_TAB_TITLE);
  private override?: string;
  private bus?: string;
  private name?: string;

  constructor(private runtime: Runtime, private cwd: string, private account = 1, private home = homedir()) {
    if (!this.active) return;
    // Save before the TUI starts; its alternate screen must be gone before restore.
    this.write("\x1b[22;0t");
    if (this.dbus) {
      const formats = [0, 1].map(n => this.session("tabTitleFormat", String(n)));
      if (formats.every(f => f !== undefined)) this.formats = formats as string[];
    }
    if (this.kitty) this.kittyTitle = this.kittyOwnTitle();
    if (this.kitty || this.konsole) {
      const color = this.color();
      const dim = "#" + color.slice(1).match(/../g)!.map(v => Math.floor(parseInt(v, 16) * .6).toString(16).padStart(2, "0")).join("");
      if (this.kitty) this.kittyTab("set-tab-color", `active_bg=${color}`, `inactive_bg=${dim}`, "active_fg=#ffff00", "inactive_fg=#dddddd");
      else this.command("konsoleprofile", [`TabColor=${color}`]);
    }
    this.tmuxClaim();
    this.refresh();
  }

  label(): string { return this.override ?? this.recommended(); }
  recommended(): string { return this.env ?? busLabel(this.bus) ?? cleanLabel(this.name) ?? directoryLabel(this.cwd, this.home); }

  // The session's own name: the default label when there is no bus name.
  set(name?: string): void { this.name = name; this.refresh(); }
  setBus(name?: string): void { this.bus = name; this.refresh(); }
  // A label the session chose (ab_tab_title): the terminal only, for this launcher's life.
  setLabel(text?: string): { current: string; recommended: string } {
    if (text !== undefined) {
      const label = cleanLabel(text);
      if (!label) throw new Error("the label needs printable text");
      this.override = label;
      this.refresh();
    }
    return { current: this.label(), recommended: this.recommended() };
  }

  private refresh(): void {
    if (!this.active || this.closed) return;
    const title = terminalTitle(this.runtime, this.label(), this.account);
    if (title === this.title) return;
    this.title = title;
    this.write(`\x1b]0;${title}\x07`);
    if (this.kitty) this.kittyTab("set-tab-title", title);
    // Konsole ignores the xterm title stack. Save and restore its formats
    // through the caller's session, never the currently focused tab.
    if (this.formats.length) this.session("setTitle", "1", title);
    if (this.tmux) this.command("tmux", ["rename-window", "-t", this.tmux.window, tmuxName(`${this.tmux.name} ${glyph(this.runtime, this.account)}`)]);
  }

  restore(): void {
    if (!this.active || this.closed) return;
    this.closed = true;
    if (this.kitty) {
      this.kittyTab("set-tab-title", this.kittyTitle);
      this.kittyTab("set-tab-color", "active_fg=NONE", "active_bg=NONE", "inactive_fg=NONE", "inactive_bg=NONE");
    } else if (this.konsole) {
      this.command("konsoleprofile", ["TabColor="]);
      for (const [i, format] of this.formats.entries()) this.session("setTabTitleFormat", String(i), format);
    }
    if (this.tmux) this.tmuxRelease(this.tmux);
    // Empty fallback relinquishes the title on terminals without a title stack.
    this.write("\x1b]0;\x07\x1b[23;0t");
  }

  // A tab title set on purpose (kitty launch --tab-title CEO) differs from its
  // window's title and comes back on exit; otherwise the tab follows its window.
  private kittyOwnTitle(): string {
    try {
      type Window = { id: number; title?: string };
      // kitty's ls takes no window match; find this window in the whole listing.
      const ls = JSON.parse(this.command("kitty", ["@", "ls"]) || "[]") as { tabs: { title?: string; windows: Window[] }[] }[];
      for (const tab of ls.flatMap(o => o.tabs)) {
        const window = tab.windows.find(w => String(w.id) === this.kitty);
        if (window) return tab.title && tab.title !== window.title ? tab.title : "";
      }
    } catch { /* unreadable: the tab follows its window */ }
    return "";
  }

  // The window records its name before the launcher renames it, so the next
  // launcher puts back a crashed one's decorated name; a live launcher
  // elsewhere in the window keeps the name.
  private tmuxClaim(): void {
    const pane = process.env.TMUX_PANE || "";
    if (!process.env.TMUX || !/^%\d+$/.test(pane)) return;
    const window = this.command("tmux", ["display-message", "-p", "-t", pane, "#{window_id}"]);
    if (!window || !/^@\d+$/.test(window)) return;
    const option = (name: string) => this.command("tmux", ["show-options", "-wqv", "-t", window, name]) ?? "";
    const owner = option("@ab_title_owner");
    if (owner && liveLauncher(Number(owner.split(":")[0]))) return;
    const name = owner ? option("@ab_title_orig") : this.command("tmux", ["display-message", "-p", "-t", window, "#{window_name}"]) ?? "";
    // The window's own automatic-rename, if set there; empty means inherited.
    const auto = owner ? option("@ab_title_auto") : (this.command("tmux", ["show-options", "-wq", "-t", window, "automatic-rename"]) ?? "").split(" ")[1] ?? "";
    this.command("tmux", ["set-option", "-w", "-t", window, "@ab_title_owner", `${process.pid}:${pane}`]);
    this.command("tmux", ["set-option", "-w", "-t", window, "@ab_title_orig", name]);
    this.command("tmux", ["set-option", "-w", "-t", window, "@ab_title_auto", auto]);
    this.tmux = { window, name, auto };
  }
  private tmuxRelease({ window, name, auto }: { window: string; name: string; auto: string }): void {
    this.command("tmux", ["rename-window", "-t", window, tmuxName(name)]);
    this.command("tmux", auto ? ["set-option", "-w", "-t", window, "automatic-rename", auto] : ["set-option", "-uw", "-t", window, "automatic-rename"]);
    for (const option of ["@ab_title_owner", "@ab_title_orig", "@ab_title_auto"]) this.command("tmux", ["set-option", "-uw", "-t", window, option]);
  }

  private write(text: string): void { process.stdout.write(text); }
  private command(bin: string, args: string[]): string | undefined {
    try {
      const result = spawnSync(bin, args, { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"], timeout: 500 });
      return result.status === 0 ? result.stdout.replace(/\n$/, "") : undefined;
    } catch { return undefined; } // Optional styling must never prevent startup/cleanup.
  }
  private kittyTab(command: string, ...args: string[]): void {
    this.command("kitty", ["@", command, "--match", `window_id:${this.kitty}`, ...args]);
  }
  private session(method: string, ...args: string[]): string | undefined {
    return this.command(this.dbus!, [process.env.KONSOLE_DBUS_SERVICE!, process.env.KONSOLE_DBUS_SESSION!, `org.kde.konsole.Session.${method}`, ...args]);
  }
  private color(): string {
    const colors = palettes[this.runtime];
    let index = 0;
    try {
      const dir = join(process.env.XDG_CACHE_HOME || join(homedir(), ".cache"), "agent-bus/terminal");
      mkdirSync(dir, { recursive: true, mode: 0o700 });
      const file = join(dir, this.runtime);
      try { index = Number(readFileSync(file, "utf8")); } catch { /* first launch */ }
      if (!Number.isSafeInteger(index) || index < 0) index = 0;
      writeState(file, String((index + 1) % colors.length));
    } catch { /* Read-only home: use the first shade. */ }
    return colors[index % colors.length]!;
  }
}
