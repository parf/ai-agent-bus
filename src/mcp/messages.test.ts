import { expect, test } from "bun:test";
import { codexMessage, describe } from "./messages.ts";
import type { Envelope } from "./bus.ts";

const message: Envelope = { message_id: "m", from: "sender@h", to: "session@h", body: "question", at: "now", topic: "original", tag: "tag" };
test("a plain pushed message has no routing or etiquette tail", () => {
  const text = codexMessage(message);
  expect(text).toBe("from sender@h · topic original · tag tag · id m\n\nquestion");
  for (const tail of ["Route {", "Reply to ", "ab_receipt", "answer this turn", "later turn", "never done", "reports, notices or receipts"]) expect(text).not.toContain(tail);
});
test("Codex replies show a different requested return route exactly once", () => {
  const text = codexMessage({ ...message, reply_to: { name: "collector@h", topic: "return", tag: "match" } });
  expect(text).toBe(`${describe(message)}\n\nReply to collector@h · topic return · tag match`);
  expect(text.match(/^Reply to /gm)).toHaveLength(1);
  expect(text).not.toContain("Route {");
  expect(text).not.toContain("ab_receipt");
});
test("an explicit return route matching the header needs no tail", () => {
  expect(codexMessage({ ...message, reply_to: { name: message.from, topic: message.topic, tag: message.tag } })).toBe(describe(message));
  const untagged = { ...message, topic: undefined, tag: undefined };
  expect(codexMessage({ ...untagged, reply_to: { name: message.from, topic: "", tag: "" } })).toBe(describe(untagged));
});
test("a change to any return-route field is shown without inheriting omitted fields", () => {
  for (const route of [
    { name: "collector@h", topic: "original", tag: "tag" },
    { name: "sender@h", topic: "return", tag: "tag" },
    { name: "sender@h", topic: "original", tag: "match" },
    { name: "sender@h" },
  ]) {
    expect(codexMessage({ ...message, reply_to: route })).toBe(`${describe(message)}\n\nReply to ${route.name} · topic ${route.topic ?? ""} · tag ${route.tag ?? ""}`);
  }
});
test("Codex receipts render unchanged even with a different return route", () => {
  for (const receipt of ["ack", "done"] as const) {
    const e = { ...message, body: "", receipt, re: "previous", reply_to: { name: "collector@h", topic: "return", tag: "match" } };
    const expected = receipt === "done"
      ? "from sender@h · topic original · tag tag · id m\n\nreceipt: done — sender@h finished previous and sent no answer. Nothing further is coming; do not wait for it."
      : "from sender@h · topic original · tag tag · id m\n\nreceipt: ack — sender@h received previous. Not an answer; the answer is still to come.";
    expect(codexMessage(e)).toBe(expected);
  }
});
test("a message says the roles its sender holds, and none when it holds none", async () => {
  const { describe } = await import("./messages.ts");
  expect(describe({ ...message, roles: ["owner", "maintainer"] })).toContain("from sender@h · roles owner, maintainer · topic original");
  expect(describe(message)).not.toContain("roles");
});
test("an ask is shown as one, with what it wants, and never as an empty body", async () => {
  const { describe } = await import("./messages.ts");
  expect(describe({ ...message, body: "", ask: "help" })).toContain("ask: help — sender@h asks what you can do");
  expect(describe({ ...message, body: "", ask: "mcp" })).toContain("ask: mcp — sender@h asks for your MCP tools");
  expect(describe(message)).not.toContain("ask:");
});

test("instructions and receipt tool teach RD103", async () => {
  for (const file of ["instructions.yaml", "tools.yaml"]) {
    const raw = await Bun.file(new URL(file, import.meta.url)).text();
    expect(Bun.YAML.parse(raw)).toBeTruthy();
    const source = raw.replace(/\s+/g, " ");
    for (const rule of ["same turn", "another turn", "Never send done after an answer", "done only for a task with no answer expected", "nothing for reports, notices or receipts"]) expect(source).toContain(rule);
    expect(source).not.toContain("Confirm receipt of every");
    expect(source).not.toContain("always send an ack");
  }
});
