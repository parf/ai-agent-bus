// The TypeScript twin of the shared subscription engine. Its contract, and the
// Python engine's, is src/agents/shared/subscriptions.md; both are held to the
// same pure-operation vectors in src/testdata/subscriptions.json. An agent
// supplies only its selector (validate, match) and the message it sends;
// admission, storage, filtering, visibility and delivery accounting are here.

/** A subscription as stored under `sub.<id>`, plus `reason` when listed invalid. */
export type Row = {
  id: string; target: string; selector: Record<string, unknown>; filter: string;
  created_by: string; created_at: string; sent: number; dropped: number; reason?: "invalid_filter";
};

/** A stored row that is not a subscription: listed to managers only, inert,
 *  and repaired only by a manager's delete. */
export type Damaged = { id: string; reason: "invalid_state" };

export type ErrorCode = "invalid_target" | "invalid_selector" | "invalid_filter" | "invalid_text" | "invalid_id" | "limit" | "not_found"
  | "not_author" | "admission_refused" | "admission_unavailable" | "store_unknown";

export type WarningCode = "admission_refused" | "admission_unavailable" | "filter_invalid" | "filter_timeout" | "invalid_text"
  | "send_refused" | "send_unknown" | "deadline" | "store_unknown";

/** The stable code is the contract; the message is prose for people. */
export class SubscriptionError extends Error {
  constructor(public readonly code: ErrorCode, message: string = code) { super(message); this.name = "SubscriptionError"; }
}

/** What an admission question can answer. A 404 is a refusal; anything that
 *  is not a well-formed yes or no is unavailable, never a yes. */
export type Admission = "yes" | "no" | "unavailable";
export type SendOutcome = "ok" | "refused" | "unknown" | "deadline";
export type Verdict = "hit" | "miss" | "slow";

/** Everything the engine needs of the bus, as the source agent itself. */
export type Port = {
  admits(who: string, target: string): Promise<Admission>;
  /** The destination's kind now, or undefined when it cannot be seen. */
  kind(target: string): Promise<string | undefined>;
  /** Stored rows by key `sub.<id>`; any failure but "none" throws. */
  load(): Promise<Map<string, unknown>>;
  /** Create with mode add: false when the key is taken. */
  create(key: string, row: Row): Promise<boolean>;
  /** Delete: false when it was not there. */
  remove(key: string): Promise<boolean>;
  /** Persist one row's counters, best effort. */
  counts(key: string, sent: number, dropped: number): Promise<void>;
  send(target: string, body: string, options: { topic?: string; tag?: string; deadline?: number }): Promise<SendOutcome>;
};

/** The authenticated message a call arrived on; bodies never supply these. */
export type Caller = { from: string; roles: readonly string[]; original_to?: string };

export type Options = {
  validateSelector: (selector: Record<string, unknown>) => void;
  selectorMatches: (selector: Record<string, unknown>, event: Record<string, unknown>) => boolean;
  warning: (code: WarningCode, id?: string) => void;
  clock?: () => Date;
  idFactory?: () => string;
  /** Tests one compiled filter against text within its own time bound. */
  matcher?: (re: RegExp, text: string) => Promise<Verdict>;
  maxSubscriptions?: number;
  maxPerCreator?: number;
};

export const MAX_PATTERN = 200;
export const MAX_TEXT = 8192;
const MAX_REPEAT = 100;
const MAX_QUANTIFIERS = 20;

/** Text a filter may be tested against: well-formed and at most 8192 UTF-8 bytes. */
export function checkText(text: string): string {
  if (typeof text !== "string" || !text.isWellFormed() || new TextEncoder().encode(text).length > MAX_TEXT) {
    throw new SubscriptionError("invalid_text");
  }
  return text;
}
const ID = /^[0-9a-f]{12}$/;
const NAME = /^#?[a-z0-9][a-z0-9._+@/-]*$/;
const SAFE = (n: unknown): n is number => Number.isSafeInteger(n) && (n as number) >= 0;
const FIELDS = ["created_at", "created_by", "dropped", "filter", "id", "selector", "sent", "target"].join();
const STAMP = /^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}Z$/;
const named = (v: unknown): v is string => typeof v === "string" && v.length <= 64 && NAME.test(v);

/** A counter saturates at the largest safe integer rather than losing precision. */
const bump = (n: number) => Math.min(n + 1, Number.MAX_SAFE_INTEGER);

const randomId = () => [...crypto.getRandomValues(new Uint8Array(6))].map(b => b.toString(16).padStart(2, "0")).join("");

/** The common JS/Python filter subset (contract § common filter grammar),
 *  checked the way the IMAP agent's compile_filter checks it, then compiled
 *  as a native RegExp: the subset is chosen so JS semantics are the answer.
 *  Empty is match-all and compiles to undefined. */
