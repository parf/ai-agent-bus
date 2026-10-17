import { expect, test } from "bun:test";
import { mkdtempSync, readFileSync, statSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawnSync } from "node:child_process";

// package.sh makes the agent.d directory a runner serves: a bundle that
// imports nothing outside it, its worker beside it, and an agent.json naming
// the bundle — the sandbox mounts that directory alone (runner wave M.9).
test("the package is self-contained and names its bundle", () => {
  const dest = join(mkdtempSync(join(tmpdir(), "slackpkg-")), "slack");
  const run = spawnSync(join(import.meta.dir, "package.sh"), [dest], { encoding: "utf8" });
  expect(run.status).toBe(0);
  const agent = JSON.parse(readFileSync(join(dest, "agent.json"), "utf8"));
  expect(agent.command).toBe("./slack.js");
  expect(agent.serves).toBe("itself");
  const bundle = readFileSync(join(dest, "slack.js"), "utf8");
  expect(bundle.startsWith("#!/usr/bin/env bun\n")).toBe(true);
  expect(bundle.split("\n").filter(l => l.startsWith("#!")).length).toBe(1);
  // Nothing reaches outside the directory: no relative import climbs out.
  expect(bundle).not.toMatch(/from\s+["']\.\.\//);
  expect(bundle).toContain('new URL("./filter-worker" + ext');
  expect(statSync(join(dest, "filter-worker.js")).isFile()).toBe(true);
  expect(statSync(dest).mode & 0o777).toBe(0o755);
});
