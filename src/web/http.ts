// Responses and the headers every one of them carries.
import { csp } from "./assets.ts";

const CSP = csp();

export function secure(res: Response, immutable = false): Response {
  const h = res.headers;
  h.set("Cache-Control", immutable ? "public, max-age=31536000, immutable" : "no-store");
  h.set("X-Content-Type-Options", "nosniff");
  h.set("Content-Security-Policy", CSP);
  h.set("Referrer-Policy", "same-origin");
  return res;
}

export function html(body: string, status = 200, cookies: string[] = []): Response {
  const res = new Response(body, { status, headers: { "Content-Type": "text/html; charset=utf-8" } });
  for (const c of cookies) res.headers.append("Set-Cookie", c);
  return res;
}

export function text(body: string, status: number): Response {
  return new Response(body + "\n", { status, headers: { "Content-Type": "text/plain; charset=utf-8" } });
}

export function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json; charset=utf-8" } });
}

export function redirect(to: string, status = 303, cookies: string[] = []): Response {
  const res = new Response(null, { status, headers: { Location: to } });
  for (const c of cookies) res.headers.append("Set-Cookie", c);
  return res;
}

/** A path and query on this host, or "/" — never a scheme, host or user. */
export function local(v: string | null | undefined): string {
  if (!v || !v.startsWith("/") || v.startsWith("//") || v.startsWith("/\\")) return "/";
  try {
    const u = new URL(v, "http://x");
    if (u.host !== "x" || u.username || u.password) return "/";
    return u.pathname + u.search + u.hash;
  } catch { return "/"; }
}

/** A local return address kept only when its path is one of the allowed ones; fragment dropped. */
export function returnTo(v: string, allowed: string[], fallback: string): string {
  const l = local(v);
  if (l === "/" && v !== "/") return fallback;
  const u = new URL(l, "http://x");
  return allowed.includes(u.pathname) ? u.pathname + u.search : fallback;
}

export function sameOrigin(req: Request, tls: boolean): boolean {
  const origin = req.headers.get("origin");
  const site = req.headers.get("sec-fetch-site");
  if (site && site !== "same-origin") return false;
  if (!origin) return false;
  try {
    const u = new URL(origin);
    return u.protocol === (tls ? "https:" : "http:") && u.host === req.headers.get("host") && !u.username && (u.pathname === "/" || u.pathname === "") && !u.search;
  } catch { return false; }
}

/** The Referer's path and query when it is this host, else "". */
export function referer(req: Request): string {
  const r = req.headers.get("referer");
  if (!r) return "";
  try {
    const u = new URL(r);
    return u.host === req.headers.get("host") ? u.pathname + u.search : "";
  } catch { return ""; }
}

export function cookie(name: string, value: string, tls: boolean, maxAge?: number): string {
  return `${name}=${encodeURIComponent(value)}; Path=/; HttpOnly; SameSite=Strict${tls ? "; Secure" : ""}${maxAge != null ? `; Max-Age=${maxAge}` : ""}`;
}
