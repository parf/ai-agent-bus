// Node pages: Overview, Activity, Diagnostics (docs/web-face/node.md).
import { h, Fragment, type Child } from "../jsx.ts";
import { Ctx, NotFound, type Rec, type Status, type Identity } from "../ctx.ts";
import { respond } from "../ui/frame.tsx";
import { Icon, Help, PageHead, Card, Figure, Name, Muted, recordHref, KindIcon, Empty } from "../ui/kit.tsx";
import { Ribbon, DayChart, SERIES, total, type Slot, Spark } from "../ui/charts.tsx";
import { html } from "../http.ts";
import { number, duration, slotLabel, stamp } from "../format.ts";
import { sectionProblem } from "../problem.tsx";

// ---------------------------------------------------------------- Overview

export type Attention = { level: "red" | "orange" | "blue"; title: string; key: string; body: Child; href: string; link: string; kind?: string };
const LEVEL = { red: 0, orange: 1, blue: 2 };

export function recordLine(r: Rec): Child {
  const parts: Child[] = [<Name>{r.name}</Name>, ` · ${number(r.queued)} held now`];
  if (r.oldest && (r.queued ?? 0) > 0) parts.push(` · oldest ${r.oldest}`);
  if (r.status === "inactive") parts.push(" · inactive");
  if (r.at_bound) parts.push(" · at capacity", r.overflow === "ring" ? " · when full: drop the oldest" : " · when full: refuse");
  if (r.dropped) parts.push(` · ${number(r.dropped)} dropped`);
  if (r.expired) parts.push(` · ${number(r.expired)} expired`);
  return parts;
}

/** Enumerated observations only; an empty list claims nothing about health. */
export function attentionItems(st: Status, records: Rec[]): Attention[] {
  const out: Attention[] = [];
  if (st.unclean) out.push({ level: "red", title: "The previous stop was not clean", key: "", body: "Memory from the previous run may not have reached the snapshot.", href: "/#node", link: "View node totals" });
  for (const [reason, n] of Object.entries(st.refused ?? {})) if (n > 0)
    out.push({ level: "blue", title: "Requests were refused", key: reason, body: <><code>{reason}</code> · {number(n)} since this daemon started</>, href: "/diagnostics#refusals", link: "View refusal reasons" });
  const oi = st.owner_inactive;
  if (oi && oi.records > 0)
    out.push({ level: "orange", title: "Records inactive because their owner is", key: "", body: `${number(oi.records)} record(s) · ${number(oi.messages)} message(s) held · node-wide; each returns when its owner is reactivated`, href: "/users?state=inactive", link: "View inactive users" });
  for (const r of records) {
    const lost = (r.dropped ?? 0) + (r.expired ?? 0);
    let a: Pick<Attention, "level" | "title"> | undefined;
    if (r.status === "inactive" && (r.queued ?? 0) > 0) a = { level: "orange", title: "Inactive and work is held" };
    else if (r.at_bound) a = { level: "red", title: "Queue at capacity when observed" };
    else if (lost > 0) a = { level: "orange", title: "Messages were lost from this inbox" };
    if (a) out.push({ ...a, key: r.name, kind: r.kind, body: recordLine(r), href: recordHref(r), link: "View record" });
  }
  return out.sort((x, y) => LEVEL[x.level] - LEVEL[y.level] || x.title.localeCompare(y.title) || x.key.localeCompare(y.key));
}

const Tile = ({ label, value, icon, spark, href, tone }: { label: string; value: Child; icon: string; spark?: Child; href?: string; tone?: string }) => {
  const inner = <><span class="tile-label"><Icon name={icon} />{label}</span><strong class="tile-value">{value}</strong>{spark ?? null}</>;
  return href ? <a class={`node-fact tile ${tone ?? ""}`} href={href}>{inner}</a> : <div class={`node-fact tile ${tone ?? ""}`}>{inner}</div>;
};

const KIND_TILES: [string, string, string, string][] = [
  ["agent", "Agents", "bot", "/agents"], ["service", "Services", "satellite-dish", "/services"], ["queue", "Queues", "inbox", "/queues"],
  ["pubsub", "PubSub", "megaphone", "/pubsub"], ["user", "Users", "user-round", "/users"], ["group", "Groups", "users", "/groups"],
];

