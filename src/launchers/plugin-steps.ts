import { closeSync, mkdirSync, openSync, rmSync, statSync } from "node:fs";
import { join } from "node:path";

// Which `claude plugin` commands bring Claude's copy of the agent-bus channel
// plugin to this launcher's own tree. Claude keeps its own copy: a release's
// directory changes with every upgrade and rollback, so a release replaces a
// plugin another release installed (an older one's bun entry, or a newer one's
// agent-bus-mcp); a checkout keeps whatever plugin is installed, as before.
// knownPath: undefined when no marketplace is recorded, "" when one is but its
// path cannot be read — then nothing is re-pointed, never every launch.
export type PluginState = { compiled: boolean; source: string; knownPath?: string; installed: boolean };

export function pluginSteps({ compiled, source, knownPath, installed }: PluginState): string[][] {
  const known = knownPath !== undefined;
  const stale = compiled && known && knownPath !== "" && knownPath !== source;
  if (installed && !stale) return [];
  const steps: string[][] = [];
  if (stale) {
    if (installed) steps.push(["uninstall", "agent-bus@agent-bus-local"]);
    steps.push(["marketplace", "remove", "agent-bus-local"]);
  }
  if (!known || stale) steps.push(["marketplace", "add", source]);
  steps.push(["install", "agent-bus@agent-bus-local", "--scope", "user"]);
  return steps;
}

/** A step whose failure means only that what it removes is already gone. */
export const removal = (step: string[]) => step[0] === "uninstall" || (step[0] === "marketplace" && step[1] === "remove");

/** Runs fn holding `<dir>/.agent-bus-plugin.lock`, waiting up to waitMs. A
 *  file older than staleMs is a holder that died, and is taken over. */
export function withPluginLock<T>(dir: string, fn: () => T, waitMs = 180_000, staleMs = 300_000): T {
  mkdirSync(dir, { recursive: true });
  const lock = join(dir, ".agent-bus-plugin.lock");
  const end = Date.now() + waitMs;
  for (;;) {
    try { closeSync(openSync(lock, "wx")); break; } catch (err) {
      if ((err as NodeJS.ErrnoException).code !== "EEXIST") throw err;
      try { if (Date.now() - statSync(lock).mtimeMs > staleMs) rmSync(lock, { force: true }); } catch {}
      if (Date.now() > end) throw new Error(`another launcher holds ${lock}; remove it if none is running`);
      Bun.sleepSync(50);
    }
  }
  try { return fn(); } finally { rmSync(lock, { force: true }); }
}
