// One parser for both call forms (Plans/R1.0-Release/telegram/README.md
// § methods): the text form and the JSON form parse to the same call, and
// anything unknown or malformed is `help` — never a guess.
//
// Text: prefix tokens (method, chat, id, …) are separated by any whitespace,
// and a tail — a post's text, an edit's text, a filter — is the original
// bytes from where it starts to the end, spaces, tabs and newlines kept.
// On `post` and `read` the chat is optional: the first word is a chat only
// when it starts with `#` or `@` or is an integer; otherwise the whole tail
// is the text and the destination is the declared default (TG.3). JSON: a
// field that is absent takes its default; one that is present but of the
// wrong type or value is no call at all, so a bad limit is never silently
// dropped.

export type Call =
  | { method: "post"; channel?: string; text: string; reply_to?: string }
  | { method: "read"; channel?: string; limit: number }
  | { method: "channels"; filter?: string }
  | { method: "channel"; channel: string }
  | { method: "users"; filter?: string }
  | { method: "user"; who: string }
  | { method: "react"; channel: string; id: string; emoji: string }
  | { method: "edit"; channel: string; id: string; text: string }
  | { method: "delete"; channel: string; id: string }
  | { method: "sub.list" }
  | { method: "sub.add"; channel: string; target: string; filter?: string }
  | { method: "sub.edit"; id: string; edits: { channel?: string; target?: string; filter?: string } }
  | { method: "sub.delete"; id: string }
  | { method: "whoami" }
  | { method: "help" };

export const helpCall: Call = { method: "help" };

/** The documented bound on `read` (README § methods). */
export const maxLimit = 200;

/** A chat names itself: `#alias`, `@alias`, or a numeric chat id. Anything
 *  else in the first position of `post`/`read` is the text, not the chat. */
export function isChat(word: string | undefined): boolean {
  return word !== undefined && (word.startsWith("#") || word.startsWith("@") || /^-?\d+$/.test(word));
}

/** The methods the parser accepts, for the test that keeps help and the
 *  parser listing the same set (tg1-review P3-2). */
