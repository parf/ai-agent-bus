// A session answers the image ask with its runtime's picture; an unknown runtime has none.
import { expect, test } from "bun:test";
import { answerSessionAsk } from "./session-help.ts";
import { imageCheck } from "../agents/shared/image.ts";
import { sessionImages } from "./session-images.ts";

function fakeBus(tags: string[]) {
  const sent: { body: string }[] = [];
  const bus = { name: "#claude/x@h", get: async () => ({ tags }), send: async (m: { body: string }) => { sent.push(m); } };
  return { bus: bus as never, sent };
}
const ask = { ask: "image", to: "#claude/x@h", from: "alice@h", topic: "t", tag: "g", message_id: "m" } as never;

test("each runtime answers its own valid picture", async () => {
  expect(Object.keys(sessionImages).sort()).toEqual(["claude", "codex", "opencode"]);
  const seen = new Set<string>();
  for (const runtime of ["claude", "codex", "opencode"]) {
    const { bus, sent } = fakeBus([runtime]);
    expect(await answerSessionAsk(bus, ask, {})).toBe(true);
    const img = imageCheck(new Uint8Array(Buffer.from(sent[0]!.body, "base64")));
    expect(img.type).toBe("image/webp");
    seen.add(img.sha256);
  }
  expect(seen.size).toBe(3);
});

test("an unknown runtime answers unsupported", async () => {
  const { bus, sent } = fakeBus([]);
  await answerSessionAsk(bus, ask, {});
  expect(sent[0]!.body).toBe("unsupported: image");
});
