// The instance's configuration, read as its own record
// (Plans/R1.0-Release/telegram/README.md § where things live):
//
//   { "default": "#dev", "aliases": { "#dev": "-100…", "@alice": "123…" } }
//
// `default` names the destination a call with none takes: an alias with its
// sigil, or a numeric chat id used as-is, not required to be declared. Alias
// keys carry their sigil — `#dev` and `dev` are different keys, and no key
// collides with a numeric id — and values are decimal integer strings, exact
// for any id Telegram issues.
//
// Validation refuses at start what a first call would only find later: an
// alias key without a sigil, a value that is not a decimal integer string,
// a default naming an alias that does not exist. Each refusal names the bad
// key. No configuration at all is not malformed: the agent runs, and the
// calls that need a destination are refused then (README § methods).
//
// The same module classifies why a read of the record's state failed, so a
// fault of the bus is never mistaken for a fact about the record: only a 404
// means the secret or the configuration is not there; a dotenv parse failure
// is the secret being malformed; anything else is the bus failing, which
// must not look unconfigured — the runner restarts the agent instead
// (tg1-review: a transient failure must not become a permanent outage).
import { BusError } from "../../clients/typescript/src/index.js";

export type Config = { default?: string; aliases: Record<string, string> };

const DECIMAL = /^-?\d+$/;

/** Why the secret cannot be read as env entries. */
export type SecretFault =
  | { kind: "absent"; message: string }    // 404: no secret stored — unconfigured
  | { kind: "malformed"; message: string } // stored, not an env file — unconfigured, named
  | { kind: "bus"; message: string };      // the bus did not answer — NOT unconfigured

const ENVFILE_FAULT = /^the bus secret: (line \d+: .+)$/;

export function secretFault(e: unknown): SecretFault {
  if (e instanceof BusError && e.status === 404) return { kind: "absent", message: "the bus secret is not set" };
  if (e instanceof Error) {
    const m = ENVFILE_FAULT.exec(e.message);
    if (m) return { kind: "malformed", message: `the bus secret is not an env file: ${m[1]}` };
  }
  return { kind: "bus", message: "the bus did not answer" };
}

/** Why the configuration cannot be read: none stored, or the bus failing. */
export type ConfigFault = { kind: "none" } | { kind: "bus"; message: string };

export function configFault(e: unknown): ConfigFault {
  if (e instanceof BusError && e.status === 404) return { kind: "none" };
  return { kind: "bus", message: "the bus did not answer" };
}

/** The configuration as it is, or the reason it is refused: one line naming
 *  the bad key, for the exit-78 line and for a caller's error alike. */
export function parseConfig(raw: unknown): Config | string {
  if (raw === null || raw === undefined) return { aliases: {} };
  if (typeof raw !== "object" || Array.isArray(raw)) return "the configuration is not an object";
  const cfg = raw as { [key: string]: unknown };
  for (const key of Object.keys(cfg)) {
    if (key !== "default" && key !== "aliases") return `${key} is not a configuration key`;
  }
  const aliases: Record<string, string> = {};
  if (cfg.aliases !== undefined) {
    if (typeof cfg.aliases !== "object" || cfg.aliases === null || Array.isArray(cfg.aliases)) return "aliases is not an object";
    for (const [key, value] of Object.entries(cfg.aliases)) {
      if (!key.startsWith("#") && !key.startsWith("@")) return `aliases: ${key} has neither # nor @`;
      if (typeof value !== "string" || !DECIMAL.test(value)) return `aliases: ${key} is not a decimal chat id`;
      aliases[key] = value;
    }
  }
  if (cfg.default !== undefined) {
    if (typeof cfg.default !== "string") return "default is not a string";
    const dflt = cfg.default;
    if (dflt.startsWith("#") || dflt.startsWith("@")) {
      if (!(dflt in aliases)) return `default names ${dflt}, which is not in aliases`;
    } else if (!DECIMAL.test(dflt)) {
      return "default is neither an alias nor a decimal chat id";
    }
    return { default: dflt, aliases };
  }
  return { aliases };
}
