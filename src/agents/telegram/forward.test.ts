import { expect, test } from "bun:test";
import { Forwarder } from "./forward.ts";

// RD141: the author's right to send is asked before every forward, and a
// check that cannot be answered sends nothing.
test("the author's may-send is asked before every send; a no or a vanished target disables, an unanswered check only skips", async () => {
  const run = async (check: (who: string, to: string) => Promise<boolean>) => {
    const sent: string[] = [], asked: string[] = [], disabled: string[] = [];
    const f = new Forwarder({
      subs: async () => [{ id: "s1", channel: "#dev", target: "#t", created_by: "author@t", created_at: "now" }],
      exists: async () => true,
      resolve: () => ({ ok: true, chat_id: "-1", named: "#dev" }),
      maySend: async (who, to) => { asked.push(`${who}>${to}`); return check(who, to); },
      send: async to => { sent.push(to); },
      disable: async (_id, why) => { disabled.push(why); },
      persistCount: async () => {}, persistError: async () => {}, pending: () => undefined, log: () => {},
    });
    const m = (id: number) => ({ message_id: id, text: "x", from_id: 1, chat_id: -1 } as any);
    await f.message(m(1), "-1", "Dev");
    await f.message(m(2), "-1", "Dev");
    return { sent, asked, disabled };
  };
  const yes = await run(async () => true);
  expect(yes.sent).toEqual(["#t", "#t"]);
  expect(yes.asked).toEqual(["author@t>#t", "author@t>#t"]);
  const no = await run(async () => false);
  expect(no.sent).toEqual([]);
  expect(no.disabled.length).toBeGreaterThan(0);
  const gone = await run(async () => { throw Object.assign(new Error("no such name"), { status: 404 }); });
  expect(gone.sent).toEqual([]);
  expect(gone.disabled.length).toBeGreaterThan(0);
  const down = await run(async () => { throw new Error("daemon away"); });
  expect(down.sent).toEqual([]);
  expect(down.disabled).toEqual([]);
});

test("an outage is asked once per event and logged once; a pair shared by subscriptions is asked once; an unknown author is disabled", async () => {
  const subs = [1, 2, 3, 4, 5].map(n => ({ id: `s${n}`, channel: "#dev", target: n < 4 ? "#t" : `#u${n}`, created_by: "author@t", created_at: "now" }));
  subs.push({ id: "orphan", channel: "#dev", target: "#t", created_by: "", created_at: "now" });
  const asked: string[] = [], logged: string[] = [], disabled: string[] = [];
  let down = true;
  const f = new Forwarder({
    exists: async () => true,
    resolve: () => ({ ok: true as const, chat_id: "-1", named: "#dev" }),
    pending: () => undefined,
    subs: async () => subs,
    maySend: async (who, to) => { asked.push(`${who}>${to}`); if (down) throw new Error("daemon away"); return true; },
    send: async () => {},
    log: line => logged.push(line),
    disable: async (id, why) => { disabled.push(`${id}:${why}`); },
    persistCount: async () => {}, persistError: async () => {},
  });
  for (let i = 1; i <= 3; i++) await f.message({ message_id: i, text: "x", from_id: 1, chat_id: -1 } as any, "-1", "Dev");
  expect(asked.length).toBe(3); // one failing call per event, however many subscriptions
  expect(logged.filter(l => l.includes("unavailable")).length).toBe(5); // once per subscription, not per event
  expect([...new Set(disabled)]).toEqual(["orphan:its author is unknown; disabled"]); // a real disable stops the later events
  down = false;
  asked.length = 0;
  await f.message({ message_id: 9, text: "y", from_id: 1, chat_id: -1 } as any, "-1", "Dev");
  expect(asked.sort()).toEqual(["author@t>#t", "author@t>#u4", "author@t>#u5"]); // the shared pair asked once
});
