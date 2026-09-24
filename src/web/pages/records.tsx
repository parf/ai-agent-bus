// Record pages: lists, register, detail, inactive view, settings, deactivate,
// Danger Zone, and the two POST handlers (docs/web-face/records.md).
// Authority is rendered, never computed: controls follow can_manage and
// can_transfer on the record the daemon answered for this visitor.
import { h, Fragment, type Child } from "../jsx.ts";
import { Ctx, NotFound, LocalProblem, ConditionsChanged, Refusal, type Rec, type Status, type UserRow } from "../ctx.ts";
import { respond, flashRedirect, type Section } from "../ui/frame.tsx";
import { Icon, Help, PageHead, Card, Name, Muted, KindIcon, KindPill, Pill, StatePill, Badge, Empty, Tabs, Segmented, Pager, Button, LinkButton, Avatar, Facts, recordHref, detailPath } from "../ui/kit.tsx";
import { TextField, LinesField, SecretField, SelectField, CheckField, ErrorSummary, FieldError, keep, terms, lines, type FormState, type FormError } from "../ui/forms.tsx";
import { DayChart, Ribbon, SERIES, total, type Slot } from "../ui/charts.tsx";
import { redirect, local, returnTo } from "../http.ts";
import { number, relative, stamp, slotLabel } from "../format.ts";
import { classify, formRefusal, lineRefusal, notYours, sectionProblem } from "../problem.tsx";
import { entity, MAINTAINER } from "../glyphs.ts";

// ------------------------------------------------------------------ kinds

type Kind = "agent" | "service" | "queue" | "pubsub";
type ListKey = "agents" | "services" | "queues" | "pubsub" | "personal";

export const KINDS: Record<Kind, { list: string; newPath: string; noun: string; plural: string; section: Section; lower: string; blurb: string }> = {
  agent: { list: "/agents", newPath: "/agents/new", noun: "Agent", plural: "agents", section: "agents", lower: "agent", blurb: "An agent is a model or a program with an inbox: peers find it by name and send it work." },
  service: { list: "/services", newPath: "/services/new", noun: "Service", plural: "services", section: "services", lower: "service", blurb: "A service is something outside the bus that callers reach at its address — a database, a mail relay, an API." },
  queue: { list: "/queues", newPath: "/queues/new", noun: "Queue", plural: "queues", section: "queues", lower: "queue", blurb: "A queue holds what was sent until one reader takes it; each message goes to exactly one consumer." },
  pubsub: { list: "/pubsub", newPath: "/pubsub/new", noun: "PubSub", plural: "pub/sub topics", section: "pubsub", lower: "pub/sub topic", blurb: "A pub/sub topic copies each publication to every inbox on its Deliver-To list." },
};
const isKind = (k: string): k is Kind => k in KINDS;

export function noun(kind: string) { return kind === "user" ? "User" : isKind(kind) ? KINDS[kind].noun : kind === "group" ? "Group" : kind; }
/** The list a record lives on; a Personal record lives on /personal. */
export function listOf(r: Rec): string {
  if (r.kind === "group") return "/groups";
  if (r.kind === "user") return "/users";
  if (r.personal) return "/personal";
  return isKind(r.kind) ? KINDS[r.kind].list : "/services";
}
function sectionOf(r: Rec): Section {
  if (r.kind === "user") return "queues";
  return isKind(r.kind) ? KINDS[r.kind].section : "services";
}

// ------------------------------------------------------------------ lists

const PAGE = 25;
const LIST_KIND: Record<Exclude<ListKey, "personal">, Kind> = { agents: "agent", services: "service", queues: "queue", pubsub: "pubsub" };

function listUrl(path: string, p: Record<string, string | undefined>, drop: string[] = []): string {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(p)) if (v && !drop.includes(k)) q.set(k, v);
  return q.size ? `${path}?${q}` : path;
}

