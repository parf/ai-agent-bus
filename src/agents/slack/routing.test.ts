import { expect, test } from "bun:test";
import { SlackRouter, routingConfig } from "./routing.ts";

function fixture() {
  let config: any = { users: { U1: "parf" }, tags: { a: "#alpha@t", b: "#beta@t" }, channels: { C2: { anyone: true, route: "#room@t" }, C3: { anyone: true }, C4: { tags: false, route: "#room@t" } } };
  const sent: any[] = [], told: any[] = [], logs: string[] = []; let saved: string | undefined;
  const router = new SlackRouter({ self: "#slack@t", config: () => routingConfig(config, async x => x), get: async () => saved,
    set: async state => { saved = JSON.stringify(state); }, send: async (route, body, reply) => { sent.push({ ...route, body, reply }); return `bus-${sent.length}`; },
    tell: async (chat, message, text) => { told.push({ chat, message, text }); return message; }, log: line => logs.push(line) });
  let n = 0;
  const event = (over: any = {}) => router.event({ type: "message", channel: "C1", user: "U1", text: "<@UBOT> #a hello", ts: `1.${++n}`, ...over }, "UBOT", "Test channel");
  return { router, sent, told, logs, event, config, saved: () => saved, setConfig: (x: any) => { config = x; } };
}

test("S.10 only addressed traffic routes, config absence keeps legacy behavior", async () => {
  const f = fixture();
  await f.event({ text: "ordinary conversation" }); expect(f.sent).toEqual([]); expect(f.told).toEqual([]);
  await f.event(); expect(f.sent[0].to).toBe("#alpha@t"); expect(f.sent[0].body).toBe("[parf (Slack)] hello");
  await f.event({ channel: "D1", text: "#b direct" }); expect(f.sent.at(-1).to).toBe("#beta@t");
  f.setConfig(undefined); const before = f.sent.length;
  await f.event(); await f.event({ channel: "D1", user: "U2" }); expect(f.sent.length).toBe(before); expect(f.told).toEqual([]);
});

test("S.10 thread owner and exact string timestamps survive restart, session replies return in the root thread", async () => {
  const f = fixture(), ts = "1740000000.000001";
  await f.router.tag("#alpha@t", await routingConfig(f.config, async x => x));
  await f.router.posted("C1", ts, { to: "#alpha@t", topic: "original", tag: "correlation", re: "posted-id" });
  await f.router.posted("C1", "1740000000.000002", { to: "#beta@t", re: "another" });
  await f.router.load();
  await f.event({ thread_ts: ts, text: "my answer" });
  expect(f.sent[0]).toMatchObject({ to: "#alpha@t", topic: "original", tag: "correlation", re: "posted-id", body: "[parf (Slack)] my answer" });
  const reply = f.sent[0].reply;
  await f.router.answer({ message_id: "spoof", re: "bus-1", from: "#beta@t", topic: reply.topic, tag: reply.tag, body: "wrong" });
  expect(f.told).toEqual([]); expect(f.logs.join()).toContain("dropped unmatched");
  await f.router.answer({ message_id: "answer", re: "bus-1", from: "#alpha@t", topic: reply.topic, tag: reply.tag, body: "fixed" });
  expect(f.told).toEqual([{ chat: "C1", message: ts, text: "fixed\n\n#a" }]);
  await f.event({ thread_ts: ts, text: "follow-up", user: "U9", channel: "C1" }); expect(f.sent.length).toBe(1);
});

