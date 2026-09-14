import { statSync } from "node:fs";
import { homedir, userInfo } from "node:os";
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

// Prefer a per-login daemon, then the installed daemon. Explicit addresses
// always win, including failures: never silently connect to a different bus.
export function localAddress(env: NodeJS.ProcessEnv, systemDir = "/run/agent-bus"): string | undefined {
  if (env.AGENT_BUS_ADDR) return env.AGENT_BUS_ADDR;
  const dirs = [...(env.XDG_RUNTIME_DIR ? [join(env.XDG_RUNTIME_DIR, "agent-bus")] : []), systemDir];
  const filename = env.AGENT_BUS_TOKEN ? "bus.sock" : `user-${userInfo().username}.sock`;
  return dirs.map(dir => join(dir, filename)).find(path => {
    try { return statSync(path).isSocket(); } catch { return false; }
  });
}