async function list(ctx: Ctx, key: ListKey): Promise<Response> {
  const path = key === "personal" ? "/personal" : `/${key}`;
  const [st, all, id] = await Promise.all([ctx.status(), ctx.records(), ctx.identity()]);
  const you = st.you, owner0 = !!st.daemon_owner;
  const q = ctx.q("q"), scope = ctx.q("scope") === "my" ? "my" : "";
  const state = ["active", "inactive"].includes(ctx.q("state")) ? ctx.q("state") : "";
  const kindParam = key === "personal" && isKind(ctx.q("kind")) ? ctx.q("kind") as Kind : undefined;
  const services = key === "services", pubsub = key === "pubsub";
  const readers = !services && ["present", "none", "unavailable"].includes(ctx.q("readers")) ? ctx.q("readers") : "";
  const work = !services && !pubsub && ctx.q("work") === "held" ? "held" : "";
  const sort = ctx.q("sort") === "updated" || (ctx.q("sort") === "queued" && !services) ? ctx.q("sort") : "";
  let ownerParam = key === "personal" ? ctx.q("owner") : "";
  if (ownerParam && !owner0) return redirect(listUrl(path, { ...Object.fromEntries(ctx.url.searchParams), owner: undefined }));

  const records = all.filter(r => isKind(r.kind));
  const personalOf = (k?: Kind) => records.filter(r => r.personal && (!k || r.kind === k) && (owner0 || r.owner === you));
  const kind = key === "personal" ? undefined : LIST_KIND[key];
  let category = key === "personal" ? personalOf(kindParam).filter(r => !ownerParam || r.owner === ownerParam) : records.filter(r => r.kind === kind && !r.personal);
  if (scope === "my") category = category.filter(r => r.owner === you);

  const needle = q.toLowerCase();
  let rows = category.filter(r => {
    if (needle && ![r.name, r.descr ?? "", r.owner].some(v => v.toLowerCase().includes(needle))) return false;
    if (state === "active" && r.status === "inactive") return false;
    if (state === "inactive" && r.status !== "inactive") return false;
    if (readers === "present" && !((r.readers ?? 0) > 0)) return false;
    if (readers === "none" && r.readers !== 0) return false;
    if (readers === "unavailable" && r.readers != null) return false;
    if (work === "held" && !((r.queued ?? 0) > 0)) return false;
    return true;
  });
  rows.sort((a, b) => sort === "updated" ? (b.at ?? "").localeCompare(a.at ?? "") || a.name.localeCompare(b.name)
    : sort === "queued" ? ((pubsub ? b.in : b.queued) ?? 0) - ((pubsub ? a.in : a.queued) ?? 0) || a.name.localeCompare(b.name)
    : a.name.localeCompare(b.name));
  const matched = rows.length, pages = Math.max(1, Math.ceil(matched / PAGE));
  const pageNo = Math.min(pages, Math.max(1, Math.floor(Number(ctx.q("page"))) || 1));
  rows = rows.slice((pageNo - 1) * PAGE, pageNo * PAGE);

  const params = { q, scope, state, readers, work, sort, kind: kindParam, owner: ownerParam };
  const here = listUrl(path, { ...params, page: pageNo > 1 ? String(pageNo) : undefined });
  const keepFilters = { state, q, readers, work, sort };
  const title = key === "personal" ? "Personal" : KINDS[kind!].noun === "PubSub" ? "PubSub" : KINDS[kind!].noun + "s";
  // On /personal the chosen kind decides everything kind-shaped: the tabs, the
  // Register action and the sidebar; with no kind chosen it reads as Agents.
  const listKind: Kind = kind ?? kindParam ?? "agent";
  const kl = KINDS[listKind];
  const tabAll = records.filter(r => r.kind === listKind && !r.personal);
  const tabs = [
    { href: listUrl(kl.list, keepFilters), text: "All", count: tabAll.length, current: key !== "personal" && !scope },
    ...(listKind === "agent" || listKind === "service" ? [{ href: listUrl(kl.list, { ...keepFilters, scope: "my" }), text: "My", count: tabAll.filter(r => r.owner === you).length, current: key !== "personal" && scope === "my", className: "my-view" }] : []),
    { href: listUrl("/personal", { ...keepFilters, kind: listKind, owner: owner0 ? ownerParam : undefined }), text: "Personal", count: key === "personal" ? category.length : personalOf(listKind).length, current: key === "personal", className: "personal-view", icon: "lock" },
  ];
  const registerHref = key === "personal" ? `${kl.newPath}?personal=1` : kl.newPath;
  const registerText = key === "personal" ? `Register Personal ${kl.lower}` : `Register ${kl.lower}`;
  const filterLink = (name: string, value: string) => listUrl(path, { ...params, [name]: value || undefined });
  const owners = [...new Set(personalOf(kindParam).map(r => r.owner))].sort();
  const nodeOwner = id?.owner ?? "";

  const columns: { head: string; num?: boolean; cell: (r: Rec) => Child }[] = [
    { head: "Type", cell: r => <KindPill kind={r.kind} /> },
    { head: "Owner", cell: r => <code class={r.owner === you ? "you" : ""}>{r.owner}</code> },
    { head: "Status", cell: r => <StatePill inactive={r.status === "inactive"} /> },
  ];
  const readersCol = { head: "Readers", num: true, cell: (r: Rec) => r.readers == null ? <Muted>unavailable</Muted> : <>{number(r.readers)}</> };
  const heldCol = (head: string) => ({ head, num: true, cell: (r: Rec) => <>{number(r.queued)}{r.at_bound ? <> <Badge tone="warn">at capacity when observed</Badge></> : null}</> });
  const inCol = { head: "Accepted", num: true, cell: (r: Rec) => <>{number(r.in)}</> };
  const outCol = (head: string) => ({ head, num: true, cell: (r: Rec) => <>{number(r.out)}</> });
  const updCol = { head: "Updated", cell: (r: Rec) => <time datetime={r.at} title={stamp(r.at)}>{relative(r.at)}</time> };
  if (key === "agents" || key === "personal") columns.push(readersCol, heldCol("Queued"), inCol, outCol("Dequeued"), updCol);
  else if (services) columns.push({ head: "Address", cell: r => <code>{r.addr}</code> }, { head: "Protocol", cell: r => r.protocol ? <Pill>{r.protocol}</Pill> : <Muted>—</Muted> }, updCol);
  else if (key === "queues") columns.push(readersCol, heldCol("Held"), inCol, outCol("Dequeued"), updCol);
  else columns.push(inCol, outCol("Copies out"), { head: "Deliver-To", num: true, cell: r => <>{number(r.subs?.length ?? 0)}</> }, updCol);
  const firstHead = key === "personal" ? "Record" : KINDS[kind!].noun;

  const filtered = !!(q || state || readers || work || sort);
  const helpItems = key === "personal"
    ? ["Personal records belong to their Owner's own view instead of the shared lists; delivery is unchanged.", "The daemon Owner can choose one owner or see every Personal record visible to them."]
    : [KINDS[kind!].blurb, "Counts on the tabs are over records visible to you, before the filters below.", "Status: Active, or Inactive — hidden from use and kept.", ...(services ? [] : ["Readers: consumers waiting on this inbox now.", "Accepted and Dequeued: messages in and taken out since the daemon started."])];

  const body = <>
    <PageHead icon={<Icon name={key === "personal" ? "lock" : entity(kind!)!.icon} />} title={title}
      help={<Help id="service-views-help" label={`About ${title}`} title={title} items={helpItems} />}>
      {category.length ? <LinkButton href={registerHref} tone="primary" icon="plus">{registerText}</LinkButton> : null}
    </PageHead>
    <Tabs label="Record views" items={tabs} />
    {key === "personal" ? (owner0
      ? <form method="get" class="toolbar owner-chooser">
          {[["state", state], ["readers", readers], ["work", work], ["q", q], ["kind", kindParam ?? ""], ["sort", sort]].map(([n, v]) => v ? <input type="hidden" name={n} value={v} /> : null)}
          <label for="owner-select" class="seg-label">Owner</label>
          <select id="owner-select" name="owner" data-submit-on-change>
            <option value="">All visible owners</option>
            {owners.map(o => <option value={o} selected={o === ownerParam}>{o}</option>)}
          </select>
        </form>
      : <p class="muted">Owned by <code>{you}</code></p>) : null}
    <form class="toolbar record-search" method="get" action={path}>
      <div class="search"><Icon name="search" /><label for="record-query" class="sr-only">Search records</label>
        <input id="record-query" type="search" name="q" value={q} placeholder="Search by name, owner, or description" /></div>
      {[["scope", scope], ["state", state], ["readers", readers], ["work", work], ["kind", kindParam ?? ""], ["owner", ownerParam]].map(([n, v]) => v ? <input type="hidden" name={n} value={v} /> : null)}
      {!services ? <><label for="record-sort" class="sr-only">Sort</label>
        <select id="record-sort" name="sort" data-submit-on-change>
          <option value="">Name (A–Z)</option>
          <option value="updated" selected={sort === "updated"}>Recently updated</option>
          <option value="queued" selected={sort === "queued"}>{pubsub ? "Accepted (high–low)" : "Queued (high–low)"}</option>
        </select></> : <><label for="record-sort" class="sr-only">Sort</label>
        <select id="record-sort" name="sort" data-submit-on-change>
          <option value="">Name (A–Z)</option>
          <option value="updated" selected={sort === "updated"}>Recently updated</option>
        </select></>}
    </form>
    <div class="toolbar filters">
      {key === "personal" ? <Segmented label="Kind" items={[{ href: filterLink("kind", ""), text: "All", current: !kindParam },
        ...(Object.keys(KINDS) as Kind[]).map(k => ({ href: filterLink("kind", k), text: <><Icon name={entity(k)!.icon} />{KINDS[k].noun}</>, current: kindParam === k }))]} /> : null}
      <Segmented label="Status" items={[["", "All"], ["active", "Active"], ["inactive", "Inactive"]].map(([v, t]) => ({ href: filterLink("state", v!), text: t!, current: state === v }))} />
      {!services && !pubsub ? <>
        <Segmented label="Readers" items={[["", "All"], ["present", "Reading now"], ["none", "No reader now"], ["unavailable", "Unavailable"]].map(([v, t]) => ({ href: filterLink("readers", v!), text: t!, current: readers === v }))} />
        <Segmented label="Queue" items={[["", "All"], ["held", "Holding work"]].map(([v, t]) => ({ href: filterLink("work", v!), text: t!, current: work === v }))} />
      </> : null}
    </div>
    {category.length === 0 ? (key !== "personal" && personalOf(kind).length
      ? <Empty icon="lock" title={`No shared ${KINDS[kind!].plural}`} action={<a class="btn" href={`/personal?kind=${kind}`}>Open the Personal tab</a>}>
          {personalOf(kind).length} Personal {KINDS[kind!].lower}{personalOf(kind).length === 1 ? " is" : "s are"} under the Personal tab, which this list omits. {KINDS[kind!].blurb}
        </Empty>
      : <Empty icon={key === "personal" ? "lock" : entity(kind!)!.icon} title={`No ${key === "personal" ? "Personal records" : KINDS[kind!].plural} yet`}
          action={<LinkButton href={registerHref} tone="primary" icon="plus">{registerText}</LinkButton>}>
          {key === "personal" ? "Personal records belong to their Owner's own view instead of the shared lists." : KINDS[kind!].blurb}
        </Empty>)
      : matched === 0 ? <Empty icon="filter" title="No records match these filters" action={<a class="btn" href={listUrl(path, { scope, owner: owner0 ? ownerParam : undefined, kind: kindParam })}>Clear filters</a>}>Change the active filters above or clear filters.</Empty>
      : <>
        {key === "personal" ? <p class="muted small">{owner0 ? "This per-owner view contains only Personal records visible through your normal access; it is not a node-wide inventory." : "Your Personal records."}</p> : null}
        <div class="results-line"><span>Showing {(pageNo - 1) * PAGE + 1}–{Math.min(pageNo * PAGE, matched)} of {number(matched)} matching records, caller-visible on this page and not a count of this node.</span>
          {filtered ? <a href={listUrl(path, { scope, owner: owner0 ? ownerParam : undefined, kind: kindParam })}>Clear filters</a> : null}</div>
        <div class="table-wrap"><div class="table-scroll"><table class="data stack record-table">
          <thead><tr><th>{firstHead}</th>{columns.map(c => <th class={c.num ? "num" : ""}>{c.head}</th>)}</tr></thead>
          <tbody>{rows.map(r => <tr class={[r.owner === you ? "owned-record" : "", r.personal ? "personal-record" : "", r.status === "inactive" ? "inactive-record" : ""].join(" ")}>
            <td data-label={firstHead}><div class="rec-cell"><KindIcon kind={r.kind} owner={r.kind === "user" && r.name === nodeOwner} />
              <div class="rec-title"><a href={recordHref(r, { return: here })}>{r.descr ? <><strong>{r.descr}</strong><code>{r.name}</code></> : <strong><code>{r.name}</code></strong>}</a>
                {r.personal ? <span class="owned-mark">PERSONAL</span> : null}</div></div></td>
            {columns.map(c => <td class={c.num ? "num" : ""} data-label={c.head}>{c.cell(r)}</td>)}
          </tr>)}</tbody>
        </table></div></div>
        <Pager label="Record pages" prev={pageNo > 1 ? listUrl(path, { ...params, page: String(pageNo - 1) }) : undefined} next={pageNo < pages ? listUrl(path, { ...params, page: String(pageNo + 1) }) : undefined}>Page {pageNo} of {pages}</Pager>
      </>}
  </>;
  return respond(ctx, { title, section: kl.section, signedIn: true, you }, body);
}

