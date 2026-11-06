import { expect, test } from "bun:test";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";

// J13: without a bus, a restart after the first session's transcript exists
// continues that conversation instead of starting a new one.
test("no-bus Claude restart continues the transcript the crashed start wrote", async () => {
  const root = resolve(import.meta.dir, "../../tmp/no-bus-tests");
  mkdirSync(root, { recursive: true });
  const dir = mkdtempSync(join(root, "run-"));
  try {
    const home = join(dir, "claude"), cwd = join(dir, "work"), calls = join(dir, "calls");
    mkdirSync(cwd);
    const project = join(home, "projects", cwd.replace(/[^a-zA-Z0-9]/g, "-"));
    // The fake runtime records its arguments; the first start writes a transcript and crashes.
    const fake = join(dir, "claude-fake");
    writeFileSync(fake, `#!${process.execPath}
import { appendFileSync, existsSync, mkdirSync, writeFileSync } from "node:fs";
const args = process.argv.slice(2);
if (args[0] === "plugin") { // the launcher's plugin setup, as the runtime fixture answers it
  const plugins = process.env.CLAUDE_CONFIG_DIR + "/plugins";
  mkdirSync(plugins, { recursive: true });
  if (args[1] === "marketplace") writeFileSync(plugins + "/known_marketplaces.json", JSON.stringify({ "agent-bus-local": { source: { source: "directory", path: args[3] } } }));
  else writeFileSync(plugins + "/installed_plugins.json", JSON.stringify({ plugins: { "agent-bus@agent-bus-local": [{ scope: "user" }] } }));
  process.exit(0);
}
const first = !existsSync(process.env.TEST_CALLS);
appendFileSync(process.env.TEST_CALLS, JSON.stringify(args) + "\\n");
if (first) {
  mkdirSync(process.env.TEST_PROJECT, { recursive: true });
  writeFileSync(process.env.TEST_PROJECT + "/11111111-1111-4111-8111-111111111111.jsonl", JSON.stringify({ type: "user", message: { content: "hi" } }) + "\\n");
  process.exit(3);
}
process.exit(0);
`, { mode: 0o700 });
    // No bus: no token or address, and an `id` that names no account, so the
    // launcher cannot find this host's own socket and register on a live bus.
    const bin = join(dir, "bin");
    mkdirSync(bin);
    writeFileSync(join(bin, "id"), "#!/bin/sh\nexit 1\n", { mode: 0o700 });
    const env: Record<string, string> = { ...process.env as Record<string, string>, AGENT_BUS_TOKEN: "", AGENT_BUS_ADDR: "", AGENT_BUS_NAME: "",
      AB_TAB_TITLE: "", AGENT_BUS_TITLE_OWNER: "", CLAUDE_BIN: fake, CLAUDE_CONFIG_DIR: home, XDG_STATE_HOME: join(dir, "state"),
      TEST_CALLS: calls, TEST_PROJECT: project, PATH: `${bin}:${process.env.PATH}` };
    const proc = Bun.spawn([process.execPath, join(import.meta.dir, "launcher.ts"), "claude", "--restart=1"], { cwd, env, stdout: "pipe", stderr: "pipe" });
    const timer = setTimeout(() => proc.kill("SIGKILL"), 20_000); // success returns; a hang fails below
    const code = await proc.exited;
    if (code !== 0) console.log(await new Response(proc.stderr).text());
    clearTimeout(timer);
    const runs = existsSync(calls) ? readFileSync(calls, "utf8").trim().split("\n").map(l => JSON.parse(l) as string[]) : [];
    expect(code).toBe(0);
    expect(runs).toHaveLength(2);
    expect(runs[0]).not.toContain("--channels"); // the plain runtime path, not the bus one
    expect(runs[0]).not.toContain("--continue");
    expect(runs[1]).toContain("--continue");
  } finally { rmSync(dir, { recursive: true, force: true }); }
});
