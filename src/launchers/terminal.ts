import { spawnSync } from "node:child_process";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";

type Runtime = "claude" | "codex" | "opencode";
const names = { claude: "Claude", codex: "Codex", opencode: "OpenCode" };
const palettes = {
  claude: ["#0d47a1", "#00acc1", "#4fc3f7"],
  codex: ["#2e7d32", "#00a86b", "#66bb6a"],
  opencode: ["#512da8", "#7e57c2", "#9575cd"],
};
const safe = (s: string) => s.replace(/[\x00-\x1f\x7f-\x9f]/g, "");

export function terminalTitle(runtime: Runtime, cwd: string, name?: string, home = homedir()): string {
  const path = cwd === home ? "~" : cwd.startsWith(home + "/") ? "~" + cwd.slice(home.length) : cwd;
  return `${names[runtime]}(${safe(name?.trim() || path.split("/").filter(Boolean).slice(-2).join("/") || "/")})`;
}

// Internal launcher helper, bundled with launcher.js; no command added to PATH.
export class Terminal {
  readonly active = !!process.stdout.isTTY && process.env.TERM !== "dumb";
  private closed = false;
  private title = "";
  private kitty = this.active && /^\d+$/.test(process.env.KITTY_WINDOW_ID || "") ? process.env.KITTY_WINDOW_ID : undefined;
  private konsole = this.active && !!process.env.KONSOLE_DBUS_SERVICE;
  private dbus = this.konsole && process.env.KONSOLE_DBUS_SESSION
    ? ["qdbus6", "qdbus", "qdbus-qt5"].find(bin => Bun.which(bin)) : undefined;
  private formats: string[] = [];

  constructor(private runtime: Runtime, private cwd: string) {
    if (!this.active) return;
    // Save before the TUI starts; its alternate screen must be gone before restore.
    this.write("\x1b[22;0t");
    if (this.dbus) {
      const formats = [0, 1].map(n => this.session("tabTitleFormat", String(n)));
      if (formats.every(f => f !== undefined)) this.formats = formats as string[];
    }
    if (this.kitty || this.konsole) {
      const color = this.color();
      const dim = "#" + color.slice(1).match(/../g)!.map(v => Math.floor(parseInt(v, 16) * .6).toString(16).padStart(2, "0")).join("");
      if (this.kitty) this.kittyTab("set-tab-color", `active_bg=${color}`, `inactive_bg=${dim}`, "active_fg=#ffff00", "inactive_fg=#dddddd");
      else this.command("konsoleprofile", [`TabColor=${color}`]);
    }
    this.set();
  }

  set(name?: string): void {
    if (!this.active || this.closed) return;
    const title = terminalTitle(this.runtime, this.cwd, name);
    if (title === this.title) return;
    this.title = title;
    this.write(`\x1b]0;${title}\x07`);
    if (this.kitty) this.kittyTab("set-tab-title", title);
    // Konsole ignores the xterm title stack. Save and restore its formats
    // through the caller's session, never the currently focused tab.
    if (this.formats.length) this.session("setTitle", "1", title);
  }

  restore(): void {
    if (!this.active || this.closed) return;
    this.closed = true;
    if (this.kitty) {
      this.kittyTab("set-tab-title", "");
      this.kittyTab("set-tab-color", "active_fg=NONE", "active_bg=NONE", "inactive_fg=NONE", "inactive_bg=NONE");
    } else if (this.konsole) {
      this.command("konsoleprofile", ["TabColor="]);
      for (const [i, format] of this.formats.entries()) this.session("setTabTitleFormat", String(i), format);
    }
    // Empty fallback relinquishes the title on terminals without a title stack.
    this.write("\x1b]0;\x07\x1b[23;0t");
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
      writeFileSync(file, String((index + 1) % colors.length), { mode: 0o600 });
    } catch { /* Read-only home: use the first shade. */ }
    return colors[index % colors.length]!;
  }
}
