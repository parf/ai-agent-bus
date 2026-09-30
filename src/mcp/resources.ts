// MCP Resources: 📚 cards on the bus, passed to MCP one to one. A card is
// information, not a service: a read is answered by the card's source — an
// Agent asked over the bus, an MCP server forwarded to, or, for a plain
// https:// card, the URL itself. The face resolves; the daemon serves no
// content (docs/03-records.md#resource-records).
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";
import { ErrorCode, McpError } from "@modelcontextprotocol/sdk/types.js";
import type { Bus, Record_ } from "./bus.ts";
import { version } from "./version.ts";

type Content = { uri: string; mimeType?: string; text?: string; blob?: string };
export type ReadResult = { contents: Content[]; ttlMs?: number; cacheScope: "public" | "private" };

export const READ_TOPIC = "resources/read";
const READ_WAIT_MS = 30_000;
const FETCH_MAX = 10 << 20;

/** The card as MCP lists it: every descriptor field, the record's description included. */
export function descriptor(r: Record_): Record<string, unknown> {
  const c = r.resource!;
  return Object.fromEntries(Object.entries({
    [c.template ? "uriTemplate" : "uri"]: c.uri,
    name: c.name || r.name,
    title: c.title,
    description: r.descr,
    mimeType: c.mimeType,
    size: c.template ? undefined : c.size || undefined,
    icons: c.icons,
    annotations: c.annotations,
  }).filter(([, v]) => v !== undefined && v !== ""));
}

/** At most what the ACL allows: public only when the card admits everyone. */
export const cacheScope = (r: Record_): "public" | "private" => (r.allow?.includes("*") ? "public" : "private");

/**
 * Whether uri is one the RFC 6570 template expands to: {+x} and {#x} take any
 * characters, the rest one segment. Simple and reserved expansion only; the
 * other operators ({.x} {/x} {;x} {?x} {&x}) are matched as one segment.
 */
export function matches(template: string, uri: string): boolean {
  const re = template
    .split(/(\{[^}]*\})/)
    .map((p) => (p.startsWith("{") ? (/^\{[+#]/.test(p) ? ".+" : "[^/?#]+") : p.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")))
    .join("");
  return new RegExp(`^${re}$`).test(uri);
}

/** The card for uri: an exact one first, then the first listed template that matches — first, not most specific. */
export function find(cards: Record_[], uri: string): Record_ | undefined {
  return cards.find((r) => !r.resource!.template && r.resource!.uri === uri)
    ?? cards.find((r) => r.resource!.template && matches(r.resource!.uri, uri));
}

export async function cards(bus: Bus): Promise<Record_[]> {
  return (await bus.ls("resource")).filter((r) => r.resource);
}

const refused = (uri: string, why = "Resource not found") => new McpError(ErrorCode.InvalidParams, why, { uri });

/** resources/read: find the card the caller may see, and ask its source. */
export async function read(bus: Bus, uri: string, signal?: AbortSignal): Promise<ReadResult> {
  const r = find(await cards(bus), uri);
  if (!r) throw refused(uri);
  const { source, mimeType } = r.resource!;
  let got: Omit<ReadResult, "cacheScope"> & { cacheScope?: string };
  try {
    got = !source ? await fetched(uri, mimeType, signal)
      : source.startsWith("#") ? await fromAgent(bus, source, uri, mimeType, signal)
      : await fromService(bus, source, uri);
  } catch (err) {
    throw err instanceof McpError ? err : refused(uri, `${source || uri} did not answer: ${(err as Error).message}`);
  }
  if (!got.contents?.length) throw refused(uri, `${source || uri} had nothing for this uri`);
  return { ...got, cacheScope: got.cacheScope === "private" ? "private" : cacheScope(r) };
}

// An Agent source is asked over the bus: the body is the uri, and the answer
// is the contents — plain text, or {contents, ttlMs, cacheScope} as JSON.
async function fromAgent(bus: Bus, agent: string, uri: string, mimeType: string | undefined, signal?: AbortSignal) {
  const tag = crypto.randomUUID();
  await bus.send({ to: agent, body: uri, topic: READ_TOPIC, tag });
  const deadline = Date.now() + READ_WAIT_MS;
  for (let left = READ_WAIT_MS; left > 0; left = deadline - Date.now()) {
    const e = await bus.consume({ topic: READ_TOPIC, tag, wait: `${left}ms` }, signal);
    if (!e || e.receipt === "done") break;
    if (e.receipt) continue;
    const reply = parsed(e.body ?? "");
    return Array.isArray(reply?.contents)
      ? { contents: reply.contents as Content[], ttlMs: typeof reply.ttlMs === "number" ? reply.ttlMs : undefined, cacheScope: reply.cacheScope as string | undefined }
      : { contents: [{ uri, ...(mimeType ? { mimeType } : {}), text: e.body ?? "" }] };
  }
  throw refused(uri, `${agent} did not answer within ${READ_WAIT_MS / 1000}s`);
}

// A Service source is an MCP server: the read is forwarded, with the
// Service's own secret (MCP_AUTHORIZATION=…) as its Authorization header.
async function fromService(bus: Bus, name: string, uri: string) {
  const svc = (await bus.ls("service")).find((r) => r.name === name);
  if (!svc || svc.protocol?.toLowerCase() !== "mcp" || !svc.addr) throw refused(uri, `${name} is not an mcp service this caller may reach`);
  const auth = (await bus.secret(name).catch(() => "")).match(/^MCP_AUTHORIZATION=(.*)$/m)?.[1];
  const client = new Client({ name: "agent-bus", version });
  await client.connect(new StreamableHTTPClientTransport(new URL(svc.addr), auth ? { requestInit: { headers: { Authorization: auth } } } : {}));
  try {
    return (await client.readResource({ uri })) as Omit<ReadResult, "cacheScope">;
  } finally {
    await client.close();
  }
}

// A plain https:// card is fetched by the face itself.
async function fetched(uri: string, mimeType: string | undefined, signal?: AbortSignal) {
  const res = await fetch(uri, { signal });
  if (!res.ok) throw refused(uri, `${uri} answered ${res.status}`);
  if (Number(res.headers.get("content-length") ?? 0) > FETCH_MAX) throw refused(uri, `${uri} is larger than ${FETCH_MAX >> 20} MiB`);
  const bytes = new Uint8Array(await res.arrayBuffer());
  if (bytes.length > FETCH_MAX) throw refused(uri, `${uri} is larger than ${FETCH_MAX >> 20} MiB`);
  const type = res.headers.get("content-type")?.split(";")[0] || mimeType || "application/octet-stream";
  const text = /^text\/|json|xml|javascript|yaml/.test(type);
  return {
    contents: [{ uri, mimeType: type, ...(text ? { text: new TextDecoder().decode(bytes) } : { blob: Buffer.from(bytes).toString("base64") }) }],
  };
}

function parsed(body: string): Record<string, unknown> | undefined {
  try {
    const v = JSON.parse(body);
    return v && typeof v === "object" ? v : undefined;
  } catch {
    return undefined;
  }
}
