import { randomUUID } from "node:crypto";
import { closeSync, linkSync, mkdirSync, openSync, readFileSync, renameSync, rmSync, writeSync } from "node:fs";
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

/** Runs fn holding `<dir>/.agent-bus-plugin.lock`, waiting up to waitMs.
 *  The file names its holder's pid; a holder that is gone left it behind, and
 *  one waiter takes it over by renaming exactly the file it found. */
export function withPluginLock<T>(dir: string, fn: () => T, waitMs = 180_000): T {
  mkdirSync(dir, { recursive: true });
  const lock = join(dir, ".agent-bus-plugin.lock");
  const mine = `${process.pid} ${randomUUID()}`;
  const end = Date.now() + waitMs;
  for (;;) {
    try {
      const fd = openSync(lock, "wx");
      try { writeSync(fd, mine); } finally { closeSync(fd); }
      break;
    } catch (err) {
      if ((err as NodeJS.ErrnoException).code !== "EEXIST") throw err;
      takeOverIfGone(lock);
      if (Date.now() > end) throw new Error(`another launcher holds ${lock}; remove it if none is running`);
      Bun.sleepSync(50);
    }
  }
  try { return fn(); } finally {
    // Only our own: never remove a lock somebody took over meanwhile.
    try { if (readFileSync(lock, "utf8") === mine) rmSync(lock, { force: true }); } catch {}
  }
}

const alive = (pid: number) => { try { process.kill(pid, 0); return true; } catch (err) { return (err as NodeJS.ErrnoException).code === "EPERM"; } };

/** Removes the lock when the holder it names has exited. Racing waiters each
 *  rename the path to their own name; whoever got a file other than the one
 *  it judged gone puts it back with link, which never replaces a new lock. */
function takeOverIfGone(lock: string): void {
  let seen: string;
  try { seen = readFileSync(lock, "utf8"); } catch { return; }
  const pid = Number(seen.split(" ")[0]);
  // A holder between create and write leaves it empty for a moment: not gone.
  if (!seen || !Number.isSafeInteger(pid) || pid <= 0 || alive(pid)) return;
  const taken = `${lock}.${process.pid}.${randomUUID()}`;
  try { renameSync(lock, taken); } catch { return; }
  try {
    if (readFileSync(taken, "utf8") !== seen) {
      try { linkSync(taken, lock); } catch {}
    }
  } finally { rmSync(taken, { force: true }); }
}
