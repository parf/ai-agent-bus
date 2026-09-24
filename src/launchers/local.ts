import { spawnSync } from "node:child_process";
import { statSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";

// Shell functions are invisible to Bun.which. Prefer OpenCode's user wrapper
// so a system binary earlier in PATH does not bypass its runtime settings.
export function runtimeBinary(runtime: string, env: NodeJS.ProcessEnv, home = homedir()): string | null {
  const override = env[`${runtime.toUpperCase()}_BIN`];
  if (override) return Bun.which(override, { PATH: env.PATH });
  if (runtime === "opencode") {
    const local = Bun.which(join(home, ".local/bin/opencode"));
    if (local) return local;
  }
  return Bun.which(runtime, { PATH: env.PATH });
}

// The account this process runs as, from its uid, as the daemon names its
// socket and the Go CLI finds it. Bun's userInfo() reports $USER instead, or
// "unknown" without it, and a caller may unset or change that at will.
export function accountName(uid = process.getuid!()): string | undefined {
  const found = Bun.spawnSync(["id", "-nu", String(uid)], { stdout: "pipe", stderr: "ignore" });
  return (found.exitCode === 0 && found.stdout.toString().trim()) || undefined;
}

// Prefer a per-login daemon, then the installed daemon. Explicit addresses
// always win, including failures: never silently connect to a different bus.
export function localAddress(env: NodeJS.ProcessEnv, systemDir = "/run/agent-bus"): string | undefined {
  if (env.AGENT_BUS_ADDR) return env.AGENT_BUS_ADDR;
  const dirs = [...(env.XDG_RUNTIME_DIR ? [join(env.XDG_RUNTIME_DIR, "agent-bus")] : []), systemDir];
  const account = env.AGENT_BUS_TOKEN ? "" : accountName();
  if (account === undefined) return undefined;
  const filename = env.AGENT_BUS_TOKEN ? "bus.sock" : `user-${account}.sock`;
  return dirs.map(dir => join(dir, filename)).find(path => {
    try { return statSync(path).isSocket(); } catch { return false; }
  });
}

// Claude keeps a directory's local MCP servers under the project it counts the
// directory in, which is the enclosing Git repository when there is one: a
// server added in /rd/tmp inside the repository /rd is written under "/rd".
export function gitRoot(cwd: string): string | undefined {
  const r = spawnSync("git", ["rev-parse", "--show-toplevel"], { cwd, encoding: "utf8" });
  return r.status === 0 ? r.stdout.trim() || undefined : undefined;
}

/** Whether Claude's configuration names the agent-bus server for cwd. */
export function channelRegistered(config: any, cwd: string, root = gitRoot(cwd)): boolean {
  return [cwd, root].some(key => !!key && !!config?.projects?.[key]?.mcpServers?.["agent-bus"]);
}