// ------------------------------------------------------------------ record form

const RECORD_KEEP = ["from_personal", "name", "descr", "kind", "addr", "protocol", "personal", "allow", "subs", "ttl", "bound", "overflow", "maintainers", "edit_allow", "edit_subs", "edit_sharing", "edit_personal"];

function RecordFields({ kind, st, mode, rec, errId }: { kind: string; st: FormState; mode: "create" | "save"; rec?: Rec; errId: string }) {
  const n = noun(kind);
  const inbox = kind === "user";
  const v = (name: string, fallback: string) => st.values[name] ?? fallback;
  const personal = mode === "create" ? st.values.personal === "on" : st.values.edit_personal != null || st.values.personal != null ? st.values.personal === "on" : !!rec?.personal;
  const canAssign = mode === "create" || !!rec?.can_transfer;
  const allowHint = kind === "pubsub" ? "Who may publish here. Who receives is the Deliver-To list above."
    : personal ? `While Personal: the Owner, the Owner’s own agents, @owner${kind === "agent" ? " or @agent" : ""}, one per line.` : "One identity, group, @owner, or * per line.";
  return <div class="form-grid">
    {mode === "create"
      ? <TextField name="name" label="Name" st={st} errId={errId} required placeholder={kind === "agent" ? "#name@realm" : "name@realm"} id="create-name"
          help={{ label: "About the name", tip: "The routing identity callers use. It cannot be changed afterwards." }}
          hint={kind === "agent" ? "An agent's name starts with #." : undefined} />
      : <input type="hidden" name="name" value={rec!.name} />}
    <TextField name="descr" label="Description" st={st} errId={errId} value={v("descr", rec?.descr ?? "")} placeholder={`What this ${n} is for`} hint="Shown first in the registry." wide={mode !== "create"} />
    {kind === "service" ? <>
      <TextField name="addr" label="Address" st={st} errId={errId} required value={v("addr", rec?.addr ?? "")} placeholder="host:port, a path, or a URL" hint="Where a caller reaches it." />
      <TextField name="protocol" label="Protocol" st={st} errId={errId} required value={v("protocol", rec?.protocol ?? "")} placeholder="https, postgresql, smtp" hint="A hint for callers; the daemon checks nothing." />
    </> : null}
    {kind === "agent" || kind === "service" ? <SecretField name="secret" label="Secret" st={st} errId={errId} placeholder="PGPASSWORD=..."
      help={{ label: "About the secret", tip: `${kind === "agent" ? "Only the agent itself" : "Only the allow list"} reads them back, and no page ever shows them again.`, title: "Secrets", items: ["Opaque bytes, stored as given; CRLF becomes LF.", kind === "agent" ? "Only the agent itself reads it back, with agent-bus secret." : "Only names its allow list admits read it back, with agent-bus secret.", "No page ever shows it again: this box is always empty."] }}
      hint={mode === "create" ? "Optional. Leave empty to register without one." : "Leave empty to keep the stored credential. Anything here replaces it."} /> : null}
    {kind === "agent" || kind === "queue" || inbox ? <>
      <TextField name="ttl" label="Retention" st={st} errId={errId} value={v("ttl", rec?.ttl ?? "")} placeholder="default" hint="How long a message waits, e.g. 1h or 7d." />
      <TextField name="bound" label="Queue bound" st={st} errId={errId} type="number" min="0" value={v("bound", String(rec?.bound ?? 0))} hint="Whole number; 0 selects the default." />
      <SelectField name="overflow" label="When full" st={st} errId={errId} value={v("overflow", rec?.overflow ?? "strict")} options={[["strict", "Refuse new messages"], ["ring", "Drop the oldest"]]} />
    </> : null}
    {(kind === "agent" || kind === "queue") ? <><TextField name="subs" label="Deliver-To route" st={st} errId={errId} value={v("subs", (rec?.subs ?? []).join(" "))} placeholder="#agent@realm, queue@realm or topic@realm" wide
      help={{ label: "About the route", tip: `Empty keeps messages here. One destination: a message sent to this ${n} moves there instead.`, title: "Deliver-To route", items: [`Empty keeps messages in this ${n}’s own queue.`, `One destination: a message sent here moves there instead.`, `The destination’s allow list must list this ${n} itself.`] }}
      hint={`Empty keeps messages here. One destination: a message sent to this ${n} moves there instead, and the destination’s allow list must list this ${n} itself.`} />
      {mode === "save" ? <input type="hidden" name="edit_subs" value="1" /> : null}</> : null}
    {kind === "pubsub" ? <><LinesField name="subs" label="Deliver-To list" st={st} errId={errId} value={v("subs", (rec?.subs ?? []).join("\n"))} placeholder={"#agent@realm\nqueue@realm\n@group"}
      help={{ label: "About Deliver-To", tip: "One inbox per line; each gets a copy of every publication. Empty reaches nobody.", title: "Deliver-To list", items: ["One inbox per line: an agent, a queue, a user or a group.", "Each receives a copy of every publication.", "An empty list reaches nobody."] }} />
      {mode === "save" ? <input type="hidden" name="edit_subs" value="1" /> : null}</> : null}
    {!inbox ? <><LinesField name="allow" label={kind === "pubsub" ? "Who may publish" : kind === "queue" ? "Who may send" : "Allow list"} st={st} errId={errId}
      value={v("allow", (rec?.allow ?? []).join("\n"))} placeholder={"#agent@realm\nuser@realm\n@group\n@owner\n*"} hint={allowHint}
      help={{ label: "About the allow list", tip: "Who may reach this record. One term per line.", title: "Allow list", items: ["One identity, group, @owner (the record's Owner) or * (everyone) per line.", "An empty list admits only the Owner and Maintainers.", kind === "pubsub" ? "On a topic it says who may publish; who receives is the Deliver-To list." : "The daemon checks it on every call."] }} />
      {mode === "save" ? <input type="hidden" name="edit_allow" value="1" /> : null}</> : null}
    {!inbox ? <fieldset class="wide"><legend>Classification</legend>
      {mode === "save" && canAssign ? <input type="hidden" name="edit_personal" value="1" /> : null}
      <CheckField name="personal" label="Personal" st={st} errId={errId} checked={personal} disabled={!canAssign}
        hint={canAssign ? `Puts this ${n} in its Owner’s Personal view instead of the shared pages; delivery is unchanged. A Personal record's allow list and Maintainers may name only its Owner and the Owner's own agents.` : "Shown for reference: only this record's Owner or the daemon Owner may change it."} />
      {mode === "create" ? <input type="hidden" name="edit_personal" value="1" /> : null}
    </fieldset> : null}
    {mode === "save" && !inbox ? <>
      {canAssign ? <input type="hidden" name="edit_sharing" value="1" /> : null}
      <LinesField name="maintainers" label={<><Icon name={MAINTAINER.icon} />Maintainers</>} st={st} errId={errId} value={v("maintainers", (rec?.maintainers ?? []).join("\n"))} disabled={!canAssign}
        placeholder={"user@realm\n@group\n#agent@realm"} hint={canAssign ? "One user, group, agent or service per line; @owner is ACL-only. Maintainers manage this record as its Owner does, except transfer." : "Shown for reference: only this record's Owner or the daemon Owner may change its Maintainers."} />
    </> : null}
  </div>;
}

