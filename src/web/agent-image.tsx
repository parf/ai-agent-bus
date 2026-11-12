// An agent's picture (Plans/R1.1/agent-images.md#how-the-web-gets-it): help names
// its hash, as the visitor; the bytes come from the installed pack on disk when
// a bundled agent's file there has that hash, else from the agent's image ask.
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { imageCheck, imageLoad } from "../agents/shared/image.ts";
import type { Ctx, Rec } from "./ctx.ts";
import { agentHelp } from "./agent-metadata.tsx";
import { h, type Child } from "./jsx.ts";
import { text } from "./http.ts";

type Picture = { data: Uint8Array; type: string };
/** The bundled pack beside the face, as the face finds VERSION; tests point it elsewhere. */
export const imagePack = { dir: join(import.meta.dir, "../agents") };
export const imageWait = { ms: 5000 };
// Keyed by SHA-256 alone: the bytes are the same for every visitor, and each
// visitor reaches them only through a help answer of their own.
const pictures = new Map<string, Picture>();
export const imageStats = { disk: 0, asked: 0 };
export function clearImageCache() { pictures.clear(); imageStats.disk = imageStats.asked = 0; }

const sha = (v: unknown) => typeof v === "string" && /^[0-9a-f]{64}$/.test(v) ? v : "";
/** The image hash help gave this visitor, or "" for none. */
export async function imageHash(ctx: Ctx, rec: Rec): Promise<string> {
  return sha((await agentHelp(ctx, rec)).data?.image_sha256);
}

/** `#name/instance@realm` → `name`, the bundled agent's folder. */
const bundledName = (name: string) => /^#([a-z0-9][a-z0-9_-]*)(?:[/@]|$)/.exec(name)?.[1] ?? "";

function fromDisk(name: string, want: string): Picture | undefined {
  const base = bundledName(name);
  if (!base) return undefined;
  try {
    const dir = join(imagePack.dir, base);
    const manifest = JSON.parse(readFileSync(join(dir, "agent.json"), "utf8")) as { image?: unknown };
    if (typeof manifest.image !== "string") return undefined;
    const img = imageLoad(dir, manifest.image);
    return img.sha256 === want ? img : undefined;
  } catch { return undefined; }
}

export async function agentPicture(ctx: Ctx, rec: Rec): Promise<Picture | undefined> {
  const want = await imageHash(ctx, rec);
  if (!want) return undefined;
  const held = pictures.get(want);
  if (held) return held;
  let got = fromDisk(rec.name, want);
  if (got) imageStats.disk++;
  else {
    const answer = await ctx.call(rec.name, "", imageWait.ms, "image").catch(() => undefined);
    if (answer?.kind !== "answer") return undefined;
    imageStats.asked++;
    try {
      const img = imageCheck(new Uint8Array(Buffer.from(answer.body, "base64")));
      if (img.sha256 !== want) return undefined;
      got = img;
    } catch { return undefined; }
  }
  if (pictures.size >= 256) pictures.delete(pictures.keys().next().value!);
  pictures.set(want, { data: got.data, type: got.type });
  return got;
}

/** GET /agent-image?name=: the picture as the visitor may see it, else 404. */
export async function imageRoute(ctx: Ctx): Promise<Response> {
  const name = ctx.q("name");
  if (!name) return text("404 page not found", 404);
  let rec: Rec;
  try { rec = await ctx.lookup(name); } catch { return text("404 page not found", 404); }
  if (rec.kind !== "agent") return text("404 page not found", 404);
  const pic = await agentPicture(ctx, rec);
  if (!pic) return text("404 page not found", 404);
  return new Response(pic.data as BodyInit, { headers: { "Content-Type": pic.type } });
}

/** A list row's thumbnail, lazily asked for and drawn over the kind icon: an
 *  agent with no picture answers 404, the empty image draws nothing, and the
 *  icon shows through. */
export const ListPicture = ({ name, hash, icon }: { name: string; hash: string; icon: Child }) =>
  <span class="thumb-stack">{icon}<img class="agent-thumb" src={`/agent-image?${new URLSearchParams(hash ? { name, h: hash } : { name })}`} alt="" width={24} height={24} loading="lazy" decoding="async" /></span>;

/** The page-header picture, or the letter placeholder when help names none. */
export const AgentPicture = ({ name, hash, size = 40 }: { name: string; hash: string; size?: number }) => hash
  ? <img class={size < 40 ? "agent-picture agent-thumb" : "agent-picture"} src={`/agent-image?${new URLSearchParams({ name, h: hash })}`} alt="" width={size} height={size} loading="lazy" decoding="async" />
  : <span class="agent-picture agent-placeholder" aria-hidden="true">{(bundledName(name) || name.replace(/^#/, "")).slice(0, 1).toUpperCase() || "?"}</span>;
