import { expect, test } from "bun:test";
import { fixture, row, caller, owner, source } from "./b29-fixture.ts";
import { migrationId } from "./subscription-migration.ts";
import { Subscriptions } from "../shared/subscriptions.ts";

const legacy = (id = "deadbeef", changes = {}) => ({ id, channel: "C1", target: "#target", filter: "", created_by: caller.from, created_at: "2026-10-30T00:00:00.000Z", ...changes });

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

test("legacy migration preserves counts and selectors, removes disabled state and is idempotent", async () => {
  const f = fixture({ "json:sub.deadbeef": legacy("deadbeef", { disabled: true, from_users: ["U1"] }), "int:stat.deadbeef.forwarded": 7n }), s = f.make();
  const listed = await s.list(owner);
  expect(listed).toEqual([row(migrationId("sub.deadbeef"), { sent: 7, selector: { channel: "C1", from_users: ["U1"] } })]);
  expect(f.values.has("json:sub.deadbeef")).toBe(false);
  expect(f.values.has("int:stat.deadbeef.forwarded")).toBe(false);
  const writes = f.writes(); expect(await s.list(owner)).toEqual(listed); expect(f.writes()).toBe(writes); s.close();
});

test("migration collisions preserve both distinct subscriptions and invalid filters stay inert", async () => {
  const occupied = migrationId("sub.deadbeef");
  const f = fixture({ [`json:sub.${occupied}`]: row(occupied), "json:sub.deadbeef": legacy("deadbeef", { filter: "/(/" }) }), s = f.make();
  const listed = await s.list(owner);
  expect(listed).toHaveLength(2); expect(listed.find(r => r.id === migrationId("sub.deadbeef", 1))).toMatchObject({ reason: "invalid_filter", filter: "/(/" });
  await s.deliver("C1", "U1", "match", "match", "chat");
  expect(f.sent).toHaveLength(1); expect(f.warnings.filter(w => w.endsWith(":filter_invalid"))).toHaveLength(1); s.close();
});

test("a lost migration commit reply serves nothing until recovery and never serves both keys", async () => {
  const f = fixture({ "json:sub.deadbeef": legacy() }), first = f.make();
  f.state.afterWrite = (_kind, key) => { if (key === `sub.${migrationId("sub.deadbeef")}`) throw Error("reply lost"); };
  await expect(first.deliver("C1", "U1", "one", "one", "chat")).rejects.toMatchObject({ code: "store_unknown" });
  expect(f.sent).toEqual([]); expect(f.values.has("json:sub.deadbeef")).toBe(true); first.close();
  f.state.afterWrite = undefined; const recovered = f.make();
  expect(await recovered.list(owner)).toHaveLength(1);
  await recovered.deliver("C1", "U1", "one", "one", "chat");
  expect(f.sent.map(x => x.body)).toEqual(["one"]); expect(f.values.has("json:sub.deadbeef")).toBe(false); recovered.close();
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
