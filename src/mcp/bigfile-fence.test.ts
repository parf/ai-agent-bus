import { afterEach, expect, test } from "bun:test";
import { mkdirSync, mkdtempSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { BIGFILE_DIR, fencedInput, fencedOutput, NOT_CONFIGURED } from "./bigfile-fence.ts";

const base = resolve(import.meta.dir, "../../tmp");
mkdirSync(base, { recursive: true });
const made: string[] = [];
afterEach(() => { for (const d of made.splice(0)) rmSync(d, { recursive: true, force: true }); });
function area() {
  const top = mkdtempSync(join(base, "fence-")); made.push(top);
  const allowed = join(top, "allowed"), outside = join(top, "outside");
  mkdirSync(allowed); mkdirSync(outside);
  writeFileSync(join(outside, "secret"), "s"); writeFileSync(join(allowed, "ok"), "o");
  return { top, allowed, outside, env: { [BIGFILE_DIR]: allowed } };
}
const refused = async (p: Promise<unknown>, text?: string) => {
  let error: any;
  try { await p; } catch (e) { error = e; }
  expect(error, "expected a refusal").toBeDefined();
  if (text) expect(error.message).toContain(text);
};

test("with no directory configured both directions refuse", async () => {
  const { allowed } = area();
  await refused(fencedInput({}, join(allowed, "ok")), NOT_CONFIGURED);
  await refused(fencedOutput({}, join(allowed, "new")), NOT_CONFIGURED);
  await refused(fencedInput({ [BIGFILE_DIR]: "  " }, join(allowed, "ok")), NOT_CONFIGURED);
});

test("a configured directory must be absolute and exist", async () => {
  const { allowed } = area();
  await refused(fencedInput({ [BIGFILE_DIR]: "relative/dir" }, "ok"), "absolute");
  await refused(fencedInput({ [BIGFILE_DIR]: join(allowed, "nope") }, "ok"), "existing directory");
  await refused(fencedInput({ [BIGFILE_DIR]: join(allowed, "ok") }, "ok"), "existing directory");
});

test("a file inside is read by absolute or relative path", async () => {
  const { allowed, env } = area();
  expect(await fencedInput(env, join(allowed, "ok"))).toBe(join(allowed, "ok"));
  expect(await fencedInput(env, "ok")).toBe(join(allowed, "ok"));
  expect(await fencedOutput(env, "fresh")).toBe(join(allowed, "fresh"));
  expect(await fencedOutput(env, join(allowed, "ok"))).toBe(join(allowed, "ok"));
});

test("a .. segment is refused even when it stays inside", async () => {
  const { allowed, outside, env } = area();
  mkdirSync(join(allowed, "sub"));
  await refused(fencedInput(env, `${allowed}/sub/../ok`), "..");
  await refused(fencedInput(env, `${allowed}/../outside/secret`), "..");
  await refused(fencedOutput(env, `${allowed}/../outside/new`), "..");
  await refused(fencedOutput(env, "../outside/new"), "..");
  void outside;
});

test("an absolute path elsewhere is refused", async () => {
  const { outside, env } = area();
  await refused(fencedInput(env, join(outside, "secret")), "outside");
  await refused(fencedOutput(env, join(outside, "new")), "outside");
  await refused(fencedInput(env, "/etc/passwd"), "outside");
});

test("a symlink that leads out is refused, one that stays inside is read", async () => {
  const { allowed, outside, env } = area();
  symlinkSync(join(outside, "secret"), join(allowed, "link-out"));
  symlinkSync(outside, join(allowed, "dir-out"));
  symlinkSync(join(allowed, "ok"), join(allowed, "link-in"));
  await refused(fencedInput(env, "link-out"), "outside");
  await refused(fencedInput(env, "dir-out/secret"), "outside");
  await refused(fencedOutput(env, "dir-out/new"), "outside");
  expect(await fencedInput(env, "link-in")).toBe(join(allowed, "ok"));
});

test("a download never goes through a link at the target, and never onto a directory", async () => {
  const { allowed, outside, env } = area();
  symlinkSync(join(outside, "secret"), join(allowed, "target-link"));
  symlinkSync(join(allowed, "ok"), join(allowed, "target-link-in"));
  mkdirSync(join(allowed, "adir"));
  await refused(fencedOutput(env, "target-link"), "link");
  await refused(fencedOutput(env, "target-link-in"), "link");
  await refused(fencedOutput(env, "adir"), "regular file");
  await refused(fencedOutput(env, "missing-dir/new"), "does not exist");
});

test("a FIFO or a device is not a regular file", async () => {
  const { allowed, env } = area();
  const made_ = spawnSync("mkfifo", [join(allowed, "pipe")]);
  expect(made_.status).toBe(0);
  await refused(fencedInput(env, "pipe"), "regular file");
  await refused(fencedOutput(env, "pipe"), "regular file");
  symlinkSync("/dev/null", join(allowed, "dev"));
  await refused(fencedInput(env, "dev"), "outside");
});