// ------------------------------------------------------------------ register

async function registerPage(ctx: Ctx, kind: Kind, st: FormState = { values: {} }, status = 200): Promise<Response> {
  const s = await ctx.status();
  const k = KINDS[kind];
  // Arriving from a Personal list: the form starts Personal, and Back returns there.
  const fromPersonal = ctx.q("personal") === "1" || st.values.from_personal === "1";
  if (fromPersonal && !st.error && st.values.personal == null) st = { ...st, values: { ...st.values, personal: "on" } };
  const back = fromPersonal ? { href: `/personal?kind=${kind}`, label: "Back to Personal" } : { href: k.list, label: `Back to ${kind === "pubsub" ? "PubSub" : k.noun + "s"}` };
  const body = <>
    <PageHead back={back} icon={<Icon name={entity(kind)!.icon} />} title={`Register ${k.lower}`}
      sub={<>You become its Owner. {k.blurb}</>} />
    <ErrorSummary id="create" error={st.error} />
    <form id="form-create" class="card form-card editor-card task-card" method="post" action="/service">
      <div class="card-body">
        <input type="hidden" name="action" value="create" /><input type="hidden" name="kind" value={kind} />
        {fromPersonal ? <input type="hidden" name="from_personal" value="1" /> : null}
        <RecordFields kind={kind} st={st} mode="create" errId="create-error" />
        <FieldError id="create-error" error={st.error} />
        <div class="actions"><Button tone="primary" icon="plus">Register {k.lower}</Button><a class="btn btn-ghost" href={back.href}>Cancel</a></div>
      </div>
    </form>
  </>;
  return respond(ctx, { title: `Register ${k.lower}`, section: k.section, signedIn: true, you: s.you }, body, status);
}

// ------------------------------------------------------------------ detail

function OwnerChip({ rec, users }: { rec: Rec; users: UserRow[] | null }) {
  const u = users?.find(x => x.kind === "user" && x.name === rec.owner);
  return u ? <a class="owner-chip" href={`/user?name=${encodeURIComponent(u.name)}`}><Avatar photo={u.photo_png} name={u.person_name || u.name} size="sm" /><span>Owner</span><code>{rec.owner}</code></a>
    : <span class="owner-chip"><Icon name="user-round" /><span>Owner</span><code>{rec.owner}</code></span>;
}

function routeState(r: Rec): Child {
  if ((r.subs ?? []).length !== 1) return null;
  const dest = r.subs![0]!;
  if (r.route_allowed === true) return <Pill tone="ok"><Icon name="check" />Route allowed now.</Pill>;
  if (r.route_allowed === false) return <p class="warn small">Configured, but <code>{dest}</code> does not allow <code>{r.name}</code> now — or <code>{dest}</code> is inactive or no longer registered. Add <code>{r.name}</code> to its allow list, or clear this route.</p>;
  return <Muted>Whether this route is usable now was not reported.</Muted>;
}

