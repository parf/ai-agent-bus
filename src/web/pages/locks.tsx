// /locks: every lock on the records the visitor may use — its Owner,
// Maintainers or own Agent — in one table, with a kind filter and search, as
// the other lists have (docs/01-identity-and-authority.md#shared-locks).
import { h, Fragment } from "../jsx.ts";
import { Ctx } from "../ctx.ts";
import { respond } from "../ui/frame.tsx";
import { Icon, Help, PageHead, KindIcon, Pill, Empty, Segmented, recordHref } from "../ui/kit.tsx";
import { LockActions } from "../ui/locks.tsx";
import { entity } from "../glyphs.ts";
import { left, number } from "../format.ts";

const KINDS = ["agent", "service", "queue", "pubsub", "resource", "group", "user"];

function listUrl(p: Record<string, string | undefined>): string {
  const q = new URLSearchParams(Object.entries(p).filter(([, v]) => v) as [string, string][]);
  return q.size ? `/locks?${q}` : "/locks";
}

async function locksPage(ctx: Ctx): Promise<Response> {
  const [st, all] = await Promise.all([ctx.status(), ctx.allLocks()]);
  const present = KINDS.filter(k => all.some(l => l.kind === k));
  // A kind with no lock left is no filter: it would hide everything with no
  // control on the page to clear it.
  const q = ctx.q("q").trim(), kind = present.includes(ctx.q("kind")) ? ctx.q("kind") : "";
  const needle = q.toLowerCase();
  const rows = all.filter(l => (!kind || l.kind === kind) && (!needle || [l.record, l.name, l.holder].some(v => v.toLowerCase().includes(needle))));
  const kindWord = (k: string) => k === "pubsub" ? "PubSub" : `${entity(k)?.word ?? k}s`;
  const clear = listUrl({});
  const body = <>
    <PageHead icon={<Icon name="lock" />} title="Locks"
      help={<Help id="locks-help" label="About Locks" title="Locks" items={[
        "Every lock on a record you may use: its Owner, its Maintainers and its own Agent.",
        "Release your own; force release another's, which the audit log records.",
        "Memory only: a restart releases every lock.",
      ]} />} />
    <form class="toolbar record-search" method="get" action="/locks">
      <div class="search"><Icon name="search" /><label for="locks-query" class="sr-only">Search locks</label>
        <input id="locks-query" type="search" name="q" value={q} placeholder="Search by record, lock or holder" /></div>
      {kind ? <input type="hidden" name="kind" value={kind} /> : null}
    </form>
    {present.length > 1 ? <div class="toolbar filters">
      <Segmented label="Kind" items={[{ href: listUrl({ q }), text: "All", current: !kind }, ...present.map(k => ({ href: listUrl({ q, kind: k }), text: kindWord(k), current: kind === k }))]} />
    </div> : null}
    {all.length === 0 ? <Empty icon="lock" title="No locks held">A lock appears here while it is held on a record you may use.</Empty>
      : rows.length === 0 ? <Empty icon="filter" title="No locks match these filters" action={<a class="btn" href={clear}>Clear filters</a>}>Change the search or the kind above, or clear filters.</Empty>
      : <>
        <div class="results-line"><span>Showing {number(rows.length)} of {number(all.length)} locks on records you may use.</span>{q || kind ? <a href={clear}>Clear filters</a> : null}</div>
        <div class="table-wrap"><div class="table-scroll"><table class="data stack record-table">
          <thead><tr><th>Record</th><th>Lock</th><th>Holder</th><th>Time left</th><th><span class="sr-only">Actions</span></th></tr></thead>
          <tbody>{rows.map(l => <tr>
            <td data-label="Record"><div class="rec-cell"><KindIcon kind={l.kind} /><a href={`${recordHref({ name: l.record, kind: l.kind })}#locks`}><code>{l.record}</code></a></div></td>
            <td data-label="Lock"><code>{l.name}</code></td>
            <td data-label="Holder"><code>{l.holder}</code>{l.holder === st.you ? <> <Pill tone="accent">you</Pill></> : null}</td>
            <td data-label="Time left">{left(new Date(l.expires).getTime() - Date.now())}</td>
            <td data-label="Actions"><LockActions record={l.record} name={l.name} holder={l.holder} you={st.you} back="/locks" /></td>
          </tr>)}</tbody>
        </table></div></div>
      </>}
  </>;
  return respond(ctx, { title: "Locks", section: "locks", signedIn: true, you: st.you }, body);
}

export const handlers = { locks: locksPage };
