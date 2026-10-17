// One parser for both call forms (Plans/R1.2/slack/README.md#methods): the
// text form and the JSON form parse to the same call, and anything unknown
// or malformed is `help` — never a guess.
//
// Text quirks the README pins: a filter runs to the end of the line, in
// `sub add` after the target and in `sub edit` after `filter=`; timestamps
// stay strings; `read`/`thread` take N.

export type Call =
  | { method: "post"; channel: string; text: string; thread_ts?: string; username?: string; icon?: string }
  | { method: "read"; channel: string; limit: number }
  | { method: "sub.list" }
  | { method: "sub.add"; channel: string; target: string; filter?: string }
  | { method: "sub.edit"; id: string; edits: { channel?: string; target?: string; filter?: string } }
  | { method: "sub.delete"; id: string }
  | { method: "channels"; filter?: string }
  | { method: "users"; filter?: string }
  | { method: "user"; who: string }
  | { method: "channel"; channel: string }
  | { method: "thread"; channel: string; ts: string; limit: number }
  | { method: "react"; channel: string; ts: string; emoji: string }
  | { method: "edit"; channel: string; ts: string; text: string }
  | { method: "delete"; channel: string; ts: string }
  | { method: "join"; channel: string }
  | { method: "whoami" }
  | { method: "help" };

export const helpCall: Call = { method: "help" };
export type MaybeCall = Call | typeof helpCall;

const dotted = new Set(["sub.list", "sub.add", "sub.edit", "sub.delete"]);

/** Parse one call body: the shared entry for the text and JSON forms. */
export function parseCall(body: string): MaybeCall {
  const trimmed = body.trim();
  if (trimmed === "") return helpCall;
  if (trimmed.startsWith("{")) return fromJSON(trimmed);
  return fromText(trimmed);
}

function fromJSON(body: string): MaybeCall {
  let parsed: any;
  try {
    parsed = JSON.parse(body);
  } catch {
    return helpCall;
  }
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed) || typeof parsed.method !== "string") return helpCall;
  const method = parsed.method;
  const str = (v: unknown): string | undefined => (typeof v === "string" && v.trim() !== "" ? v : undefined);
  const num = (v: unknown): number | undefined => (typeof v === "number" && Number.isInteger(v) && v > 0 ? v : undefined);
  const ts = (v: unknown): string | undefined => (typeof v === "string" || typeof v === "number" ? String(v) : undefined);
  switch (method) {
    case "help":
    case "whoami":
    case "sub.list":
      return { method };
    case "post": {
      const channel = str(parsed.channel);
      const text = typeof parsed.text === "string" ? parsed.text : undefined;
      if (!channel || text === undefined) return helpCall;
      return { method, channel, text, thread_ts: ts(parsed.thread_ts), username: str(parsed.username), icon: str(parsed.icon) };
    }
    case "read":
      return str(parsed.channel) ? { method, channel: parsed.channel, limit: num(parsed.limit) ?? 20 } : helpCall;
    case "sub.add": {
      const channel = str(parsed.channel);
      const target = str(parsed.target);
      if (!channel || !target) return helpCall;
      return { method, channel, target, filter: str(parsed.filter) };
    }
    case "sub.edit": {
      const id = str(parsed.id);
      if (!id) return helpCall;
      const edits: { channel?: string; target?: string; filter?: string } = {};
      if ("channel" in parsed && parsed.channel !== undefined) { const c = str(parsed.channel); if (!c) return helpCall; edits.channel = c; }
      if ("target" in parsed && parsed.target !== undefined) { const t = str(parsed.target); if (!t) return helpCall; edits.target = t; }
      if ("filter" in parsed && parsed.filter !== undefined) { if (typeof parsed.filter !== "string") return helpCall; edits.filter = parsed.filter; }
      if (Object.keys(edits).length === 0) return helpCall; // nothing named is not an edit
      return { method, id, edits };
    }
    case "sub.delete":
      return str(parsed.id) ? { method, id: parsed.id } : helpCall;
    case "channels":
    case "users":
      return { method, filter: str(parsed.filter) };
    case "user":
      return str(parsed.who) ? { method, who: parsed.who } : helpCall;
    case "channel":
      return str(parsed.channel) ? { method, channel: parsed.channel } : helpCall;
    case "thread": {
      const channel = str(parsed.channel);
      const at = ts(parsed.ts);
      if (!channel || at === undefined) return helpCall;
      return { method, channel, ts: at, limit: num(parsed.limit) ?? 20 };
    }
    case "react": {
      const channel = str(parsed.channel);
      const at = ts(parsed.ts);
      const emoji = str(parsed.emoji);
      return channel && at !== undefined && emoji ? { method, channel, ts: at, emoji } : helpCall;
    }
    case "edit": {
      const channel = str(parsed.channel);
      const at = ts(parsed.ts);
      const text = typeof parsed.text === "string" ? parsed.text : undefined;
      return channel && at !== undefined && text !== undefined ? { method, channel, ts: at, text } : helpCall;
    }
    case "delete": {
      const channel = str(parsed.channel);
      const at = ts(parsed.ts);
      return channel && at !== undefined ? { method, channel, ts: at } : helpCall;
    }
    case "join":
      return str(parsed.channel) ? { method, channel: parsed.channel } : helpCall;
    default:
      return dotted.has(method) ? helpCall : helpCall;
  }
}

