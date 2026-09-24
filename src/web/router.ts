// The route table. A page module registers its addresses here; the server asks
// it which handler answers a request.
import type { Ctx } from "./ctx.ts";

export type Handler = (ctx: Ctx) => Promise<Response>;
type Route = { handler: Handler; auth: boolean };

const table = new Map<string, Map<string, Route>>();

/** auth: a signed-out visitor gets the sign-in page instead (401). */
export function route(method: "GET" | "POST", path: string, handler: Handler, auth = true) {
  let m = table.get(path);
  if (!m) table.set(path, (m = new Map()));
  m.set(method, { handler, auth });
}

export function match(method: string, path: string): { route?: Route; known: boolean } {
  const m = table.get(path);
  if (!m) return { known: false };
  return { route: m.get(method === "HEAD" ? "GET" : method), known: true };
}

export const paths = () => [...table.keys()];
