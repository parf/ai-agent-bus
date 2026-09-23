import { expect, test } from "bun:test";
import { existsSync, mkdtempSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { faceMarks, markFace } from "./face-mark.ts";

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
