import { expect, test } from "bun:test";
import { existsSync, mkdirSync, mkdtempSync, rmSync, utimesSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { Bindings, claudeSessions, claudeStart } from "./sessions.ts";

// J34: two names share one directory and B wrote last. Restarting A must reopen
// A's own transcript by its ID; --continue would reopen B's, the newest.
test("a named Claude session resumes its own transcript, not the directory's newest", async () => {
  const root = resolve(import.meta.dir, "../../tmp");
  mkdirSync(root, { recursive: true });
  const dir = mkdtempSync(join(root, "launcher-sessions-"));
  try {
    const cwd = "/work/Plans", home = join(dir, "claude");
    const projects = join(home, "projects", cwd.replace(/[^a-zA-Z0-9]/g, "-"));
    mkdirSync(projects, { recursive: true });
    const a = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", b = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
    for (const [id, at] of [[a, 1000], [b, 2000]] as const) {
      const file = join(projects, `${id}.jsonl`);
      writeFileSync(file, "{}\n");
      utimesSync(file, at, at);
    }
    const bindings = new Bindings(join(dir, "state"), "claude");
    bindings.save(a, "#claude/lead2@h", "#claude/lead2@h");
    bindings.save(b, "#claude/w2a@h", "#claude/w2a@h");
    const sessions = await claudeSessions(home, cwd);
    expect(sessions.map(s => s.id)).toEqual([b, a]);   // B is the newest in the directory
    const session = bindings.select(bindings.candidates(sessions, "#claude/lead2@h"));
    expect(session.id).toBe(a);
    expect(claudeStart(session)).toEqual(["--resume", a]);
    // A transcript removed since the listing starts a fresh session under that ID.
    rmSync(join(projects, `${a}.jsonl`));
    expect(claudeStart(session)).toEqual(["--session-id", a]);
    expect(claudeStart({ id: a })).toEqual(["--session-id", a]);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

// J40: a restart that must not reopen the bound conversation. The default
// still resumes it; --fresh gets a new ID under the same name, leaves the old
// transcript, and the start after that resumes the new one.
test("a fresh start gets a new ID under the same name and later starts resume it", async () => {
  const root = resolve(import.meta.dir, "../../tmp");
  mkdirSync(root, { recursive: true });
  const dir = mkdtempSync(join(root, "launcher-fresh-"));
  try {
    const cwd = "/work/Plans", home = join(dir, "claude"), name = "#claude/w1b@h";
    const projects = join(home, "projects", cwd.replace(/[^a-zA-Z0-9]/g, "-"));
    mkdirSync(projects, { recursive: true });
    const old = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
    const oldFile = join(projects, `${old}.jsonl`);
    writeFileSync(oldFile, "{}\n");
    utimesSync(oldFile, 1000, 1000);
    const bindings = new Bindings(join(dir, "state"), "claude");
    bindings.save(old, name, name);
    // The default resumes the bound session.
    const resumed = bindings.select(bindings.candidates(await claudeSessions(home, cwd), name));
    expect(resumed.id).toBe(old);
    expect(claudeStart(resumed)).toEqual(["--resume", old]);
    bindings.unlock(`claude:${old}`);
    // --fresh: a new ID, started as a new session, the old transcript untouched.
    const fresh = bindings.select(bindings.candidates(await claudeSessions(home, cwd), name, true));
    expect(fresh.id).not.toBe(old);
    expect(claudeStart(fresh)).toEqual(["--session-id", fresh.id]);
    expect(existsSync(oldFile)).toBe(true);
    // The runtime writes the new transcript; the launcher binds the name to it.
    writeFileSync(join(projects, `${fresh.id}.jsonl`), "{}\n");
    bindings.save(fresh.id, name, name);
    bindings.close();
    // A plain start now resumes the new session, not the old one.
    const later = bindings.select(bindings.candidates(await claudeSessions(home, cwd), name));
    expect(later.id).toBe(fresh.id);
    expect(claudeStart(later)).toEqual(["--resume", fresh.id]);
    bindings.close();
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