export function compileFilter(filter: string): RegExp | undefined {
  const bad = (why: string): never => { throw new SubscriptionError("invalid_filter", why); };
  if (typeof filter !== "string") bad("a filter is text");
  if (filter === "") return undefined;
  if (!filter.isWellFormed()) bad("a filter holds a lone surrogate");
  let pattern = filter, flags = "";
  if (filter.startsWith("/")) {
    // After the LAST UNESCAPED slash: /a\/b/ is the pattern a\/b.
    let end = -1;
    for (let i = 1; i < filter.length; i++) {
      if (filter[i] === "\\") i++;
      else if (filter[i] === "/") end = i;
    }
    if (end < 1) bad("a /filter/ needs its closing slash");
    pattern = filter.slice(1, end); flags = filter.slice(end + 1);
  }
  if ([...pattern].length > MAX_PATTERN || [...pattern].some(c => { const n = c.codePointAt(0)!; return n < 32 || (n >= 0x7f && n <= 0x9f); })) {
    bad("a filter is at most 200 characters without controls");
  }
  if (new Set(flags).size !== flags.length || [...flags].some(f => !"imsu".includes(f))) bad("filter flags are i m s u, once each");
  if (/(?<!\\)[+*?}]\)(?:[+*?]|\{)/.test(pattern)) bad("nested quantified groups are unsupported");
  let quantifiers = 0;
  // Whether each open group holds a quantifier, and whether the group that
  // just closed did: a quantifier on such a group is refused.
  const groups: boolean[] = [];
  let closed = false;
  for (let i = 0; i < pattern.length; i++) {
    const c = pattern[i];
    if (c === "\\") {
      i++;
      if (i === pattern.length) bad("a filter ends with a backslash");
      if (!"dDwWsSnrt\\.^$|?*+(){}[]/-".includes(pattern[i])) bad("unsupported filter escape");
    } else if (c === "[") {
      const end = pattern.indexOf("]", i + 1);
      const body = end >= 0 ? pattern.slice(i + 1, end) : "";
      if (!body || !/^[\x00-\x7f]*$/.test(body) || body.includes("[") || body.includes("\\")) bad("filter classes are nonempty ASCII without escapes");
      if (["&&", "--", "~~", "||"].some(op => body.includes(op))) bad("class set operations are unsupported");
      // One hyphen at most: a range, or a lone leading or trailing literal.
      if ([...body].filter(ch => ch === "-").length > 1) bad("a class has at most one hyphen");
      i = end;
    } else if (c === "(") {
      if (pattern[i + 1] === "?") {
        if (pattern[i + 2] !== ":") bad("only ordinary and noncapturing groups are supported");
        i += 2;
      }
      groups.push(false);
    } else if (c === ")") {
      closed = groups.pop() ?? false;
      if (groups.length) groups[groups.length - 1] ||= closed;
      continue;
    } else if (c === "]") {
      bad("a lone ] is unsupported");
    } else if (c === "{") {
      if (closed) bad("a quantifier on a group that holds one is unsupported");
      if (groups.length) groups[groups.length - 1] = true;
      const q = /^\{([0-9]+)(?:,([0-9]*))?\}/.exec(pattern.slice(i));
      if (!q || [q[1], q[2]].some(n => n && Number(n) > MAX_REPEAT)) bad("invalid or excessive filter quantifier");
      if (++quantifiers > MAX_QUANTIFIERS) bad("a filter has at most 20 quantifiers");
      i += q![0].length - 1;
    } else if (c === "}" || ("*+?".includes(c) && i > 0 && "*+?}".includes(pattern[i - 1]))) {
      bad("unsupported filter quantifier");
    } else if ("*+?".includes(c)) {
      if (closed) bad("a quantifier on a group that holds one is unsupported");
      if (groups.length) groups[groups.length - 1] = true;
      if (++quantifiers > MAX_QUANTIFIERS) bad("a filter has at most 20 quantifiers");
    }
    closed = false;
  }
  try {
    // \- is escaped punctuation in the common grammar, but u-mode JS refuses
    // it as an identity escape; outside a class it is the literal -, and
    // classes hold no backslash, so the plain - means the same everywhere.
    return new RegExp(pattern.replace(/\\(.)/gsu, (all, c) => (c === "-" ? "-" : all)), flags);
  } catch {
    return bad("not a valid filter");
  }
}

const nativeMatch = async (re: RegExp, text: string): Promise<Verdict> => (re.test(text) ? "hit" : "miss");

