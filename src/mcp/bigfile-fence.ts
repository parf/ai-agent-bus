import { lstat, realpath, stat } from "node:fs/promises";
import { basename, dirname, isAbsolute, join, sep } from "node:path";

// The one directory the face's bigfile tools may read from and write to. The
// model picks the path; this decides whether the path is allowed. There is no
// default: a face with no directory configured refuses, because a path chosen
// by a model (or by an injected message) must never reach the user's whole
// filesystem (docs/04-messaging.md#bigfiles).
export const BIGFILE_DIR = "AGENT_BUS_BIGFILE_DIR";
export const NOT_CONFIGURED = `bigfile is not configured: set the allowed directory (${BIGFILE_DIR})`;

type Env = Record<string, string | undefined>;

async function root(env: Env): Promise<string> {
  const dir = env[BIGFILE_DIR]?.trim();
  if (!dir) throw new Error(NOT_CONFIGURED);
  if (!isAbsolute(dir)) throw new Error(`${BIGFILE_DIR} must be an absolute path`);
  let real: string;
  try { real = await realpath(dir); } catch { throw new Error(`${BIGFILE_DIR} does not name an existing directory`); }
  if (!(await stat(real)).isDirectory()) throw new Error(`${BIGFILE_DIR} does not name an existing directory`);
  return real;
}

const inside = (dir: string, path: string) => path === dir || path.startsWith(dir.endsWith(sep) ? dir : dir + sep);

// A relative path means relative to the allowed directory, never to a cwd.
function lexical(dir: string, given: string): string {
  if (typeof given !== "string" || !given.trim() || given.includes("\0")) throw new Error("the path is required");
  if (given.split(/[\\/]+/).includes("..")) throw new Error("the path may not contain ..");
  return isAbsolute(given) ? given : join(dir, given);
}

/** The real path of a regular file inside the allowed directory, or a refusal. */
export async function fencedInput(env: Env, given: string): Promise<string> {
  const dir = await root(env);
  const wanted = lexical(dir, given);
  let real: string;
  try { real = await realpath(wanted); } catch { throw new Error("the file does not exist inside the allowed directory"); }
  if (!inside(dir, real)) throw new Error("the path leads outside the allowed directory");
  if (!(await stat(real)).isFile()) throw new Error("the path is not a regular file");
  return real;
}

/**
 * Where a download may be written: a name inside the allowed directory whose
 * parent is already there. An existing target must be a plain file, never a
 * link or a device, so the write cannot be steered somewhere else.
 */
export async function fencedOutput(env: Env, given: string): Promise<string> {
  const dir = await root(env);
  const wanted = lexical(dir, given);
  const name = basename(wanted);
  let parent: string;
  try { parent = await realpath(dirname(wanted)); } catch { throw new Error("the directory does not exist inside the allowed directory"); }
  if (!inside(dir, parent)) throw new Error("the path leads outside the allowed directory");
  if (!name || name === "." || name === "..") throw new Error("the path must name a file");
  const target = join(parent, name);
  const found = await lstat(target).catch(() => undefined);
  if (found && !found.isFile()) throw new Error("the target is a link or not a regular file");
  return target;
}
