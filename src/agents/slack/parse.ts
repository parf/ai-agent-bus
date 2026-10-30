// One parser for both call forms (Plans/R1.2/done/slack/README.md#methods): the
// text form and the JSON form parse to the same call, and anything unknown
// or malformed is `help` — never a guess.
//
// Text: prefix tokens (method, channel, ts, …) are separated by any
// whitespace, and a tail — a message's text, an edit's text, a filter — is
// the original bytes from where it starts to the end, spaces, tabs and
// newlines kept (slack-review-S.2). JSON: a field that is absent takes its
// default; one that is present but of the wrong type or value is no call at
// all, so a bad filter or limit is never silently dropped.

export type Call =
  | { method: "post"; channel: string; text: string; thread_ts?: string; username?: string; icon?: string }
  | { method: "read"; channel: string; limit: number }
  | { method: "sub.list" }
  | { method: "sub.add"; channel: string; target: string; filter?: string; from_users?: string[] }
  | { method: "sub.delete"; id: string }
  | { method: "channels"; filter?: string; member_only?: boolean }
  | { method: "users"; filter?: string }
  | { method: "user"; who: string }
  | { method: "channel"; channel: string }
  | { method: "thread"; channel: string; ts: string; limit: number }
  | { method: "react"; channel: string; ts: string; emoji: string }
  | { method: "edit"; channel: string; ts: string; text: string }
  | { method: "delete"; channel: string; ts: string }
  | { method: "join"; channel: string }
  | { method: "whoami" }
  | { method: "refused" }
  | { method: "help"; why?: string };

export const helpCall: Call = { method: "help" };
export type MaybeCall = Call;

/** The documented bound on `read` and `thread`. */
export const maxLimit = 200;

/** Parse one call body: the shared entry for the text and JSON forms. */
export function parseCall(body: string): Call {
  const trimmed = body.trim();
  if (trimmed === "") return helpCall;
  if (trimmed.startsWith("{")) return fromJSON(trimmed);
  return fromText(body);
}

/** A field's verdict: absent, its value, or invalid. */
const BAD = Symbol("invalid");
type Field<T> = T | undefined | typeof BAD;

/** The keys each JSON method takes, besides "method" (Plans/R1.2/done/slack/README.md#methods). */
export const KEYS: Record<string, string[]> = {
  help: [], whoami: [], refused: [], "sub.list": [],
  post: ["channel", "text", "thread_ts", "username", "icon"],
  read: ["channel", "limit"],
  "sub.add": ["channel", "target", "filter", "from_users"],
  "sub.delete": ["id"],
  channels: ["filter", "member_only"],
  users: ["filter"],
  user: ["who"],
  channel: ["channel"], join: ["channel"],
  thread: ["channel", "ts", "limit"],
  react: ["channel", "ts", "emoji"],
  edit: ["channel", "ts", "text"],
  delete: ["channel", "ts"],
};