export class Subscriptions {
  private rows = new Map<string, Row>();
  private damaged = new Map<string, Damaged>();
  private readonly clock: () => Date;
  private readonly idFactory: () => string;
  private readonly matcher: (re: RegExp, text: string) => Promise<Verdict>;
  private readonly maxSubscriptions: number;
  private readonly maxPerCreator: number;

  constructor(private readonly port: Port, readonly source: string, private readonly options: Options) {
    this.clock = options.clock ?? (() => new Date());
    this.idFactory = options.idFactory ?? randomId;
    this.matcher = options.matcher ?? nativeMatch;
    this.maxSubscriptions = options.maxSubscriptions ?? 100;
    this.maxPerCreator = options.maxPerCreator ?? 20;
  }

  /** Reads every stored row. A damaged row stays listed to its managers,
   *  inert; an unsupported filter stays visible with its reason. */
  async reload(): Promise<void> {
    const stored = await this.port.load().catch(() => { throw new SubscriptionError("store_unknown"); });
    const rows = new Map<string, Row>();
    const damaged = new Map<string, Damaged>();
    for (const [key, value] of stored) {
      const id = key.slice(4);
      if (!key.startsWith("sub.") || !ID.test(id)) continue;
      if (this.isRow(value, id)) rows.set(id, structuredClone(value));
      else damaged.set(id, { id, reason: "invalid_state" });
    }
    this.rows = rows;
    this.damaged = damaged;
  }

  /** A stored value is a row only whole: exact fields, each well-formed. */
  private isRow(value: unknown, id: string): value is Row {
    if (typeof value !== "object" || value === null || Array.isArray(value)) return false;
    const row = value as Record<string, unknown>;
    if (Object.keys(row).sort().join() !== FIELDS || row.id !== id) return false;
    if (!named(row.target) || !named(row.created_by) || typeof row.filter !== "string") return false;
    if (typeof row.created_at !== "string" || !STAMP.test(row.created_at) || Number.isNaN(Date.parse(row.created_at))) return false;
    if (!SAFE(row.sent) || !SAFE(row.dropped)) return false;
    const selector = row.selector;
    if (typeof selector !== "object" || selector === null || Array.isArray(selector)) return false;
    try {
      this.options.validateSelector(selector as Record<string, unknown>);
    } catch {
      return false;
    }
    return true;
  }

  /** Whether the caller manages this source: a daemon-asserted owner or
   *  maintainer role on a message addressed here, never through a forward. */
  manages(caller: Caller): boolean {
    const to = caller.original_to ?? "";
    if (to !== "" && to.toLowerCase() !== this.source.toLowerCase()) return false;
    return caller.roles.includes("owner") || caller.roles.includes("maintainer");
  }

  async add(caller: Caller, target: string, selector: Record<string, unknown>, filter = ""): Promise<Row> {
    // Everything that needs no network first.
    const to = typeof target === "string" ? target.trim().toLowerCase() : "";
    if (!NAME.test(to) || to.length > 64) throw new SubscriptionError("invalid_target");
    try {
      this.options.validateSelector(selector);
    } catch {
      throw new SubscriptionError("invalid_selector");
    }
    compileFilter(filter);
    const creator = caller.from;
    if (this.rows.size + this.damaged.size >= this.maxSubscriptions || [...this.rows.values()].filter(r => r.created_by === creator).length >= this.maxPerCreator) {
      throw new SubscriptionError("limit");
    }
    for (const asked of [to, this.source]) {
      const answer = await this.port.admits(creator, asked);
      if (answer === "no") throw new SubscriptionError("admission_refused");
      if (answer === "unavailable") throw new SubscriptionError("admission_unavailable");
    }
    const kind = await this.port.kind(to);
    if (kind !== "queue" && kind !== "agent") throw new SubscriptionError("invalid_target");
    const created_at = this.clock().toISOString();
    for (let attempt = 0; attempt < 3; attempt++) {
      const id = this.idFactory();
      if (!ID.test(id)) throw new SubscriptionError("invalid_id");
      const row: Row = { id, target: to, selector, filter, created_by: creator, created_at, sent: 0, dropped: 0 };
      let created: boolean;
      try {
        created = await this.port.create(`sub.${id}`, row);
      } catch {
        // Whether it was stored is unknown: read the store again, never guess.
        await this.reload().catch(() => {});
        throw new SubscriptionError("store_unknown");
      }
      if (created) {
        this.rows.set(id, row);
        return { ...row };
      }
    }
    // Three taken ids in a row: the id space is as full as the store allows.
    throw new SubscriptionError("limit");
  }

