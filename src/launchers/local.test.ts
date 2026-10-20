import "./restart.test.ts";
import { expect, test } from "bun:test";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { Bus, BusError, clientSocket, defaultSocket, runtimeTags, SYSTEM_RUNTIME_DIR } from "../mcp/bus.ts";
import { channelRegistered, gitRoot, localAddress, runtimeBinary } from "./local.ts";

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
    expect(String(e)).toContain(`GET /status no daemon found at ${path}`);
    expect(String(e)).not.toContain("private-token");
  }
});

test("a channel registered under the enclosing repository counts for a directory inside it", () => {
  const config = { projects: { "/rd": { mcpServers: { "agent-bus": {} } } } };
  expect(channelRegistered(config, "/rd/tmp", "/rd")).toBe(true); // Claude keyed it by the git root
  expect(channelRegistered(config, "/rd", undefined)).toBe(true);
  expect(channelRegistered(config, "/elsewhere/tmp", "/elsewhere")).toBe(false);
  expect(channelRegistered({ projects: { "/rd": { mcpServers: {} } } }, "/rd/tmp", "/rd")).toBe(false);
});

test("the git root of a directory inside a repository is the repository", () => {
  const here = resolve(import.meta.dir);
  expect(gitRoot(here)).toBe(resolve(import.meta.dir, "../.."));
  expect(gitRoot("/")).toBeUndefined();
});

test("runtime identity labels name the actual launcher",()=>{
 expect(runtimeTags("claude")).toEqual(["claude"]);
 expect(runtimeTags("codex")).toEqual(["codex"]);
 expect(runtimeTags("opencode")).toEqual(["opencode"]);
 expect(runtimeTags("opencode1")).toEqual(["opencode"]);
 expect(runtimeTags(undefined)).toEqual([]);
});

// The face finds the daemon as the Go CLI does, and with nothing listening it
// names the installed path: never a guess in /tmp, which anyone may plant a
// socket in to be handed the token.
test("the face's socket discovery falls back to the installed path, never /tmp", () => {
  const root = resolve(import.meta.dir, "../../tmp/face-discovery");
  mkdirSync(root, { recursive: true });
  const dir = mkdtempSync(join(root, "run-"));
  const login = join(dir, "login"), system = join(dir, "system");
  mkdirSync(login); mkdirSync(system);
  const listener = Bun.listen({ unix: join(system, "user-me.sock"), socket: { data() {} } });
  try {
    expect(clientSocket([login, system])).toBe(join(system, "bus.sock"));
    expect(clientSocket([login, system], "me")).toBe(join(system, "user-me.sock"));
    expect(clientSocket([login, system], "you")).toBe(join(system, "user-you.sock"));
    expect(clientSocket([login, SYSTEM_RUNTIME_DIR])).toBe(SYSTEM_RUNTIME_DIR + "/bus.sock");
    expect(SYSTEM_RUNTIME_DIR).toBe("/run/agent-bus");
    expect(defaultSocket({ AGENT_BUS_TOKEN: "t" })).toBe("/run/agent-bus/bus.sock");
    expect(defaultSocket({ AGENT_BUS_TOKEN: "t", XDG_RUNTIME_DIR: login })).toBe("/run/agent-bus/bus.sock");
  } finally {
    listener.stop(true);
    rmSync(dir, { recursive: true, force: true });
  }
});

// New sessions must leave existing records untouched, even when dormant.
// Observe unregister attempts independently of daemon-side refusals.
import { claimIdentity } from "./identity.ts";
import { Bindings } from "./sessions.ts";

for (const scenario of [
  { name: "fresh registration", at: -1000, readers: 0 },
  { name: "recent use of an old registration", at: -120000, last_used: -1000, readers: 0 },
  { name: "active reader of an old registration", at: -120000, readers: 1 },
  { name: "dormant registration older than sixty seconds", at: -120000, readers: 0 },
  { name: "old registration and old use", at: -120000, last_used: -90000, readers: 0 },
  { name: "unknown registration time", at: NaN, readers: 0 },
  { name: "unknown last-use time", at: -120000, last_used: NaN, readers: 0 },
  { name: "unknown reader count", at: -120000, readers: undefined },
]) test(`new launcher preserves ${scenario.name}`, async () => {
  const root = resolve(import.meta.dir, "../../tmp/identity-check");
  mkdirSync(root, { recursive: true });
  const dir = mkdtempSync(join(root, "run-"));
  const bindings = new Bindings(dir, "claude");
  const base = "#claude/claim@fixture";
  const timestamp = (offset: number) => Number.isNaN(offset) ? "invalid" : new Date(Date.now() + offset).toISOString();
  let records: any[] = [{ name: base, kind: "agent", descr: "Claim", at: timestamp(scenario.at),
    readers: scenario.readers, ...(scenario.last_used === undefined ? {} : { last_used: timestamp(scenario.last_used) }) }];
  const removed: string[] = [];
  const owner = {
    ls: async () => structuredClone(records),
    unregister: async (name: string) => { removed.push(name); records = records.filter(r => r.name !== name); },
    register: async (record: any) => { expect(records.some(r => r.name === record.name)).toBe(false); records.push(record); },
  } as unknown as Bus;
  try {
    const claimed = await claimIdentity(owner, bindings, base, "Claim");
    expect(removed).toEqual([]);
    expect(claimed.name).toBe("#claude/claim.2@fixture");
    expect(claimed.label).toBe("Claim #2");
  } finally { bindings.close(); rmSync(dir, { recursive: true, force: true }); }
});

