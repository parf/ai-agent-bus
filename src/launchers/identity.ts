import { Bus, BusError, withOwnerACL, runtimeTags } from "../mcp/bus.ts";
import { normalizeTags } from "../mcp/tags.ts";
import { sleep } from "../mcp/push.ts";
import { Bindings, numberedName } from "./sessions.ts";

// Conditional registration arbitrates new names across launchers. Existing
// records belong to their saved sessions until explicitly released.
export async function claimIdentity(owner: Bus, bindings: Bindings, base: string, title: string, previous?: string, saved?: string, runtime?: string, active?: string, description?: string, tags?: string[]) {
  let records = await owner.ls();
  let name = saved || base, number = 1;
  // Only an in-process rename can already own this reader. A saved binding
  // or explicit address at startup is a reservation, not proof of liveness.
  const refuseLive = () => {
    if (saved && (records.find(r => r.name === name)?.readers || 0) > (active === name ? 1 : 0))
      throw new Error(`${name} is already live`);
  };
  refuseLive();
  if (saved && !bindings.lock(`bus:${saved}`)) throw new Error("session bus identity is already launched");
  for (;;) {
    if (!saved) for (;;) {
      // Do not even temporarily lock a reserved name: its saved session may
      // resume while this registry call is in flight. Conditional registration
      // below arbitrates a competing claim made after this listing.
      if (!(await owner.ls()).some(r => r.name === name) && bindings.lock(`bus:${name}`)) break;
      name = numberedName(base, ++number);
    }
    let label = title;
    // Fresh for the same reason the name was: a label is only unique against
    // what is registered now, and launchers starting together each need to see
    // what the others already took.
    records = await owner.ls();
    refuseLive();
    const used = new Set(records.filter(r => r.name !== name && r.name !== previous).map(r => r.descr));
    let keepLabelLock: boolean;
    for (let n = 2; ; n++) {
      keepLabelLock = bindings.holds(`label:${label}`);
      if (!used.has(label) && bindings.lock(`label:${label}`)) break;
      label = `${title} #${n}`;
    }
    try {
      const existing = records.find(r => r.name === name);
      // Personal: a session belongs to the launching user, not on the shared
      // web pages (docs/03-records.md#personal-and-shared).
      const purpose = description ?? existing?.descr ?? "";
      const labels = normalizeTags([...(tags ?? existing?.tags ?? records.find(r => r.name === previous)?.tags ?? []), ...runtimeTags(runtime)]);
      await owner.register({ name, kind: "agent", descr: purpose, allow: withOwnerACL(existing?.allow), personal: true, tags: labels }, !saved);
      // Register adds labels; only an explicit --tag replaces the custom set.
      // Keep the Owner's existing Personal classification unchanged.
      if (tags !== undefined && existing) await owner.post("/manage", { name, tags: normalizeTags([
        ...labels, ...(existing.personal || existing.tags?.includes("personal") ? ["personal"] : []),
      ]) });
      return { name, label, description: purpose };
    } catch (e) {
      if (saved || !(e instanceof BusError) || e.status !== 412) throw e;
      bindings.unlock(`bus:${name}`);
      if (!keepLabelLock) bindings.unlock(`label:${label}`);
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
        // The cancellation can finish between the refusal and this listing.
        // An empty, now-idle record still needs the conditional retry.
        if (record && !record.queued) { await sleep(20); continue; }
      }
      return `kept ${name}: ${e instanceof Error ? e.message : e}`;
    }
  }
}
