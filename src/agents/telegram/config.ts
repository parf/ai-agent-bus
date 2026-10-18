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

export type Config = { default?: string; aliases: Record<string, string> };

const DECIMAL = /^-?\d+$/;

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