for (const scenario of [
  { name: "one second after last use", age: 1000, crash: false },
  { name: "after a crash with no reader", age: 1000, crash: true },
  { name: "after an hour idle", age: 3600000, crash: false },
]) test(`saved launcher names resume exactly ${scenario.name}`, async () => {
  const root = resolve(import.meta.dir, "../../tmp/identity-check");
  mkdirSync(root, { recursive: true });
  const dir = mkdtempSync(join(root, "resume-"));
  const base = "#claude/resume@fixture";
  let records: any[] = [];
  const removed: string[] = [];
  const owner = {
    ls: async () => structuredClone(records),
    unregister: async (name: string) => { removed.push(name); records = records.filter(r => r.name !== name); },
    register: async (record: any, onlyNew: boolean) => {
      if (onlyNew) expect(records.some(r => r.name === record.name)).toBe(false);
      records = [...records.filter(r => r.name !== record.name), record];
    },
  } as unknown as Bus;
  try {
    // Two sessions in one directory reserve distinct names, then both restart.
    for (const id of ["first", "second"]) {
      const bindings = new Bindings(dir, "claude");
      const claimed = await claimIdentity(owner, bindings, base, "Resume");
      expect(claimed.name).toBe(id === "first" ? base : "#claude/resume.2@fixture");
      bindings.save(id, claimed.name, base);
      bindings.close();
    }
    for (const record of records) Object.assign(record, {
      at: new Date(Date.now() - scenario.age).toISOString(),
      last_used: new Date(Date.now() - scenario.age).toISOString(), readers: 0,
    });
    for (const id of ["first", "second"]) {
      const expected = id === "first" ? base : "#claude/resume.2@fixture";
      if (scenario.crash) {
        // A dead child leaves the same on-disk bus lock as an abrupt exit.
        const child = Bun.spawnSync([process.execPath, "-e",
          `import { Bindings } from ${JSON.stringify(join(import.meta.dir, "sessions.ts"))};
           const b = new Bindings(${JSON.stringify(dir)}, "claude");
           if (!b.lock(${JSON.stringify(`bus:${expected}`)})) process.exit(1);`]);
        expect(child.exitCode).toBe(0);
      }
      const restarted = new Bindings(dir, "claude");
      try {
        const saved = restarted.saved({ id });
        expect(saved?.name).toBe(expected);
        const claimed = await claimIdentity(owner, restarted, base, "Resume", saved?.name, saved?.name);
        expect(claimed.name).toBe(expected);
      } finally { restarted.close(); }
    }
    expect(removed).toEqual([]);
    expect(records.map(r => r.name).sort()).toEqual([base, "#claude/resume.2@fixture"].sort());
  } finally { rmSync(dir, { recursive: true, force: true }); }
});