  list(caller: Caller): (Row | Damaged)[] {
    const all = this.manages(caller);
    const rows: (Row | Damaged)[] = [...this.rows.values()]
      .filter(r => all || r.created_by === caller.from)
      .map(r => (this.filterOf(r) === null ? { ...r, reason: "invalid_filter" as const } : structuredClone(r)));
    if (all) rows.push(...[...this.damaged.values()].map(d => ({ ...d })));
    return rows.sort((a, b) => (a.id < b.id ? -1 : 1));
  }

  async delete(caller: Caller, id: string): Promise<boolean> {
    if (typeof id !== "string" || !ID.test(id)) throw new SubscriptionError("invalid_id");
    const row = this.rows.get(id);
    if (!row && !this.damaged.has(id)) throw new SubscriptionError("not_found");
    // A damaged row has no creator to trust: only a manager repairs it.
    if (!this.manages(caller) && row?.created_by !== caller.from) throw new SubscriptionError("not_author");
    let gone: boolean;
    try {
      gone = await this.port.remove(`sub.${id}`); // storage first, then memory
    } catch {
      await this.reload().catch(() => {});
      throw new SubscriptionError("store_unknown");
    }
    this.rows.delete(id);
    this.damaged.delete(id);
    if (!gone) throw new SubscriptionError("not_found");
    return true;
  }

  /** One event: selector first, filter second, then one checked send per
   *  match. A refused or uncertain copy is dropped, counted and warned; the
   *  subscription stays active. Nothing is retried, held or replayed. */
  async deliver(event: Record<string, unknown>, text: string, body: string, options: { topic?: string; tag?: string; deadline?: number } = {}): Promise<{ matched: number; sent: number; dropped: number }> {
    const result = { matched: 0, sent: 0, dropped: 0 };
    const admitted = new Map<string, Admission>();
    const ask = async (who: string, target: string): Promise<Admission> => {
      const key = who + "\n" + target;
      if (!admitted.has(key)) admitted.set(key, await this.port.admits(who, target));
      return admitted.get(key)!;
    };
    // Malformed text fails closed for every selector candidate, match-all
    // included: dropped and warned, never asked about or sent.
    let sample: string | undefined;
    try {
      sample = checkText(text);
    } catch {
      sample = undefined;
    }
    const changed = new Set<Row>();
    for (const row of [...this.rows.values()].sort((a, b) => (a.id < b.id ? -1 : 1))) {
      if (!this.options.selectorMatches(row.selector, event)) continue;
      const drop = (code: WarningCode) => {
        result.dropped++;
        this.options.warning(code, row.id);
        row.dropped = bump(row.dropped);
        changed.add(row);
      };
      const expired = () => options.deadline !== undefined && Date.now() >= options.deadline;
      // An expired deadline, malformed text or an inert filter drops every
      // selector candidate before any matching or admission question.
      const late = expired();
      if (late || sample === undefined) {
        result.matched++;
        drop(late ? "deadline" : "invalid_text");
        continue;
      }
      const re = this.filterOf(row);
      if (re === null) {
        result.matched++;
        drop("filter_invalid");
        continue;
      }
      if (re) {
        const verdict = await this.matcher(re, sample);
        if (verdict === "slow") {
          this.options.warning("filter_timeout", row.id);
          continue;
        }
        if (verdict === "miss") continue;
      }
      result.matched++;
      if (expired()) {
        drop("deadline");
        continue;
      }
      // Target first; a failed first check never asks the second.
      let refused: WarningCode | undefined;
      for (const asked of [row.target, this.source]) {
        const answer = await ask(row.created_by, asked);
        if (answer !== "yes") {
          refused = answer === "no" ? "admission_refused" : "admission_unavailable";
          break;
        }
      }
      if (!refused) {
        const kind = await this.port.kind(row.target).catch(() => undefined);
        if (kind !== "queue" && kind !== "agent") refused = "admission_refused";
      }
      if (refused) {
        drop(refused);
        continue;
      }
      const outcome = await this.port.send(row.target, body, options).catch((): SendOutcome => "unknown");
      if (outcome === "ok") {
        result.sent++;
        row.sent = bump(row.sent);
        changed.add(row);
      } else {
        drop(outcome === "refused" ? "send_refused" : outcome === "deadline" ? "deadline" : "send_unknown");
      }
    }
    // Counters are written once per row after the batch, best effort: a lost
    // write leaves the durable count behind and never replays the entry.
    let lost = false;
    for (const row of changed) {
      try {
        await this.port.counts(`sub.${row.id}`, row.sent, row.dropped);
      } catch {
        lost = true;
      }
    }
    if (lost) this.options.warning("store_unknown");
    return result;
  }

  /** The row's compiled filter, undefined for match-all, null when invalid. */
  private filterOf(row: Row): RegExp | undefined | null {
    try {
      return compileFilter(row.filter ?? "");
    } catch {
      return null;
    }
  }

}
