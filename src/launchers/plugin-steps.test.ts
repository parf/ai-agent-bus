import { expect, test } from "bun:test";
import { pluginSteps } from "./plugin-steps.ts";

const add = (p: string) => ["marketplace", "add", p];
const install = ["install", "agent-bus@agent-bus-local", "--scope", "user"];
test("a fresh Claude home installs the current marketplace and plugin", () => {
  expect(pluginSteps({source: "/current", installed: false})).toEqual([add("/current"), install]);
});

test("a different or unreadable marketplace is refused without conversion", () => {
  for (const installed of [false, true]) for (const knownPath of ["/previous", ""])
    expect(() => pluginSteps({source: "/current", knownPath, installed})).toThrow("clean installation");
  expect(() => pluginSteps({source: "/current", installed: true})).toThrow("clean installation");
});

test("the current marketplace resumes or finishes its own setup", () => {
  expect(pluginSteps({source: "/current", knownPath: "/current", installed: true})).toEqual([]);
  expect(pluginSteps({source: "/current", knownPath: "/current", installed: false})).toEqual([install]);
});

test("two launchers at once change the plugin one after the other", async () => {
  const { mkdtempSync, readFileSync, rmSync } = await import("node:fs");
  const { join } = await import("node:path");
  const dir = mkdtempSync(join((await import("node:os")).tmpdir(), "agent-bus-plugin-lock-"));
  try {
    const log = join(dir, "log");
    // Each holder writes begin, waits, writes end: overlapping holders interleave.
    const child = (n: number) => Bun.spawn([process.execPath, "-e",
      `import { withPluginLock } from ${JSON.stringify(join(import.meta.dir, "plugin-steps.ts"))};
       import { appendFileSync } from "node:fs";
       withPluginLock(${JSON.stringify(dir)}, () => { appendFileSync(${JSON.stringify(log)}, "begin ${n}\\n"); Bun.sleepSync(300); appendFileSync(${JSON.stringify(log)}, "end ${n}\\n"); });`]);
    const runs = [child(1), child(2)];
    expect(await Promise.all(runs.map(r => r.exited))).toEqual([0, 0]);
    const lines = readFileSync(log, "utf8").trim().split("\n");
    expect(lines.length).toBe(4);
    expect(lines[0]!.split(" ")[1]).toBe(lines[1]!.split(" ")[1]); // the first holder ends before the second begins
    expect(lines.map(l => l.split(" ")[0])).toEqual(["begin", "end", "begin", "end"]);
  } finally { rmSync(dir, { recursive: true, force: true }); }
});

test("a lock whose holder has exited is taken over; a live holder's is not", async () => {
  const { mkdtempSync, writeFileSync, existsSync, readFileSync, rmSync } = await import("node:fs");
  const { join } = await import("node:path");
  const { withPluginLock } = await import("./plugin-steps.ts");
  const dir = mkdtempSync(join((await import("node:os")).tmpdir(), "agent-bus-plugin-lock-"));
  try {
    const lock = join(dir, ".agent-bus-plugin.lock");
    const { readlinkSync, utimesSync } = await import("node:fs");
    const space = readlinkSync("/proc/self/ns/pid");
    // A live holder (this test's own pid) is waited for, never taken over.
    writeFileSync(lock, `${process.pid} ${space} live`);
    expect(() => withPluginLock(dir, () => 1, 150)).toThrow(/another launcher holds/);
    expect(readFileSync(lock, "utf8")).toBe(`${process.pid} ${space} live`);
    // A holder in another pid namespace cannot be judged, even by a dead pid here.
    const gone = Bun.spawnSync([process.execPath, "-e", "console.log(process.pid)"]);
    const dead = String(gone.stdout).trim();
    writeFileSync(lock, `${dead} pid:[1] elsewhere`);
    expect(() => withPluginLock(dir, () => 1, 150)).toThrow(/another launcher holds/);
    // A fresh empty lock is a holder mid-create; an old one is gone.
    writeFileSync(lock, "");
    expect(() => withPluginLock(dir, () => 1, 150)).toThrow(/another launcher holds/);
    utimesSync(lock, new Date(Date.now() - 60_000), new Date(Date.now() - 60_000));
    expect(withPluginLock(dir, () => 5, 2_000)).toBe(5);
    // An exited holder's lock is taken over, and ours is removed after.
    writeFileSync(lock, `${dead} ${space} gone`);
    expect(withPluginLock(dir, () => 7, 2_000)).toBe(7);
    expect(existsSync(lock)).toBe(false);
  } finally { rmSync(dir, { recursive: true, force: true }); }
});
