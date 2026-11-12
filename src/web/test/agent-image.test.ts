// The agent picture: disk first by hash, then the image ask, always behind the
// visitor's own help answer (Plans/R1.1/agent-images.md#how-the-web-gets-it).
import { afterEach, expect, test } from "bun:test";
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createHash } from "node:crypto";
import { AgentPicture, clearImageCache, imagePack, imageRoute, imageStats } from "../agent-image.tsx";
import { clearMetadataCache } from "../agent-metadata.tsx";
import type { Ctx, Rec } from "../ctx.ts";
import { render } from "../jsx.ts";

const vectors = join(import.meta.dir, "../../testdata/agent-images");
const avif = readFileSync(join(vectors, "a.avif")), webp = readFileSync(join(vectors, "lossless.webp"));
const sha = (b: Uint8Array) => createHash("sha256").update(b).digest("hex");
const original = imagePack.dir;
const dirs: string[] = [];
afterEach(() => { imagePack.dir = original; for (const d of dirs.splice(0)) rmSync(d, { recursive: true }); clearImageCache(); clearMetadataCache(); });

function pack(withImage = true): string {
  const dir = mkdtempSync(join(tmpdir(), "pack-")); dirs.push(dir);
  mkdirSync(join(dir, "pictured"));
  writeFileSync(join(dir, "pictured", "agent.json"), JSON.stringify({ description: "Pictured.", ...(withImage ? { image: "agent.avif" } : {}) }));
  if (withImage) copyFileSync(join(vectors, "a.avif"), join(dir, "pictured", "agent.avif"));
  return dir;
}

type Answer = { kind: string; body?: string };
function visitor(session: string, help: Record<string, unknown> | undefined, image: () => Answer, asks: string[] = []) {
  const url = new URL(`http://localhost/agent-image?name=${encodeURIComponent("#pictured/one@h")}`);
  return {
    session, url,
    q: (name: string) => url.searchParams.get(name) ?? "",
    lookup: async (name: string) => ({ kind: "agent", name, readers: 1, at: "t" }) as Rec,
    call: async (_to: string, _body: string, _ms: number, ask?: string) => {
      asks.push(ask ?? "");
      if (ask === "image") return image();
      if (!help) return { kind: "lost", why: "refused" };
      return { kind: "answer", body: JSON.stringify({ description: "Pictured.", methods: {}, ...help }) };
    },
  } as unknown as Ctx;
}
const never = (): Answer => { throw new Error("the image ask must not be sent"); };

test("a bundled agent's file on disk with the help hash is served without an image ask", async () => {
  imagePack.dir = pack();
  const asks: string[] = [];
  const res = await imageRoute(visitor("a", { image: "agent.avif", image_sha256: sha(avif) }, never, asks));
  expect(res.status).toBe(200);
  expect(res.headers.get("Content-Type")).toBe("image/avif");
  expect(Buffer.from(await res.arrayBuffer()).equals(avif)).toBe(true);
  expect([imageStats.disk, imageStats.asked, asks]).toEqual([1, 0, ["help-json"]]);
});

test("with no matching file on disk the agent is asked once, and its hash is cached", async () => {
  imagePack.dir = pack(false);
  const asks: string[] = [];
  const answer = () => ({ kind: "answer", body: webp.toString("base64") });
  const help = { image: "agent.webp", image_sha256: sha(webp) };
  for (const session of ["a", "b"]) {
    const res = await imageRoute(visitor(session, help, answer, asks));
    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Type")).toBe("image/webp");
  }
  expect([imageStats.disk, imageStats.asked]).toEqual([0, 1]);
  expect(asks.filter(a => a === "image")).toHaveLength(1);
});

test("a disk file with another hash is not served; the agent's answer is", async () => {
  imagePack.dir = pack();
  const res = await imageRoute(visitor("a", { image: "agent.webp", image_sha256: sha(webp) }, () => ({ kind: "answer", body: webp.toString("base64") })));
  expect(Buffer.from(await res.arrayBuffer()).equals(webp)).toBe(true);
  expect([imageStats.disk, imageStats.asked]).toEqual([0, 1]);
});

test("bytes whose hash differs from help, or that break the caps, are refused", async () => {
  imagePack.dir = pack(false);
  const wide = readFileSync(join(vectors, "wide.png"));
  for (const [body, want] of [[webp, sha(avif)], [wide, sha(wide)]] as const) {
    clearImageCache();
    const res = await imageRoute(visitor("a", { image_sha256: want }, () => ({ kind: "answer", body: Buffer.from(body).toString("base64") })));
    expect(res.status).toBe(404);
  }
});

test("no hash in help, or help refused to this visitor, serves nothing even when cached", async () => {
  imagePack.dir = pack();
  expect((await imageRoute(visitor("owner", { image_sha256: sha(avif) }, never))).status).toBe(200);
  const asks: string[] = [];
  expect((await imageRoute(visitor("plain", {}, never, asks))).status).toBe(404);
  expect((await imageRoute(visitor("stranger", undefined, never, asks))).status).toBe(404);
  expect(asks).not.toContain("image");
});

test("the header shows the picture element only with a hash, else a placeholder", () => {
  const pic = render(AgentPicture({ name: "#pictured/one@h", hash: sha(avif) }));
  expect(pic).toMatch(/^<img class="agent-picture" src="\/agent-image\?name=%23pictured%2Fone%40h&amp;h=[0-9a-f]{64}"/);
  const none = render(AgentPicture({ name: "#pictured/one@h", hash: "" }));
  expect(none).not.toContain("<img");
  expect(none).toContain(">P</span>");
});

test("an agent list shows a thumbnail only for an agent whose help named an image hash", async () => {
  const { agentHelp, knownImages } = await import("../agent-metadata.tsx");
  const pictured = { kind: "agent", name: "#pictured/one@h", readers: 1, at: "t" } as Rec;
  const plain = { kind: "agent", name: "#plain@h", readers: 1, at: "t" } as Rec;
  await agentHelp(visitor("a", { image_sha256: sha(avif) }, never), pictured);
  await agentHelp(visitor("a", {}, never), plain);
  expect([knownImages.get(pictured.name), knownImages.has(plain.name)]).toEqual([sha(avif), false]);
  const thumb = render(AgentPicture({ name: pictured.name, hash: knownImages.get(pictured.name)!, size: 24 }));
  expect(thumb).toMatch(/^<img class="agent-picture agent-thumb" [^>]*width="24"/);
});
