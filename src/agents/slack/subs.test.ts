import { expect, test } from "bun:test";
import { fixture, row, caller, owner, source } from "./b29-fixture.ts";
import { Subscriptions } from "../shared/subscriptions.ts";


test("chat adapter instantiates the shared engine, not a local subscription implementation", () => {
  const f = fixture(); const s = f.make(); expect(s.engine).toBeInstanceOf(Subscriptions); s.close();
});

test("refused and unknown copies warn once, count once, and leave the next match active", async () => {
  for (const failure of ["refusal", "uncertain"] as const) {
    const f = fixture({ "json:sub.000000000001": row() }), s = f.make();
    f.state[failure] = true;
    expect(await s.deliver("C1", "U1", "first", "first", "chat")).toEqual({ matched: 1, sent: 0, dropped: 1 });
    expect(f.warnings).toEqual([`000000000001:${failure === "refusal" ? "send_refused" : "send_unknown"}`]);
    f.state[failure] = false;
    expect(await s.deliver("C1", "U1", "next", "next", "chat")).toEqual({ matched: 1, sent: 1, dropped: 0 });
    expect(f.sent.map(x => x.body)).toEqual(["next"]);
    expect(f.values.get("json:sub.000000000001")).toMatchObject({ sent: 1, dropped: 1 });
    expect(f.sent[0]!.options).not.toHaveProperty("wait"); s.close();
  }
});

test("each event refreshes both author grants; a revoked source or target sends nothing", async () => {
  for (const what of ["allowed", "sourceAllowed", "unavailable"] as const) {
    const f = fixture({ "json:sub.000000000001": row() }), s = f.make();
    await s.deliver("C1", "U1", "before", "before", "chat");
    f.state[what] = what === "unavailable";
    await s.deliver("C1", "U1", "revoked", "revoked", "chat");
    expect(f.sent.map(x => x.body)).toEqual(["before"]);
    expect(f.warnings).toHaveLength(1);
    expect(f.checks).toContain(`${caller.from}>${source}`);
    expect(f.values.get("json:sub.000000000001")).toMatchObject({ sent: 1, dropped: 1 }); s.close();
  }
});

test("add checks authenticated author, filter, both grants and target kind before storage", async () => {
  const f = fixture(), s = f.make();
  await expect(s.add(caller, "C1", "#target", "/(/")).rejects.toMatchObject({ code: "invalid_filter" });
  expect(f.checks).toEqual([]); expect(f.writes()).toBe(0);
  f.state.sourceAllowed = false;
  await expect(s.add(caller, "C1", "#target")).rejects.toMatchObject({ code: "admission_refused" });
  expect(f.writes()).toBe(0); f.state.sourceAllowed = true; f.state.kind = "pubsub";
  await expect(s.add(caller, "C1", "#target")).rejects.toMatchObject({ code: "invalid_target" });
  expect(f.writes()).toBe(0); s.close();
});

test("visibility and deletion are creator or direct source staff, never forwarded staff", async () => {
  const f = fixture({ "json:sub.000000000001": row(), "json:sub.000000000002": row("000000000002", { created_by: "other@fixture" }) }), s = f.make();
  expect((await s.list(caller)).map(r => r.id)).toEqual(["000000000001"]);
  expect(await s.list(owner)).toHaveLength(2);
  const forwarded = { ...owner, original_to: "#front" };
  expect(await s.list(forwarded)).toHaveLength(0);
  await expect(s.delete(forwarded, "000000000001")).rejects.toMatchObject({ code: "not_author" });
  await s.delete(caller, "000000000001"); await s.delete(owner, "000000000002");
  expect(await s.list(owner)).toEqual([]); s.close();
});

test("unsupported subscription formats refuse without writes or delivery", async () => {
  for (const initial of [
    {"json:sub.deadbeef": {id:"deadbeef", channel:"C1"}},
    {"json:sub.000000000001": {...row(), channel:"C1"}},
    {"json:migration.sub.deadbeef": {oldKey:"sub.deadbeef",nextKey:"sub.deadbeef0000"}},
    {"int:stat.deadbeef.forwarded": 7n},
    {"string:stat.deadbeef.last_error": "refused"},
  ]) {
    const f=fixture(initial), s=f.make(), before=structuredClone(f.values);
    await expect(s.ready()).rejects.toMatchObject({code:"store_unknown"});
    await expect(s.deliver("C1","U1","match","match","chat")).rejects.toMatchObject({code:"store_unknown"});
    expect(f.values).toEqual(before); expect(f.writes()).toBe(0); expect(f.sent).toEqual([]); s.close();
  }
});

test("current rows and unrelated current chat state reload unchanged", async () => {
  const f=fixture({"json:sub.000000000001":row("000000000001",{sent:7,dropped:3}),"string:reply_routes":"{}","string:offset":"12","string:seen:123":"[]","string:users":"[]","string:chats":"[]"}), s=f.make();
  const before=structuredClone(f.values);
  await s.ready();
  expect(await s.list(owner)).toEqual([row("000000000001",{sent:7,dropped:3})]);
  expect(f.values).toEqual(before); expect(f.writes()).toBe(0); s.close();
});

test("counter failure never retries a send or resurrects a deleted row", async () => {
  const f = fixture({ "json:sub.000000000001": row() }), s = f.make();
  f.state.beforeWrite = (_kind, key) => { if (key.startsWith("sub.")) throw Error("counter unavailable"); };
  await s.deliver("C1", "U1", "one", "one", "chat");
  expect(f.sent).toHaveLength(1); expect(f.warnings).toEqual(["undefined:store_unknown"]);
  f.state.beforeWrite = undefined;
  await s.delete(caller, "000000000001");
  await s.deliver("C1", "U1", "next", "next", "chat");
  expect(f.sent).toHaveLength(1); expect(f.values.has("json:sub.000000000001")).toBe(false); s.close();
});
