import { expect, test } from "bun:test";
import { Forwarder } from "./forward.ts";
import { fixture, row } from "../slack/b29-fixture.ts";

test("Telegram refused send drops one copy and its next message is delivered with the provider body", async () => {
  const fx = fixture({ "json:sub.000000000001": row() }), subscriptions = fx.make();
  const f = new Forwarder({ subscriptions }); fx.state.refusal = true;
  await f.message({ message_id: 1, text: "refused" } as any, "C1");
  fx.state.refusal = false;
  await f.message({ message_id: 2, text: "next", from_id: 10, from: "ada", reply_to: 1 } as any, "C1", "Dev");
  expect(fx.sent).toHaveLength(1); expect(fx.warnings).toEqual(["000000000001:send_refused"]);
  expect(JSON.parse(fx.sent[0]!.body)).toEqual({ chat: "C1", chat_name: "Dev", user: 10, user_name: "@ada", text: "next", message_id: 2, reply_to: 1 });
  expect(fx.sent[0]!.options).toMatchObject({ topic: "telegram", tag: "C1" }); f.stop();
});

test("Telegram checks author authority for each message and stop prevents later sends", async () => {
  const fx = fixture({ "json:sub.000000000001": row() }), subscriptions = fx.make();
  const f = new Forwarder({ subscriptions });
  await f.message({ message_id: 1, text: "first" } as any, "C1");
  fx.state.allowed = false;
  await f.message({ message_id: 2, text: "revoked" } as any, "C1");
  expect(fx.sent).toHaveLength(1); expect(fx.warnings).toEqual(["000000000001:admission_refused"]);
  f.stop(); fx.state.allowed = true;
  expect(await f.message({ message_id: 3, text: "late" } as any, "C1")).toBe(false); expect(fx.sent).toHaveLength(1);
});
