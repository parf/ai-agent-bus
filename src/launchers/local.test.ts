import { expect, test } from "bun:test";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { Bus } from "../mcp/bus.ts";
import { localAddress, runtimeBinary } from "./local.ts";

test("OpenCode uses the user wrapper ahead of PATH, with explicit executable overrides authoritative", () => {
  const root = resolve(import.meta.dir, "../../tmp/runtime-discovery");
  mkdirSync(root, { recursive: true });
  const home = mkdtempSync(join(root, "run-"));
  const local = join(home, ".local/bin/opencode"), system = join(home, "system");
  mkdirSync(join(home, ".local/bin"), { recursive: true });
  mkdirSync(system);
  const binary = join(system, "opencode");
  writeFileSync(binary, "#!/bin/sh\nexit 0\n", { mode: 0o700 });
  try {
    expect(runtimeBinary("opencode", { PATH: system }, home)).toBe(binary);
    writeFileSync(local, "#!/bin/sh\nexit 0\n", { mode: 0o700 });
    expect(runtimeBinary("opencode", { PATH: system }, home)).toBe(local);
    expect(runtimeBinary("opencode", { PATH: system, OPENCODE_BIN: binary }, home)).toBe(binary);
    expect(runtimeBinary("opencode", { PATH: system, OPENCODE_BIN: join(home, "missing") }, home)).toBeNull();
  } finally { rmSync(home, { recursive: true, force: true }); }
});

test("local discovery prefers the login socket, falls back to the system socket, and honors overrides", () => {
  const root = resolve(import.meta.dir, "../../tmp/local-discovery");
  mkdirSync(root, { recursive: true });
  const dir = mkdtempSync(join(root, "run-"));
  const login = join(dir, "login");
  const system = join(dir, "system");
  mkdirSync(join(login, "agent-bus"), { recursive: true });
  mkdirSync(system);
  const filename = `user-${Bun.spawnSync(["id", "-nu"]).stdout.toString().trim()}.sock`;
  const systemPath = join(system, filename);
  const loginPath = join(login, "agent-bus", filename);
  const listeners: ReturnType<typeof Bun.listen>[] = [];
  const listen = (unix: string) => listeners.push(Bun.listen({ unix, socket: { data() {} } }));
  try {
    expect(localAddress({}, system)).toBeUndefined();
    writeFileSync(systemPath, "not a socket");
    expect(localAddress({}, system)).toBeUndefined();
    rmSync(systemPath);
    listen(systemPath);
    expect(localAddress({ XDG_RUNTIME_DIR: login }, system)).toBe(systemPath);
    listen(loginPath);
    expect(localAddress({ XDG_RUNTIME_DIR: login }, system)).toBe(loginPath);
    expect(localAddress({ XDG_RUNTIME_DIR: login, AGENT_BUS_ADDR: "/missing/explicit.sock" }, system)).toBe("/missing/explicit.sock");
    // A token must not be replaced by the mapped socket's owner identity.
    expect(localAddress({ AGENT_BUS_TOKEN: "session-token" }, system)).toBeUndefined();
    const shared = join(system, "bus.sock");
    listen(shared);
    expect(localAddress({ AGENT_BUS_TOKEN: "session-token" }, system)).toBe(shared);
  } finally {
    for (const listener of listeners) listener.stop(true);
    rmSync(dir, { recursive: true, force: true });
  }
});

test("local discovery names the socket after the uid's account, whatever $USER says", () => {
  const root = resolve(import.meta.dir, "../../tmp/local-discovery");
  mkdirSync(root, { recursive: true });
  const system = mkdtempSync(join(root, "account-"));
  const account = Bun.spawnSync(["id", "-nu"]).stdout.toString().trim();
  const own = join(system, `user-${account}.sock`);
  const listener = Bun.listen({ unix: own, socket: { data() {} } });
  const decoy = Bun.listen({ unix: join(system, "user-someone-else.sock"), socket: { data() {} } });
  // Bun reads $USER once, at startup: each environment needs its own process.
  const discovered = (env: Record<string, string>) => Bun.spawnSync([process.execPath, "-e",
    `import { localAddress } from ${JSON.stringify(join(import.meta.dir, "local.ts"))}; console.log(localAddress({}, ${JSON.stringify(system)}))`],
    { env: { PATH: process.env.PATH!, ...env } }).stdout.toString().trim();
  try {
    // A person's socket is theirs by uid; an environment cannot rename them.
    expect(discovered({ USER: "someone-else", LOGNAME: "someone-else" })).toBe(own);
    expect(discovered({})).toBe(own);
  } finally {
    listener.stop(true); decoy.stop(true);
    rmSync(system, { recursive: true, force: true });
  }
});

test("connection errors identify the bus endpoint without disclosing the credential", async () => {
  const path = resolve(import.meta.dir, "../../tmp/nonexistent-bus/socket");
  try {
    await new Bus({ AGENT_BUS_ADDR: path, AGENT_BUS_TOKEN: "private-token" }).status();
    throw new Error("unexpected connection");
  } catch (e) {
    expect(String(e)).toContain(`GET /status failed at ${path}`);
    expect(String(e)).not.toContain("private-token");
  }
});
