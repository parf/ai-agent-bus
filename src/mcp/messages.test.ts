import { expect, test } from "bun:test";
import { codexMessage } from "./messages.ts";
import type { Envelope } from "./bus.ts";

const message: Envelope = { message_id: "m", from: "sender@h", to: "session@h", body: "question", at: "now", topic: "original", tag: "tag" };
test("Codex replies follow the requested return route", () => {
  const text = codexMessage({ ...message, reply_to: { name: "collector@h", topic: "return", tag: "match" } });
  expect(text).toContain('"to":"collector@h","topic":"return","tag":"match"');
  expect(text).not.toContain('"to":"sender@h"');
  expect(text).toContain("ab_receipt");
  expect(text).toContain('"message_id":"m"');
  expect(text).not.toContain("Confirm receipt using");
  expect(text.match(/"to":/g)).toHaveLength(1);
  for (const rule of ["answer this turn: no ack", "later turn: ack", "never done after an answer", "done only for a task expecting no answer", "nothing for reports, notices or receipts"]) expect(text).toContain(rule);
});
test("Codex receipts never ask for a reply", () => {
  for (const receipt of ["ack", "done"] as const) {
    const text = codexMessage({ ...message, body: "", receipt });
    expect(text).toContain(`receipt: ${receipt}`);
    expect(text).not.toContain("ab_send");
  }
  expect(codexMessage({ ...message, receipt: "done" })).toContain("do not wait");
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