for (const collision of ["listed", "conditional"]) test(`a ${collision} collision does not lock a dormant session out of resuming`, async () => {
  const root = resolve(import.meta.dir, "../../tmp/identity-check");
  mkdirSync(root, { recursive: true });
  const dir = mkdtempSync(join(root, "neighbor-"));
  const base = "#claude/neighbor@fixture";
  const original = new Bindings(dir, "claude");
  original.save("original", base, base); original.close();
  const newcomer = new Bindings(dir, "claude"), resumed = new Bindings(dir, "claude");
  const dormant = { name: base, descr: "Neighbor", readers: 0 };
  let records: any[] = collision === "listed" ? [dormant] : [];
  let raced = false;
  const owner = {
    ls: async () => {
      if (collision === "listed") expect(newcomer.holds(`bus:${base}`)).toBe(false);
      return structuredClone(records);
    },
    unregister: async () => { throw new Error("must not reclaim a reservation"); },
    register: async (record: any, onlyNew: boolean) => {
      if (collision === "conditional" && !raced && onlyNew && record.name === base) {
        raced = true; records.push(dormant);
        throw new BusError(412, "another state directory registered first");
      }
      records = [...records.filter(r => r.name !== record.name), record];
    },
  } as unknown as Bus;
  try {
    expect((await claimIdentity(owner, newcomer, base, "Neighbor")).name).toBe("#claude/neighbor.2@fixture");
    const saved = resumed.saved({ id: "original" });
    const restored = await claimIdentity(owner, resumed, base, "Neighbor", saved?.name, saved?.name);
    expect(restored.name).toBe(base);
    expect(restored.label).toBe("Neighbor");
    expect(records.map(r => r.name).sort()).toEqual([base, "#claude/neighbor.2@fixture"].sort());
  } finally { newcomer.close(); resumed.close(); rmSync(dir, { recursive: true, force: true }); }
});

for (const reservation of ["saved", "explicit"]) test(`${reservation} reservation refuses an existing reader without changing its record`, async () => {
  const root = resolve(import.meta.dir, "../../tmp/identity-check");
  mkdirSync(root, { recursive: true });
  const dir = mkdtempSync(join(root, "live-"));
  const bindings = new Bindings(dir, "claude");
  const name = "#claude/pinned@fixture";
  const record = { name, descr: "live owner's label", readers: 1 };
  const writes: any[] = [];
  const owner = { ls: async () => [structuredClone(record)], register: async (r: any) => { writes.push(r); } } as unknown as Bus;
  try {
    await expect(claimIdentity(owner, bindings, name, "replacement", name, name)).rejects.toThrow(`${name} is already live`);
    expect(writes).toEqual([]);
  } finally { bindings.close(); rmSync(dir, { recursive: true, force: true }); }
});

test("a binding reader spanning publication sees one complete saved identity", async () => {
  const { openSync, readSync, closeSync, readFileSync, statSync } = await import("node:fs");
  const root = resolve(import.meta.dir, "../../tmp/binding-publication");
  mkdirSync(root, { recursive: true });
  const dir = mkdtempSync(join(root, "run-")), bindings = new Bindings(dir, "claude");
  const id = "saved-session", oldName = "#claude/old-name@fixture", newName = "#claude/new-name@fixture";
  try {
    bindings.save(id, oldName, oldName);
    const old = readFileSync(bindings.file(id));
    const fd = openSync(bindings.file(id), "r"), observed = Buffer.alloc(old.length);
    try {
      // Pause a reader halfway through the old name, publish, then continue on
      // the same descriptor. Direct truncation makes it read a hybrid binding.
      const split = old.indexOf("old-name") + 3;
      expect(readSync(fd, observed, 0, split, null)).toBe(split);
      bindings.save(id, newName, newName);
      expect(readSync(fd, observed, split, old.length - split, null)).toBe(old.length - split);
      expect(observed.toString()).toBe(old.toString());
      expect(bindings.saved({ id })?.name).toBe(newName);
      expect(statSync(bindings.file(id)).mode & 0o777).toBe(0o600);
    } finally { closeSync(fd); }
  } finally { bindings.close(); rmSync(dir, { recursive: true, force: true }); }
});


// Isolate filesystem interleaving hooks from other tests and module caches.
function probe(body: string) {
  const root = resolve(import.meta.dir, "../../tmp/lock-tests");
  mkdirSync(root, { recursive: true });
  const dir = mkdtempSync(join(root, "run-"));
  try {
    const child = Bun.spawnSync([process.execPath, "-e", `
      import { mock, expect } from "bun:test";
      import real from "node:fs";
      import { join } from "node:path";
      const fs = { ...real };
      let afterMkdir = () => {}, beforeWrite = () => {}, afterRead = () => {};
      mock.module("node:fs", () => ({ ...fs,
        mkdirSync(...args) { const value = fs.mkdirSync(...args); afterMkdir(String(args[0])); return value; },
        writeFileSync(...args) { beforeWrite(String(args[0])); return fs.writeFileSync(...args); },
        readFileSync(...args) { const value = fs.readFileSync(...args); afterRead(String(args[0])); return value; },
      }));
      const { Bindings, hash } = await import(${JSON.stringify(join(import.meta.dir, "sessions.ts"))});
      const dir = ${JSON.stringify(dir)};
      const a = new Bindings(dir, "codex"), b = new Bindings(dir, "codex");
      const path = join(dir, hash("claim") + ".lock");
      try { ${body} } finally { a.close(); b.close(); }
    `]);
    expect(child.stderr.toString()).toBe("");
    expect(child.exitCode).toBe(0);
  } finally { rmSync(dir, { recursive: true, force: true }); }
}

