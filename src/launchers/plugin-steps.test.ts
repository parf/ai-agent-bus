import { expect, test } from "bun:test";
import { pluginSteps } from "./plugin-steps.ts";

const add = (p: string) => ["marketplace", "add", p];
const install = ["install", "agent-bus@agent-bus-local", "--scope", "user"];
const uninstall = ["uninstall", "agent-bus@agent-bus-local"];
const remove = ["marketplace", "remove", "agent-bus-local"];

test("a fresh Claude home gets the marketplace and the plugin, compiled or not", () => {
  for (const compiled of [true, false]) expect(pluginSteps({ compiled, source: "/r/1", installed: false })).toEqual([add("/r/1"), install]);
});

test("a release replaces a plugin another release or a checkout installed", () => {
  expect(pluginSteps({ compiled: true, source: "/rel/0.9.164", knownPath: "/rel/0.9.163", installed: true })).toEqual([uninstall, remove, add("/rel/0.9.164"), install]);
  expect(pluginSteps({ compiled: true, source: "/rel/0.9.163", knownPath: "/usr/local/src/ab/src", installed: true })).toEqual([uninstall, remove, add("/rel/0.9.163"), install]);
  expect(pluginSteps({ compiled: true, source: "/rel/0.9.164", knownPath: "/rel/0.9.163", installed: false })).toEqual([remove, add("/rel/0.9.164"), install]);
});

test("the same release, and any checkout, leave an installed plugin alone", () => {
  expect(pluginSteps({ compiled: true, source: "/rel/0.9.164", knownPath: "/rel/0.9.164", installed: true })).toEqual([]);
  expect(pluginSteps({ compiled: false, source: "/tmp/worktree/src", knownPath: "/usr/local/src/ab/src", installed: true })).toEqual([]);
  expect(pluginSteps({ compiled: false, source: "/usr/local/src/ab/src", knownPath: "/usr/local/src/ab/src", installed: false })).toEqual([install]);
});

test("a recorded marketplace whose path cannot be read is never re-pointed", () => {
  expect(pluginSteps({ compiled: true, source: "/rel/0.9.164", knownPath: "", installed: true })).toEqual([]);
  expect(pluginSteps({ compiled: true, source: "/rel/0.9.164", knownPath: "", installed: false })).toEqual([install]);
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

test("a lock its holder left behind is taken over once stale", async () => {
  const { mkdtempSync, writeFileSync, utimesSync, existsSync, rmSync } = await import("node:fs");
  const { join } = await import("node:path");
  const { withPluginLock } = await import("./plugin-steps.ts");
  const dir = mkdtempSync(join((await import("node:os")).tmpdir(), "agent-bus-plugin-lock-"));
  try {
    const lock = join(dir, ".agent-bus-plugin.lock");
    writeFileSync(lock, "");
    expect(() => withPluginLock(dir, () => 1, 150, 60_000)).toThrow(/another launcher holds/);
    utimesSync(lock, new Date(Date.now() - 120_000), new Date(Date.now() - 120_000));
    expect(withPluginLock(dir, () => 7, 2_000, 60_000)).toBe(7);
    expect(existsSync(lock)).toBe(false);
  } finally { rmSync(dir, { recursive: true, force: true }); }
});
