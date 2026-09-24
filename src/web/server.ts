// agent-bus web face: a stateless renderer over the daemon's shared socket.
// Every page is built from daemon answers fetched for that request with the
// visitor's own session; the face holds no credential, session map or cache.
import { load, type Config } from "./config.ts";
import { Daemon } from "./daemon.ts";
import { Ctx, SignInRequired } from "./ctx.ts";
import { VERSION } from "./version.ts";
import { buildLocal } from "./assets.ts";
import { setAssets } from "./ui/frame.tsx";
import { secure, text, html } from "./http.ts";
import { sameOrigin } from "./http.ts";
import { match } from "./router.ts";
import { problemPage } from "./problem.tsx";
import { notFoundPage, methodPage } from "./pages/errors.tsx";
import "./pages/index.ts";

const BODY_LIMIT = 1 << 20;

export function makeHandler(cfg: Config) {
  const daemon = new Daemon(cfg.daemon);
  const local = buildLocal();
  setAssets(local);
  const statics = new Map([local.css, local.js, local.hero, local.favicon].map(a => [a.path, a]));

  return async function handle(req: Request): Promise<Response> {
    const url = new URL(req.url);
    const head = req.method === "HEAD";
    let res: Response;
    try {
      res = await dispatch(req, url);
    } catch (e) {
      console.error("web: unhandled", e);
      res = text("internal error", 500);
    }
    const immutable = res.headers.get("X-Immutable") === "1";
    res.headers.delete("X-Immutable");
    secure(res, immutable);
    return head ? new Response(null, { status: res.status, headers: res.headers }) : res;
  };

  async function dispatch(req: Request, url: URL): Promise<Response> {
    const path = url.pathname;
    if (req.method === "GET" || req.method === "HEAD") {
      if (path === "/healthz") return new Response(null, { status: 200 });
      if (path === "/favicon.ico") return text("404 page not found", 404);
      const a = statics.get(path);
      if (a) {
        const r = new Response(a.body as BodyInit, { headers: { "Content-Type": a.type } });
        if (a.path !== "/favicon.svg") r.headers.set("X-Immutable", "1");
        return r;
      }
    }
    const ctx = new Ctx(req, daemon, !!cfg.tls);
    if (req.method === "POST") {
      // Every POST, sign-in and sign-out included, must come from a page of this face.
      if (!sameOrigin(req, !!cfg.tls)) return text("403 same-origin form required", 403);
      const len = Number(req.headers.get("content-length") ?? 0);
      if (len > BODY_LIMIT) return text("413 request body too large", 413);
      const buf = await req.arrayBuffer();
      if (buf.byteLength > BODY_LIMIT) return text("413 request body too large", 413);
      try { ctx.form = new URLSearchParams(new TextDecoder().decode(buf)); }
      catch { ctx.form = new URLSearchParams(); }
    }
    const m = match(req.method, path);
    try {
      if (!m.known) return await notFoundPage(ctx);
      if (!m.route) return await methodPage(ctx);
      if (m.route.auth && !ctx.signedIn) throw new SignInRequired("sign in to open this page");
      return await m.route.handler(ctx);
    } catch (e) {
      return problemPage(ctx, e);
    }
  }
}

if (import.meta.main) {
  if (process.argv.includes("--version") || process.argv.includes("-version")) {
    console.log(VERSION);
    console.log("build_info: source");
    process.exit(0);
  }
  const cfg = load();
  const handler = makeHandler(cfg);
  const server = Bun.serve({
    hostname: cfg.listen.hostname,
    port: cfg.listen.port,
    fetch: handler,
    maxRequestBodySize: 2 << 20,
    ...(cfg.tls ? { tls: { cert: Bun.file(cfg.tls.cert), key: Bun.file(cfg.tls.key) } } : {}),
  });
  console.log(`agent-bus-web ${VERSION} listening on ${cfg.tls ? "https" : "http"}://${server.hostname}:${server.port}/ (daemon ${cfg.daemon})`);
}
