import { randomUUID } from "node:crypto";

// Two capabilities: rename this launcher, and name a fork of its session. Never accepts an arbitrary bus address
// or principal. The credential is kept in the existing private session file.
export function serveControl(rename: (title?: string) => Promise<string>, fork?: (id: string) => Promise<string>) {
  const token = randomUUID();
  const server = Bun.serve({
    hostname: "127.0.0.1", port: 0, maxRequestBodySize: 4096,
    async fetch(req) {
      if (req.headers.get("authorization") !== `Bearer ${token}`) return new Response("unauthorized", { status: 401 });
      const path = new URL(req.url).pathname;
      // The second capability: a forked session asks for a name of its own. It
      // names its session id, never an address or a principal, and is answered
      // with the name only; its credential is written by the launcher into the
      // private run directory.
      if (req.method === "POST" && path === "/fork" && fork) {
        try {
          const input = await req.json() as { id?: unknown };
          if (typeof input.id !== "string" || !/^[0-9a-f-]{36}$/i.test(input.id)) return new Response("id must be a session id", { status: 400 });
          return new Response(await fork(input.id));
        } catch (e) { return new Response(e instanceof Error ? e.message : String(e), { status: 400 }); }
      }
      if (req.method !== "POST" || path !== "/rename") return new Response("not found", { status: 404 });
      try {
        const input = await req.json() as { name?: unknown };
        if (input.name !== undefined && (typeof input.name !== "string" || !input.name.trim())) return new Response("name must be a non-empty string", { status: 400 });
        return new Response(await rename(input.name as string | undefined));
      } catch (e) { return new Response(e instanceof Error ? e.message : String(e), { status: 400 }); }
    },
  });
  return { stop: () => server.stop(true), env: { AGENT_BUS_CONTROL_ADDR: `http://127.0.0.1:${server.port}`, AGENT_BUS_CONTROL_TOKEN: token } };
}
