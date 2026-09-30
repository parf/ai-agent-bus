import { afterAll, expect, test } from "bun:test";
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { WebStandardStreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/webStandardStreamableHttp.js";
import { ReadResourceRequestSchema } from "@modelcontextprotocol/sdk/types.js";
import { BusError, type Bus, type Envelope, type Record_ } from "./bus.ts";
import { cacheScope, descriptor, find, matches, read, READ_TOPIC } from "./resources.ts";

const rec = (name: string, resource: Record_["resource"], more: Partial<Record_> = {}): Record_ =>
  ({ name, kind: "resource", owner: "o", resource, ...more }) as Record_;

// A fake bus: the listing it is given, and an agent that answers with reply.
function fakeBus(records: Record_[], reply?: (e: { body: string; tag: string }) => Envelope[], secret = "") {
  const inbox: Envelope[] = [];
  const sent: { to: string; body: string; topic?: string; tag?: string }[] = [];
  const bus = {
    ls: async (kind?: string) => records.filter((r) => !kind || r.kind === kind),
    send: async (m: { to: string; body: string; topic?: string; tag?: string }) => {
      sent.push(m);
      inbox.push(...(reply?.({ body: m.body, tag: m.tag! }) ?? []));
      return { message_id: "m" } as Envelope;
    },
    consume: async (o: { topic?: string; tag?: string }) => {
      const i = inbox.findIndex((e) => e.topic === o.topic && e.tag === o.tag);
      return i < 0 ? null : inbox.splice(i, 1)[0]!;
    },
    secret: async () => secret,
  };
  return { bus: bus as unknown as Bus, sent };
}

test("a card is listed as MCP names it, one to one", () => {
  expect(descriptor(rec("notes@h", { uri: "md://notes/a.md", mimeType: "text/markdown", size: 3, title: "A" }, { descr: "a note" })))
    .toEqual({ uri: "md://notes/a.md", name: "notes@h", title: "A", description: "a note", mimeType: "text/markdown", size: 3 });
  const t = descriptor(rec("all@h", { uri: "md://notes/{+path}", template: true, name: "Notes", size: 9 }));
  expect(t).toEqual({ uriTemplate: "md://notes/{+path}", name: "Notes" });
});

test("a template matches what it expands to, and {x} is one segment", () => {
  expect(matches("md://notes/{+path}", "md://notes/a/b.md")).toBe(true);
  expect(matches("md://notes/{path}", "md://notes/a/b.md")).toBe(false);
  expect(matches("md://notes/{path}", "md://notes/b.md")).toBe(true);
  expect(matches("md://notes/{path}.md", "md://other/b.md")).toBe(false);
});

test("an exact card wins over a template", () => {
  const exact = rec("one@h", { uri: "md://n/a.md" }), tmpl = rec("all@h", { uri: "md://n/{+p}", template: true });
  expect(find([tmpl, exact], "md://n/a.md")?.name).toBe("one@h");
  expect(find([tmpl, exact], "md://n/b.md")?.name).toBe("all@h");
});

test("cacheScope is public only when the card admits everyone", () => {
  expect(cacheScope(rec("a", { uri: "x:y" }, { allow: ["*"] }))).toBe("public");
  expect(cacheScope(rec("a", { uri: "x:y" }, { allow: ["bob"] }))).toBe("private");
});

test("an agent source answers a read over the bus; plain text is the contents", async () => {
  const card = rec("notes@h", { uri: "md://notes/{+path}", template: true, source: "#reader@h", mimeType: "text/markdown" });
  const { bus, sent } = fakeBus([card], ({ body, tag }) => [
    { topic: READ_TOPIC, tag, receipt: "ack" } as Envelope,
    { topic: READ_TOPIC, tag, body: `# ${body}` } as Envelope,
  ]);
  const got = await read(bus, "md://notes/a.md");
  expect(sent[0]).toMatchObject({ to: "#reader@h", body: "md://notes/a.md", topic: READ_TOPIC });
  expect(got).toEqual({ contents: [{ uri: "md://notes/a.md", mimeType: "text/markdown", text: "# md://notes/a.md" }], cacheScope: "private" });
});

test("an agent's JSON answer passes through, and its public scope is capped by the ACL", async () => {
  const contents = [{ uri: "md://a", mimeType: "image/png", blob: "AAAA" }];
  const { bus } = fakeBus([rec("a@h", { uri: "md://a", source: "#r@h" })], ({ tag }) => [
    { topic: READ_TOPIC, tag, body: JSON.stringify({ contents, ttlMs: 500, cacheScope: "public" }) } as Envelope,
  ]);
  expect(await read(bus, "md://a")).toEqual({ contents, ttlMs: 500, cacheScope: "private" });
});

test("an answer with no contents is -32602, never an empty read", async () => {
  const { bus } = fakeBus([rec("a@h", { uri: "md://a", source: "#r@h" })], ({ tag }) => [
    { topic: READ_TOPIC, tag, body: JSON.stringify({ contents: [] }) } as Envelope,
  ]);
  await expect(read(bus, "md://a")).rejects.toMatchObject({ code: -32602 });
});

test("an unknown uri, and an agent that says done, are -32602", async () => {
  const { bus } = fakeBus([rec("a@h", { uri: "md://a", source: "#r@h" })], ({ tag }) => [{ topic: READ_TOPIC, tag, receipt: "done" } as Envelope]);
  await expect(read(bus, "md://nope")).rejects.toMatchObject({ code: -32602 });
  await expect(read(bus, "md://a")).rejects.toMatchObject({ code: -32602 });
});

// A card with no source is fetched by the face itself.
const web = Bun.serve({ port: 0, fetch: () => new Response("hello", { headers: { "content-type": "text/plain" } }) });
// An MCP server a Service source forwards to; it checks the Service's secret.
let seenAuth = "";
const mcp = Bun.serve({
  port: 0,
  async fetch(req) {
    seenAuth = req.headers.get("authorization") ?? "";
    const server = new Server({ name: "up", version: "1" }, { capabilities: { resources: {} } });
    server.setRequestHandler(ReadResourceRequestSchema, async (r) => ({ contents: [{ uri: r.params.uri, text: "from upstream" }] }));
    const t = new WebStandardStreamableHTTPServerTransport({ sessionIdGenerator: undefined, enableJsonResponse: true });
    await server.connect(t);
    return t.handleRequest(req);
  },
});
afterAll(() => { web.stop(true); mcp.stop(true); });

test("a card with no source is fetched by the face", async () => {
  const uri = `http://127.0.0.1:${web.port}/a.txt`;
  const { bus } = fakeBus([rec("w@h", { uri }, { allow: ["*"] })]);
  expect(await read(bus, uri)).toEqual({ contents: [{ uri, mimeType: "text/plain", text: "hello" }], cacheScope: "public" });
});

test("a service source is an MCP server the read is forwarded to, with the service's secret", async () => {
  const svc = { name: "up@h", kind: "service", owner: "o", protocol: "mcp", addr: `http://127.0.0.1:${mcp.port}/mcp` } as Record_;
  const { bus } = fakeBus([rec("u@h", { uri: "db://x/schema", source: "up@h" }), svc], undefined, "MCP_AUTHORIZATION=Bearer t0k\n");
  const got = await read(bus, "db://x/schema");
  expect(got.contents).toEqual([{ uri: "db://x/schema", text: "from upstream" }]);
  expect(seenAuth).toBe("Bearer t0k");
});

test("a reader who may not read a service's secret is refused, not sent on without it", async () => {
  const svc = { name: "up@h", kind: "service", owner: "o", protocol: "mcp", addr: `http://127.0.0.1:${mcp.port}/mcp` } as Record_;
  const { bus } = fakeBus([rec("u@h", { uri: "db://x/schema", source: "up@h" }), svc]);
  (bus as unknown as { secret: () => Promise<string> }).secret = async () => { throw new BusError(403, "private"); };
  seenAuth = "unset";
  await expect(read(bus, "db://x/schema")).rejects.toMatchObject({ code: -32602 });
  expect(seenAuth).toBe("unset");
});
