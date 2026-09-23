import { Bus, BusError, withOwnerACL } from "../mcp/bus.ts";
import { sleep } from "../mcp/push.ts";
import { Bindings, numberedName } from "./sessions.ts";

// Conditional registration arbitrates across launchers, including distinct
// local state directories. Only the launching account claims new identities.
// When this launcher started. A record registered before it belongs to a
// session that has already ended; one that appeared after belongs to a
// launcher starting right now, which must be left alone even though it has no
// reader attached yet. That gap between registering and reading is the only
// thing the daemon cannot arbitrate for us.
const started = Date.now();

export async function claimIdentity(owner: Bus, bindings: Bindings, base: string, title: string, previous?: string, saved?: string) {
  let records = await owner.ls();
  let name = saved || base, number = 1;
  if (saved && !bindings.lock(`bus:${saved}`)) throw new Error("session bus identity is already launched");
  for (;;) {
    // Take the name back rather than stepping around it. The daemon is the
    // arbiter: Unregister refuses a name that has waiting readers or queued
    // messages, so a live session and a backlog are both safe, and what gives
    // way is only a record whose session ended. Without this a directory
    // collects thing.2, .3, .4 for good, because a dormant record is meant to
    // outlive its launcher and nothing ever reclaimed one.
    if (!saved) for (;;) {
      if (!bindings.lock(`bus:${name}`)) { name = numberedName(base, ++number); continue; }
      // Read it again here: the listing above is old enough that a launcher
      // starting beside us may have registered since, and deciding from the
      // stale copy is how its name gets taken.
      const found = (await owner.ls()).find(r => r.name === name) as { at?: string } | undefined;
      if (!found) break;                       // free; the conditional register below settles any tie
      if (!(found.at && Date.parse(found.at) < started)) { name = numberedName(base, ++number); continue; }
      try { await owner.unregister(name); }
      catch (e) {
        if (e instanceof BusError && e.status === 409) { name = numberedName(base, ++number); continue; }
        if (!(e instanceof BusError) || e.status !== 404) throw e;   // 404: nothing there, which is what we want
      }
      break;
    }
    let label = title;
    // Fresh for the same reason the name was: a label is only unique against
    // what is registered now, and launchers starting together each need to see
    // what the others already took.
    records = await owner.ls();
    const used = new Set(records.filter(r => r.name !== name && r.name !== previous).map(r => r.descr));
    for (let n = 2; used.has(label) || !bindings.lock(`label:${label}`); n++) label = `${title} #${n}`;
    try {
      const existing = records.find(r => r.name === name);
      // Personal: a session belongs to the launching user, not on the shared
      // web pages (docs/03-records.md#personal-and-shared).
      await owner.register({ name, kind: "agent", descr: label, allow: withOwnerACL(existing?.allow), personal: true }, !saved);
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