export const methods = ["post", "read", "channels", "channel", "users", "user", "react", "edit", "delete", "sub add", "sub list", "sub edit", "sub delete", "whoami", "help"] as const;

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
  const limit = (k: string): Field<number> => (!has(k) ? undefined : Number.isSafeInteger(p[k]) && p[k] >= 1 && p[k] <= maxLimit ? p[k] : BAD);
  const filter = (k: string): Field<string> => (!has(k) ? undefined : typeof p[k] === "string" ? p[k] : BAD);
  // An id arrives as a JSON number from Telegram's own answers and as a
  // word from the text form: both become the string of its digits, so the
  // two forms agree on one call.
  const id = (k: string): Field<string> => (!has(k) ? undefined : Number.isSafeInteger(p[k]) && p[k] >= 1 ? String(p[k]) : typeof p[k] === "string" && /^\d+$/.test(p[k]) ? p[k] : BAD);
  const need = <T,>(v: Field<T>): v is T => v !== undefined && v !== BAD;
  const fine = (...vs: Field<unknown>[]) => vs.every(v => v !== BAD);
  // A JSON destination must name a chat by the same rule the text form
  // uses: a bare word is not silently taken as one. The key is "channel",
  // the same keyword Slack's JSON takes (tg3 review).
  const chat = (k: string): Field<string> => {
    const v = str(k);
    return v === undefined || v === BAD ? v : isChat(v as string) ? v : BAD;
  };
  const m = p.method;
  switch (m) {
    case "help":
    case "whoami":
    case "sub.list":
      return { method: m };
    case "post": {
      const [c, t, r] = [chat("channel"), text("text"), id("reply_to")];
      if (!need(t) || !fine(c, r)) return helpCall;
      return { method: m, ...(c !== undefined ? { channel: c as string } : {}), text: t as string, ...(r !== undefined ? { reply_to: r as string } : {}) };
    }
    case "read": {
      const [c, l] = [chat("channel"), limit("limit")];
      if (!fine(c, l)) return helpCall;
      return { method: m, ...(c !== undefined ? { channel: c as string } : {}), limit: (l as number | undefined) ?? 20 };
    }
    case "channels":
    case "users": {
      const f = filter("filter");
      if (!fine(f)) return helpCall;
      return { method: m, ...(f !== undefined ? { filter: f as string } : {}) };
    }
    case "channel": {
      const c = chat("channel");
      return need(c) ? { method: m, channel: c as string } : helpCall;
    }
    case "user": {
      const w = str("who");
      return need(w) ? { method: m, who: w } : helpCall;
    }
    case "react": {
      const [c, mid, e] = [chat("channel"), id("id"), str("emoji")];
      return need(c) && need(mid) && need(e) ? { method: m, channel: c as string, id: mid as string, emoji: e } : helpCall;
    }
    case "edit": {
      const [c, mid, t] = [chat("channel"), id("id"), text("text")];
      return need(c) && need(mid) && need(t) ? { method: m, channel: c as string, id: mid as string, text: t } : helpCall;
    }
    case "delete": {
      const [c, mid] = [chat("channel"), id("id")];
      return need(c) && need(mid) ? { method: m, channel: c as string, id: mid as string } : helpCall;
    }
    case "sub.add": {
      const [c, t, f] = [chat("channel"), str("target"), filter("filter")];
      if (!need(c) || !need(t) || !fine(f)) return helpCall;
      return { method: m, channel: c as string, target: t, ...(f !== undefined && f !== "" ? { filter: f as string } : {}) };
    }
    case "sub.edit": {
      const [id, c, t, f] = [str("id"), chat("channel"), str("target"), filter("filter")];
      if (!need(id) || !fine(c, t, f)) return helpCall;
      const edits: { channel?: string; target?: string; filter?: string } = {};
      if (c !== undefined) edits.channel = c as string;
      if (t !== undefined) edits.target = t as string;
      if (f !== undefined) edits.filter = f as string;
      if (Object.keys(edits).length === 0) return helpCall; // nothing named is not an edit
      return { method: m, id, edits };
    }
    case "sub.delete": {
      const id = str("id");
      return need(id) ? { method: m, id } : helpCall;
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
        const filter = tail(4);
        return { method: "sub.add", channel, target, ...(filter !== "" ? { filter } : {}) };
      }
      case "edit":
        return subEdit(body, toks);
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
      return n === 1 ? { method: "whoami" } : helpCall;
    case "post": {
      // The first word is a chat only when it names one (#alias, @alias, or
      // an integer); otherwise it starts the text and the default takes it.
      const chat = isChat(second) ? second! : undefined;
      const text = tail(chat !== undefined ? 2 : 1);
      return text !== "" ? { method: "post", ...(chat !== undefined ? { channel: chat } : {}), text } : helpCall;
    }
    case "read": {
      // read [<chat>] [N]: a leading chat by the first-word rule, then an
      // optional count. A bare integer first word is a chat, so `read 5`
      // reads chat 5, never "the last 5 of the default".
      if (n > 3) return helpCall;
      const chat = isChat(second) ? second! : undefined;
      const limitAt = chat !== undefined ? 2 : 1;
      const limit = n > limitAt ? count(limitAt) : 20;
      return limit !== undefined ? { method: "read", ...(chat !== undefined ? { channel: chat } : {}), limit } : helpCall;
    }
    case "channels":
    case "users":
      return n <= 2 ? { method: first, ...(word(1) !== undefined ? { filter: word(1)! } : {}) } : helpCall;
    case "user":
      return n === 2 ? { method: "user", who: word(1)! } : helpCall;
    case "channel":
      return n === 2 ? { method: "channel", channel: word(1)! } : helpCall;
    case "react":
      return n === 4 ? { method: "react", channel: word(1)!, id: word(2)!, emoji: word(3)! } : helpCall;
    case "edit": {
      const text = tail(3);
      return n >= 4 && text !== "" ? { method: "edit", channel: word(1)!, id: word(2)!, text } : helpCall;
    }
    case "delete":
      return n === 3 ? { method: "delete", channel: word(1)!, id: word(2)! } : helpCall;
    default:
      return helpCall;
  }
}

/** `sub edit <id> [chat=…] [target=…] [filter=…]`; `filter=` is the last key
 *  and its value is the original bytes to the end, `filter=` alone clearing
 *  it. */
function subEdit(body: string, toks: { word: string; at: number }[]): Call {
  const id = toks[2]?.word;
  if (!id) return helpCall;
  const edits: { channel?: string; target?: string; filter?: string } = {};
  for (let i = 3; i < toks.length; i++) {
    const { word, at } = toks[i]!;
    if (word.startsWith("filter=")) {
      edits.filter = body.slice(at + "filter=".length).replace(/\s+$/, "");
      break;
    }
    const eq = word.indexOf("=");
    if (eq <= 0 || eq === word.length - 1) return helpCall;
    const key = word.slice(0, eq);
    const value = word.slice(eq + 1);
    if (key === "channel") edits.channel = value;
    else if (key === "target") edits.target = value;
    else return helpCall;
  }
  if (Object.keys(edits).length === 0) return helpCall; // nothing named is not an edit
  return { method: "sub.edit", id, edits };
}