function fromJSON(body: string): Call {
  let p: any;
  try {
    p = JSON.parse(body);
  } catch {
    return helpCall;
  }
  if (!p || typeof p !== "object" || Array.isArray(p) || typeof p.method !== "string") return helpCall;
  const has = (k: string) => Object.prototype.hasOwnProperty.call(p, k) && p[k] !== undefined;
  const str = (k: string): Field<string> => (!has(k) ? undefined : typeof p[k] === "string" && p[k].trim() !== "" ? p[k] : BAD);
  const text = (k: string): Field<string> => (!has(k) ? undefined : typeof p[k] === "string" && p[k] !== "" ? p[k] : BAD);
  const ts = (k: string): Field<string> => (!has(k) ? undefined : typeof p[k] === "string" && p[k].trim() !== "" ? p[k] : BAD);
  const bool = (k: string): Field<boolean> => (!has(k) ? undefined : typeof p[k] === "boolean" ? p[k] : BAD);
  const limit = (k: string): Field<number> => (!has(k) ? undefined : Number.isSafeInteger(p[k]) && p[k] >= 1 && p[k] <= maxLimit ? p[k] : BAD);
  const filter = (k: string): Field<string> => (!has(k) ? undefined : typeof p[k] === "string" ? p[k] : BAD);
  const authors = (): Field<string[]> => !has("from_users") ? undefined : Array.isArray(p.from_users) && p.from_users.every((v: unknown) => typeof v === "string" && /^[^\s,]+$/.test(v)) ? p.from_users : BAD;
  const need = <T,>(v: Field<T>): v is T => v !== undefined && v !== BAD;
  const fine = (...vs: Field<unknown>[]) => vs.every(v => v !== BAD);
  const m = p.method;
  if (m === "sub.edit") return { method: "help", why: "sub edit was removed; use sub delete <id> and sub add <channel> <target> [filter]" };
  // A key the method does not take is refused by name, never ignored: a
  // misspelt "chanel" would otherwise post as if no channel were named.
  const allowed = Object.hasOwn(KEYS, m) ? KEYS[m] : undefined;
  const extra = allowed && Object.keys(p).find(k => k !== "method" && !allowed.includes(k));
  if (extra) return { method: "help", why: `${m} takes no "${extra}"` };
  switch (m) {
    case "help":
    case "whoami":
    case "refused":
    case "sub.list":
      return { method: m };
    case "post": {
      const [c, t, th, u, i] = [str("channel"), text("text"), ts("thread_ts"), str("username"), str("icon")];
      if (!need(c) || !need(t) || !fine(th, u, i)) return helpCall;
      return { method: m, channel: c, text: t, ...(th !== undefined ? { thread_ts: th as string } : {}), ...(u !== undefined ? { username: u as string } : {}), ...(i !== undefined ? { icon: i as string } : {}) };
    }
    case "read": {
      const [c, l] = [str("channel"), limit("limit")];
      if (!need(c) || !fine(l)) return helpCall;
      return { method: m, channel: c, limit: (l as number | undefined) ?? 20 };
    }
    case "sub.add": {
      const [c, t, f, a] = [str("channel"), str("target"), filter("filter"), authors()];
      if (!need(c) || !need(t) || !fine(f, a)) return helpCall;
      return { method: m, channel: c, target: t, ...(a !== undefined ? { from_users: a as string[] } : {}), ...(f !== undefined && f !== "" ? { filter: f as string } : {}) };
    }
    case "sub.delete": {
      const id = str("id");
      return need(id) ? { method: m, id } : helpCall;
    }
    case "channels": {
      const [f, mo] = [str("filter"), bool("member_only")];
      if (!fine(f, mo)) return helpCall;
      return { method: m, ...(f !== undefined ? { filter: f as string } : {}), ...(mo !== undefined ? { member_only: mo as boolean } : {}) };
    }
    case "users": {
      const f = str("filter");
      if (!fine(f)) return helpCall;
      return { method: m, ...(f !== undefined ? { filter: f as string } : {}) };
    }
    case "user": {
      const w = str("who");
      return need(w) ? { method: m, who: w } : helpCall;
    }
    case "channel":
    case "join": {
      const c = str("channel");
      return need(c) ? { method: m, channel: c } : helpCall;
    }
    case "thread": {
      const [c, at, l] = [str("channel"), ts("ts"), limit("limit")];
      if (!need(c) || !need(at) || !fine(l)) return helpCall;
      return { method: m, channel: c, ts: at, limit: (l as number | undefined) ?? 20 };
    }
    case "react": {
      const [c, at, e] = [str("channel"), ts("ts"), str("emoji")];
      return need(c) && need(at) && need(e) ? { method: m, channel: c, ts: at, emoji: e } : helpCall;
    }
    case "edit": {
      const [c, at, t] = [str("channel"), ts("ts"), text("text")];
      return need(c) && need(at) && need(t) ? { method: m, channel: c, ts: at, text: t } : helpCall;
    }
    case "delete": {
      const [c, at] = [str("channel"), ts("ts")];
      return need(c) && need(at) ? { method: m, channel: c, ts: at } : helpCall;
    }
    default:
      return helpCall;
  }
}