async function detail(ctx: Ctx, pathKind: string): Promise<Response> {
  const name = ctx.q("name");
  if (!name) throw new NotFound();
  const [st, inactive] = await Promise.all([ctx.status(), ctx.inactive().catch(() => [] as Rec[])]);
  const off = inactive.find(r => r.name === name);
  if (off) return inactiveView(ctx, st, off);
  const rec = await ctx.lookup(name);
  if (rec.kind === "group") return redirect(`/group?name=${encodeURIComponent(rec.name)}`, 302);
  const canonical = detailPath(rec.kind);
  if (canonical !== pathKind) return redirect(`${canonical}${ctx.url.search}`, 302);
  const [users, act] = await Promise.all([
    ctx.users().catch(() => null),
    ctx.get<Slot[]>("/activity", { name }).then(a => ({ ok: a ?? [] }), e => ({ err: sectionProblem(e) })),
  ]);
  const back = returnTo(ctx.q("return"), [listOf(rec)], listOf(rec));
  const manage = !!rec.can_manage, inbox = rec.kind === "user", n = noun(rec.kind);
  const editHref = `${detailPath(rec.kind)}/edit?${new URLSearchParams({ name, ...(ctx.q("return") ? { return: back } : {}) })}`;
  const slots = "ok" in act ? act.ok : [];
  const onList = (rec.subs ?? []).includes(st.you);

  const counters = rec.kind !== "service" ? <Card title="Queue & counters" icon="gauge">
    <Facts rows={[
      ["Readers", rec.readers == null ? <Muted>unavailable</Muted> : number(rec.readers)],
      ["Held now", <>{number(rec.queued)}{rec.at_bound ? <> <Badge tone="warn">at capacity when observed</Badge></> : null}</>],
      ["Oldest held", (rec.queued ?? 0) > 0 && rec.oldest ? rec.oldest : <Muted>—</Muted>],
      ["Accepted", number(rec.in)], ["Dequeued", number(rec.out)], ["Dropped / expired", `${number(rec.dropped)} / ${number(rec.expired)}`],
    ]} />
  </Card> : null;
  const policy = rec.kind !== "service" ? <Card title="Policy" icon="sliders-horizontal">
    <Facts rows={[
      ["Reached", rec.protocol ? "external" : "this bus"], ["Queue bound", rec.bound ? number(rec.bound) : "default"],
      ["Retention", rec.ttl || "none"], ["When full", rec.overflow === "ring" ? "drop the oldest" : "refuse"],
    ]} />
  </Card> : null;
  const where = rec.kind === "service" ? <Card title="Where it is" icon="map-pin">
    <Facts rows={[["Address", <code>{rec.addr}</code>], ["Protocol", rec.protocol ? <Pill>{rec.protocol}</Pill> : <Muted>—</Muted>], ["Secret", rec.secret_sha ? <code title="SHA-256 prefix, never the bytes">{rec.secret_sha}</code> : "none"]]} />
  </Card> : null;

  const body = <>
    <PageHead back={{ href: back, label: "Back to records" }} icon={<Icon name={entity(rec.kind)?.icon ?? "box"} />}
      title={<><Name copy>{rec.name}</Name>{rec.personal ? <span class="muted"> · Personal</span> : null}</>}
      sub={<div class="meta-row">
        <KindPill kind={rec.kind} />
        <OwnerChip rec={rec} users={users} />
        {rec.maintainers?.length ? <span class="pill"><Icon name={MAINTAINER.icon} />Maintainers: {rec.maintainers.join(", ")}</span> : null}
        <span class="pill"><Icon name={rec.kind === "pubsub" ? "megaphone" : "arrow-right"} />Delivery: {rec.kind === "pubsub" ? "a copy to each subscriber" : "one at a time"}</span>
      </div>}>
      {manage ? <LinkButton href={editHref} icon="settings">Edit settings</LinkButton> : null}
    </PageHead>
    {rec.descr ? <p class="descr lead-descr">{rec.descr}</p> : null}
    {!manage ? <p class="muted small"><Icon name="eye" /> You can view this record; its owner and assigned maintainers can manage it.</p> : null}
    <div class="detail-grid">
      <div>
        <Card title="Activity" icon="activity" actions={<a class="btn btn-ghost btn-sm" href={`/activity?name=${encodeURIComponent(name)}`}>All activity<Icon name="chevron-right" /></a>}>
          {"err" in act ? <p class="warn">Activity unavailable: {act.err}</p>
            : !slots.length ? <p class="muted">The daemon answered no activity.</p>
            : <>
              <p class="muted small">Scope: <code>{name}</code> · {slotLabel(slots[0]!.at)} to {slotLabel(slots.at(-1)!.at)}, ten-minute slots.</p>
              <Ribbon slots={slots} label={`Traffic per ten-minute slot for ${name}`} />
              {SERIES.some(s => total(slots, s.key) > 0) ? <DayChart slots={slots} id="record-chart" compact />
                : <p class="muted small">All five series: <strong>0</strong> in the last day.</p>}
            </>}
        </Card>
        {rec.kind === "agent" || rec.kind === "queue" ? <Card title="Deliver-To route" icon="waypoints" id="route" className="route-card">
          {(rec.subs ?? []).length ? <>
            <div class="route-flow"><code>{rec.name}</code><Icon name="arrow-right" /><code>{rec.subs![0]}</code></div>
            {routeState(rec)}
            <p class="muted small">The destination checks this {n} itself against its allow list — not the sender, and not its Owner.</p>
          </> : <p class="muted">No route. A message sent here stays in this {n}’s own queue.</p>}
          {manage ? <p><a href={`${editHref}#f-subs`}>{(rec.subs ?? []).length ? "Replace or clear the route" : "Set a route"}</a> in the settings.</p> : null}
        </Card> : null}
        {rec.kind === "pubsub" ? <Card title="Deliver-To" icon="send" id="subscribers" actions={<span class="count">{rec.subs?.length ?? 0}</span>}>
          {(rec.subs ?? []).length ? <ul class="subs-list">{rec.subs!.map(s => <li><code>{s}</code>
            {manage ? <form method="post" action="/service" class="inline-form"><input type="hidden" name="name" value={rec.name} /><input type="hidden" name="subscriber" value={s} />
              <Button name="action" value="remove-subscriber" tone="ghost btn-sm" icon="x">Remove</Button></form> : null}</li>)}</ul>
            : <p class="muted">Nobody. A publication here reaches no inbox.</p>}
          {onList ? <form method="post" action="/service" class="actions"><input type="hidden" name="name" value={rec.name} /><Button name="action" value="unsubscribe" icon="log-out">Take my inbox off this list</Button></form> : null}
        </Card> : null}
      </div>
      <div>
        <Card title="Status" icon="power">
          <div class="status-line"><span class="big-state"><StatePill /></span>
            {manage && !inbox ? <form method="get" action="/service-deactivate" class="inline-form"><input type="hidden" name="name" value={rec.name} /><Button tone="ghost btn-sm" icon="power">Deactivate…</Button></form> : null}</div>
          <p class="muted small">Updated {stamp(rec.at)} · Config {rec.config_sha || "—"}</p>
        </Card>
        {where}{policy}{counters}
        {manage && !inbox ? <a class="danger-link" href={`/service-danger?name=${encodeURIComponent(rec.name)}`}><span><Icon name="flame" /> Danger Zone</span><span class="small">configuration{rec.can_transfer && !inbox ? ", transfer" : ""}{inbox ? "" : ", removal"}</span></a> : null}
      </div>
    </div>
  </>;
  return respond(ctx, { title: `${n} ${rec.name}`, section: sectionOf(rec), signedIn: true, you: st.you, charts: true }, body);
}

async function inactiveView(ctx: Ctx, st: Status, rec: Rec): Promise<Response> {
  const n = noun(rec.kind);
  const back = returnTo(ctx.q("return"), [listOf(rec)], listOf(rec));
  const body = <>
    <PageHead back={{ href: back, label: "Back to records" }} icon={<Icon name={entity(rec.kind)?.icon ?? "box"} />} title={<><Name>{rec.name}</Name> <Badge>INACTIVE</Badge></>}
      sub={<div class="meta-row"><KindPill kind={rec.kind} /><span class="owner-chip"><Icon name="user-round" /><span>Owner</span><code>{rec.owner}</code></span></div>} />
    <Card title="Status" icon="power" className="confirm-card">
      <p class="big-state"><StatePill inactive /></p>
      <ul>
        <li>It is hidden from listings for everyone but those who may manage it, and refuses use.</li>
        <li>Its queued work is kept{rec.kind === "service" ? "" : <>: {number(rec.queued)} held when observed</>}.</li>
        <li>Reactivating returns it exactly as it was.</li>
      </ul>
      {rec.can_manage && rec.kind !== "user"
        ? <form method="post" action="/service" class="actions"><input type="hidden" name="name" value={rec.name} /><Button name="action" value="reactivate" tone="primary" icon="power">Reactivate</Button></form>
        : <p class="muted">Its owner and assigned maintainers can reactivate it.</p>}
    </Card>
  </>;
  return respond(ctx, { title: `${n} ${rec.name}`, section: sectionOf(rec), signedIn: true, you: st.you }, body);
}

/** The record for a page that needs it active and visible; inactive reads as absent. */
async function activeRecord(ctx: Ctx, name: string): Promise<Rec> {
  if (!name) throw new NotFound();
  const inactive = await ctx.inactive().catch(() => [] as Rec[]);
  if (inactive.some(r => r.name === name)) throw new NotFound();
  return ctx.lookup(name);
}

// ------------------------------------------------------------------ settings