test("S.10 anyone is confined to that channel route or its own mapped thread", async () => {
  const f = fixture(); f.config.route = "#global@t";
  await f.event({ channel: "C2", user: "U9", text: "<@UBOT> #b cannot escape" });
  expect(f.sent.at(-1)).toMatchObject({ to: "#room@t", body: "[Slack user U9 (Slack)] #b cannot escape" });
  await f.event({ channel: "C3", user: "U9", text: "<@UBOT> #b cannot escape" });
  expect(f.sent.length).toBe(1); expect(f.told.at(-1).text).toContain("no route");
  await f.router.posted("C2", "1.000001", { to: "#alpha@t", re: "post" });
  await f.event({ channel: "C2", user: "U9", thread_ts: "1.000001", text: "reply" }); expect(f.sent.at(-1).to).toBe("#alpha@t");
  await f.event({ channel: "C3", user: "U9", thread_ts: "1.000001", text: "<@UBOT> crossing" }); expect(f.sent.length).toBe(2);
  await f.event({ channel: "C4", text: "<@UBOT> #b literal" }); expect(f.sent.at(-1).to).toBe("#room@t");
});

test("S.10 refusal audit, hourly notice, no-route advice and own-property maps", async () => {
  const f = fixture();
  await f.event({ user: "U9", text: "<@UBOT> " + "x".repeat(80) });
  await f.event({ user: "U9", text: "<@UBOT> again" });
  expect(f.sent).toEqual([]); expect(f.told.map(x => x.text)).toEqual(["we do not know you"]);
  expect(f.router.refused()).toMatchObject([{ id: "U9", chat: "C1", count: 2 }]); expect(f.logs.join()).not.toContain("x".repeat(41));
  await f.event({ text: "<@UBOT> what now?" }); expect(f.told.at(-1).text).toContain("#a");
  await f.event({ user: "constructor" }); expect(f.sent).toEqual([]);
  f.config.users = Object.create({ U9: "parf" });
  await f.event({ user: "U9" }); expect(f.sent).toEqual([]);
  await f.event({ text: "<@UBOT> #constructor hidden" }); expect(f.sent).toEqual([]);
});

test("S.10 config validates all routing maps and names by own properties", async () => {
  for (const config of [{users:{constructor:"parf"}}, {tags:{a:"bad"}}, {channels:{constructor:{anyone:true}}}, {channels:{C1:{route:"--bad"}}}]) {
    expect(routingConfig(config, async x => x)).rejects.toThrow();
  }
  const config = await routingConfig(JSON.parse('{"tags":{"constructor":"#alpha@t"},"channels":{"#dev":{"route":"#room@t"}}}'), async () => "C1");
  expect(Object.hasOwn(config.tags!, "constructor")).toBe(true); expect(config.chats!.C1!.route).toBe("#room@t");
});

test("an unresolved channel entry is skipped while healthy channels still route",async()=>{
 const config=await routingConfig({channels:{"#old":{route:"#stale@t"},C1:{route:"#healthy@t"}}},async name=>name==="#old"?undefined:name);
 expect(Object.keys(config.chats!)).toEqual(["C1"]);
});


test("drop discards listed and unknown Slack messages while normal channels keep routing", async () => {
 const f = fixture(); f.config.channels.C1 = {handler: "drop"}; await f.router.load(); const saved = f.saved();
 await f.event(); await f.event({user: "U9"});
 expect(f.sent).toEqual([]); expect(f.told).toEqual([]); expect(f.logs).toEqual([]);
 expect(f.router.refused()).toEqual([]); expect(f.saved()).toBe(saved);
 await f.event({channel: "D1"}); expect(f.sent).toHaveLength(1);
 await f.event({channel: "D1", user: "U9"}); expect(f.told[0].text).toBe("we do not know you");
});

test("Slack handler refuses unknown values and combinations", async () => {
 expect((await routingConfig({channels: {C1: {handler: "drop"}}}, async x => x)).chats?.C1).toEqual({handler: "drop"});
 for (const handler of ["other", true, null, 1, {}]) await expect(routingConfig({channels: {C1: {handler}}}, async x => x)).rejects.toThrow("channels: C1.handler must be one of: drop");
 for (const key of ["route", "anyone", "tags"]) await expect(routingConfig({channels: {C1: {handler: "drop", [key]: key === "route" ? "#alpha@t" : false}}}, async x => x)).rejects.toThrow("channels: C1.handler cannot be combined with route, anyone or tags");
});
