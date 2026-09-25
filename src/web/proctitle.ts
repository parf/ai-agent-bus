// The ps line, like the Go programs' (docs/11-processes.md#process-titles).
// Bun's process.title leaves /proc/<pid>/cmdline alone, so the title is written
// over the process's own argv area, whose bounds /proc/self/stat names. The
// title is cut to that area; the unit's argv is long enough for it.
import { openSync, readFileSync, writeSync } from "node:fs";

export function titleArea(stat: string): { start: number; size: number } | null {
  // Fields after "(comm) " are numbered from state (3); arg_start is 48, arg_end 49.
  const f = stat.slice(stat.lastIndexOf(")") + 2).trim().split(" ");
  const start = Number(f[45]), end = Number(f[46]);
  return Number.isSafeInteger(start) && Number.isSafeInteger(end) && end > start + 1 ? { start, size: end - start } : null;
}

export function titleBytes(title: string, size: number): Uint8Array {
  const buf = new Uint8Array(size); // zero fill: no stale argv shows after the title
  buf.set(new TextEncoder().encode(title).subarray(0, size - 1));
  return buf;
}

// start owns the title until stop is called; calls() is read once a second.
export function start(name: string, version: string, calls: () => number): () => void {
  let fd: number, area: { start: number; size: number } | null;
  try {
    area = titleArea(readFileSync("/proc/self/stat", "utf8"));
    if (!area) return () => {};
    fd = openSync("/proc/self/mem", "r+");
  } catch {
    return () => {}; // not Linux, or /proc/self/mem refused: ps keeps the command line
  }
  let shown = -1;
  const show = () => {
    const n = calls();
    if (n === shown) return;
    shown = n;
    try { writeSync(fd, titleBytes(`${name} ${version} ; Calls: ${n}`, area!.size), 0, area!.size, area!.start); }
    catch { clearInterval(tick); }
  };
  show();
  const tick = setInterval(show, 1000);
  tick.unref();
  return () => clearInterval(tick);
}
