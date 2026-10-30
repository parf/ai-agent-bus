import { expect, test } from "bun:test";
import { Forwarder } from "./forward.ts";
import { fixture, row } from "../slack/b29-fixture.ts";

test("J22 a malformed stored filter drops only its own copy", async () => {
  const fx = fixture({
    "json:sub.000000000001": row("000000000001", { target: "#bad", filter: "/(/" }),
    "json:sub.000000000002": row("000000000002", { target: "#good" }),
  });
  const subscriptions = fx.make();
  const f = new Forwarder({
    subscriptions,
  } as any);
  await f.message({ message_id: 1, text: "match" } as any, "C1").catch(() => {});
  expect(fx.sent.map(s => s.target)).toEqual(["#good"]);
  expect(fx.warnings.filter(s => s.includes("filter_invalid"))).toHaveLength(1);
  expect(fx.values.get("json:sub.000000000001").dropped).toBe(1);
  expect(fx.values.get("json:sub.000000000002").sent).toBe(1);
  subscriptions.close();
});
