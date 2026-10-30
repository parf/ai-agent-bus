import { expect, test } from "bun:test";
import { mkdirSync, mkdtempSync, readFileSync, rmSync, statSync, existsSync } from "node:fs";
import { join, resolve } from "node:path";
import { spawnSync } from "node:child_process";

// package.sh makes the agent.d directory a runner serves: a bundle that
// imports nothing outside it, its worker beside it, and an agent.json naming
// the bundle — the sandbox mounts that directory alone (runner wave M.9).
test("the package is self-contained and names its bundle", () => {
  const scratch = join(resolve(import.meta.dir, "../../.."), "tmp");
  mkdirSync(scratch, { recursive: true });
  const parent = mkdtempSync(join(scratch, "slackpkg-"));
  const dest = join(parent, "slack");
  try {
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
    // Nor reads a file of the checkout's: the version is written in.
    expect(bundle).not.toContain("internal/version/VERSION");
    expect(bundle).toContain("class Subscriptions");
    expect(bundle).toContain("admission_refused");
    expect(bundle).toContain("filter_invalid");
    expect(existsSync(join(dest, "filter-worker.js"))).toBe(false);
    expect(statSync(dest).mode & 0o777).toBe(0o755);
  } finally {
    rmSync(parent, { recursive: true, force: true });
  }
});
