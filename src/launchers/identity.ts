import { Bus, BusError } from "../mcp/bus.ts";
import { sleep } from "../mcp/push.ts";
import { Bindings, numberedName } from "./sessions.ts";

// Conditional registration arbitrates across launchers, including distinct
// local state directories. Only the launching account claims new identities.
export async function claimIdentity(owner: Bus, bindings: Bindings, base: string, title: string, previous?: string, saved?: string) {
  let records = await owner.ls();
  let name = saved || base, number = 1;
  if (saved && !bindings.lock(`bus:${saved}`)) throw new Error("session bus identity is already launched");
  for (;;) {
    if (!saved) while (records.some(r => r.name === name) || !bindings.lock(`bus:${name}`)) name = numberedName(base, ++number);
    let label = title;
    const used = new Set(records.filter(r => r.name !== name && r.name !== previous).map(r => r.descr));
    for (let n = 2; used.has(label) || !bindings.lock(`label:${label}`); n++) label = `${title} #${n}`;
    try {
      await owner.register({ name, kind: "agent", descr: label }, !saved);
      return { name, label };
    } catch (e) {
      if (saved || !(e instanceof BusError) || e.status !== 412) throw e;
      name = numberedName(base, ++number);
      records = await owner.ls();
    }
  }
}

export async function releaseIdle(owner: Bus, name: string): Promise<string> {
  // Aborting the client read can finish before the daemon releases its waiter.
  // Retry only that short cancellation window; never discard a backlog.
  for (let i = 0; ; i++) {
    try { await owner.unregister(name); return `released ${name}`; }
    catch (e) {
      if (e instanceof BusError && e.status === 404) return `released ${name}`;
      if (e instanceof BusError && e.status === 409 && i < 25) {
        const record = (await owner.ls()).find(r => r.name === name);
        if (record?.reading && !record.queued) { await sleep(20); continue; }
      }
      return `kept ${name}: ${e instanceof Error ? e.message : e}`;
    }
  }
}