async function settingsPage(ctx: Ctx, name: string, st: FormState = { values: {} }, status = 200): Promise<Response> {
  const [s, rec] = await Promise.all([ctx.status(), activeRecord(ctx, name)]);
  if (rec.kind === "group") return redirect(`/group/edit?name=${encodeURIComponent(rec.name)}`, 302);
  if (!rec.can_manage) throw notYours("only the owner or an assigned Maintainer can change this record's settings");
  const ret = returnTo(st.values.return ?? ctx.q("return"), [listOf(rec)], "");
  const detailHref = recordHref(rec, ret ? { return: ret } : undefined);
  const body = <>
    <PageHead back={{ href: detailHref, label: `Back to ${rec.name}` }} icon={<Icon name={entity(rec.kind)?.icon ?? "box"} />} title={<>Edit {noun(rec.kind)} <Name>{rec.name}</Name></>} />
    <ErrorSummary id="save" error={st.error} />
    <form id="form-save" class="card form-card editor-card task-card" method="post" action="/service">
      <div class="card-body">
        <input type="hidden" name="action" value="save" />
        {ret ? <input type="hidden" name="return" value={ret} /> : null}
        <RecordFields kind={rec.kind} st={st} mode="save" rec={rec} errId="save-error" />
        <FieldError id="save-error" error={st.error} />
        <div class="actions"><Button tone="primary" icon="check">Save settings</Button><a class="btn btn-ghost" href={detailHref}>Cancel</a></div>
      </div>
    </form>
    <a class="danger-link" href={`/service-danger?name=${encodeURIComponent(rec.name)}`}><span><Icon name="flame" /> Danger Zone</span><Icon name="chevron-right" /></a>
  </>;
  return respond(ctx, { title: `Edit ${rec.name}`, section: sectionOf(rec), signedIn: true, you: s.you }, body, status);
}

// ------------------------------------------------------------------ deactivate

async function deactivatePage(ctx: Ctx): Promise<Response> {
  const [s, rec] = await Promise.all([ctx.status(), activeRecord(ctx, ctx.q("name"))]);
  if (!rec.can_manage || rec.kind === "user") throw notYours("only the owner or an assigned Maintainer can deactivate this record");
  const body = <>
    <PageHead back={{ href: recordHref(rec), label: `Back to ${rec.name}` }} icon={<Icon name="triangle-alert" />} title="Confirm deactivation" />
    <Card className="confirm-card" tone="danger" title={<>Deactivate <code>{rec.name}</code>?</>} icon="power">
      <ul>
        <li>It disappears from listings and refuses use until it is reactivated.</li>
        <li>Queued work is kept ({number(rec.queued)} held now), and running processes are not stopped.</li>
        <li>Its owner and maintainers reactivate it from its Inactive view.</li>
      </ul>
      <form method="post" action="/service" class="actions"><input type="hidden" name="name" value={rec.name} />
        <Button name="action" value="deactivate" tone="danger" icon="power">Deactivate {rec.name}</Button>
        <a class="btn btn-ghost" href={recordHref(rec)}>Cancel</a></form>
    </Card>
  </>;
  return respond(ctx, { title: `Confirm deactivation · ${rec.name}`, section: sectionOf(rec), signedIn: true, you: s.you }, body);
}

// ------------------------------------------------------------------ danger zone

export async function dangerPage(ctx: Ctx, name: string, err?: { section: "configure" | "transfer"; error: FormError; owner?: string }, status = 200): Promise<Response> {
  const [s, rec] = await Promise.all([ctx.status(), activeRecord(ctx, name)]);
  if (!rec.can_manage) throw notYours("only the owner or an assigned Maintainer can manage this record");
  if (rec.kind === "user") throw notYours("a user's inbox has no configuration, transfer or removal; it follows its user");
  const group = rec.kind === "group";
  const prefixed = group && /^@[^/]+\//.test(rec.name);
  const configurable = rec.kind === "agent" || rec.kind === "service" || group;
  const transferable = !!rec.can_transfer && rec.name !== "@administrators" && rec.name !== rec.owner && !prefixed;
  const cfgSt: FormState = { values: {}, error: err?.section === "configure" ? err.error : undefined };
  const trSt: FormState = { values: { owner: err?.owner ?? "" }, error: err?.section === "transfer" ? err.error : undefined };
  const backHref = group ? `/group?name=${encodeURIComponent(rec.name)}` : recordHref(rec);
  const body = <>
    <PageHead back={{ href: backHref, label: `Back to ${rec.name}` }} icon={<Icon name="flame" />} title={<>Danger Zone · <Name>{rec.name}</Name></>}
      sub="Each action here changes who controls this record or what it holds. Transfer and removal ask once more before they happen." />
    <ErrorSummary id={err?.section ?? "configure"} error={err?.error} />
    <div class="stack-gap">
      {configurable ? <Card title="Replace configuration" icon="file-cog" tone="danger">
        <form id="form-configure" method="post" action="/service">
          <input type="hidden" name="name" value={rec.name} /><input type="hidden" name="action" value="configure" />
          <SecretField name="config" label="New configuration (JSON)" st={cfgSt} errId="configure-error" rows={6} required placeholder='{"key": "value"}'
            hint="Existing private configuration and a refused replacement are never displayed. Only the record itself reads it back." />
          <FieldError id="configure-error" error={cfgSt.error} />
          <div class="actions"><Button tone="danger" icon="file-cog">Replace configuration</Button></div>
        </form>
      </Card> : null}
      {transferable ? <Card title="Transfer ownership" icon="arrow-right-left" tone="danger">
        <form id="form-transfer" method="post" action="/service-confirm">
          <input type="hidden" name="name" value={rec.name} /><input type="hidden" name="action" value="transfer" />
          <TextField name="owner" label="New owner" st={trSt} errId="transfer-error" required placeholder="user@realm" hint="The new Owner takes every right on it, transfer included. Credentials already issued for it keep working." />
          <FieldError id="transfer-error" error={trSt.error} />
          <div class="actions"><Button tone="danger" icon="arrow-right-left">Continue to confirmation</Button></div>
        </form>
      </Card> : prefixed ? <Card title="Transfer ownership" icon="arrow-right-left"><p class="muted">A group named for its owner (<code>{rec.name.split("/")[0]}/…</code>) is never transferred. Its new owner creates their own instead.</p></Card> : null}
      {!group && rec.kind !== "user" ? <Card title="Remove registration" icon="trash-2" tone="danger">
        <p>No registration, no access: the name stops routing, its queue and counters go, and anyone holding it loses it.</p>
        <form method="post" action="/service-confirm"><input type="hidden" name="name" value={rec.name} /><input type="hidden" name="action" value="delete" />
          <div class="actions"><Button tone="danger" icon="trash-2">Continue to confirmation</Button></div></form>
      </Card> : group ? <Card title="Removal" icon="trash-2"><p class="muted">A group is retired by emptying its members, never removed: records that name it keep a stable meaning.</p></Card> : null}
    </div>
  </>;
  return respond(ctx, { title: `Danger Zone · ${rec.name}`, section: group ? "groups" : sectionOf(rec), signedIn: true, you: s.you }, body, status);
}

