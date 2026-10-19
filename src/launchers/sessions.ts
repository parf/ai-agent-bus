import { writeState } from "./state.ts";
import { createHash, randomUUID } from "node:crypto";
import { chmodSync, createReadStream, mkdirSync, readFileSync, readdirSync, statSync, rmSync, mkdtempSync, renameSync, writeFileSync, unlinkSync, rmdirSync } from "node:fs";
import { createInterface } from "node:readline";
import { join } from "node:path";

export type Session = { id: string; name?: string | null; file?: string };
type Binding = { name: string; base?: string };
export const hash = (s: string) => createHash("sha256").update(s).digest("hex");
export function numberedName(base: string, number: number): string {
  if (number === 1) return base;
  const at = base.lastIndexOf("@"), suffix = `.${number}`;
  return `${base.slice(0, at).slice(0, 64 - (base.length - at) - suffix.length)}${suffix}${base.slice(at)}`;
}
export function sameBase(saved: Binding, base: string): boolean {
  if (saved.base) return saved.base === base;
  // Older bindings stored only the allocated address, possibly suffixed/truncated.
  const suffix = saved.name.match(/\.([2-9]|[1-9][0-9]+)@/);
  return saved.name === base || (!!suffix && saved.name === numberedName(base, Number(suffix[1])));
}
const titles = new Map<string, { offset: number; mtime: number; ino: number; title?: string }>();

export async function claudeTitle(file: string): Promise<string | undefined> {
  try {
    const stat = statSync(file);
    let cached = titles.get(file);
    if (cached?.mtime === stat.mtimeMs) return cached.title;
    if (!cached || cached.ino !== stat.ino || stat.size <= cached.offset) cached = { offset: 0, mtime: 0, ino: stat.ino };
    if (stat.size > cached.offset) for await (const line of createInterface({ input: createReadStream(file, { start: cached.offset, end: stat.size - 1 }), crlfDelay: Infinity })) {
      const bytes = Buffer.byteLength(line) + 1;
      if (cached.offset + bytes > stat.size) break; // retry an unfinished final line next time
      cached.offset += bytes;
      // Only metadata is retained; no transcript content enters the bus.
      if (!line.includes('"custom-title"')) continue;
      try {
        const row = JSON.parse(line);
        if (row.type === "custom-title" && typeof row.customTitle === "string") cached.title = row.customTitle;
      } catch { /* a runtime may still be appending the last line */ }
    }
    cached.mtime = stat.mtimeMs;
    titles.set(file, cached);
    return cached.title;
  } catch (e: any) { if (e.code !== "ENOENT") throw e; }
}

export async function claudeSessions(home: string, cwd: string): Promise<Session[]> {
  const dir = join(home, "projects", cwd.replace(/[^a-zA-Z0-9]/g, "-"));
  let files: string[];
  try { files = readdirSync(dir).filter(f => /^[0-9a-f-]{36}\.jsonl$/.test(f)); }
  catch (e: any) { if (e.code === "ENOENT") return []; throw e; }
  files.sort((a, b) => statSync(join(dir, b)).mtimeMs - statSync(join(dir, a)).mtimeMs);
  return Promise.all(files.map(async f => ({ id: f.slice(0, -6), name: await claudeTitle(join(dir, f)), file: join(dir, f) })));
}

export class Bindings {
  #locks: string[] = [];
  constructor(readonly dir: string, readonly runtime: string) {
    mkdirSync(dir, { recursive: true, mode: 0o700 });
    chmodSync(dir, 0o700);
  }
  lock(key: string): boolean {
    const path = join(this.dir, `${hash(key)}.lock`);
    if (this.#locks.includes(path)) return true;
    // Publish a complete, nonempty directory. No claimant can mistake
    // another process's PID initialization for an abandoned reservation.
    const temporary = mkdtempSync(path + ".");
    try {
      writeFileSync(join(temporary, "pid." + randomUUID()), String(process.pid), { mode: 0o600, flag: "wx" });
      for (let attempt = 0; attempt < 2; attempt++) {
        try {
          renameSync(temporary, path);
          this.#locks.push(path);
          return true;
        } catch (e: any) {
          if (e.code !== "EEXIST" && e.code !== "ENOTEMPTY") throw e;
          if (attempt || !this.#retire(path)) return false;
        }
      }
      return false;
    } finally { rmSync(temporary, { recursive: true, force: true }); }
  }

  select(sessions: Session[], requested?: string): Session {
    const found = requested ? sessions.filter(s => s.id === requested || s.name === requested) : sessions;
    if (requested && found.length !== 1) throw new Error("session selector must match exactly one session in this directory");
    for (const s of found) if (this.lock(`${this.runtime}:${s.id}`)) return s;
    if (requested) throw new Error("session is already launched (or has a stale launcher lock)");
    const session = { id: randomUUID() };
    if (!this.lock(`${this.runtime}:${session.id}`)) throw new Error("cannot lock new session");
    return session;
  }
  saved(s: Session, explicit?: string): Binding | undefined {
    const file = this.file(s.id);
    let saved: Binding | undefined;
    try { saved = JSON.parse(readFileSync(file, "utf8")); }
    catch (e: any) { if (e.code !== "ENOENT") throw e; }
    if (saved && explicit && saved.name !== explicit && !sameBase(saved, explicit)) throw new Error("resumed session has a different bus name; use its existing identity");
    return saved;
  }
  file(id: string): string { return join(this.dir, `${hash(`${this.runtime}:${id}`)}.json`); }
  save(id: string, name: string, base: string): void { writeState(this.file(id), JSON.stringify({ name, base })); }
  // Retire only the observed dead owner's file. A competing reclamation
  // publishes a different filename, so neither unlink nor rmdir can erase it.
  #retire(path: string): boolean {
    try {
      const files = readdirSync(path);
      if (files.length !== 1 || !/^pid(?:\.[a-f0-9-]+)?$/.test(files[0]!)) return false;
      // TODO after 0.9: remove legacy fixed "pid" compatibility.
      const owner = join(path, files[0]!);
      const pid = Number(readFileSync(owner, "utf8").trim());
      if (!Number.isInteger(pid) || pid <= 0) return false;
      try { process.kill(pid, 0); return false; }
      catch (e: any) { if (e.code !== "ESRCH") return false; }
      unlinkSync(owner);
      rmdirSync(path);
      return true;
    } catch (e: any) {
      if (["ENOENT", "ENOTEMPTY", "EEXIST"].includes(e.code)) return false;
      throw e;
    }
  }
  holds(key: string): boolean { return this.#locks.includes(join(this.dir, `${hash(key)}.lock`)); }
  unlock(key: string): void {
    const path = join(this.dir, `${hash(key)}.lock`);
    const index = this.#locks.indexOf(path);
    if (index < 0) return; // Never remove another Bindings instance's lock.
    rmSync(path, { recursive: true, force: true });
    this.#locks.splice(index, 1);
  }
  close(): void { for (const path of this.#locks.splice(0)) rmSync(path, { recursive: true, force: true }); }
}
