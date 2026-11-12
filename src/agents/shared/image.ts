// An agent's image: the same rules as internal/agentmeta/image.go (Plans/R1.1/agent-images.md#rules).
import { createHash } from "node:crypto";
import { closeSync, constants, fstatSync, lstatSync, openSync, readSync } from "node:fs";
import { isAbsolute, join } from "node:path";

export const IMAGE_MAX_BYTES = 256 << 10;
export const IMAGE_MAX_PIXELS = 512;
export class ImageError extends Error {}
export type Image = { data: Uint8Array; sha256: string; type: string };

export function imageCheckName(name: string): void {
  if (typeof name !== "string" || !name.trim() || name !== name.trim()) throw new ImageError("agent.json: image must be a file name inside the agent's folder");
  if (name.startsWith("/") || name.includes("\\") || isAbsolute(name)) throw new ImageError(`agent.json: image "${name}" must not be an absolute path`);
  if (name.split("/").some(part => part === "" || part === "." || part === "..")) throw new ImageError(`agent.json: image "${name}" must not contain empty, . or .. parts`);
}

/** Validated image name inside dir; never through a symlink. */
export function imageLoad(dir: string, name: string): Image {
  imageCheckName(name);
  const parts = name.split("/");
  let at = dir;
  parts.forEach((part, i) => {
    at = join(at, part);
    let st;
    try { st = lstatSync(at); } catch (e) { throw new ImageError(`agent.json: image "${name}": ${(e as NodeJS.ErrnoException).code === "ENOENT" ? "no such file or directory" : String(e)}`); }
    if (st.isSymbolicLink()) throw new ImageError(`agent.json: image "${name}": a symlink is refused`);
    if (i < parts.length - 1 && !st.isDirectory()) throw new ImageError(`agent.json: image "${name}": ${part} is not a directory`);
    if (i === parts.length - 1 && !st.isFile()) throw new ImageError(`agent.json: image "${name}" is not a regular file`);
  });
  const fd = openSync(at, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
  try {
    if (!fstatSync(fd).isFile()) throw new ImageError(`agent.json: image "${name}" is not a regular file`);
    const buf = new Uint8Array(IMAGE_MAX_BYTES + 1);
    let n = 0, got = 0;
    while (n < buf.length && (got = readSync(fd, buf, n, buf.length - n, null)) > 0) n += got;
    try { return imageCheck(buf.subarray(0, n)); } catch (e) { throw new ImageError(`agent.json: image "${name}": ${(e as Error).message}`); }
  } finally { closeSync(fd); }
}

/** The caps on bytes from any source; the type comes from the first bytes. */
export function imageCheck(data: Uint8Array): Image {
  if (data.length > IMAGE_MAX_BYTES) throw new ImageError(`over ${IMAGE_MAX_BYTES >> 10} KiB`);
  const [type, w, h] = imageSize(data);
  if (w <= 0 || h <= 0) throw new ImageError("has no size in its header");
  if (w > IMAGE_MAX_PIXELS || h > IMAGE_MAX_PIXELS) throw new ImageError(`is ${w}×${h}, over ${IMAGE_MAX_PIXELS}×${IMAGE_MAX_PIXELS}`);
  if (w !== h) throw new ImageError(`is ${w}×${h}, not square`);
  return { data, sha256: createHash("sha256").update(data).digest("hex"), type };
}

/** The image ask's answer: the file in base64. */
export const imageAnswer = (dir: string, name: string): string => Buffer.from(imageLoad(dir, name).data).toString("base64");

const ascii = (b: Uint8Array, at: number, n: number) => String.fromCharCode(...b.subarray(at, at + n));
const view = (b: Uint8Array) => new DataView(b.buffer, b.byteOffset, b.byteLength);

export function imageSize(b: Uint8Array): [string, number, number] {
  const v = view(b);
  if (b.length >= 8 && ascii(b, 0, 8) === "\x89PNG\r\n\x1a\n") {
    if (b.length < 24 || ascii(b, 12, 4) !== "IHDR") throw new ImageError("is a PNG without IHDR");
    return ["image/png", v.getUint32(16), v.getUint32(20)];
  }
  if (b.length >= 2 && b[0] === 0xff && b[1] === 0xd8) return ["image/jpeg", ...jpeg(b)];
  if (b.length >= 12 && ascii(b, 0, 4) === "RIFF" && ascii(b, 8, 4) === "WEBP") return ["image/webp", ...webp(b)];
  if (b.length >= 12 && ascii(b, 4, 4) === "ftyp" && isAvif(b)) return ["image/avif", ...avif(b)];
  throw new ImageError("is not AVIF, WebP, PNG or JPEG");
}

function jpeg(b: Uint8Array): [number, number] {
  const v = view(b);
  let i = 2;
  while (i + 4 <= b.length) {
    if (b[i] !== 0xff) throw new ImageError("is a JPEG with a broken marker");
    const marker = b[i + 1]!;
    if (marker === 0xff) { i++; continue; }
    if (marker === 0xd8 || marker === 0x01 || (marker >= 0xd0 && marker <= 0xd7)) { i += 2; continue; }
    const size = v.getUint16(i + 2);
    if (size < 2 || i + 2 + size > b.length) break;
    if (marker >= 0xc0 && marker <= 0xcf && marker !== 0xc4 && marker !== 0xc8 && marker !== 0xcc) {
      if (size < 7) break;
      return [v.getUint16(i + 7), v.getUint16(i + 5)];
    }
    i += 2 + size;
  }
  throw new ImageError("is a JPEG without a frame size");
}

function webp(b: Uint8Array): [number, number] {
  if (b.length < 30) throw new ImageError("is a short WebP");
  const v = view(b);
  switch (ascii(b, 12, 4)) {
    case "VP8X": return [(b[24]! | b[25]! << 8 | b[26]! << 16) + 1, (b[27]! | b[28]! << 8 | b[29]! << 16) + 1];
    case "VP8L": {
      if (b[20] !== 0x2f) throw new ImageError("is a WebP with a bad lossless signature");
      const bits = v.getUint32(21, true);
      return [(bits & 0x3fff) + 1, ((bits >>> 14) & 0x3fff) + 1];
    }
    case "VP8 ":
      if (b[23] !== 0x9d || b[24] !== 0x01 || b[25] !== 0x2a) throw new ImageError("is a WebP with a bad lossy start code");
      return [v.getUint16(26, true) & 0x3fff, v.getUint16(28, true) & 0x3fff];
  }
  throw new ImageError("is a WebP without a known image chunk");
}

function boxes(b: Uint8Array): [string, Uint8Array][] {
  const out: [string, Uint8Array][] = [];
  while (b.length) {
    if (b.length < 8) throw new ImageError("is an AVIF with a truncated box");
    const v = view(b);
    let size = v.getUint32(0), head = 8;
    if (size === 0) size = b.length;
    else if (size === 1) {
      if (b.length < 16) throw new ImageError("is an AVIF with a truncated box");
      const big = v.getBigUint64(8);
      size = big > BigInt(b.length) ? b.length + 1 : Number(big); head = 16;
    }
    if (size < head || size > b.length) throw new ImageError("is an AVIF with a box past its end");
    out.push([ascii(b, 4, 4), b.subarray(head, size)]);
    b = b.subarray(size);
  }
  return out;
}

function isAvif(b: Uint8Array): boolean {
  const size = view(b).getUint32(0);
  if (size < 16 || size > b.length) return false;
  for (let i = 8; i + 4 <= size; i += 4) if (i !== 12 && ["avif", "avis"].includes(ascii(b, i, 4))) return true;
  return false;
}

function avif(b: Uint8Array): [number, number] {
  const find = (list: [string, Uint8Array][], kind: string) => list.find(([k]) => k === kind)?.[1] ?? new Uint8Array();
  const meta = find(boxes(b), "meta");
  if (meta.length < 4) throw new ImageError("is an AVIF without meta");
  const props = boxes(find(boxes(find(boxes(meta.subarray(4)), "iprp")), "ipco"));
  let first: [number, number] | undefined;
  for (const [kind, body] of props) {
    if (kind !== "ispe") continue;
    if (body.length < 12) throw new ImageError("is an AVIF with a short ispe");
    const v = view(body), w = v.getUint32(4), h = v.getUint32(8);
    if (w > IMAGE_MAX_PIXELS || h > IMAGE_MAX_PIXELS) return [w, h];
    first ??= [w, h];
  }
  if (!first) throw new ImageError("is an AVIF without ispe");
  return first;
}