async function postConfirm(ctx: Ctx): Promise<Response> {
  const name = ctx.f("name"), action = ctx.f("action");
  const [s, rec] = await Promise.all([ctx.status(), activeRecord(ctx, name)]);
  const back = `/service-danger?name=${encodeURIComponent(name)}`;
  if (action === "transfer") {
    if (!rec.can_transfer || rec.name === rec.owner) throw notYours("only this record's owner or the daemon owner can transfer it");
    const owner = ctx.f("owner").trim();
    if (!owner) return dangerPage(ctx, name, { section: "transfer", error: { message: "New owner is required.", field: "owner", status: 400 } }, 400);
    const body = <>
      <PageHead back={{ href: back, label: "Back to the Danger Zone" }} icon={<Icon name="arrow-right-left" />} title="Confirm ownership transfer" />
      <Card className="confirm-card" tone="danger" title={<>Transfer <code>{rec.name}</code> from <code>{rec.owner}</code> to <code>{owner}</code>?</>} icon="arrow-right-left">
        <ul><li>{owner} becomes its Owner, with every right on it, transfer included.</li><li>You keep only what its allow list and Maintainers give you.</li></ul>
        <form method="post" action="/service" class="actions">
          {[["action", "transfer"], ["name", rec.name], ["owner", owner], ["expected_owner", rec.owner], ["confirmed", "1"]].map(([n, v]) => <input type="hidden" name={n} value={v} />)}
          <Button tone="danger" icon="arrow-right-left">Transfer ownership</Button><a class="btn btn-ghost" href={back}>Cancel</a>
        </form>
      </Card>
    </>;
    return respond(ctx, { title: `Confirm ownership transfer · ${rec.name}`, section: sectionOf(rec), signedIn: true, you: s.you }, body);
  }
  if (action === "delete") {
    if (!rec.can_manage || rec.kind === "group" || rec.kind === "user") throw notYours("only the owner or an assigned Maintainer can remove this record");
    const readers = rec.readers == null ? "unavailable" : String(rec.readers);
    const body = <>
      <PageHead back={{ href: back, label: "Back to the Danger Zone" }} icon={<Icon name="trash-2" />} title="Confirm removal" />
      <Card className="confirm-card" tone="danger" title={<>Remove <code>{rec.name}</code>?</>} icon="trash-2">
        <p>It currently holds {number(rec.queued)} messages and has {readers} outstanding reads.</p>
        <ul><li>The name stops routing and its queue is discarded.</li><li>This cannot be undone; registering the name again starts from nothing.</li></ul>
        <form method="post" action="/service" class="actions">
          {[["action", "delete"], ["name", rec.name], ["expected_owner", rec.owner], ["expected_queued", String(rec.queued ?? 0)], ["expected_readers", readers], ["confirmed", "1"]].map(([n, v]) => <input type="hidden" name={n} value={v} />)}
          <Button tone="danger" icon="trash-2">Remove registration</Button><a class="btn btn-ghost" href={back}>Cancel</a>
        </form>
      </Card>
    </>;
    return respond(ctx, { title: `Confirm removal · ${rec.name}`, section: sectionOf(rec), signedIn: true, you: s.you }, body);
  }
  throw new LocalProblem(400, "That confirmation action is not available.");
}

// ------------------------------------------------------------------ POST /service

const bodyOf = (ctx: Ctx, name: string) => ctx.form?.get(name) ?? undefined;

/** A refusal naming a submitted line of a list field: which field and line, prefixed "Line N: ". */
function attribute(ctx: Ctx, message: string, fields: string[]): { field: string; line: number; message: string } | undefined {
  const hits = fields.map(f => ({ f, line: lineRefusal(message, lines(ctx.f(f))) })).filter(x => x.line > 0);
  if (!hits.length) return undefined;
  const label: Record<string, string> = { subs: "Deliver-To", allow: "Allow list", maintainers: "Maintainers", members: "Members" };
  const prefix = hits.length > 1 ? hits.map(x => `${label[x.f]} line ${x.line}`).join(" and ") + ": " : `Line ${hits[0]!.line}: `;
  return { field: hits[0]!.f, line: hits[0]!.line, message: message.startsWith("Line ") ? message : prefix + message };
}

async function storeSecret(ctx: Ctx, name: string, what: string): Promise<void> {
  const secret = ctx.f("secret");
  if (!secret) return;
  try {
    await ctx.bus("POST", "/secret", { body: { name, secret: secret.replace(/\r\n/g, "\n") } });
  } catch (e) {
    const reason = e instanceof Refusal ? e.detail : "the daemon did not answer";
    throw new LocalProblem(502, `${what} and its secret was not stored: ${reason} Set it with: agent-bus secret ${name} '...'`, true);
  }
}

