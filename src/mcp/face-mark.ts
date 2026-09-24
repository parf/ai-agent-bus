// A launcher's MCP face runs under the runtime, not under the launcher, so the
// launcher cannot wait for it. Each face leaves a mark in the launcher's private
// run directory and removes it only when its runtime lets it go. A mark whose
// process is gone is a face that died on its own: the session has lost its bus
// tools, and the launcher says so and recovers
// (docs/08-runner-role.md#runtime-isolation-and-recovery).
import { readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";

const MARK = /^face-(\d+)$/;

/** Marks this face as running; the returned function clears it on a let-go. */
export function markFace(sessionFile: string | undefined, pid = process.pid): () => void {
  if (!sessionFile) return () => {};
  const mark = join(dirname(sessionFile), `face-${pid}`);
  writeFileSync(mark, "", { mode: 0o600 });
  return () => rmSync(mark, { force: true });
}

// A zombie still answers kill(0) until its runtime reaps it, so its state is
// asked too: "Z" is a face that has already died.
export function alive(pid: number, stat = (p: number) => readFileSync(`/proc/${p}/stat`, "utf8")): boolean {
  try { process.kill(pid, 0); }
  catch (e: any) { if (e.code !== "EPERM") return false; }
  let line: string;
  try { line = stat(pid); } catch { return true; } // no /proc: kill(0) is the answer
  return line[line.lastIndexOf(")") + 2] !== "Z"; // the state follows the name
}

/** What a watch of the marks means for the launcher. A lost face with a live
 *  one beside it was already replaced, so it is recovery, not a loss to act
 *  on: acting would reload the healthy replacement. */
export function faceEvent(down: boolean, marks: { live: number; lost: number }): "lost" | "back" | "none" {
  if (marks.lost && !marks.live) return "lost";
  if ((down || marks.lost) && marks.live) return "back";
  return "none";
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
