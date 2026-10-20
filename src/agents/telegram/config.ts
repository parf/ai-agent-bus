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

export type ChatConfig = { route?: string; tags?: boolean; anyone?: boolean; handler?: "drop" };
export type Config = { default?: string; aliases: Record<string, string>; tags?: Record<string, string>; users?: Record<string, string>; route?: string; chats?: Record<string, ChatConfig> };

import { busName as BUS_NAME, agentName, userName, parseBusName } from "../shared/names.ts";
export { own } from "../shared/own.ts";
import { own } from "../shared/own.ts";

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

/** Where a call goes: the chat it names, resolved through the aliases — or
 *  the declared default when it names none — or the reason it does not go
 *  at all. An unknown alias is refused with no send; an absent destination
 *  with no `default` is refused naming the configuration key; a numeric id
 *  is used as is, declared or not (README § where things live). */
export function resolveDestination(chat: string | undefined, config: Config): { ok: true; chat_id: string; named: string } | { ok: false; error: string } {
  const alias = (name: string): string | undefined => own(config.aliases, name);
  if (chat !== undefined) {
    if (chat.startsWith("#") || chat.startsWith("@")) {
      const id = alias(chat);
      return id !== undefined ? { ok: true, chat_id: id, named: chat } : { ok: false, error: `unknown alias ${chat}` };
    }
    return { ok: true, chat_id: chat, named: chat }; // a numeric id, as is
  }
  if (config.default === undefined) return { ok: false, error: "no destination and no default in the configuration" };
  const d = config.default;
  if (d.startsWith("#") || d.startsWith("@")) {
    const id = alias(d);
    return id !== undefined ? { ok: true, chat_id: id, named: d } : { ok: false, error: `unknown alias ${d}` }; // parseConfig refuses a default alias that does not exist
  }
  return { ok: true, chat_id: d, named: d }; // a numeric default, as is
}

/** The configuration as it is, or the reason it is refused: one line naming
 *  the bad key, for the exit-78 line and for a caller's error alike. */
export function parseConfig(raw: unknown): Config | string {
  if (raw === null || raw === undefined) return { aliases: {} };
  if (typeof raw !== "object" || Array.isArray(raw)) return "the configuration is not an object";
  const cfg = Object.assign(Object.create(null), raw) as { [key: string]: unknown };
  for (const key of Object.keys(cfg)) {
    if (key !== "default" && key !== "aliases" && key !== "tags" && key !== "users" && key !== "route" && key !== "chats") return `${key} is not a configuration key`;
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
  const routing: Pick<Config, "tags" | "users" | "route" | "chats"> = {};
  if (cfg.route !== undefined) {
    if (typeof cfg.route !== "string" || !BUS_NAME.test(cfg.route)) return "route is not a bus name";
    routing.route = parseBusName(cfg.route)!;
  }
  for (const field of ["tags", "users"] as const) {
    if (cfg[field] === undefined) continue;
    if (!cfg[field] || typeof cfg[field] !== "object" || Array.isArray(cfg[field])) return `${field} is not an object`;
    const values: Record<string, string> = {};
    const senders = new Set<string>();
    for (const [key, value] of Object.entries(cfg[field] as object)) {
      if (typeof value !== "string" || (field === "tags"
        ? !/^[a-z][a-z0-9]*$/.test(key) || !agentName.test(value)
        : !/^\d+$/.test(key) || !userName.test(value))) return `${field}: ${key} is invalid`;
      if (field === "tags" && senders.has(parseBusName(value)!)) return `tags: ${value} has more than one tag`;
      senders.add(parseBusName(value)!); values[key] = parseBusName(value)!;
    }
    routing[field] = values;
  }
  if (cfg.chats !== undefined) {
    if (!cfg.chats || typeof cfg.chats !== "object" || Array.isArray(cfg.chats)) return "chats is not an object";
    routing.chats = {};
    for (const [key, value] of Object.entries(cfg.chats)) {
      const id = DECIMAL.test(key) ? key : own(aliases, key);
      if (!id) return `chats: ${key} is not a chat id or a declared alias`;
      if (Object.hasOwn(routing.chats, id)) return `chats: ${key} repeats chat ${id}`;
      if (!value || typeof value !== "object" || Array.isArray(value)) return `chats: ${key} is not an object`;
      const entry = value as Record<string, unknown>;
      for (const option of Object.keys(entry)) if (!["route", "tags", "anyone", "handler"].includes(option)) return `chats: ${key}.${option} is unknown`;
      if (Object.hasOwn(entry, "handler")) {
        if (entry.handler !== "drop") return `chats: ${key}.handler must be one of: drop`;
        if (["route", "anyone", "tags"].some(option => Object.hasOwn(entry, option))) return `chats: ${key}.handler cannot be combined with route, anyone or tags`;
      }
      if (entry.route !== undefined && (typeof entry.route !== "string" || !BUS_NAME.test(entry.route))) return `chats: ${key}.route is not a bus name`;
      for (const option of ["tags", "anyone"]) if (entry[option] !== undefined && typeof entry[option] !== "boolean") return `chats: ${key}.${option} is not a boolean`;
      routing.chats[id] = { ...entry, ...(entry.route === undefined ? {} : {route:parseBusName(entry.route)!}) } as ChatConfig;
    }
  }
  if (cfg.default !== undefined) {
    if (typeof cfg.default !== "string") return "default is not a string";
    const dflt = cfg.default;
    if (dflt.startsWith("#") || dflt.startsWith("@")) {
      if (!Object.hasOwn(aliases, dflt)) return `default names ${dflt}, which is not in aliases`;
    } else if (!DECIMAL.test(dflt)) {
      return "default is neither an alias nor a decimal chat id";
    }
    return { default: dflt, aliases, ...routing };
  }
  return { aliases, ...routing };
}
