import { randomUUID } from "node:crypto";

export const SMALL_BODY = 4096;

// Four capabilities: rename this launcher, name a fork of its session, set its terminal label, and take a
// runtime hook's event for the tab's state marker. Never accepts an arbitrary bus address or principal. The
// credential is kept in the existing private session file.
export function serveControl(rename: (title?: string) => Promise<string>, fork?: (id: string) => Promise<string>,
  label?: (text?: string) => { current: string; recommended: string }, state?: (event: Record<string, unknown>) => void) {
  const token = randomUUID();
  const server = Bun.serve({
    // A Stop hook carries the last answer; the other capabilities stay small by their own checks.
    hostname: "127.0.0.1", port: 0, maxRequestBodySize: 1 << 20,
    async fetch(req) {
      if (req.headers.get("authorization") !== `Bearer ${token}`) return new Response("unauthorized", { status: 401 });
      const path = new URL(req.url).pathname;
      // Everything but /state keeps the old 4 KiB bound, in bytes: refused from
      // Content-Length before any read, else when the read passes it.
      const small = async () => {
        if (Number(req.headers.get("content-length") ?? 0) > SMALL_BODY) throw new Error("request body too large");
        const chunks: Uint8Array[] = [];
        let size = 0;
        for await (const chunk of req.body ?? []) {
          size += chunk.byteLength;
          if (size > SMALL_BODY) throw new Error("request body too large");
          chunks.push(chunk);
        }
        return JSON.parse(Buffer.concat(chunks).toString("utf8"));
      };
      // The second capability: a forked session asks for a name of its own. It
      // names its session id, never an address or a principal, and is answered
      // with the name only; its credential is written by the launcher into the
      // private run directory.
      if (req.method === "POST" && path === "/fork" && fork) {
        try {
          const input = await small() as { id?: unknown };
          if (typeof input.id !== "string" || !/^[0-9a-f-]{36}$/i.test(input.id)) return new Response("id must be a session id", { status: 400 });
          return new Response(await fork(input.id));
        } catch (e) { return new Response(e instanceof Error ? e.message : String(e), { status: 400 }); }
      }
      // The fourth: a Claude HTTP hook. Its answer is always 204 and empty, so
      // nothing in it can read as a hook decision; it sets the tab marker only.
      if (req.method === "POST" && path === "/state" && state) {
        try {
          const input = await req.json();
          if (input && typeof input === "object" && !Array.isArray(input)) state(input as Record<string, unknown>);
        } catch { /* a state marker is never worth a hook error */ }
        return new Response(null, { status: 204 });
      }
      // The third: the terminal label only. It reaches neither the rename nor
      // the binding, so it is no authority over the bus name.
      if (req.method === "POST" && path === "/label" && label) {
        try {
          const input = await small() as { label?: unknown };
          if (input.label !== undefined && typeof input.label !== "string") return new Response("label must be a string", { status: 400 });
          return Response.json(label(input.label as string | undefined));
        } catch (e) { return new Response(e instanceof Error ? e.message : String(e), { status: 400 }); }
      }
      if (req.method !== "POST" || path !== "/rename") return new Response("not found", { status: 404 });
      try {
        const input = await small() as { name?: unknown };
        if (input.name !== undefined && (typeof input.name !== "string" || !input.name.trim())) return new Response("name must be a non-empty string", { status: 400 });
        return new Response(await rename(input.name as string | undefined));
      } catch (e) { return new Response(e instanceof Error ? e.message : String(e), { status: 400 }); }
    },
  });
  return { stop: () => server.stop(true), env: { AGENT_BUS_CONTROL_ADDR: `http://127.0.0.1:${server.port}`, AGENT_BUS_CONTROL_TOKEN: token } };
}
