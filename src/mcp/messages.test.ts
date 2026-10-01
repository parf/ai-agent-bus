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
  expect(text).toContain('"kind":"ack"');
  expect(text).toContain('"kind":"done"');
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