function callTiles(id: Identity | null): Child {
  const c = id?.calls;
  if (!c) return <Tile label="Calls" value="unavailable" icon="zap" />;
  return <>
    {(c.windows ?? []).map(w => <Tile label={w.window === "1m" ? "Calls, minute" : "Calls, hour"} icon="zap"
      value={w.available ? <Figure n={w.count} /> : <span class="muted small">collecting history</span>} />)}
    <Tile label="Calls, total" icon="sigma" value={<Figure n={c.total} />} />
  </>;
}

async function overview(ctx: Ctx): Promise<Response> {
  const [st, records, id, act] = await Promise.all([ctx.status(), ctx.records(), ctx.identity(), ctx.get<Slot[]>("/activity").catch(() => null)]);
  const items = attentionItems(st, records);
  const slots = act ?? [];
  const kinds = st.kinds ?? {};
  const body = <>
    <PageHead icon={<Icon name="layout-dashboard" />} title="Overview"
      help={<Help id="overview-help" label="About Overview" tip="Only enumerated observations appear. An empty list does not claim the node is healthy." title="Overview scope"
        items={["Attention covers conditions over records visible to you, node-wide refusals, the previous-stop marker and, for Administrators, owner-inactive records.", "A backlog alone is ordinary work, so it is not an attention item.", "Node totals and your lists have different scopes; they never have to agree."]} />}
      sub={<>Signed in as <code>{st.you}</code>{st.daemon_owner ? <> · <span class="pill tone-accent"><Icon name="crown" />Daemon owner</span></> : st.administrator ? <> · <span class="pill">Daemon administrator</span></> : null}</>} />

    {items.length ? <section class="attention" aria-labelledby="attention">
      <h2 id="attention" class="section-title"><Icon name="bell-ring" />Needs attention <span class="count">{items.length}</span></h2>
      <div class="attention-list">
        {items.map(a => <article class={`attention-item attention-${a.level}`}>
          <div class="attention-mark" aria-hidden="true"><Icon name={a.level === "red" ? "octagon-alert" : a.level === "orange" ? "triangle-alert" : "info"} /></div>
          <div class="attention-text"><h3>{a.title}</h3><p>{a.body}</p></div>
          <a class="attention-link" href={a.href}>{a.link}<Icon name="chevron-right" /></a>
        </article>)}
      </div>
    </section> : <section class="all-clear card"><Icon name="circle-check" /><div><h2>Nothing to report</h2><p class="muted">No enumerated condition holds right now. That is not a claim the node is healthy.</p></div></section>}

    <section aria-labelledby="node" class="node-section">
      <div class="page-title"><h2 id="node" class="section-title"><Icon name="server" />This node</h2>
        <Help id="node-help" label="About node totals" tip="Whole-node values. Caller-visible lists may show a smaller set." title="Node totals"
          items={["Whole-node values, not your view: your lists may show fewer records.", "A dash means none.", "Kind counts count registered records of that kind.", "Readers are consumers waiting on an inbox now.", "Calls are requests the daemon served; history fills in as the node runs, and says collecting history until it has.", "Uptime is this daemon process's."]} /></div>
      <div class="node-strip">
        <Tile label="Readers" icon="radio" value={<Figure n={st.waiting} />} />
        <Tile label="Queued" icon="layers" value={<Figure n={st.queued} />} spark={<Spark values={slots.map(s => s.in)} label="Accepted per ten minutes today" />} />
        {Object.keys(kinds).length
          ? KIND_TILES.map(([k, label, icon, href]) => <Tile label={label} icon={icon} href={href} value={<Figure n={kinds[k]} />} />)
          : <Tile label="Records" icon="database" value={<Figure n={st.services} />} />}
      </div>
      <div class="node-strip node-break">
        <Tile label="Uptime" icon="clock" value={id?.up || st.up || "unavailable"} />
        {callTiles(id)}
      </div>
      <p class="muted small">Node-wide. The lists linked below contain only records visible to you; the two never have to agree.</p>
    </section>

    {slots.length ? <Card title="Today, visible records" icon="activity" actions={<a href="/activity" class="btn btn-ghost btn-sm">Open Activity<Icon name="chevron-right" /></a>}>
      <Ribbon slots={slots} label="Traffic per ten-minute slot over the last day, visible records" />
      <div class="ribbon-axis" aria-hidden="true"><span>{slotLabel(slots[0]!.at)}</span><span>{slotLabel(slots.at(-1)!.at)}</span></div>
      <p class="muted small">{SERIES.map(s => `${s.label} ${number(total(slots, s.key))}`).join(" · ")}</p>
    </Card> : null}

    <nav class="overview-links quick" aria-label="Find records">
      <strong>Find</strong>
      <a href="/agents?sort=queued&work=held" class="chip"><Icon name="bot" />Agents holding work</a>
      <a href="/queues?sort=queued&work=held" class="chip"><Icon name="inbox" />Queues holding work</a>
      <a href="/services" class="chip"><Icon name="satellite-dish" />External services</a>
    </nav>
  </>;
  return respond(ctx, { title: "Overview", section: "overview", signedIn: true, you: st.you }, body);
}

