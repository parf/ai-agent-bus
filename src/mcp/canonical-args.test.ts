import { expect, test } from "bun:test";
import { join } from "node:path";

test("MCP accepts only sole canonical info flags before loading session state", () => {
  for (const args of [["--help"], ["--version"], ["-h"], ["-version"], ["--version", "extra"], ["--help", "extra"]]) {
    const p = Bun.spawnSync([process.execPath, join(import.meta.dir, "server.ts"), ...args], {
      env: { ...process.env, AGENT_BUS_SESSION_FILE: "/nonexistent-canonical-args-fixture", AGENT_BUS_TOKEN: "" },
      stdout: "pipe", stderr: "pipe", timeout: 5000,
    });
    const accepted = args.length === 1 && args[0]!.startsWith("--");
    expect(p.exitCode).toBe(accepted ? 0 : 2);
    if (!accepted) expect(p.stderr.toString()).toContain("unsupported arguments");
  }
});
