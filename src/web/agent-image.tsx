// An agent's picture (Plans/R1.1/agent-images.md#how-the-web-gets-it): help names
// its hash, as the visitor; the bytes come from the installed pack on disk when
// a bundled agent's file there has that hash, else from the agent's image ask.
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { imageCheck, imageLoad } from "../agents/shared/image.ts";
import type { Ctx, Rec } from "./ctx.ts";
import { agentHelp } from "./agent-metadata.tsx";
import { h } from "./jsx.ts";
import { assets } from "./ui/frame.tsx";
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

// Without a hash in help, the visitor's own image ask is the check; its outcome
// is kept per visitor and agent for a minute, as help is.
const asked = new Map<string, { until: number; sha: string }>();

export async function agentPicture(ctx: Ctx, rec: Rec): Promise<Picture | undefined> {
  let want = await imageHash(ctx, rec);
  const key = JSON.stringify([ctx.session, rec.name, rec.at]);
  if (!want) {
    const seen = asked.get(key);
    if (seen && seen.until > Date.now()) want = seen.sha;
    if (seen && !want) return undefined;
  }
  const held = want ? pictures.get(want) : undefined;
  if (held) return held;
  let got = want ? fromDisk(rec.name, want) : undefined;
  if (got) imageStats.disk++;
  else {
    if (!rec.readers) return undefined;
    const answer = await ctx.call(rec.name, "", imageWait.ms, "image").catch(() => undefined);
    imageStats.asked++;
    let img: Picture & { sha256: string } | undefined;
    if (answer?.kind === "answer") try { img = imageCheck(new Uint8Array(Buffer.from(answer.body, "base64"))); } catch {}
    if (!want) {
      if (asked.size >= 4096) asked.delete(asked.keys().next().value!);
      asked.set(key, { until: Date.now() + 60_000, sha: img?.sha256 ?? "" });
    }
    if (!img || (want && img.sha256 !== want)) return undefined;
    got = img; want = img.sha256;
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

const placeholderPath = () => assets()?.placeholder.path ?? "/agent-placeholder.webp";

/** An agent's picture over the universal placeholder: an agent without one
 *  answers 404, the empty image draws nothing, and the placeholder shows.
 *  An agent nobody serves is not asked; it shows the placeholder alone. */
export const AgentPicture = ({ name, hash, live = true, size = 40 }: { name: string; hash: string; live?: boolean; size?: number }) =>
  <span class={size < 40 ? "pic-stack pic-small" : "pic-stack"}>
    <img class="agent-picture" src={placeholderPath()} alt="" width={size} height={size} />
    {live || hash ? <img class="agent-picture" src={`/agent-image?${new URLSearchParams(hash ? { name, h: hash } : { name })}`} alt="" width={size} height={size} loading="lazy" decoding="async" /> : null}
  </span>;