export async function home(ctx: Ctx): Promise<Response> {
  if (!ctx.signedIn) {
    const { signInPage } = await import("./landing.tsx");
    return signInPage(ctx);
  }
  return overview(ctx);
}

// ---------------------------------------------------------------- Activity

export async function activity(ctx: Ctx): Promise<Response> {
  const name = ctx.q("name");
  const [st, records] = await Promise.all([ctx.status(), ctx.records()]);
  if (name && !records.some(r => r.name === name)) throw new NotFound();
  const rec = records.find(r => r.name === name);
  const slots = rec?.status === "inactive" ? [] : (await ctx.get<Slot[]>("/activity", name ? { name } : undefined)) ?? [];
  const names = records.map(r => r.name).sort();
  // Hits: everything the five series counted in the last day, per record, for the chooser.
  const dayHits = (sl: Slot[] | null | undefined) => (sl ?? []).reduce((a, x) => a + (x.in ?? 0) + (x.out ?? 0) + (x.dropped ?? 0) + (x.expired ?? 0) + (x.refused ?? 0), 0);
  const hits = new Map<string, number | undefined>(await Promise.all(records.map(async r =>
    [r.name, r.status === "inactive" ? undefined : await ctx.get<Slot[]>("/activity", { name: r.name }).then(dayHits, () => undefined)] as const)));
  const allHits = name ? await ctx.get<Slot[]>("/activity").then(dayHits, () => undefined) : dayHits(slots);
  const hitLabel = (n: number | undefined, inactive?: boolean) => inactive ? " — inactive" : n == null ? "" : ` — ${number(n)} ${n === 1 ? "hit" : "hits"}`;
  const id = await ctx.identity();
  const start = slots[0]?.at, end = slots.at(-1)?.at;
  const endLabel = end ? slotEnd(end) : "";
  const zero = SERIES.filter(s => total(slots, s.key) === 0);
  const body = <>
    <PageHead icon={<Icon name="activity" />} title="Activity graphs"
      help={<Help id="activity-help" label="About activity history" title="Activity history"
        items={["The last 24 hours in ten-minute slots of the node's clock, 00:00 … 23:50, saved across restarts.", "Time the daemon was down reads as zero.", "The last slot is still counting.", "Unfiltered Refused is node-wide for the daemon Owner and covers visible records for others.", "Dequeued is not completion: a message taken is not a message finished.", "The chooser shows each record's hits: everything its five series counted in the last day."]} />}
      sub={<>{name ? <>Scope: <code>{name}</code></> : "Scope: visible records"}{start ? <> · {slotLabel(start)} to {endLabel}, ten-minute slots</> : null} · Uptime {id?.up || st.up || "unavailable"}</>}>
      <form method="get" class="scope-form">
        <label for="scope-name" class="sr-only">Record</label>
        <select id="scope-name" name="name" data-submit-on-change>
          <option value="">All visible{hitLabel(allHits)}</option>
          {names.map(n => <option value={n} selected={n === name}>{n}{hitLabel(hits.get(n), records.find(r => r.name === n)?.status === "inactive")}</option>)}
        </select>
      </form>
    </PageHead>
    {rec?.status === "inactive" ? <Empty icon="moon" title="Inactive record">An inactive record serves no activity while it is inactive; its history is kept and returns when it is reactivated.</Empty>
      : !slots.length ? <p class="muted">The daemon answered no activity.</p>
      : zero.length === SERIES.length ? <Empty icon="activity" title="A quiet day">All five series: <strong>0</strong> in the last day.</Empty>
      : <Card className="chart-card">
          <DayChart slots={slots} id="activity-chart" />
          {zero.length ? <p class="muted small">Zero all day: {zero.map(z => z.label).join(", ")}.</p> : null}
          <p class="muted small">Shared scale per ten-minute slot over the displayed nonzero series; a tick at every hour.</p>
        </Card>}
    {slots.length ? <details class="card slot-table">
      <summary>Slot values</summary>
      <div class="table-scroll"><table class="data fit" aria-label="Slot values, one row per ten-minute slot">
        <thead><tr><th>Slot</th>{SERIES.map(s => <th class="num">{s.label}</th>)}</tr></thead>
        <tbody>{slots.map(s => <tr><td>{slotLabel(s.at)}</td>{SERIES.map(x => <td class="num">{number(s[x.key])}</td>)}</tr>)}</tbody>
      </table></div>
    </details> : null}
  </>;
  return respond(ctx, { title: "Activity graphs", section: "activity", signedIn: true, you: st.you, charts: true }, body);
}