/** The body's whitespace-separated tokens, each with where it starts. */
function tokens(body: string): { word: string; at: number }[] {
  const out: { word: string; at: number }[] = [];
  for (const m of body.matchAll(/\S+/g)) out.push({ word: m[0], at: m.index! });
  return out;
}

function fromText(body: string): Call {
  const toks = tokens(body);
  const word = (i: number): string | undefined => toks[i]?.word;
  const n = toks.length;
  // The original bytes from token i to the end, trailing whitespace dropped.
  const tail = (i: number): string => (i < n ? body.slice(toks[i]!.at).replace(/\s+$/, "") : "");
  const count = (i: number): number | undefined => {
    const v = Number(word(i));
    return word(i) !== undefined && /^\d+$/.test(word(i)!) && v >= 1 && v <= maxLimit ? v : undefined;
  };
  const [first, second] = [word(0), word(1)];
  if (first === "sub") {
    switch (second) {
      case "list":
        return n === 2 ? { method: "sub.list" } : helpCall;
      case "add": {
        const [channel, target] = [word(2), word(3)];
        if (!channel || !target) return helpCall;
        const from = word(4)?.startsWith("from=") ? word(4)!.slice(5) : undefined;
        const from_users = from === undefined ? undefined : from === "" ? [] : from.split(",");
        if (from_users?.some(v => !v)) return helpCall;
        const filter = tail(from === undefined ? 4 : 5);
        return { method: "sub.add", channel, target, ...(from_users !== undefined ? { from_users } : {}), ...(filter !== "" ? { filter } : {}) };
      }
      case "edit":
        return { method: "help", why: "sub edit was removed; use sub delete <id> and sub add <channel> <target> [filter]" };
      case "delete":
        return n === 3 ? { method: "sub.delete", id: word(2)! } : helpCall;
      default:
        return helpCall;
    }
  }
  switch (first) {
    case "help":
      return helpCall;
    case "whoami":
    case "refused":
      return n === 1 ? { method: first } : helpCall;
    case "post": {
      const channel = word(1);
      const text = tail(2);
      return channel && text !== "" ? { method: "post", channel, text } : helpCall;
    }
    case "read": {
      if (n < 2 || n > 3) return helpCall;
      const limit = n === 3 ? count(2) : 20;
      return limit !== undefined ? { method: "read", channel: word(1)!, limit } : helpCall;
    }
    case "channels":
    case "users":
      return n <= 2 ? { method: first, ...(word(1) !== undefined ? { filter: word(1)! } : {}) } : helpCall;
    case "user":
      return n === 2 ? { method: "user", who: word(1)! } : helpCall;
    case "channel":
      return n === 2 ? { method: "channel", channel: word(1)! } : helpCall;
    case "join":
      return n === 2 ? { method: "join", channel: word(1)! } : helpCall;
    case "thread": {
      if (n < 3 || n > 4) return helpCall;
      const limit = n === 4 ? count(3) : 20;
      return limit !== undefined ? { method: "thread", channel: word(1)!, ts: word(2)!, limit } : helpCall;
    }
    case "react":
      return n === 4 ? { method: "react", channel: word(1)!, ts: word(2)!, emoji: word(3)! } : helpCall;
    case "edit": {
      const text = tail(3);
      return n >= 4 && text !== "" ? { method: "edit", channel: word(1)!, ts: word(2)!, text } : helpCall;
    }
    case "delete":
      return n === 3 ? { method: "delete", channel: word(1)!, ts: word(2)! } : helpCall;
    default:
      return helpCall;
  }
}