test("preparing a lock never exposes a PID-less reservation to a competing claimant", () => probe(`
  let injected = false, exposed = false;
  const compete = () => {
    if (injected) return;
    injected = true;
    exposed = fs.existsSync(path);
    // B claims and closes A's incomplete lock before A publishes its PID.
    expect(b.lock("claim")).toBe(true);
    b.close();
  };
  afterMkdir = p => { if (p === path) compete(); };
  beforeWrite = p => { if (p.includes(".lock") && p.includes("/pid")) compete(); };
  let won = false, failure;
  try { won = a.lock("claim"); } catch (e) { failure = e; }
  expect(injected).toBe(true);
  expect(failure).toBeUndefined();
  expect(exposed).toBe(false);
  expect(won).toBe(true);
  expect(b.lock("claim")).toBe(false);
`));

test("a stale claimant cannot remove a replacement live lock", () => probe(`
  const dead = Bun.spawnSync([process.execPath, "-e", "console.log(process.pid)"]);
  expect(dead.exitCode).toBe(0);
  fs.mkdirSync(path);
  fs.writeFileSync(join(path, "pid"), dead.stdout.toString().trim());
  let injected = false, secondWon = false;
  afterRead = p => {
    if (!injected && p === join(path, "pid")) {
      injected = true;
      secondWon = b.lock("claim");
    }
  };
  expect(a.lock("claim")).toBe(false);
  expect(injected).toBe(true);
  expect(secondWon).toBe(true);
  const c = new Bindings(dir, "codex");
  try { expect(c.lock("claim")).toBe(false); } finally { c.close(); }
`));

test("concurrent lock claimants have exactly one winner and all exit cleanly", async () => {
  const root = resolve(import.meta.dir, "../../tmp/lock-tests");
  mkdirSync(root, { recursive: true });
  for (const stale of [false, true]) {
    const dir = mkdtempSync(join(root, "parallel-"));
    const module = JSON.stringify(join(import.meta.dir, "sessions.ts"));
    const script = `
      import { existsSync, writeFileSync } from "node:fs";
      import { join } from "node:path";
      import { Bindings } from ${module};
      const dir = ${JSON.stringify(dir)}, id = process.argv[1];
      const b = new Bindings(dir, "codex");
      writeFileSync(join(dir, id + ".ready"), "");
      while (!existsSync(join(dir, "go"))) await Bun.sleep(1);
      const won = b.lock("claim");
      writeFileSync(join(dir, id + ".result"), String(won));
      while (!existsSync(join(dir, "release"))) await Bun.sleep(1);
      b.close();
    `;
    if (stale) {
      const dead = Bun.spawnSync([process.execPath, "-e",
        `import { Bindings } from ${module}; const b = new Bindings(${JSON.stringify(dir)}, "codex"); if (!b.lock("claim")) process.exit(1);`]);
      expect(dead.exitCode).toBe(0);
    }
    const children = Array.from({ length: 12 }, (_, i) => Bun.spawn([process.execPath, "-e", script, String(i)], { stdout: "ignore", stderr: "pipe" }));
    const errors = children.map(c => new Response(c.stderr).text());
    const fs = await import("node:fs");
    const until = async (suffix: string) => {
      const deadline = performance.now() + 10000;
      while (performance.now() < deadline && !children.every((_, i) => fs.existsSync(join(dir, i + suffix)))) await Bun.sleep(5);
      return children.every((_, i) => fs.existsSync(join(dir, i + suffix)));
    };
    try {
      expect(await until(".ready")).toBe(true);
      fs.writeFileSync(join(dir, "go"), "");
      expect(await until(".result")).toBe(true);
      expect(children.filter((_, i) => fs.readFileSync(join(dir, i + ".result"), "utf8") === "true").length).toBe(1);
      fs.writeFileSync(join(dir, "release"), "");
      expect(await Promise.all(children.map(c => c.exited))).toEqual(children.map(() => 0));
      expect(await Promise.all(errors)).toEqual(children.map(() => ""));
    } finally {
      for (const c of children) if (c.exitCode === null) c.kill();
      await Promise.all(children.map(c => c.exited));
      rmSync(dir, { recursive: true, force: true });
    }
  }
}, 30000);

