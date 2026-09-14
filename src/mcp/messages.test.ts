import { expect, test } from "bun:test";
import { codexMessage } from "./messages.ts";
import type { Envelope } from "./bus.ts";

const message: Envelope = { message_id: "m", from: "sender@h", to: "session@h", body: "question", at: "now", topic: "original", tag: "tag" };
test("Codex replies follow the requested return route", () => {
  const text = codexMessage({ ...message, reply_to: { service: "collector@h", topic: "return", tag: "match" } });
  expect(text).toContain('"to":"collector@h","topic":"return","tag":"match"');
  expect(text).not.toContain('"to":"sender@h"');
});
test("Codex receipts never ask for a reply", () => {
  for (const receipt of ["ack", "done"] as const) {
    const text = codexMessage({ ...message, body: "", receipt });
    expect(text).toContain(`receipt: ${receipt}`);
    expect(text).not.toContain("ab_send");
  }
  expect(codexMessage({ ...message, receipt: "done" })).toContain("do not wait");
});