/** The end of a slot, ten minutes on, on the daemon's clock. */
function slotEnd(iso: string): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/.exec(iso);
  if (!m) return iso;
  const d = new Date(Date.UTC(+m[1]!, +m[2]! - 1, +m[3]!, +m[4]!, +m[5]! + 10));
  return slotLabel(d.toISOString());
}

// ---------------------------------------------------------------- Diagnostics

export const REASONS = ["acl", "busy", "credential", "enrolment", "full", "malformed", "name-taken", "second-reader", "suspended", "unknown"];

export type Envelope = {
  message_id: string; from: string; to: string; topic?: string; tag?: string; at: string; deadline?: string;
  receipt?: "ack" | "done"; re?: string; reply_to?: { name: string; topic?: string; tag?: string }; original_to?: string;
};
export type Exchange = { env: Envelope; receipts: Envelope[]; notice?: string; matches: string[]; late: boolean; latest: number };

const t = (iso?: string) => (iso ? new Date(iso).getTime() : 0);
const hasDeadline = (e: Envelope) => !!e.deadline && !e.deadline.startsWith("0001-");

export function replyRoute(e: Envelope) {
  return e.reply_to ? { to: e.reply_to.name, topic: e.reply_to.topic ?? e.topic ?? "", tag: e.reply_to.tag ?? e.tag ?? "" } : { to: e.from, topic: e.topic ?? "", tag: e.tag ?? "" };
}
const onRoute = (x: Envelope, orig: Envelope) => {
  const r = replyRoute(orig);
  return x.to === r.to && (x.topic ?? "") === r.topic && (x.tag ?? "") === r.tag;
};
/** A response answers a message when it comes from where the message went and travels its reply route. */
const answers = (x: Envelope, orig: Envelope) => (x.from === orig.to || x.from === orig.original_to) && onRoute(x, orig);

/** Rows from retained envelopes; bodies are never read (docs/web-face/node.md#diagnostics). */
export function exchanges(envs: Envelope[]): Exchange[] {
  const byId = new Map<string, Envelope[]>();
  for (const e of envs) byId.set(e.message_id, [...(byId.get(e.message_id) ?? []), e]);
  const rows = new Map<string, Exchange>();
  const out: Exchange[] = [];
  for (const e of envs) if (!e.receipt) { const x = { env: e, receipts: [], matches: [], late: false, latest: t(e.at) }; rows.set(e.message_id, x); out.push(x); }
  for (const r of envs) {
    if (!r.receipt) continue;
    let notice: string | undefined;
    const cands = r.re ? byId.get(r.re) ?? [] : [];
    if (!r.re) notice = "Receipt has no original message reference.";
    else if (!cands.length) notice = "Original message not in this visible history.";
    else if (cands.length > 1) notice = "Reference is ambiguous in this history. Kept separate.";
    else if (cands[0]!.receipt) notice = "The referenced message is itself a receipt. Kept separate.";
    // A receipt says which message it is about; whoever consumed a queue's
    // message sends it, so its route is checked, not its sender.
    else if (!onRoute(r, cands[0]!) || t(r.at) < t(cands[0]!.at)) notice = "Reference found; route or time differs from the original. Kept separate.";
    if (notice) { out.push({ env: r, receipts: [], notice, matches: [], late: false, latest: t(r.at) }); continue; }
    const row = rows.get(r.re!)!;
    row.receipts.push(r);
    row.latest = Math.max(row.latest, t(r.at));
  }
  for (const x of out) {
    if (x.env.receipt || !x.env.tag) continue;
    const earlier = envs.filter(o => !o.receipt && o !== x.env && t(o.at) <= t(x.env.at) && answers(x.env, o));
    x.matches = earlier.map(o => o.message_id);
    x.late = earlier.length === 1 && hasDeadline(earlier[0]!) && t(x.env.at) > t(earlier[0]!.deadline);
  }
  return out.sort((a, b) => b.latest - a.latest || a.env.message_id.localeCompare(b.env.message_id));
}