test("session purpose is explicit and does not change a saved identity", async () => {
  const { descriptionOption } = await import("./description.ts");
  expect(descriptionOption(["resume", "--descr", "Review billing"]).description).toBe("Review billing");
  expect(descriptionOption(["resume"]).description).toBeUndefined();
  expect(descriptionOption(["--descr=Review billing", "resume"]).args).toEqual(["resume"]);
  expect(() => descriptionOption(["--descr"])).toThrow();
  const name = "#codex/billing@t";
  let record: any = { name, kind: "agent", descr: "codex(/work/billing)", allow: ["@owner"], personal: true, tags: ["codex"], readers: 0 };
  const owner: any = { ls: async () => [record], register: async (next: any) => { record = { ...record, ...next }; } };
  const bindings: any = { lock: () => true, holds: () => false, unlock: () => {} };
  const before = { ...record };
  const claimed = await claimIdentity(owner, bindings, name, "Billing", name, name, "codex", undefined, "");
  expect(claimed.name).toBe(name);
  expect(record).toEqual({ ...before, descr: "" });
  await claimIdentity(owner, bindings, name, "Billing", name, name, "codex", undefined, "Review billing");
  expect(record).toEqual({ ...before, descr: "Review billing" });
  await claimIdentity(owner, bindings, name, "Billing", name, name, "codex");
  expect(record.descr).toBe("Review billing");
});

test("session tags are set explicitly, preserved on resume and inherited on rename", async () => {
  for (const runtime of ["claude", "codex", "opencode", "opencode1"]) {
    const name = `#${runtime}/tags@t`, next = `#${runtime}/renamed@t`;
    const records: any[] = [];
    const owner: any = {ls: async () => records, register: async (record: any) => {
      const old = records.find(r => r.name === record.name);
      if (old) Object.assign(old, {...record, tags: [...new Set([...(old.tags ?? []), ...record.tags])].sort()}); else records.push({...record});
    }, post: async (path: string, change: any) => { expect(path).toBe("/manage"); Object.assign(records.find(r => r.name === change.name), change); }};
    const bindings: any = {lock: () => true, holds: () => false, unlock: () => {}};
    const automatic = runtime === "opencode1" ? "opencode" : runtime;
    await claimIdentity(owner, bindings, name, "Tags", undefined, undefined, runtime, undefined, undefined, ["review", "billing"]);
    expect(records[0].tags).toEqual(["billing", automatic, "review"].sort());
    records[0].tags.push("web edit");
    await claimIdentity(owner, bindings, name, "Tags", name, name, runtime);
    expect(records[0].tags).toContain("web edit");
    await claimIdentity(owner, bindings, next, "Tags", name, undefined, runtime);
    expect(records[1].tags).toEqual(records[0].tags);
    await claimIdentity(owner, bindings, name, "Tags", name, name, runtime, undefined, undefined, []);
    expect(records[0].tags).toEqual([automatic, "personal"].sort());
  }
});

test("all launcher entrypoints refuse bad tags by name and advertise grouping", async () => {
  for (const runtime of ["claude", "codex", "opencode", "opencode1"]) {
    const bad = Bun.spawn([process.execPath, `${import.meta.dir}/launcher.ts`, runtime, "--tag", "123", "--help"], {stdout: "pipe", stderr: "pipe"});
    expect(await bad.exited).toBe(1);
    expect(await new Response(bad.stderr).text()).toContain('invalid tag "123"');
    const good = Bun.spawn([process.execPath, `${import.meta.dir}/launcher.ts`, runtime, "--tag", "review,billing", "--help"], {stdout: "pipe", stderr: "pipe"});
    expect(await good.exited).toBe(0);
    expect(await new Response(good.stdout).text()).toContain("--tag");
  }
});

test("launcher tags use the protocol vectors and repeatable comma options", async () => {
  const { normalizeTags } = await import("../mcp/tags.ts");
  const { tagOption } = await import("./tags.ts");
  const vectors = await Bun.file(resolve(import.meta.dir, "../internal/protocol/testdata/tags.json")).json();
  for (const vector of vectors) expect(normalizeTags(vector.input)).toEqual(vector.expected);
  expect(tagOption(["resume", "--tag", " BUILD Notes,review ", "--tag=c++", "--tag", "review"]).tags).toEqual(["build notes", "c", "review"]);
  expect(tagOption(["resume"]).tags).toBeUndefined();
  expect(tagOption(["--tag", ""]).tags).toEqual([]);
  expect(tagOption(["--tag=review", "--", "--tag", "runtime"]).args).toEqual(["--", "--tag", "runtime"]);
  for (const args of [["--tag"], ["--tag", "--help"], ["--tag", "no-tags"], ["--tag", "review,,billing"], ["--tag", "", "--tag=review"]]) expect(() => tagOption(args)).toThrow();
});
