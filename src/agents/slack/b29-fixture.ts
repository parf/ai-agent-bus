// In-memory transport fixture for both provider adapters; production still
// goes through ChatPort and the shared engine, never a second test engine.
import { Bus, BusError, KVNotFound } from "../../clients/typescript/src/index.js";
import { ChatPort, ChatSubscriptions } from "./subscription-port.ts";
import type { Row } from "../shared/subscriptions.ts";
export const source = "#chat@fixture";
export const caller = { from: "author@fixture", roles: [] as string[] };
export const owner = { from: "owner@fixture", roles: ["owner"] };
export const row = (id = "000000000001", changes: Partial<Row> = {}): Row => ({ id, target: "#target", selector: { channel: "C1" }, filter: "", created_by: caller.from, created_at: "2026-10-30T00:00:00.000Z", sent: 0, dropped: 0, ...changes });
export function fixture(initial: Record<string, any> = {}) {
  const values = new Map(Object.entries(structuredClone(initial)));
  const sent: { target: string; body: string; options: any }[] = [], warnings: string[] = [], checks: string[] = [];
  let writes = 0;
  const state = { allowed: true, sourceAllowed: true, unavailable: false, kind: "agent", refusal: false, uncertain: false,
    beforeWrite: undefined as undefined | ((kind: string, key: string, value: any) => void),
    afterWrite: undefined as undefined | ((kind: string, key: string, value: any) => void),
  };
  const kv = {
    async get(kind: string, key: string) { if (!values.has(`${kind}:${key}`)) throw new KVNotFound("none"); return { value: structuredClone(values.get(`${kind}:${key}`)) }; },
    async list() { return Object.fromEntries(["json", "int", "string"].map(kind => [kind, [...values.keys()].filter(k => k.startsWith(kind + ":")).map(k => ({ name: k.slice(kind.length + 1) }))])); },
    async set(kind: string, key: string, value: any, options: { mode: string }) {
      state.beforeWrite?.(kind, key, value);
      if (options.mode === "add" && values.has(`${kind}:${key}`)) throw new BusError(412, "taken");
      if (options.mode === "replace" && !values.has(`${kind}:${key}`)) throw new BusError(412, "gone");
      writes++; values.set(`${kind}:${key}`, structuredClone(value)); state.afterWrite?.(kind, key, value);
    },
    async delete(kind: string, key: string) { state.beforeWrite?.(kind, key, undefined); writes++; return values.delete(`${kind}:${key}`); },
    async jsonApply(key: string, ops: any[]) {
      const value = (await this.get("json", key)).value;
      for (const op of ops) value[op.key] = op.value;
      await this.set("json", key, value, { mode: "replace" });
    },
  };
  const bus = {
    record: () => ({ kv }),
    async maySend(who: string, target: string) {
      checks.push(`${who}>${target}`);
      if (state.unavailable) throw Error("unavailable");
      return target === source ? state.sourceAllowed : state.allowed;
    },
    async request() { return { kind: state.kind }; },
    async send(target: string, body: string, options: any) {
      if (state.refusal) throw new BusError(403, "refused");
      if (state.uncertain) throw Error("unknown");
      sent.push({ target, body, options }); return "message-id";
    },
  } as unknown as Bus;
  const port = new ChatPort(bus, source);
  const make = () => new ChatSubscriptions(port, { resolve: async channel => channel === "#dev" ? "C1" : channel, warning: (code, id) => warnings.push(`${id}:${code}`) });
  return { values, sent, warnings, checks, state, kv, port, make, bus, writes: () => writes };
}
