import { expect, test } from "bun:test";
import { existsSync, mkdirSync, mkdtempSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { alive, faceEvent, faceMarks, markFace, holders, evict, startTime } from "./face-mark.ts";

const scratch = () => mkdtempSync(join(resolve(import.meta.dir, "../../tmp"), "face-mark-"));

test("a face that is let go leaves no mark; a running one counts as live", () => {
  const dir = scratch();
  try {
    const unmark = markFace(join(dir, "bus-env.json"));
    expect(faceMarks(dir)).toEqual({ live: 1, lost: 0 });
    unmark();
    expect(faceMarks(dir)).toEqual({ live: 0, lost: 0 });
  } finally { rmSync(dir, { recursive: true, force: true }); }
});

test("a face that died on its own is reported once, then forgotten", async () => {
  const dir = scratch();
  try {
    // A real process that is gone: its pid was ours to see and is now free.
    const child = Bun.spawn(["true"]);
    await child.exited;
    markFace(join(dir, "bus-env.json"), child.pid);
    writeFileSync(join(dir, "bus-env.json"), "{}"); // not a mark
    const live = markFace(join(dir, "bus-env.json"));
    expect(faceMarks(dir)).toEqual({ live: 1, lost: 1 });
    expect(existsSync(join(dir, `face-${child.pid}`))).toBe(false);
    expect(faceMarks(dir)).toEqual({ live: 1, lost: 0 });
    live();
    expect(readdirSync(dir)).toEqual(["bus-env.json"]);
  } finally { rmSync(dir, { recursive: true, force: true }); }
});

test("without a launcher session file there is nothing to mark", () => {
  expect(markFace(undefined)()).toBeUndefined();
});

test("a zombie face is dead, whatever kill(0) says", () => {
  expect(alive(process.pid, () => `${process.pid} (bun) Z 1 2 3`)).toBe(false);
  expect(alive(process.pid, () => `${process.pid} (x) Z) S 1 2 3`)).toBe(true); // a name with ") Z" in it: the state is after the last ")"
  expect(alive(process.pid, () => { throw new Error("no /proc"); })).toBe(true);
});

test("a lost face beside a live one is recovery, not a loss to act on", () => {
  expect(faceEvent(false, { live: 0, lost: 1 })).toBe("lost");
  expect(faceEvent(false, { live: 1, lost: 1 })).toBe("back"); // replaced inside one poll
  expect(faceEvent(true, { live: 1, lost: 0 })).toBe("back");
  expect(faceEvent(true, { live: 0, lost: 0 })).toBe("none");
  expect(faceEvent(false, { live: 1, lost: 0 })).toBe("none");
});

// A run directory as the launcher lays it out: <runs>/<claude-X>/bus-env.json
// plus a face-<pid> mark. Two launchers of one name sit side by side.
const run = (runs: string, dir: string, name: string) => {
  const d = join(runs, dir);
  mkdirSync(d, { recursive: true });
  writeFileSync(join(d, "bus-env.json"), JSON.stringify({ AGENT_BUS_NAME: name }));
  return join(d, "bus-env.json");
};

test("holders names a live face of the same name, here or in a sibling launcher, and not itself", () => {
  const runs = scratch();
  try {
    const mine = run(runs, "claude-A", "#x@h"), other = run(runs, "claude-B", "#x@h"), foreign = run(runs, "claude-C", "#y@h");
    const unmark = markFace(mine);
    expect(holders(mine, "#x@h")).toEqual({ pids: [], unverified: 0 }); // alone: nobody else holds it
    const child = Bun.spawn(["sleep", "30"]);
    try {
      markFaceAs(other, child.pid);
      markFaceAs(foreign, child.pid);
      expect(holders(mine, "#x@h")).toEqual({ pids: [child.pid], unverified: 0 });
      expect(holders(mine, "#z@h")).toEqual({ pids: [], unverified: 0 }); // another name is not this holder
    } finally { child.kill(); await_(child); }
    unmark();
  } finally { rmSync(runs, { recursive: true, force: true }); }
});

// The mark outlives its process and the pid is later reused. Signalling it
// would terminate somebody else (SIGUSR2's default action), so only a mark
// whose start time still matches is a face.
test("a recycled pid is never signalled", async () => {
  const runs = scratch();
  try {
    const mine = run(runs, "claude-A", "#x@h"), other = run(runs, "claude-B", "#x@h");
    const child = Bun.spawn(["sleep", "30"]);
    try {
      writeFileSync(join(runs, "claude-B", `face-${child.pid}`), "1"); // written by a face that is gone: wrong start time
      const found = holders(mine, "#x@h");
      expect(found).toEqual({ pids: [], unverified: 1 });
      expect(evict(found, mine, "#x@h")).toEqual([]);
      await Bun.sleep(50);
      expect(child.killed || child.exitCode !== null).toBe(false); // still running: nothing was sent
    } finally { child.kill(); }
  } finally { rmSync(runs, { recursive: true, force: true }); }
});

test("evict signals a verified holder", async () => {
  const runs = scratch();
  try {
    const mine = run(runs, "claude-A", "#x@h"), other = run(runs, "claude-B", "#x@h");
    const child = Bun.spawn(["sleep", "30"]);
    markFaceAs(other, child.pid);
    const found = holders(mine, "#x@h");
    expect(evict(found, mine, "#x@h", "SIGTERM")).toEqual([child.pid]);
    expect(await Promise.race([child.exited, Bun.sleep(2000).then(() => "alive")])).not.toBe("alive");
  } finally { rmSync(runs, { recursive: true, force: true }); }
});

function markFaceAs(file: string, pid: number) {
  writeFileSync(join(dirname(file), `face-${pid}`), startTime(pid));
}
function await_(p: { kill: () => void }) { p.kill(); }
