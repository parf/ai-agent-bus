import { expect, test } from "bun:test";
import { Forwarder } from "./forward.ts";

test("J22 a malformed stored filter drops only its own copy", async () => {
  const sent: string[] = [], warnings: string[] = [];
  const rows = [
    { id: "000000000001", channel: "C1", target: "#bad", filter: "/(/", created_by: "author@fixture", created_at: "2026-10-30T00:00:00.000Z" },
    { id: "000000000002", channel: "C1", target: "#good", created_by: "author@fixture", created_at: "2026-10-30T00:00:00.000Z" },
  ];
  const f = new Forwarder({
    subs: async () => rows, exists: async () => true, maySend: async () => true,
    send: async (to: string) => { sent.push(to); }, disable: async () => {},
    persistCount: async () => {}, persistError: async () => {}, pending: () => undefined,
    log: (s: string) => warnings.push(s),
    channelId: async (c: string) => c, channelName: async () => "", userName: async () => "", identity: async () => ({}), posted: async () => false, forget: async () => {}, serialize: (_id: string, fn: any) => fn(), scrub: (s: string) => s,
  } as any);
  f.admit({ type: "message", channel: "C1", user: "U1", text: "match", ts: "1" }, "J22");
  await f.stop(2000);
  expect(sent).toEqual(["#good"]);
  expect(warnings.filter(s => s.includes("filter_invalid"))).toHaveLength(1);
});