const Route = ({ e }: { e: Envelope }) => <>
  <code>{e.from}</code> → <code>{e.to}</code><br />
  <span class="muted small">Topic: {e.topic || "not supplied"} · Tag: {e.tag || "not supplied"}</span>
  {e.reply_to ? <><br /><span class="muted small">Reply route: <code>{e.reply_to.name}</code>{e.reply_to.topic ? ` · ${e.reply_to.topic}` : ""}{e.reply_to.tag ? ` · ${e.reply_to.tag}` : ""}</span></> : null}
</>;

function Evidence({ x }: { x: Exchange }): JSX.Element {
  const e = x.env;
  if (e.receipt) return <><strong>{e.receipt} receipt</strong> about <code>{e.re || "—"}</code><p class="muted small">{x.notice}</p></>;
  const ack = x.receipts.some(r => r.receipt === "ack"), done = x.receipts.some(r => r.receipt === "done");
  return <>
    {ack ? <p><span class="pill tone-info"><Icon name="check" />Acknowledgement observed.</span></p> : null}
    {done ? <p><span class="pill tone-ok"><Icon name="check-check" />Completion receipt observed.</span></p> : <p class="muted small">No completion receipt observed in retained history.</p>}
    {x.matches.length ? <p class="small">Possible response — matching earlier routes: {x.matches.map((m, i) => <>{i ? ", " : ""}<a href={`#message-${m}`}><code>{m}</code></a></>)}</p> : null}
    {x.late ? <p class="warn small">After the matching message's deadline.</p> : null}
    {x.receipts.length ? <details class="receipts"><summary>Receipt evidence</summary>
      <ul>{x.receipts.map(r => <li><strong>{r.receipt}</strong> <code>{r.message_id}</code> re <code>{r.re}</code> · <code>{r.from}</code> → <code>{r.to}</code> · {stamp(r.at)}{hasDeadline(e) && t(r.at) > t(e.deadline) ? " · after the request's deadline" : ""}</li>)}</ul>
    </details> : null}
  </>;
}

async function diagnostics(ctx: Ctx): Promise<Response> {
  const [st, records] = await Promise.all([ctx.status(), ctx.records()]);
  const [recent, users] = await Promise.all([
    ctx.get<Envelope[]>("/recent").then(r => ({ ok: r ?? [] as Envelope[] }), e => ({ err: sectionProblem(e) })),
    ctx.users().then(u => ({ ok: u }), e => ({ err: sectionProblem(e) })),
  ]);
  const refused = { ...Object.fromEntries(REASONS.map(r => [r, 0])), ...(st.refused ?? {}) } as Record<string, number>;
  const refusals = Object.entries(refused).sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]));
  const held = records.filter(r => (r.queued ?? 0) > 0).sort((a, b) => duration(b.oldest) - duration(a.oldest) || (b.queued ?? 0) - (a.queued ?? 0) || a.name.localeCompare(b.name));
  const loss = records.filter(r => (r.dropped ?? 0) + (r.expired ?? 0) > 0).sort((a, b) => ((b.dropped ?? 0) + (b.expired ?? 0)) - ((a.dropped ?? 0) + (a.expired ?? 0)) || a.name.localeCompare(b.name));
  const leftovers = "ok" in users ? users.ok.filter(u => u.kind !== "user") : [];
  const nameCell = (r: Rec) => <td data-label="Name"><a href={recordHref(r)} class="rec-link"><KindIcon kind={r.kind} /><code>{r.name}</code></a>{r.status === "inactive" ? <> <span class="badge">INACTIVE</span></> : null}</td>;
  const body = <>
    <PageHead icon={<Icon name="scan-search" />} title="Diagnostics" help={<Help label="About Diagnostics" tip="Caller-visible queues, loss and retained envelope evidence. Bodies are never shown." />}>
      <a href="/diagnostics" class="btn btn-ghost btn-sm"><Icon name="rotate-cw" />Refresh</a>
    </PageHead>
    <nav class="jump" aria-label="On this page">
      <a href="#refusals">Refusals</a><a href="#stuck">Held inboxes</a><a href="#exchanges">Exchanges</a><a href="#loss">Loss</a>{leftovers.length ? <a href="#leftovers">Leftover names</a> : null}
    </nav>
    <div class="grid-2">
      <Card title="Refusals" icon="shield-x" id="refusals" actions={<Help label="About refusals" tip="Whole-node handled API refusals since process start; zero is measured. Router misses and internal failures are excluded." />}>
        <table class="data fit"><caption class="sr-only">Refusals since this daemon started, by reason</caption>
          <thead><tr><th>Reason</th><th class="num">Count</th></tr></thead>
          <tbody>{refusals.map(([r, n]) => <tr class={n ? "hot" : ""}><td><code>{r}</code></td><td class="num">{number(n)}</td></tr>)}</tbody>
        </table>
      </Card>
      <Card title="Loss by name" icon="trash-2" id="loss">
        {loss.length ? <table class="data fit"><caption class="sr-only">Loss by name</caption>
          <thead><tr><th>Name</th><th class="num">Dropped</th><th class="num">Expired</th></tr></thead>
          <tbody>{loss.map(r => <tr>{nameCell(r)}<td class="num">{number(r.dropped)}</td><td class="num">{number(r.expired)}</td></tr>)}</tbody>
        </table> : <p class="muted">nothing lost</p>}
      </Card>
    </div>
    <Card title="Inboxes holding messages" icon="inbox" id="stuck">
      <table class="data stack"><caption>Inboxes holding messages, longest wait first — visible to you</caption>
        <thead><tr><th>Name</th><th class="num">Readers</th><th class="num">Held now</th><th>Oldest held</th><th>Capacity</th></tr></thead>
        <tbody>{held.length ? held.map(r => <tr>{nameCell(r)}
          <td class="num" data-label="Readers">{r.readers == null ? <Muted>unavailable</Muted> : number(r.readers)}</td>
          <td class="num" data-label="Held now">{number(r.queued)}</td>
          <td data-label="Oldest held">{r.oldest || <Muted>—</Muted>}</td>
          <td data-label="Capacity">{r.at_bound ? <b class="warn">at capacity when observed</b> : <Muted>—</Muted>}</td></tr>)
          : <tr><td colspan="5" class="muted">every queue you can see is empty</td></tr>}</tbody>
      </table>
    </Card>
    <Card title="Exchanges in retained history" icon="arrow-right-left" id="exchanges">
      {"err" in recent ? <p class="warn">Envelope history unavailable: {recent.err}</p>
        : !recent.ok.length ? <p class="muted">No envelopes in your retained history. This is not a count of all traffic.</p>
        : <table class="data stack exchanges"><caption>Messages and explicitly referenced receipts</caption>
            <thead><tr><th>Observed message</th><th>Route and conversation</th><th class="num">Envelopes</th><th>Evidence</th></tr></thead>
            <tbody>{exchanges(recent.ok).map(x => <tr id={`message-${x.env.message_id}`}>
              <td data-label="Observed message"><time datetime={x.env.at}>{stamp(x.env.at)}</time><br /><code class="small">{x.env.message_id}</code></td>
              <td data-label="Route and conversation"><Route e={x.env} /></td>
              <td class="num" data-label="Envelopes">{1 + x.receipts.length}</td>
              <td data-label="Evidence"><Evidence x={x} /></td>
            </tr>)}</tbody>
          </table>}
    </Card>
    {"err" in users ? <Card title="Leftover names" icon="ghost" id="leftovers"><p class="warn">Leftover names unavailable: {users.err}</p></Card>
      : leftovers.length ? <Card title="Leftover names" icon="ghost" id="leftovers">
        <table class="data stack"><thead><tr><th>Name</th><th>What it is</th><th>Next step</th></tr></thead>
          <tbody>{leftovers.map(u => {
            const href = `/user?${new URLSearchParams({ name: u.name, return: "/diagnostics" })}`;
            return <tr><td data-label="Name"><code>{u.name}</code></td>
              <td data-label="What it is">{u.kind === "record" ? "Self-owned record, no User profile" : "Credential with no record"}</td>
              <td data-label="Next step">{u.kind === "record" ? <a href={href}>Inspect before deciding</a> : u.can_remove ? <a href={href}>Review credential removal</a> : <Muted>An authorized administrator can review removal.</Muted>}</td></tr>;
          })}</tbody></table>
      </Card> : null}
  </>;
  return respond(ctx, { title: "Diagnostics", section: "diagnostics", signedIn: true, you: st.you }, body);
}

export { overview, diagnostics };
