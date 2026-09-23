// A launcher's MCP face runs under the runtime, not under the launcher, so the
// launcher cannot wait for it. Each face leaves a mark in the launcher's private
// run directory and removes it only when its runtime lets it go. A mark whose
// process is gone is a face that died on its own: the session has lost its bus
// tools, and the launcher says so and recovers
// (docs/08-runner-role.md#runtime-isolation-and-recovery).
import { readdirSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";

const MARK = /^face-(\d+)$/;

/** Marks this face as running; the returned function clears it on a let-go. */
export function markFace(sessionFile: string | undefined, pid = process.pid): () => void {
  if (!sessionFile) return () => {};
  const mark = join(dirname(sessionFile), `face-${pid}`);
  writeFileSync(mark, "", { mode: 0o600 });
  return () => rmSync(mark, { force: true });
}

function alive(pid: number): boolean {
  try { process.kill(pid, 0); return true; }
  catch (e: any) { return e.code === "EPERM"; }
}

/** Faces still running, and faces that died without being let go. Lost marks
 *  are removed, so each loss is reported once. */
export function faceMarks(runDir: string): { live: number; lost: number } {
  let live = 0, lost = 0;
  let names: string[] = [];
  try { names = readdirSync(runDir); } catch { return { live, lost }; }
  for (const name of names) {
    const pid = Number(name.match(MARK)?.[1] ?? 0);
    if (!pid) continue;
    if (alive(pid)) { live++; continue; }
    lost++;
    rmSync(join(runDir, name), { force: true });
  }
  return { live, lost };
}