function fromText(body: string): MaybeCall {
  // `sub edit`'s filter runs to the end of the line: split conservatively.
  const words = body.split(/\s+/);
  const [first, second] = words;
  const rest = (from: number) => words.slice(from).join(" ");
  const one = (i: number): string | undefined => words[i] && words[i] !== "" ? words[i] : undefined;
  const count = (i: number): number | undefined => {
    const n = Number(words[i]);
    return words[i] !== undefined && Number.isInteger(n) && n > 0 ? n : undefined;
  };
  if (first === "sub") {
    switch (second) {
      case "list":
        return words.length === 2 ? { method: "sub.list" } : helpCall;
      case "add": {
        const channel = one(2);
        const target = one(3);
        if (!channel || !target) return helpCall;
        const filter = words.slice(4).join(" ");
        return { method: "sub.add", channel, target, filter: filter || undefined };
      }
      case "edit":
        return subEdit(body);
      case "delete": {
        const id = one(2);
        return id && words.length === 3 ? { method: "sub.delete", id } : helpCall;
      }
      default:
        return helpCall;
    }
  }
  switch (first) {
    case "help":
      return words.length === 1 ? helpCall : helpCall;
    case "whoami":
      return { method: "whoami" };
    case "post": {
      const channel = one(1);
      const text = words.slice(2).join(" ");
      if (!channel || text === "") return helpCall;
      return { method: "post", channel, text };
    }
    case "read": {
      const channel = one(1);
      if (!channel || words.length > 3) return helpCall;
      const limit = words.length === 3 ? count(2) : undefined;
      if (words.length === 3 && limit === undefined) return helpCall;
      return { method: "read", channel, limit: limit ?? 20 };
    }
    case "channels":
    case "users":
      return words.length <= 2 ? { method: first, filter: one(1) } : helpCall;
    case "user":
      return one(1) && words.length === 2 ? { method: "user", who: words[1]! } : helpCall;
    case "channel":
      return one(1) && words.length === 2 ? { method: "channel", channel: words[1]! } : helpCall;
    case "thread": {
      const channel = one(1);
      const at = one(2);
      if (!channel || !at || words.length > 4) return helpCall;
      const limit = words.length === 4 ? count(3) : undefined;
      if (words.length === 4 && limit === undefined) return helpCall;
      return { method: "thread", channel, ts: at, limit: limit ?? 20 };
    }
    case "react": {
      const channel = one(1);
      const at = one(2);
      const emoji = one(3);
      return channel && at && emoji && words.length === 4 ? { method: "react", channel, ts: at, emoji } : helpCall;
    }
    case "edit": {
      const channel = one(1);
      const at = one(2);
      const text = rest(3);
      if (!channel || !at || text === "") return helpCall;
      return { method: "edit", channel, ts: at, text };
    }
    case "delete": {
      const channel = one(1);
      const at = one(2);
      return channel && at && words.length === 3 ? { method: "delete", channel, ts: at } : helpCall;
    }
    case "join":
      return one(1) && words.length === 2 ? { method: "join", channel: words[1]! } : helpCall;
    default:
      return helpCall;
  }
}

/** `sub edit <id> [channel=…] [target=…] [filter=…]`, the filter to the end of the line. */
function subEdit(body: string): MaybeCall {
  const after = body.slice(body.indexOf("sub edit") + "sub edit".length).trim();
  const id = after.split(/\s+/)[0];
  if (!id) return helpCall;
  const remainder = after.slice(id.length).trim();
  const edits: { channel?: string; target?: string; filter?: string } = {};
  if (remainder === "") return helpCall; // nothing named is not an edit
  const filterAt = remainder.search(/(?:^|\s)filter=/);
  const pairs = filterAt < 0 ? remainder : remainder.slice(0, filterAt).trim();
  if (filterAt >= 0) edits.filter = remainder.slice(remainder.indexOf("filter=", filterAt) + "filter=".length);
  for (const pair of pairs === "" ? [] : pairs.split(/\s+/)) {
    const eq = pair.indexOf("=");
    if (eq <= 0) return helpCall;
    const key = pair.slice(0, eq);
    const value = pair.slice(eq + 1);
    if (value === "") return helpCall;
    if (key === "channel") edits.channel = value;
    else if (key === "target") edits.target = value;
    else return helpCall;
  }
  if (Object.keys(edits).length === 0) return helpCall;
  return { method: "sub.edit", id, edits };
}
