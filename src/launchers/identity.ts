import { Bus, BusError, withOwnerACL, runtimeTags } from "../mcp/bus.ts";
import { normalizeTags } from "../mcp/tags.ts";
import { sleep } from "../mcp/push.ts";
import { Bindings, numberedName, type Session } from "./sessions.ts";

// Conditional registration arbitrates new names across launchers. Existing
// records belong to their saved sessions until explicitly released.
export async function claimIdentity(owner: Bus, bindings: Bindings, base: string, title: string, previous?: string, saved?: string, runtime?: string, active?: string, description?: string, tags?: string[], cwd = process.cwd(), takeover = false) {
  let records = await owner.ls();
  let name = saved || base, number = 1;
  // Only an in-process rename can already own this reader. A saved binding
  // or explicit address at startup is a reservation, not proof of liveness.
  const refuseLive = () => {
    if (!takeover && saved && (records.find(r => r.name === name)?.readers || 0) > (active === name ? 1 : 0))
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
      const stored = existing?.descr ?? "";
      const automatic = /^(claude|codex|opencode) \/.*$/.test(stored);
      const kind = runtime === "opencode1" ? "opencode" : runtime;
      const purpose = description ?? (!stored || automatic ? (kind ? `${kind} ${cwd}` : "") : stored);
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

// A fork is a session the runtime started from another; it never shares its
// parent's name (RD129, docs/06-launchers.md#one-live-session-per-name). Its
// base is the parent's with `.job` before the realm, numbered by claimIdentity
// when two forks share a parent. The binding is keyed by the parent session and
// the fork's own id, so a resumed fork gets its name back.
export function forkBase(base: string): string {
  const at = base.includes("@") ? base.lastIndexOf("@") : base.length;
  const suffix = ".job", room = 64 - (base.length - at) - suffix.length;
  return `${base.slice(0, at).slice(0, room)}${suffix}${base.slice(at)}`;
}

export async function claimFork(owner: Bus, bindings: Bindings, parent: Session, base: string, forkId: string, title: string, runtime?: string, description?: string, tags?: string[], cwd = process.cwd()) {
  const key = { id: `${parent.id}+${forkId}` };
  const previous = bindings.saved(key);
  const forkName = forkBase(base);
  const claimed = await claimIdentity(owner, bindings, forkName, title, previous?.name, previous?.name, runtime, undefined, description, tags, cwd);
  bindings.save(key.id, claimed.name, forkName);
  return claimed;
}