async function postService(ctx: Ctx): Promise<Response> {
  const action = ctx.f("action"), name = ctx.f("name");
  await ctx.status();
  switch (action) {
    case "create": {
      const kind = ctx.f("kind");
      if (!isKind(kind)) throw new LocalProblem(400, "Choose a valid record kind.");
      const values = keep(ctx.form, RECORD_KEEP);
      const bound = ctx.f("bound").trim();
      if (bound && !/^\d+$/.test(bound)) return registerPage(ctx, kind, { values, error: { message: "Queue capacity must be a whole number.", field: "bound", status: 400 } }, 400);
      const body: Record<string, unknown> = { name, kind, descr: ctx.f("descr"), allow: terms(ctx.f("allow")), personal: ctx.f("personal") === "on", subs: terms(ctx.f("subs")) };
      if (kind === "service") { body.addr = ctx.f("addr"); body.protocol = ctx.f("protocol"); }
      if (kind === "agent" || kind === "queue") { body.ttl = ctx.f("ttl"); body.overflow = ctx.f("overflow") || "strict"; body.bound = Number(bound || 0); }
      try {
        await ctx.bus("POST", "/register", { body, headers: { "If-None-Match": "*" } });
      } catch (e) {
        const r = formRefusal(e);
        const line = r && r.status !== 412 ? attribute(ctx, r.message, ["subs", "allow"]) : undefined;
        if (!r || (!r.preserve && !line)) throw e;
        return registerPage(ctx, kind, { values, error: { status: r.status, message: line?.message ?? r.message, field: line?.field ?? (r.status === 412 ? "name" : undefined), line: line?.line } }, r.status);
      }
      await storeSecret(ctx, name, `The ${noun(kind)} was registered`);
      const rec = await ctx.lookup(name).catch(() => undefined);
      return flashRedirect(ctx, rec ? recordHref(rec) : `/service?name=${encodeURIComponent(name)}`, "registered");
    }
    case "save": {
      const rec = await activeRecord(ctx, name);
      const values = keep(ctx.form, [...RECORD_KEEP, "return"]);
      const change: Record<string, unknown> = { name, descr: ctx.f("descr") };
      if (ctx.form!.has("addr") || ctx.form!.has("protocol")) { change.addr = ctx.f("addr"); change.protocol = ctx.f("protocol"); }
      if (ctx.form!.has("bound") || ctx.form!.has("ttl") || ctx.form!.has("overflow")) {
        const b = ctx.f("bound").trim();
        if (b && !/^\d+$/.test(b)) return settingsPage(ctx, name, { values, error: { message: "Queue capacity must be a whole number.", field: "bound", status: 400 } }, 400);
        change.bound = Number(b || 0); change.ttl = ctx.f("ttl"); change.overflow = ctx.f("overflow") || "strict";
      }
      if (ctx.form!.has("edit_allow")) change.allow = terms(ctx.f("allow"));
      if (ctx.form!.has("edit_subs")) change.subs = terms(ctx.f("subs"));
      if (ctx.form!.has("edit_sharing")) change.maintainers = terms(ctx.f("maintainers"));
      if (ctx.form!.has("edit_personal")) change.personal = ctx.f("personal") === "on";
      try {
        await ctx.bus("POST", "/manage", { body: change });
      } catch (e) {
        const r = formRefusal(e);
        const line = r ? attribute(ctx, r.message, ["subs", "allow", ...(ctx.form!.has("edit_sharing") ? ["maintainers"] : [])]) : undefined;
        if (!r || (!r.preserve && !line)) throw e;
        const field = line?.field ?? (/maintainer/i.test(r.message) ? "maintainers" : undefined);
        ctx.forget("/lookup"); ctx.forget("/inactive");
        return settingsPage(ctx, name, { values, error: { status: r.status, message: line?.message ?? r.message, field, line: line?.line } }, r.status);
      }
      await storeSecret(ctx, name, "The settings were saved");
      ctx.forget("/lookup");
      const fresh = await ctx.lookup(name).catch(() => rec);
      const ret = returnTo(ctx.f("return"), [listOf(fresh)], "");
      return flashRedirect(ctx, recordHref(fresh, ret ? { return: ret } : undefined), "saved");
    }
    case "configure": {
      const raw = ctx.f("config");
      try { JSON.parse(raw); } catch {
        return dangerPage(ctx, name, { section: "configure", error: { message: "Configuration must be valid JSON. The submitted configuration is not shown again.", field: "config", status: 400 } }, 400);
      }
      try {
        await ctx.bus("POST", "/configure", { body: { Name: name, Config: JSON.parse(raw) } });
      } catch (e) {
        const r = formRefusal(e);
        if (!r?.preserve) throw e;
        return dangerPage(ctx, name, { section: "configure", error: { message: r.message, status: r.status } }, r.status);
      }
      const rec = await ctx.lookup(name);
      return flashRedirect(ctx, rec.kind === "group" ? `/group?name=${encodeURIComponent(name)}` : recordHref(rec), "configured");
    }
    case "deactivate": {
      const rec = await activeRecord(ctx, name);
      if (rec.kind === "user") throw notYours("a user's inbox follows its user; deactivate the user instead");
      await ctx.bus("POST", "/manage", { body: { name, status: "inactive" } });
      return flashRedirect(ctx, recordHref(rec), "deactivated");
    }
    case "reactivate": {
      await ctx.bus("POST", "/manage", { body: { name, status: "active" } });
      const rec = await ctx.lookup(name);
      return flashRedirect(ctx, recordHref(rec), "reactivated");
    }
    case "transfer": {
      if (ctx.f("confirmed") !== "1") throw new LocalProblem(400, "A transfer is confirmed on its confirmation page first. Start it from the Danger Zone.");
      const rec = await activeRecord(ctx, name);
      const back = `/service-danger?name=${encodeURIComponent(name)}`;
      if (rec.owner !== ctx.f("expected_owner") || !rec.can_transfer || rec.name === rec.owner)
        throw new ConditionsChanged(`${name} changed since the confirmation: its owner or your right to transfer it is not what the confirmation showed.`, back);
      const owner = ctx.f("owner").trim();
      try {
        await ctx.bus("POST", "/manage", { body: { name, owner } });
      } catch (e) {
        const r = formRefusal(e);
        if (!r?.preserve) throw e;
        ctx.forget("/lookup");
        return dangerPage(ctx, name, { section: "transfer", error: { message: r.message, field: "owner", status: r.status }, owner }, r.status);
      }
      ctx.forget("/lookup"); ctx.forget("/ls");
      const after = await ctx.lookup(name).catch(() => undefined);
      return flashRedirect(ctx, after ? (after.kind === "group" ? `/group?name=${encodeURIComponent(name)}` : recordHref(after)) : listOf(rec), "transferred");
    }
    case "delete": {
      if (ctx.f("confirmed") !== "1") throw new LocalProblem(400, "A removal is confirmed on its confirmation page first. Start it from the Danger Zone.");
      const rec = await activeRecord(ctx, name);
      const back = `/service-danger?name=${encodeURIComponent(name)}`;
      const readers = rec.readers == null ? "unavailable" : String(rec.readers);
      if (rec.owner !== ctx.f("expected_owner") || String(rec.queued ?? 0) !== ctx.f("expected_queued") || readers !== ctx.f("expected_readers") || !rec.can_manage)
        throw new ConditionsChanged(`${name} changed since the confirmation: its owner, held messages or outstanding reads are not what the confirmation showed.`, back);
      if (rec.kind === "user" || rec.kind === "group") throw notYours("a user's inbox or a group is not removed here");
      await ctx.bus("POST", "/unregister", { body: { name } });
      return flashRedirect(ctx, listOf(rec), "removed");
    }
    case "unsubscribe": {
      await ctx.bus("POST", "/subscribe", { body: { channel: name, off: true } });
      const rec = await ctx.lookup(name);
      return flashRedirect(ctx, recordHref(rec), "unsubscribed");
    }
    case "remove-subscriber": {
      await ctx.bus("POST", "/subscriber/remove", { body: { channel: name, subscriber: ctx.f("subscriber") } });
      const rec = await ctx.lookup(name);
      return flashRedirect(ctx, recordHref(rec) + "#subscribers", "recipient-removed");
    }
  }
  throw new LocalProblem(400, "That service action is not available.");
}

// ------------------------------------------------------------------ legacy addresses

function channelsRedirect(to: { queue: string; pubsub: string }) {
  return async (ctx: Ctx) => {
    const p = new URLSearchParams(ctx.url.search);
    const kind = p.get("kind"); p.delete("kind");
    const qs = p.size ? `?${p}` : "";
    return redirect((kind === "pubsub" ? to.pubsub : to.queue) + qs, 301);
  };
}

function channelRedirect(edit: boolean) {
  return async (ctx: Ctx) => {
    if (!ctx.signedIn) throw new (await import("../ctx.ts")).SignInRequired("sign in to open this page");
    const name = ctx.q("name");
    const off = (await ctx.inactive().catch(() => [] as Rec[])).find(r => r.name === name);
    // An inactive record is served here, as its own view; only a visible queue or topic moves.
    if (off) return edit ? Promise.reject(new NotFound()) : inactiveView(ctx, await ctx.status(), off);
    const rec = await ctx.lookup(name);
    const base = rec.kind === "pubsub" ? "/pubsub/topic" : rec.kind === "queue" ? "/queue" : detailPath(rec.kind);
    return redirect(`${base}${edit ? "/edit" : ""}${ctx.url.search}`, 301);
  };
}

export const handlers = {
  agents: (ctx: Ctx) => list(ctx, "agents"), services: (ctx: Ctx) => list(ctx, "services"), queues: (ctx: Ctx) => list(ctx, "queues"),
  pubsub: (ctx: Ctx) => list(ctx, "pubsub"), personal: (ctx: Ctx) => list(ctx, "personal"),
  newAgent: (ctx: Ctx) => registerPage(ctx, "agent"), newService: (ctx: Ctx) => registerPage(ctx, "service"),
  newQueue: (ctx: Ctx) => registerPage(ctx, "queue"), newPubsub: (ctx: Ctx) => registerPage(ctx, "pubsub"),
  agent: (ctx: Ctx) => detail(ctx, "/agent"), service: (ctx: Ctx) => detail(ctx, "/service"), queue: (ctx: Ctx) => detail(ctx, "/queue"), topic: (ctx: Ctx) => detail(ctx, "/pubsub/topic"),
  edit: (ctx: Ctx) => settingsPage(ctx, ctx.q("name")),
  deactivate: deactivatePage, danger: (ctx: Ctx) => dangerPage(ctx, ctx.q("name")),
  post: postService, confirm: postConfirm,
  channels: channelsRedirect({ queue: "/queues", pubsub: "/pubsub" }), channelsNew: channelsRedirect({ queue: "/queues/new", pubsub: "/pubsub/new" }),
  channel: channelRedirect(false), channelEdit: channelRedirect(true),
};
