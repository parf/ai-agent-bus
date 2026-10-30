// Migrate chat selectors/counters to shared rows before the engine is loaded.
// KV has no multi-key transaction: a journal makes a committed new row plus
// an old key recoverable, but they are never both served. Finish every journal
// before returning; an uncertain write stops service until the next reload.
import { createHash } from "node:crypto";
import { isDeepStrictEqual } from "node:util";
import { Bus, BusError, KVNotFound } from "../../clients/typescript/src/index.js";

type KV = ReturnType<Bus["record"]>["kv"];
type Op = () => { timeout: number };
type Journal = { oldKey: string; nextKey: string; oldHash: string; value: Record<string, any> };
const digest = (value: unknown): string => createHash("sha256").update(JSON.stringify(value, (_k, v) => v && typeof v === "object" && !Array.isArray(v) ? Object.fromEntries(Object.entries(v).sort(([a], [b]) => a.localeCompare(b))) : v)).digest("hex");
export const migrationId = (key: string, collision = 0) => createHash("sha256").update(`chat-subscription-v1:${key}:${collision}`).digest("hex").slice(0, 12);
const M = "migration.sub.";

async function maybe(kv: KV, kind: "json" | "int", key: string, op: Op): Promise<any | undefined> {
  try { return (await kv.get(kind, key, op())).value; }
  catch (e) { if (e instanceof KVNotFound) return undefined; throw e; }
}

async function finish(kv: KV, journalKey: string, j: Journal, op: Op): Promise<void> {
  if (!/^sub\.[0-9a-f]{8}(?:[0-9a-f]{4})?$/.test(j.oldKey) || !/^sub\.[0-9a-f]{12}$/.test(j.nextKey) ||
      !/^[0-9a-f]{64}$/.test(j.oldHash) || !j.value || typeof j.value !== "object" || Array.isArray(j.value)) throw Error("invalid subscription migration journal");
  let next = await maybe(kv, "json", j.nextKey, op);
  if (j.nextKey === j.oldKey) {
    if (!isDeepStrictEqual(next, j.value)) {
      if (digest(next) !== j.oldHash) throw Error("subscription changed during migration");
      await kv.set("json", j.nextKey, j.value, { ...op(), mode: "replace" });
    }
  } else {
    if (next === undefined) {
      try { await kv.set("json", j.nextKey, j.value, { ...op(), mode: "add" }); }
      catch (e) {
        if (!(e instanceof BusError && e.status === 412)) throw e;
        next = await maybe(kv, "json", j.nextKey, op);
        if (!isDeepStrictEqual(next, j.value)) throw Error("subscription migration id collision");
      }
    } else if (!isDeepStrictEqual(next, j.value)) throw Error("subscription migration id collision");
    const old = await maybe(kv, "json", j.oldKey, op);
    if (old !== undefined && digest(old) !== j.oldHash) throw Error("subscription changed during migration");
    // The new row is durable before the old one is removed. A crash here
    // leaves this journal; next load finishes it before any event is served.
    await kv.delete("json", j.oldKey, op());
  }
  const oldId = j.oldKey.slice(4);
  await kv.delete("int", `stat.${oldId}.forwarded`, op());
  await kv.delete("string", `stat.${oldId}.last_error`, op());
  await kv.delete("json", journalKey, op());
}

export async function migrateSubscriptions(kv: KV, op: Op): Promise<void> {
  let keys = (await kv.list(op())).json ?? [];
  for (const { name } of keys) {
    if (typeof name !== "string" || !name.startsWith(M)) continue;
    const journal = await maybe(kv, "json", name, op);
    if (journal !== undefined) await finish(kv, name, journal, op);
  }
  keys = (await kv.list(op())).json ?? [];
  for (const { name } of keys) {
    if (typeof name !== "string" || !/^sub\.[0-9a-f]{8}(?:[0-9a-f]{4})?$/.test(name)) continue;
    const raw = await maybe(kv, "json", name, op);
    if (raw === undefined) continue;
    const id = name.slice(4);
    // Canonical twelve-character rows (valid or damaged) are the engine's;
    // only the legacy channel field identifies an old twelve-character row.
    if (id.length === 12 && (!raw || typeof raw !== "object" || !Object.hasOwn(raw, "channel"))) continue;
    let nextId = id;
    if (id.length === 8) {
      for (let attempt = 0; ; attempt++) {
        if (attempt === 16) throw Error("subscription migration id space occupied");
        nextId = migrationId(name, attempt);
        if (await maybe(kv, "json", `sub.${nextId}`, op) === undefined) break;
      }
    }
    const count = await maybe(kv, "int", `stat.${id}.forwarded`, op) ?? raw?.forwarded ?? 0;
    const sent = Number(count);
    const legacyFields = ["id", "channel", "target", "filter", "from_users", "created_by", "created_at", "disabled", "forwarded", "last_error"];
    const valid = raw && typeof raw === "object" && !Array.isArray(raw) && raw.id === id && Object.keys(raw).every(k => legacyFields.includes(k)) &&
      typeof raw.channel === "string" && typeof raw.target === "string" && typeof raw.created_by === "string" && typeof raw.created_at === "string" &&
      (raw.filter === undefined || typeof raw.filter === "string") && (typeof count === "bigint" || typeof count === "number") && Number.isSafeInteger(sent) && sent >= 0;
    const value = valid ? {
      id: nextId, target: raw.target.trim().toLowerCase(), selector: { channel: raw.channel, ...(raw.from_users !== undefined ? { from_users: raw.from_users } : {}) },
      filter: raw.filter ?? "", created_by: raw.created_by, created_at: raw.created_at, sent, dropped: 0,
    } : { ...raw, id: nextId, invalid_legacy: true };
    const journal: Journal = { oldKey: name, nextKey: `sub.${nextId}`, oldHash: digest(raw), value };
    const journalKey = M + id;
    await kv.set("json", journalKey, journal as any, { ...op(), mode: "add" });
    await finish(kv, journalKey, journal, op);
  }
}
