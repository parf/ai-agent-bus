// One request: its visitor's session, and every daemon answer fetched for it.
// Nothing survives the request. Answers used by more than one part of a page
// are fetched once per request, never cached across requests.
import { Daemon, Refusal, Unreachable, type CallOptions } from "./daemon.ts";

export const COOKIE = "agent_bus_session";

export class SignInRequired extends Error {
  constructor(readonly message: string, readonly status = 401) { super(message); }
}
/** Input the face rejected before any daemon call. */
export class LocalProblem extends Error {
  constructor(readonly status: number, readonly detail: string, readonly saved = false) { super(detail); }
}
/** A Danger Zone recheck found the record changed since the confirmation. */
export class ConditionsChanged extends Error {
  constructor(readonly detail: string, readonly back: string) { super(detail); }
}
export class NotFound extends Error {}

export type Identity = {
  version?: string; build_info?: string; owner?: string; up?: string; host?: string;
  calls?: { total?: number; windows?: { window: string; observed: string; available: boolean; count: number }[] };
};

export type Status = {
  up?: string; services?: number; queued?: number; waiting?: number; dropped?: number; expired?: number;
  kinds?: Record<string, number>; refused?: Record<string, number>; unclean?: boolean;
  owner_inactive?: { records: number; messages: number };
  you: string; administrator?: boolean; daemon_owner?: boolean;
  today?: number; activity_days_kept?: number;
};

export type Rec = {
  name: string; kind: string; descr?: string; owner: string; maintainers?: string[]; personal?: boolean;
  status?: string; at?: string; created_at?: string; addr?: string; protocol?: string;
  ttl?: string; bound?: number; overflow?: string; allow?: string[]; subs?: string[];
  secret_sha?: string; config_sha?: string; can_manage?: boolean; can_transfer?: boolean;
  route_allowed?: boolean; readers?: number; queued?: number; in?: number; out?: number;
  dropped?: number; expired?: number; oldest?: string; at_bound?: boolean; last_used?: string;
};

export type UserRow = {
  name: string; kind: string; person_name?: string; email?: string; status?: string;
  github_user?: string; github_company?: string; github_location?: string; github_twitter_username?: string;
  photo_png?: string; administrator?: boolean; daemon_owner?: boolean;
  can_edit?: boolean; can_activate?: boolean; can_remove?: boolean; can_set_email?: boolean;
  groups?: string[]; services?: string[]; created_at?: string; updated_at?: string;
};

export function parseCookies(h: string | null): Record<string, string> {
  const out: Record<string, string> = {};
  for (const part of (h ?? "").split(";")) {
    const i = part.indexOf("=");
    if (i <= 0) continue;
    // A malformed escape is a cookie we did not set: ignore it, never fail the request.
    try { out[part.slice(0, i).trim()] = decodeURIComponent(part.slice(i + 1).trim()); } catch {}
  }
  return out;
}

export class Ctx {
  readonly url: URL;
  readonly cookies: Record<string, string>;
  readonly session: string;
  readonly started = new Date();
  readonly setCookies: string[] = [];
  private memo = new Map<string, Promise<unknown>>();
  form?: URLSearchParams;

  constructor(readonly req: Request, readonly daemon: Daemon, readonly tls: boolean) {
    this.url = new URL(req.url);
    this.cookies = parseCookies(req.headers.get("cookie"));
    // The daemon's session id is hex; anything else is not a session this face set.
    const sid = this.cookies[COOKIE] ?? "";
    this.session = /^[0-9a-f]{8,128}$/.test(sid) ? sid : "";
  }

  get signedIn() { return this.session !== ""; }
  get method() { return this.req.method === "HEAD" ? "GET" : this.req.method; }
  get path() { return this.url.pathname; }
  q(name: string) { return this.url.searchParams.get(name)?.trim() ?? ""; }
  f(name: string) { return this.form?.get(name) ?? ""; }
  /** The request URI, path and query, as the sign-in form carries it back. */
  get uri() { return this.url.pathname + this.url.search; }

  private once<T>(key: string, fn: () => Promise<T>): Promise<T> {
    let p = this.memo.get(key) as Promise<T> | undefined;
    if (!p) { p = fn(); this.memo.set(key, p); p.catch(() => {}); }
    return p;
  }

  /** The node's published facts, asked anonymously; never an error. */
  identity(): Promise<Identity | null> {
    return this.once("identity", () => this.daemon.call<Identity>("GET", "/identity", { timeoutMs: 3000 }).catch(() => null));
  }

  /** A call as the visitor. No session means no call at all. */
  bus<T = any>(method: string, path: string, o: Omit<CallOptions, "cred"> = {}): Promise<T> {
    if (!this.session) return Promise.reject(new SignInRequired("sign in to open this page"));
    return this.daemon.call<T>(method, path, { ...o, cred: this.session });
  }

  get<T = any>(path: string, query?: Record<string, string | undefined>): Promise<T> {
    const key = path + (query ? "?" + new URLSearchParams(query as Record<string, string>) : "");
    return this.once(key, () => this.bus<T>("GET", path, { query }));
  }

  status() { return this.get<Status>("/status"); }
  ls() { return this.get<Rec[]>("/ls").then(r => r ?? []); }
  inactive() { return this.get<Rec[]>("/inactive").then(r => (r ?? []).map(x => ({ ...x, status: "inactive" }))); }
  users() { return this.get<UserRow[]>("/users").then(r => r ?? []); }
  groups() { return this.get<Record<string, string[] | null>>("/groups").then(r => r ?? {}); }
  lookup(name: string) { return this.get<Rec>("/lookup", { name }); }

  /** Every record the visitor may see, inactive ones marked. */
  async records(): Promise<Rec[]> {
    const [a, i] = await Promise.all([this.ls(), this.inactive()]);
    const seen = new Set(a.map(r => r.name));
    return [...a, ...i.filter(r => !seen.has(r.name))];
  }

  /** The visitor's name, when a status answer is to be had; never throws. */
  async you(): Promise<string> {
    try { return (await this.status()).you; } catch { return ""; }
  }

  forget(path: string) { for (const k of [...this.memo.keys()]) if (k === path || k.startsWith(path + "?")) this.memo.delete(k); }
}

export { Refusal, Unreachable };
