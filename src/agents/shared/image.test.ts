// The TypeScript image rules match the shared vectors the Go and Python checks read.
import { expect, test } from "bun:test";
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { IMAGE_MAX_BYTES, imageCheck, imageLoad, imageSize } from "./image.ts";
import { dispatchAsk, helpAnswer, imageHandler } from "./metadata.ts";

const vectors = join(import.meta.dir, "../../testdata/agent-images");
const cases: Record<string, { type?: string; size?: number; refused?: string }> = JSON.parse(readFileSync(join(vectors, "cases.json"), "utf8"));

test("every shared vector is accepted or refused as cases.json says", () => {
  for (const [name, want] of Object.entries(cases)) {
    const data = new Uint8Array(readFileSync(join(vectors, name)));
    if (want.refused) { expect(() => imageCheck(data), name).toThrow(want.refused); continue; }
    const img = imageCheck(data);
    expect([name, img.type, ...imageSize(data).slice(1)]).toEqual([name, want.type!, want.size!, want.size!]);
    expect(img.sha256).toHaveLength(64);
  }
});

test("256 KiB is accepted and one byte more refused", () => {
  const png = readFileSync(join(vectors, "a.png"));
  const at = new Uint8Array(IMAGE_MAX_BYTES); at.set(png);
  imageCheck(at);
  const over = new Uint8Array(IMAGE_MAX_BYTES + 1); over.set(png);
  expect(() => imageCheck(over)).toThrow("over 256 KiB");
});

test("help names the image and its hash, and the image ask answers its bytes", () => {
  const dir = mkdtempSync(join(tmpdir(), "img-"));
  try {
    mkdirSync(join(dir, "art"));
    copyFileSync(join(vectors, "lossless.webp"), join(dir, "art", "agent.webp"));
    const manifest = { description: "Pictured agent.", image: "art/agent.webp" };
    const help = JSON.parse(helpAnswer(manifest, "help-json", "", [], true, "", dir));
    expect([help.image, help.image_sha256, help.asks]).toEqual(["art/agent.webp", imageLoad(dir, "art/agent.webp").sha256, ["help", "help-config", "help-secret", "help-json", "image"]]);
    const answer = dispatchAsk("image", () => "", imageHandler(manifest, dir))!;
    expect(Buffer.from(answer, "base64").equals(readFileSync(join(vectors, "lossless.webp")))).toBe(true);
    expect(dispatchAsk("image", () => "", imageHandler({ description: "Plain." }, dir))).toBe("unsupported: image");
    expect(helpAnswer({ description: "Plain." }, "help-json", "", [], true)).not.toContain("image");
  } finally { rmSync(dir, { recursive: true }); }
});

test("paths out of the package and links are refused", () => {
  const dir = mkdtempSync(join(tmpdir(), "img-")), outside = mkdtempSync(join(tmpdir(), "img-out-"));
  try {
    copyFileSync(join(vectors, "a.png"), join(outside, "a.png"));
    symlinkSync(join(outside, "a.png"), join(dir, "link.png"));
    symlinkSync(outside, join(dir, "linked"));
    mkdirSync(join(dir, "dir"));
    writeFileSync(join(dir, "x"), "");
    for (const [name, want] of [[join(outside, "a.png"), "absolute path"], ["../a.png", ". or .. parts"], ["link.png", "symlink"], ["linked/a.png", "symlink"], ["dir", "not a regular file"]])
      expect(() => imageLoad(dir, name!), name).toThrow(want);
  } finally { rmSync(dir, { recursive: true }); rmSync(outside, { recursive: true }); }
});
