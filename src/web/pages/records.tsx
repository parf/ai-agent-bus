import { PAGE_SIZE as PAGE } from "../pagination.ts";
import { TagBadges } from "../ui/tags.tsx";
import { agentHelp, AgentDescription, ConfigurationHelp, SecretHelp, type AgentHelp } from "../agent-metadata.tsx";
import { configuration, ConfigurationCard, formatConfiguration } from "../configuration.tsx";
import { normalizeTags, recordTags, filterTag } from "../tags.ts";
// Record pages: lists, register, detail, inactive view, settings, deactivate,
// Danger Zone, and the two POST handlers (src/web/web-face/records.md).
// Authority is rendered, never computed: controls follow can_manage and
// can_transfer on the record the daemon answered for this visitor.
import { h, Fragment, type Child } from "../jsx.ts";
import { Ctx, NotFound, LocalProblem, ConditionsChanged, Refusal, type Rec, type Status, type UserRow } from "../ctx.ts";
import { LocksCard, recordLocks } from "../ui/locks.tsx";
import { KVCard, recordKV, storeHref } from "../ui/kv.tsx";
import { respond, flashRedirect, type Section } from "../ui/frame.tsx";
import { Icon, Help, PageHead, Card, Name, Muted, KindIcon, KindPill, Pill, StatePill, Badge, Empty, Tabs, Segmented, Pager, Button, LinkButton, Avatar, Facts, recordHref, detailPath } from "../ui/kit.tsx";
import { TextField, LinesField, SecretField, TagsField, ConfigurationField, privateHint, CheckField, MaintainersField, PersonalField, ErrorSummary, FieldError, keep, terms, lines, type FormState, type FormError } from "../ui/forms.tsx";
import { Ribbon, type Slot } from "../ui/charts.tsx";
import { rangeFor, loadRange, rangeTitle, RangeNav, RangeChart } from "../ui/range.tsx";
import { redirect, local, returnTo } from "../http.ts";
import { number, relative, stamp, slotLabel, validTime, duration } from "../format.ts";
import { classify, formRefusal, lineRefusal, notYours, sectionProblem } from "../problem.tsx";
import { entity, MAINTAINER } from "../glyphs.ts";

/** Display declared grants, without inventing group membership or authority. */
export function roleAssignments(rec: Rec): string[] {
  const rows = [`${rec.owner}: owner (automatic)`, ...(rec.maintainers ?? []).map(name => `${name}: maintainer (automatic)`)];
  for (const term of rec.allow ?? []) {
    const match = /^(.*?)\(([a-z0-9_, ]+)\)$/.exec(term);
    const principal = match ? match[1] : term;
    if (principal === "@owner" && !match) continue;
    rows.push(`${principal === "@owner" ? rec.owner : principal}: ${match ? match[2].split(",").map(role => role.trim()).join(", ") : "access without an assigned role"}`);
  }
  return rows;
}

// Shared by listing columns and the record's status row.
function recordTime(at?: string, updated = false): Child {
  return validTime(at) ? <time datetime={at} title={updated ? stamp(at) : at}>{relative(at)}</time> : null;
}

// One compact summary for the listing and record page; no derived backlog.
function MessageCounts({ rec }: { rec: Rec }) {
  const accepted = rec.in ?? 0, waiting = rec.queued ?? 0;
  const dropped = rec.dropped ?? 0, expired = rec.expired ?? 0, lost = dropped + expired;
  if (!accepted && !waiting && !lost) return null;
  return <span class="message-counts">
    {accepted ? <span title="Accepted messages">{number(accepted)}</span> : null}
    {waiting ? <Badge tone="warn" title={rec.at_bound ? "At capacity when observed" : undefined}>{number(waiting)} waiting</Badge> : null}
    {lost ? <Badge tone="bad" title={`Dropped ${number(dropped)}, expired ${number(expired)}`}>{number(lost)} lost</Badge> : null}
  </span>;
}

// ------------------------------------------------------------------ kinds

type Kind = "agent" | "service" | "queue" | "pubsub" | "resource";
type ListKey = "agents" | "services" | "queues" | "pubsub" | "resources";

export const KINDS: Record<Kind, { list: string; newPath: string; noun: string; plural: string; section: Section; lower: string; blurb: string }> = {
  agent: { list: "/agents", newPath: "/agents/new", noun: "Agent", plural: "agents", section: "agents", lower: "agent", blurb: "An agent is a model or a program with an inbox: peers find it by name and send it work." },
  service: { list: "/services", newPath: "/services/new", noun: "Service", plural: "services", section: "services", lower: "service", blurb: "A service is something outside the bus that callers reach at its address — a database, a mail relay, an API." },
  queue: { list: "/queues", newPath: "/queues/new", noun: "Queue", plural: "queues", section: "queues", lower: "queue", blurb: "A queue holds what was sent until one reader takes it; each message goes to exactly one consumer." },
  pubsub: { list: "/pubsub", newPath: "/pubsub/new", noun: "PubSub", plural: "pub/sub channels", section: "pubsub", lower: "pub/sub channel", blurb: "A pub/sub channel copies each publication to every inbox on its Deliver-To list." },
  resource: { list: "/resources", newPath: "/resources/new", noun: "Resource", plural: "resources", section: "resources", lower: "resource", blurb: "A resource card names data an MCP client may read; the card says where, the face never fetches." },
};
const isKind = (k: string): k is Kind => k in KINDS;

export function noun(kind: string) { return kind === "user" ? "User" : isKind(kind) ? KINDS[kind].noun : kind === "group" ? "Group" : kind; }
/** The list a record lives on; a Personal record lives on its kind's list with `personal=1`. */
export function listOf(r: Rec): string {
  if (r.kind === "group") return r.personal ? "/groups?personal=1" : "/groups";
  if (r.kind === "user") return "/users";
  const list = isKind(r.kind) ? KINDS[r.kind].list : "/services";
  return r.personal ? `${list}?personal=1` : list;
}
/** The path alone of listOf: what a return address is checked against. */
export const listPath = (r: Rec) => listOf(r).split("?")[0]!;
function sectionOf(r: Rec): Section {
  if (r.kind === "user") return "queues";
  return isKind(r.kind) ? KINDS[r.kind].section : "services";
}

// ------------------------------------------------------------------ lists


const LIST_KIND: Record<ListKey, Kind> = { agents: "agent", services: "service", queues: "queue", pubsub: "pubsub", resources: "resource" };

function listUrl(path: string, p: Record<string, string | undefined>, drop: string[] = []): string {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(p)) if (v && !drop.includes(k)) q.set(k, v);
  return q.size ? `${path}?${q}` : path;
}

// One list per kind; `personal=1` is the Personal filter on it. The sidebar is
// the kind axis, so the page has no kind filter of its own.
async function list(ctx: Ctx, key: ListKey): Promise<Response> {
  const kind = LIST_KIND[key];
  const k = KINDS[kind];
  const path = k.list;
  const [st, all, id] = await Promise.all([ctx.status(), ctx.records(), ctx.identity()]);
  const you = st.you, owner0 = !!st.daemon_owner;
  const tag = filterTag(ctx.q("tag") || (ctx.q("personal") === "1" ? "personal" : ""));
  const personal = tag === "personal";
  const q = ctx.q("q"), scope = !personal && ctx.q("scope") === "my" && (kind === "agent" || kind === "service") ? "my" : "";
  const state = ctx.q("state") === "inactive" ? "inactive" : "active";
  const services = key === "services", pubsub = key === "pubsub", cards = key === "resources";
  const noQueue = services || cards;
  const readers = !noQueue && ["present", "none", "unavailable"].includes(ctx.q("readers")) ? ctx.q("readers") : "";
  const work = !noQueue && !pubsub && ctx.q("work") === "held" ? "held" : "";
  const sort = ctx.q("sort") === "updated" || (ctx.q("sort") === "last_used" && key === "agents") || (ctx.q("sort") === "queued" && !noQueue) ? ctx.q("sort") : "";
  const ownerParam = personal ? ctx.q("owner") : "";
  if (ownerParam && !owner0) return redirect(listUrl(path, { ...Object.fromEntries(ctx.url.searchParams), owner: undefined }));

  const records = all.filter(r => r.kind === kind);
  const personalRecs = records.filter(r => r.personal && (owner0 || r.owner === you));
  const shared = records.filter(r => !r.personal);
  let category = [...shared, ...personalRecs.filter(r => !ownerParam || r.owner === ownerParam)];
  let allCount=0, myCount=0;

  const needle = q.toLowerCase();
  const tagCounts = new Map<string, number>();
  const counts = { state: { active: 0, inactive: 0 }, readers: { all: 0, present: 0, none: 0, unavailable: 0 }, work: { all: 0, held: 0 } };
  let rows: Rec[] = [];
  for (const r of category) {
    if (needle && ![r.name, r.descr ?? "", r.owner].some(v => v.toLowerCase().includes(needle))) continue;
    const recordState = r.status === "inactive" ? "inactive" : "active";
    const readerState = r.readers == null ? "unavailable" : r.readers > 0 ? "present" : "none";
    const holding = (r.queued ?? 0) > 0;
    const stateMatches = recordState === state;
    const labels = recordTags(r);
    const tagMatches = tag ? labels.includes(tag) : !r.personal;
    const readersMatch = !readers || readerState === readers;
    const workMatches = !work || holding;
    if (stateMatches && readersMatch && workMatches && !r.personal) { allCount++; if (r.owner === you) myCount++; }
    if (scope === "my" && r.owner !== you) continue;
    if (stateMatches && readersMatch && workMatches) for (const t of labels) tagCounts.set(t, (tagCounts.get(t) ?? 0) + 1);
    if (!tagMatches) continue;
    // Each facet counts against the other selected filters, before pagination.
    if (readersMatch && workMatches) counts.state[recordState]++;
    if (stateMatches && workMatches) { counts.readers.all++; counts.readers[readerState]++; }
    if (stateMatches && readersMatch) { counts.work.all++; if (holding) counts.work.held++; }
    if (stateMatches && readersMatch && workMatches) rows.push(r);
  }
  rows.sort((a, b) => sort === "updated" ? (b.at ?? "").localeCompare(a.at ?? "") || a.name.localeCompare(b.name)
    : sort === "last_used" ? (validTime(b.last_used)?.getTime() ?? 0) - (validTime(a.last_used)?.getTime() ?? 0) || a.name.localeCompare(b.name)
    : sort === "queued" ? ((pubsub ? b.in : b.queued) ?? 0) - ((pubsub ? a.in : a.queued) ?? 0) || a.name.localeCompare(b.name)
    : (key === "agents" ? Number(!!b.reading || (b.readers ?? 0) > 0) - Number(!!a.reading || (a.readers ?? 0) > 0) : 0) || a.name.localeCompare(b.name));
  const matched = rows.length, pages = Math.max(1, Math.ceil(matched / PAGE));
  const pageNo = Math.min(pages, Math.max(1, Math.floor(Number(ctx.q("page"))) || 1));
  rows = rows.slice((pageNo - 1) * PAGE, pageNo * PAGE);

  const pflag = personal ? "1" : undefined;
  const params = { personal: pflag, q, scope, state, readers, work, sort, tag, owner: ownerParam };
  const here = listUrl(path, { ...params, page: pageNo > 1 ? String(pageNo) : undefined });
  const keepFilters = { state, q, readers, work, sort };
  const tagEntries = [...tagCounts].filter(([tag]) => tag !== "no-tags").sort((a,b) => b[1]-a[1] || a[0].localeCompare(b[0]));
  const primaryTags = tagEntries.slice(0, 25);
  if (tag && !primaryTags.some(([t]) => t === tag)) primaryTags.push([tag, tagCounts.get(tag) ?? 0]);
  const moreTags = tagEntries.filter(([t]) => !primaryTags.some(([p]) => p === t));
  const plural = kind === "pubsub" ? "PubSub" : k.noun + "s";
  const title = personal ? `${plural} · Personal` : plural;
  const tabs = [
    { href: listUrl(path, keepFilters), text: "All", count: allCount, current: !tag && !scope },
    ...(kind === "agent" || kind === "service" ? [{ href: listUrl(path, { ...keepFilters, scope: "my" }), text: "My", count: myCount, current: !tag && scope === "my", className: "my-view" }] : []),
    ...primaryTags.map(([t,count]) => ({ href: listUrl(path, { ...keepFilters, scope, tag: t }), text: t, count, current: tag === t, className: t === "personal" ? "personal-view" : "", icon: t === "personal" ? "lock" : undefined })),
  ];
  const registerHref = personal ? `${k.newPath}?personal=1` : k.newPath;
  const registerText = personal ? `Register Personal ${k.lower}` : `Register ${k.lower}`;
  const filterLink = (name: string, value: string) => listUrl(path, { ...params, [name]: value || undefined });
  const owners = [...new Set(personalRecs.map(r => r.owner))].sort();
  const nodeOwner = id?.owner ?? "";
  // Only columns that tell rows apart: the kind is the page's own, and on a
  // Personal list the owner is you, unless the daemon Owner sees every owner.
  const showOwner = scope !== "my" && (!personal || (owner0 && !ownerParam && owners.length > 1));

  const columns: { head: string; num?: boolean; cell: (r: Rec) => Child }[] = [
    ...(showOwner ? [{ head: "Owner", cell: (r: Rec) => r.owner === you ? <Pill tone="accent">you</Pill> : <code>{r.owner}</code> }] : []),
  ];
  const readersCol = { head: "Readers", num: true, cell: (r: Rec) => r.readers == null ? <Muted>unavailable</Muted> : key === "agents" && r.readers === 0 ? null : <>{number(r.readers)}</> };
  const heldCol = (head: string) => ({ head, num: true, cell: (r: Rec) => <>{number(r.queued)}{r.at_bound ? <> <Badge tone="warn">at capacity when observed</Badge></> : null}</> });
  const inCol = { head: "Accepted", num: true, cell: (r: Rec) => <>{number(r.in)}</> };
  const outCol = (head: string) => ({ head, num: true, cell: (r: Rec) => <>{number(r.out)}</> });
  const updCol = { head: "Updated", cell: (r: Rec) => recordTime(r.at, true) };
  if (key === "agents") columns.push(readersCol, {head:"Messages",num:true,cell:r=><MessageCounts rec={r} />}, { head: "Last used", cell: r => recordTime(r.last_used) }, updCol);
  else if (services) columns.push({ head: "Address", cell: r => <code>{r.addr}</code> }, { head: "Protocol", cell: r => r.protocol ? <Pill>{r.protocol}</Pill> : <Muted>—</Muted> }, updCol);
  else if (key === "queues") columns.push(readersCol, heldCol("Held"), inCol, outCol("Dequeued"), updCol);
  else if (cards) columns.push({ head: "URI", cell: r => <code>{r.resource?.uri}</code> }, { head: "Source", cell: r => r.resource?.source ? <code>{r.resource.source}</code> : <Muted>fetched</Muted> }, { head: "MIME type", cell: r => r.resource?.mimeType ?? <Muted>—</Muted> }, updCol);
  else columns.push(inCol, outCol("Copies out"), { head: "Deliver-To", num: true, cell: r => <>{number(r.subs?.length ?? 0)}</> }, updCol);
  const firstHead = k.noun;

  const filtered = !!(q || tag || state === "inactive" || readers || work || sort);
  const clear = listUrl(path, { personal: pflag, scope, owner: owner0 ? ownerParam : undefined });
  const helpItems = [k.blurb, personal ? "Personal: records their Owner keeps out of the shared lists; delivery is unchanged. The sidebar keeps this choice from kind to kind." : "Personal records are under the Personal tab, not in these lists.",
    "All and My keep Personal records out by default. A selected tag searches visible Personal records too. Counts respect the other selected filters, before pagination.", "Filter counts include the other selected filters, before pagination; zero-count choices are disabled.", "Status: Active by default, or Inactive — hidden from use and kept.",
    ...(services ? [] : ["Readers: consumers waiting on this inbox now.", key === "agents" ? "Messages: accepted total, with waiting (held now) and lost (dropped plus expired) when non-zero. Zero counts and unused times are blank." : "Accepted and Dequeued: messages in and taken out, kept across restarts."])];

  const body = <>
    <PageHead icon={<Icon name={personal ? "lock" : entity(kind)!.icon} />} title={title}
      help={<Help id="record-views-help" label={`About ${title}`} title={title} items={helpItems} />}>
      <LinkButton href={registerHref} tone="primary" icon="plus">{registerText}</LinkButton>
    </PageHead>
    <Tabs label="Record views" items={tabs} />
    {moreTags.length ? <details><summary>{moreTags.length} more tags</summary><Tabs label="More record tags" items={moreTags.map(([t,count]) => ({ href: listUrl(path, { ...keepFilters, scope, tag: t }), text: t, count }))} /></details> : null}
    {personal && owner0 ? <form method="get" class="toolbar owner-chooser">
        <input type="hidden" name="personal" value="1" />
        {[["state", state], ["readers", readers], ["work", work], ["q", q], ["sort", sort]].map(([n, v]) => v ? <input type="hidden" name={n} value={v} /> : null)}
        <label for="owner-select" class="seg-label">Owner</label>
        <select id="owner-select" name="owner" data-submit-on-change>
          <option value="">All visible owners</option>
          {owners.map(o => <option value={o} selected={o === ownerParam}>{o}</option>)}
        </select>
      </form> : null}
    <form class="toolbar record-search" method="get" action={path}>
      <div class="search"><Icon name="search" /><label for="record-query" class="sr-only">Search records</label>
        <input id="record-query" type="search" name="q" value={q} placeholder={showOwner ? "Search by name, owner, or description" : "Search by name or description"} /></div>
      {[["tag", tag], ["personal", pflag ?? ""], ["scope", scope], ["state", state], ["readers", readers], ["work", work], ["owner", ownerParam]].map(([n, v]) => v ? <input type="hidden" name={n} value={v} /> : null)}
      <label for="record-sort" class="sr-only">Sort</label>
      <select id="record-sort" name="sort" data-submit-on-change>
        <option value="">Name (A–Z)</option>
        <option value="updated" selected={sort === "updated"}>Recently updated</option>
        {key === "agents" ? <option value="last_used" selected={sort === "last_used"}>Recently used</option> : null}
        {!services && !cards ? <option value="queued" selected={sort === "queued"}>{pubsub ? "Accepted (high–low)" : "Queued (high–low)"}</option> : null}
      </select>
    </form>
    <div class="toolbar filters">
      <Segmented label="Status" items={(["active", "inactive"] as const).map(v => ({ href: filterLink("state", v), text: `${v === "active" ? "Active" : "Inactive"} (${number(counts.state[v])})`, current: state === v, disabled: counts.state[v] === 0 }))} />
      {!services && !pubsub && !cards ? <>
        <Segmented label="Readers" items={([["", "All", counts.readers.all], ["present", "Reading now", counts.readers.present], ["none", "No reader now", counts.readers.none], ["unavailable", "Unavailable", counts.readers.unavailable]] as const).map(([v, t, count]) => ({ href: filterLink("readers", v), text: `${t} (${number(count)})`, current: readers === v, disabled: count === 0 }))} />
        <Segmented label="Queue" items={([["", "All", counts.work.all], ["held", "Holding work", counts.work.held]] as const).map(([v, t, count]) => ({ href: filterLink("work", v), text: `${t} (${number(count)})`, current: work === v, disabled: count === 0 }))} />
      </> : null}
    </div>
    {category.length === 0 ? (!personal && personalRecs.length
      ? <Empty icon="lock" title={`No shared ${k.plural}`} action={<a class="btn" href={listUrl(path, { personal: "1" })}>Open the Personal tab</a>}>
          {personalRecs.length} Personal {k.lower}{personalRecs.length === 1 ? " is" : "s are"} under the Personal tab, which this list omits. {k.blurb}
        </Empty>
      : <Empty icon={personal ? "lock" : entity(kind)!.icon} title={`No ${personal ? "Personal " : ""}${k.plural} yet`}>
          {personal ? "Personal records belong to their Owner's own view instead of the shared lists." : k.blurb}
        </Empty>)
      : matched === 0 ? <Empty icon="filter" title="No records match these filters" action={<a class="btn" href={clear}>Clear filters</a>}>Change the active filters above or clear filters.</Empty>
      : <>
        {personal ? <p class="muted small">{owner0 ? "Only Personal records visible through your normal access; not a node-wide inventory." : <>Owned by <code>{you}</code>.</>}</p> : null}
        <div class="results-line"><span>Showing {(pageNo - 1) * PAGE + 1}–{Math.min(pageNo * PAGE, matched)} of {number(matched)} matching records, caller-visible on this page and not a count of this node.</span>
          {filtered ? <a href={clear}>Clear filters</a> : null}</div>
        <div class="table-wrap"><div class="table-scroll"><table class="data stack record-table">
          <thead><tr><th>{firstHead}</th>{columns.map(c => <th class={c.num ? "num" : ""}>{c.head}</th>)}</tr></thead>
          <tbody>{rows.map(r => <tr class={[r.owner === you ? "owned-record" : "", r.personal && !personal ? "personal-record" : "", r.status === "inactive" ? "inactive-record" : ""].join(" ")}>
            <td data-label={firstHead}><div class="rec-cell"><KindIcon kind={r.kind} owner={r.kind === "user" && r.name === nodeOwner} template={r.resource?.template} />
              <div class="rec-title"><a href={recordHref(r, { return: here })}>{r.descr ? <><strong>{r.descr}</strong><code>{r.name}</code></> : <strong><code>{r.name}</code></strong>}</a><TagBadges rec={r} href={tag => listUrl(k.list, { ...params, tag })} /></div></div></td>
            {columns.map(c => <td class={c.num ? "num" : ""} data-label={c.head}>{c.cell(r)}</td>)}
          </tr>)}</tbody>
        </table></div></div>
        <Pager label="Record pages" prev={pageNo > 1 ? listUrl(path, { ...params, page: String(pageNo - 1) }) : undefined} next={pageNo < pages ? listUrl(path, { ...params, page: String(pageNo + 1) }) : undefined}>Page {pageNo} of {pages}</Pager>
      </>}
  </>;
  return respond(ctx, { title, section: k.section, signedIn: true, you, personal }, body);
}

// ------------------------------------------------------------------ record form

const RECORD_KEEP = ["config", "from_personal", "name", "descr", "tags", "roles", "kind", "addr", "protocol", "personal", "allow", "subs", "ttl", "bound", "overflow", "maintainers", "edit_allow", "edit_subs", "edit_sharing", "edit_personal", "uri", "template", "source", "mime", "title"];

const personalValue = (st: FormState, rec?: Rec) => st.values.edit_personal != null || st.values.personal != null ? st.values.personal === "on" : !!rec?.personal;

function RecordPersonalField({ kind, st, rec, errId }: { kind: string; st: FormState; rec?: Rec; errId: string }) {
  if (kind === "user") return null;
  const canAssign = !rec || !!rec.can_transfer;
  return <PersonalField st={st} errId={errId} checked={personalValue(st, rec)} canAssign={canAssign}
    hint={`Puts this ${noun(kind)} in its Owner’s Personal view. Its allow list and Maintainers may name only its Owner and the Owner's own agents; delivery is unchanged.`} />;
}

function RecordFields({ kind, st, mode, rec, errId, metadata = {} }: { metadata?: AgentHelp; kind: string; st: FormState; mode: "create" | "save"; rec?: Rec; errId: string }) {
  const n = noun(kind);
  const inbox = kind === "user";
  const v = (name: string, fallback: string) => st.values[name] ?? fallback;
  const personal = personalValue(st, rec);
  const canAssign = mode === "create" || !!rec?.can_transfer;
  const allowHint = kind === "pubsub" ? "Who may publish here. Who receives is the Deliver-To list."
    : personal ? `While Personal: the Owner, the Owner’s own agents, @owner${kind === "agent" ? " or @agent" : ""}, one per line.` : "One identity, group, @owner, or * per line.";
  return <div class="form-grid">
    {mode === "create"
      ? <TextField name="name" label="Name" st={st} errId={errId} required placeholder={kind === "agent" ? "#name@realm" : "name@realm"} id="create-name" besideLabel={<RecordPersonalField kind={kind} st={st} rec={rec} errId={errId} />}
          help={{ label: "About the name", tip: "The routing identity callers use. It cannot be changed afterwards." }}
          hint={kind === "agent" ? "An agent's name starts with #." : undefined} />
      : <input type="hidden" name="name" value={rec!.name} />}
    <TextField name="descr" label="Description" st={st} errId={errId} value={v("descr", rec?.descr ?? "")} placeholder={`What this ${n} is for`} hint="Shown first in the registry." wide={mode !== "create"} />
    <TagsField st={st} errId={errId} rec={rec} />
    {kind === "service" ? <>
      <TextField name="addr" label="Address" st={st} errId={errId} required value={v("addr", rec?.addr ?? "")} placeholder="host:port, a path, or a URL" hint="Where a caller reaches it." />
      <TextField name="protocol" label="Protocol" st={st} errId={errId} required value={v("protocol", rec?.protocol ?? "")} placeholder="https, postgresql, smtp" hint="A hint for callers; the daemon checks nothing." />
    </> : null}
    {kind === "resource" && mode === "save" ? <p class="muted small">A card's URI, source and type change by registering it again; the detail page shows them.</p> : null}
    {kind === "resource" && mode === "create" ? <>
      <TextField name="uri" label="URI" wide besideLabel={<CheckField name="template" label="Template" st={st} errId={errId} checked={st.values.template === "on"}
        help={{ label: "About resource templates", tip: "A URI pattern for several resources, with named variables in braces.", title: "Resource templates", items: [
          <>A template is an RFC 6570 URI pattern, not a glob or regular expression. Leave Template unchecked for one fixed URI.</>,
          <><code>md://notes/{'{name}'}</code>: a variable is encoded as one value. With name <code>docs/intro.md</code>, read <code>md://notes/docs%2Fintro.md</code>.</>,
          <><code>md://notes/{'{+path}'}</code>: reserved characters such as slashes are preserved. With path <code>docs/intro.md</code>, read <code>md://notes/docs/intro.md</code>.</>,
          <><code>db://catalog/{'{table}'}/{'{row}'}</code> uses two variables; table <code>users</code> and row <code>42</code> give <code>db://catalog/users/42</code>.</>,
          <>Example Sources can implement these reads:</>,
          <><code>md://notes/{'{+path}'}</code> → read file under the root (path jail applies)</>,
          <><code>mysql://realmo/{'{table}'}/schema</code> → <code>SHOW CREATE TABLE</code></>,
          <><code>log://{'{source}'}/tail</code> → snapshot the ring buffer</>,
          <>Register the pattern with braces; clients read an expanded URI. Templates need an Agent or MCP Service as Source and have no fixed size.</>,
          <><a href="https://datatracker.ietf.org/doc/html/rfc6570">URI Template syntax</a></>,
        ] }} />} st={st} errId={errId} required value={v("uri", rec?.resource?.uri ?? "")} placeholder={st.values.template === "on" ? "md://notes/{+path}" : "md://notes/a.md"} hint="The MCP resource’s address; use {…} for a template."
        help={{ label: "About the uri", tip: "An absolute URI. A {…} part is an RFC 6570 template; tick Template for that.", title: "Resource uri", items: ["An absolute URI, like md://notes/a.md.", "A {…} part is an RFC 6570 template; tick Template for that.", "A plain https:// URI may name no source: the face fetches it."] }} />
      <TextField name="source" label="Source" st={st} errId={errId} value={v("source", rec?.resource?.source ?? "")} placeholder="#agent@realm or mcp-service@realm" hint="Who answers reads; optional for plain https." help={{label:"About the source",tip:"An Agent on this bus, or a Service of protocol mcp. Required unless the URI is plain https."}} />
      <TextField name="mime" label="MIME type" st={st} errId={errId} value={v("mime", rec?.resource?.mimeType ?? "")} placeholder="text/markdown" hint="The content's type." />
      <TextField name="title" label="Title" st={st} errId={errId} value={v("title", rec?.resource?.title ?? "")} placeholder="A human title" hint="Shown by MCP clients." />
    </> : null}
    {!inbox ? <>
      <MaintainersField st={st} errId={errId} value={v("maintainers", (rec?.maintainers ?? []).join("\n"))} canAssign={canAssign} />
    </> : null}
    {!inbox ? <><LinesField name="allow" label={kind === "pubsub" ? "Who may publish" : kind === "queue" ? "Who may send" : "Allow list"} st={st} errId={errId}
      value={v("allow", (rec?.allow ?? []).join("\n"))} placeholder={"#agent@realm\nuser@realm\n@group\n@owner\n*"} hint={allowHint}
      help={{ label: "About the allow list", tip: "Who may reach this record, one term per line. Add roles after a term: @dev(deploy, read_only). Role names use lowercase letters, digits and _. owner and maintainer roles are generated automatically.", title: "Allow list", items: ["One identity, group, @owner (the record's Owner) or * (everyone) per line.", "An empty list admits only the Owner and Maintainers.", ...(kind === "pubsub" ? ["On a pubsub channel it says who may publish; who receives is the Deliver-To list."] : []), "Add application roles after a term: @dev(deploy, read_only). The agent decides their meaning; roles grant no AgentBus management authority.", "owner and maintainer roles are generated automatically."] }} />
      {mode === "save" ? <input type="hidden" name="edit_allow" value="1" /> : null}</> : null}
    {kind === "agent" ? <TextField name="roles" label="Roles it understands" st={st} errId={errId} value={v("roles", (rec?.roles ?? []).join(", "))} placeholder="deploy, read_only"
      help={{ label: "About roles", tip: "Informational roles this agent understands. Assign them in the allow list: @dev(deploy, read_only). Roles grant no AgentBus management authority.", title: "Roles and usage", items: ["This list is informational: it does not grant roles or restrict assignments.", "Assign roles in the allow list: @dev(deploy, read_only). Matching grants combine their roles.", "The agent reads roles on each message, or AB_ROLE_DEPLOY=1 in a per-message script, and decides what they mean.", "Names use lowercase letters, digits and _. owner and maintainer roles are generated automatically."] }}
      hint="Roles callers can be given, e.g. deploy, read_only. Owner and Maintainer are automatic." wide /> : null}
    {(kind === "agent" || kind === "queue") ? <><TextField name="subs" label="Deliver-To route" st={st} errId={errId} value={v("subs", (rec?.subs ?? []).join(" "))} placeholder="#agent@realm, queue@realm or pubsub@realm" wide
      help={{ label: "About the route", tip: `Empty keeps messages here. One destination: a message sent to this ${n} moves there instead.`, title: "Deliver-To route", items: [`Empty keeps messages in this ${n}’s own queue.`, `One destination: a message sent here moves there instead.`, `The destination’s allow list must list this ${n} itself.`] }}
      hint={`Empty keeps messages here; the destination must allow this ${n.toLowerCase()}.`} />
      {rec ? routeState(rec) : null}
      {mode === "save" ? <input type="hidden" name="edit_subs" value="1" /> : null}</> : null}
    {kind === "pubsub" ? <><LinesField name="subs" label="Deliver-To list" st={st} errId={errId} value={v("subs", (rec?.subs ?? []).join("\n"))} placeholder={"#agent@realm\nqueue@realm\n@group"}
      help={{ label: "About Deliver-To", tip: "One inbox per line; each gets a copy of every publication. Empty reaches nobody.", title: "Deliver-To list", items: ["One inbox per line: an agent, a queue, a user or a group.", "Each receives a copy of every publication.", "An empty list reaches nobody."] }} />
      {mode === "save" ? <input type="hidden" name="edit_subs" value="1" /> : null}</> : null}
    {kind === "agent" || kind === "queue" || inbox ? <fieldset class="queue-fields wide">
      <legend>Queue<Help label="About the queue" tip="Retention: how long a message waits, e.g. 1h or 7d. Bound: a whole number; empty or 0 selects the default. When full: refuse new messages or drop the oldest." /></legend>
      <div class="queue-controls">
        <label for="f-ttl">Retention<input id="f-ttl" name="ttl" value={v("ttl", rec?.ttl ?? "")} placeholder="default" aria-invalid={st.error?.field === "ttl" ? "true" : "false"} /></label>
        <label for="f-bound">Bound<input id="f-bound" name="bound" type="number" min="0" value={v("bound", rec?.bound ? String(rec.bound) : "")} placeholder="default" aria-invalid={st.error?.field === "bound" ? "true" : "false"} /></label>
        <label for="f-overflow">When full<select id="f-overflow" name="overflow">
          <option value="strict" selected={v("overflow", rec?.overflow ?? "strict") === "strict"}>Refuse new messages</option>
          <option value="ring" selected={v("overflow", rec?.overflow ?? "strict") === "ring"}>Drop the oldest</option>
        </select></label>
      </div>
    </fieldset> : null}
    {kind === "agent" || kind === "service" ? <SecretField name="secret" label="Secret (ENV)" st={st} errId={errId} placeholder="PGPASSWORD=..."
      help={{ label: "About the secret", tip: `Keep secrets in Secret and config in Configuration. The stored Secret is never shown.`, title: "Secret (ENV)", items: ["Dotenv assignments, checked before saving. Comments, optional export and quoted multiline values are allowed; nothing is evaluated; CRLF becomes LF.", kind === "agent" ? "Its Owner, Maintainers and the agent itself read it back, with agent-bus secret." : "Its Owner and Maintainers read it back, with agent-bus secret.", "No page displays the stored value; a form starts empty."] }}
      hint={mode === "create" ? "Optional. Leave empty to register without one." : privateHint("secret", !!rec?.secret_sha)} /> : null}
    {kind === "agent" ? <SecretHelp help={metadata} /> : null}
    {mode === "save" && st.values.config !== undefined && (kind === "agent" || kind === "service") ? <ConfigurationField st={st} errId={errId} kind={kind} stored={!!rec?.config_sha} /> : null}
    {kind === "agent" ? <ConfigurationHelp help={metadata} value={st.values.config} /> : null}
  </div>;
}

// ------------------------------------------------------------------ register

async function registerPage(ctx: Ctx, kind: Kind, st: FormState = { values: {} }, status = 200): Promise<Response> {
  const s = await ctx.status();
  const k = KINDS[kind];
  // Arriving from a Personal list: the form starts Personal, and Back returns there.
  const fromPersonal = ctx.q("personal") === "1" || st.values.from_personal === "1";
  if (fromPersonal && !st.error && st.values.personal == null) st = { ...st, values: { ...st.values, personal: "on" } };
  const back = fromPersonal ? { href: `${k.list}?personal=1`, label: `Back to Personal ${kind === "pubsub" ? "PubSub" : k.noun + "s"}` } : { href: k.list, label: `Back to ${kind === "pubsub" ? "PubSub" : k.noun + "s"}` };
  const body = <>
    <PageHead back={back} icon={<Icon name={entity(kind)!.icon} />} title={`Register ${k.lower}`}
      sub={<>You become its Owner. {k.blurb}</>} />
    <ErrorSummary id="create" error={st.error} />
    {kind === "resource" ? <p><a href="https://modelcontextprotocol.io/specification/latest/server/resources">Official MCP Resources specification</a></p> : null}
    <form id="form-create" class="card form-card editor-card task-card" method="post" action="/record">
      <div class="card-body">
        <input type="hidden" name="action" value="create" /><input type="hidden" name="kind" value={kind} />
        {fromPersonal ? <input type="hidden" name="from_personal" value="1" /> : null}
        <RecordFields kind={kind} st={st} mode="create" errId="create-error" />
        <FieldError id="create-error" error={st.error} />
        <div class="actions"><Button tone="primary" icon="plus">Register {k.lower}</Button><a class="btn btn-ghost" href={back.href}>Cancel</a></div>
      </div>
    </form>
  </>;
  return respond(ctx, { title: `Register ${k.lower}`, section: k.section, signedIn: true, you: s.you, personal: fromPersonal }, body, status);
}

// ------------------------------------------------------------------ detail

function OwnerChip({ rec, users }: { rec: Rec; users: UserRow[] | null }) {
  const u = users?.find(x => x.kind === "user" && x.name === rec.owner);
  return u ? <a class="owner-chip" href={`/user?name=${encodeURIComponent(u.name)}`}><Avatar photo={u.photo_png} name={u.person_name || u.name} size="sm" /><span>Owner</span><code>{rec.owner}</code></a>
    : <span class="owner-chip"><Icon name="kind:user" /><span>Owner</span><code>{rec.owner}</code></span>;
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
    loadRange(ctx, rangeFor(ctx, st), name).then(a => ({ ok: a }), e => ({ err: sectionProblem(e) })),
  ]);
  const back = returnTo(ctx.q("return"), [listPath(rec), "/runner", "/runner-settings"], listOf(rec));
  const [config, metadata] = await Promise.all([configuration(ctx, rec), agentHelp(ctx, rec)]);
  const manage = !!rec.can_manage, inbox = rec.kind === "user", n = noun(rec.kind);
  const editHref = `${detailPath(rec.kind)}/edit?${new URLSearchParams({ name, ...(ctx.q("return") ? { return: back } : {}) })}`;
  const range = rangeFor(ctx, st);
  const rangeHref = (p: { range?: string; at?: number }) => {
    const q = new URLSearchParams({ name });
    if (ctx.q("return")) q.set("return", back);
    if (p.range) q.set("range", p.range);
    if (p.at) q.set("at", String(p.at));
    return `${detailPath(rec.kind)}?${q}#activity`;
  };
  const fullHref = (() => { const q = new URLSearchParams({ name }); if (range.kind !== "day") q.set("range", range.kind); if (range.at !== range.today) q.set("at", String(range.at)); return `/activity?${q}`; })();
  const [locks, kv] = await Promise.all([recordLocks(ctx, name), recordKV(ctx, name)]);
  const onList = (rec.subs ?? []).includes(st.you);
  const declaredConfig = metadata.data?.config;
  const hasDeclaredConfig = !!declaredConfig && typeof declaredConfig === "object" && Object.keys(declaredConfig).length > 0;
  const hasDeclaredSecrets = Array.isArray(metadata.data?.secrets) && metadata.data.secrets.length > 0;

  const counterRows: [Child, Child][] = [];
  if (rec.readers) counterRows.push(["Readers", number(rec.readers)]);
  if (rec.in || rec.queued || rec.dropped || rec.expired) counterRows.push(["Messages", <MessageCounts rec={rec} />]);
  if (rec.queued && rec.oldest) counterRows.push(["Oldest held", rec.oldest]);
  const counters = rec.kind !== "service" && rec.kind !== "resource" && counterRows.length ? <Card title="Queue & counters" icon="gauge" id="counters">
    <Facts rows={counterRows} />
  </Card> : null;
  const policy: [Child, Child][] = [];
  if (rec.kind !== "service" && rec.kind !== "pubsub" && rec.kind !== "resource") {
    if (rec.protocol) policy.push(["Reached", "external"]);
    if (rec.bound) policy.push(["Queue bound", number(rec.bound)]);
    if (duration(rec.ttl) > 0) policy.push(["Retention", rec.ttl!]);
    if (rec.overflow === "ring") policy.push(["When full", "drop the oldest"]);
  }
  const card = rec.kind === "resource" && rec.resource ? <Card title={rec.resource.template ? "Resource template" : "Resource card"} icon={rec.resource.template ? "kind:resource-template" : "kind:resource"}>
    <Facts rows={[
      [rec.resource.template ? "URI template" : "URI", <code>{rec.resource.uri}</code>],
      ["Source", rec.resource.source ? <code>{rec.resource.source}</code> : <Muted>none — the face fetches it</Muted>],
      ...(rec.resource.mimeType ? [["MIME type", rec.resource.mimeType] as [Child, Child]] : []),
      ...(rec.resource.title ? [["Title", rec.resource.title] as [Child, Child]] : []),
      ...(rec.resource.size ? [["Size", `${rec.resource.size} bytes`] as [Child, Child]] : []),
    ]} />
    <p class="muted small">The card says where data is; the face reads through the source. The daemon stores no content.</p>
  </Card> : null;
  // What agent-bus run serves an agent with, the dedicated runner's own note
  // (docs/06-dedicated-runner.md#script-agents).
  const runs = rec.kind === "agent" && rec.script ? <Card title="Script" icon="terminal">
    <Facts rows={[["Runs", <code>{rec.script}</code>]]} />
    <p class="muted small">Written by <code>agent-bus run</code> each time it serves this agent.</p>
  </Card> : null;
  const assignments = roleAssignments(rec);
  const roles = rec.kind === "agent" && (rec.roles?.length || assignments.length > 1 + (rec.maintainers?.length ?? 0)) ? <Card title="Roles" icon="badge-check" id="roles">
    <p>Understands: {rec.roles?.length ? rec.roles.join(", ") : "no application roles declared"}.</p>
    <ul>{assignments.map(row => <li>{row}</li>)}</ul>
    <p class="muted small">Group entries apply to their members. Matching grants combine; the agent’s configured limits still apply.</p>
  </Card> : null;
  const where = rec.kind === "service" ? <Card title="Where it is" icon="map-pin">
    <Facts rows={[["Address", <code>{rec.addr}</code>], ["Protocol", rec.protocol ? <Pill>{rec.protocol}</Pill> : <Muted>—</Muted>], ["Secret", rec.secret_sha ? <code title="SHA-256 prefix, never the bytes">{rec.secret_sha}</code> : <span class="muted" title="None stored, or not yours to see: the digest reaches only those who may read the secret">—</span>]]} />
  </Card> : null;

  const body = <>
    <PageHead back={{ href: back, label: "Back to records" }} icon={<Icon name={entity(rec.kind)?.icon ?? "box"} />}
      title={<><Name copy>{rec.name}</Name>{rec.personal ? <span class="muted"> · Personal</span> : null}</>}
      sub={<div class="meta-row">
        <KindPill kind={rec.kind} template={rec.resource?.template} />
        <OwnerChip rec={rec} users={users} />
        {rec.maintainers?.length ? <span class="pill"><Icon name={MAINTAINER.icon} />Maintainers: {rec.maintainers.join(", ")}</span> : null}
        {rec.kind !== "resource" ? <span class="pill"><Icon name={rec.kind === "pubsub" ? "kind:pubsub" : "arrow-right"} />Delivery: {rec.kind === "pubsub" ? "a copy to each recipient" : "one at a time"}</span> : null}
      </div>}>
      {manage ? <LinkButton href={editHref} tone="primary" icon="settings">Edit settings</LinkButton> : null}
    </PageHead>
    {rec.descr ? <p class="descr lead-descr">{rec.descr}</p> : null}
    <TagBadges rec={rec} href={tag => listUrl(listPath(rec), { tag })} />
    {!manage ? <p class="muted small"><Icon name="eye" /> You can view this record; its owner and assigned maintainers can manage it.</p> : null}
    <AgentDescription help={metadata} name={rec.name} filter={ctx.q("m")} path={canonical} returnTo={back} />
    <div class="detail-grid">
      <div>
        {rec.kind === "resource" ? card : <Card title="Activity" icon="activity" id="activity" actions={<a class="btn btn-ghost btn-sm" href={fullHref}>Open in Activity<Icon name="chevron-right" /></a>}>
          <RangeNav r={range} href={rangeHref} label={`Activity range for ${name}`} />
          {"err" in act ? <p class="warn">Activity unavailable: {act.err}</p>
            : <>
              <p class="range-title"><strong>{rangeTitle(range)}</strong></p>
              {range.kind === "day" && act.ok.slots.length ? <Ribbon slots={act.ok.slots} label={`Traffic per ten-minute slot for ${name}`} /> : null}
              <RangeChart r={range} data={act.ok} id="record-chart" compact dayHref={at => rangeHref({ at })} />
            </>}
        </Card>}
        {counters}
        {rec.kind === "pubsub" && onList && !manage ? <form method="post" action="/record" class="actions">
          <span>Your inbox receives publications from this list.</span><input type="hidden" name="name" value={rec.name} /><Button name="action" value="unsubscribe" tone="secondary" icon="log-out">Take my inbox off this list</Button>
        </form> : null}
      </div>
      <div>
        <Card title={<>Status &amp; Policy <StatePill />{rec.kind === "agent" && (rec.readers || rec.reading) ? <span class="agent-status-answer" data-agent-status={`/agent-status?name=${encodeURIComponent(rec.name)}`} aria-live="polite"></span> : null}</>} icon="power" id="status">
          <div class="status-times"><Facts rows={[
            ["Updated", recordTime(rec.at, true)],
            ...(validTime(rec.last_used) ? [["Last used", recordTime(rec.last_used)] as [Child, Child]] : []),
          ]} /></div>
          {rec.config_sha || policy.length ? <Facts rows={[
            ...(rec.config_sha ? [["Config", <code>{rec.config_sha}</code>] as [Child, Child]] : []),
            ...policy,
          ]} /> : null}
        </Card>
        {config || hasDeclaredConfig ? <ConfigurationCard value={config} /> : null}
        {hasDeclaredConfig || hasDeclaredSecrets ? <Card title="Package settings" icon="settings">{hasDeclaredConfig ? <ConfigurationHelp help={metadata} value={config} /> : null}{hasDeclaredSecrets ? <SecretHelp help={metadata} /> : null}</Card> : null}
        {locks && Object.keys(locks).length ? <LocksCard record={rec.name} locks={locks} you={st.you} /> : null}
        {kv ? <KVCard kv={kv} /> : null}
        {runs}{roles}{where}
        {manage && !inbox ? <a class="danger-link" href={`/record-danger?name=${encodeURIComponent(rec.name)}`}><span><Icon name="flame" /> Danger Zone</span><span class="small">deactivation{rec.can_transfer ? ", transfer" : ""}, deletion</span></a> : null}
      </div>
    </div>
  </>;
  return respond(ctx, { title: `${n} ${rec.name}`, section: sectionOf(rec), signedIn: true, you: st.you, charts: true, personal: !!rec.personal }, body);
}

async function inactiveView(ctx: Ctx, st: Status, rec: Rec): Promise<Response> {
  const n = noun(rec.kind);
  const back = returnTo(ctx.q("return"), [listPath(rec), "/runner", "/runner-settings"], listOf(rec));
  const body = <>
    <PageHead back={{ href: back, label: "Back to records" }} icon={<Icon name={entity(rec.kind)?.icon ?? "box"} />} title={<><Name>{rec.name}</Name> <Badge>INACTIVE</Badge></>}
      sub={<div class="meta-row"><KindPill kind={rec.kind} template={rec.resource?.template} /><span class="owner-chip"><Icon name="kind:user" /><span>Owner</span><code>{rec.owner}</code></span></div>} />
    <Card title="Status" icon="power" className="confirm-card">
      <p class="big-state"><StatePill inactive /></p>
      <ul>
        <li>It is hidden from listings for everyone but those who may manage it, and refuses use.</li>
        <li>Its queued work is kept{rec.kind === "service" ? "" : <>: {number(rec.queued)} held when observed</>}.</li>
        <li>Reactivating returns it exactly as it was.</li>
      </ul>
      {rec.can_manage && rec.kind !== "user"
        ? <form method="post" action="/record" class="actions"><input type="hidden" name="name" value={rec.name} /><Button name="action" value="reactivate" tone="primary" icon="power">Reactivate</Button></form>
        : <p class="muted">Its owner and assigned maintainers can reactivate it.</p>}
    </Card>
  </>;
  return respond(ctx, { title: `${n} ${rec.name}`, section: sectionOf(rec), signedIn: true, you: st.you, personal: !!rec.personal }, body);
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
  if (st.values.config === undefined) st.values.config = await configuration(ctx, rec);
  const metadata = await agentHelp(ctx, rec);
  const ret = returnTo(st.values.return ?? ctx.q("return"), [listPath(rec), "/runner", "/runner-settings"], "");
  const detailHref = recordHref(rec, ret ? { return: ret } : undefined);
  const body = <>
    <form id="form-save" method="post" action="/record">
    <PageHead back={{ href: detailHref, label: `Back to ${rec.name}` }} icon={<Icon name={entity(rec.kind)?.icon ?? "box"} />} title={<>Edit {noun(rec.kind)} <Name>{rec.name}</Name><span class="muted small"> (owner: <Icon name="kind:user" /><Name>{rec.owner}</Name>)</span></>}>
      <a class="btn btn-ghost" href={`/locks?q=${encodeURIComponent(name)}`}>Locks</a>
      <a class="btn btn-ghost" href={storeHref(name)}>Store</a>
    </PageHead>
    <ErrorSummary id="save" error={st.error} />
    <div class="card form-card editor-card task-card">
      <div class="card-body">
        <input type="hidden" name="action" value="save" />
        {ret ? <input type="hidden" name="return" value={ret} /> : null}
        <RecordFields kind={rec.kind} st={st} mode="save" rec={rec} metadata={metadata} errId="save-error" />
        <FieldError id="save-error" error={st.error} />
        <div class="actions"><Button tone="primary" icon="check">Save settings</Button><a class="btn btn-ghost" href={detailHref}>Cancel</a></div>
      </div>
    </div>
    </form>
    <a class="danger-link" href={`/record-danger?name=${encodeURIComponent(rec.name)}`}><span><Icon name="flame" /> Danger Zone</span><Icon name="chevron-right" /></a>
  </>;
  return respond(ctx, { title: `Edit ${rec.name}`, section: sectionOf(rec), signedIn: true, you: s.you, personal: !!rec.personal }, body, status);
}

// ------------------------------------------------------------------ deactivate

async function deactivatePage(ctx: Ctx): Promise<Response> {
  const [s, rec] = await Promise.all([ctx.status(), activeRecord(ctx, ctx.q("name"))]);
  if (!rec.can_manage || rec.kind === "user") throw notYours("only the owner or an assigned Maintainer can deactivate this record");
  const back = `/record-danger?name=${encodeURIComponent(rec.name)}`;
  const body = <>
    <PageHead back={{ href: back, label: "Back to the Danger Zone" }} icon={<Icon name="triangle-alert" />} title="Confirm deactivation" />
    <Card className="confirm-card" tone="danger" title={<>Deactivate <code>{rec.name}</code>?</>} icon="power">
      <ul>
        <li>It disappears from listings and refuses use until it is reactivated.</li>
        <li>Queued work is kept ({number(rec.queued)} held now), and running processes are not stopped.</li>
        <li>Its owner and maintainers reactivate it from its Inactive view.</li>
      </ul>
      <form method="post" action="/record" class="actions"><input type="hidden" name="name" value={rec.name} />
        <Button name="action" value="deactivate" tone="danger" icon="power">Deactivate {rec.name}</Button>
        <a class="btn btn-ghost" href={back}>Cancel</a></form>
    </Card>
  </>;
  return respond(ctx, { title: `Confirm deactivation · ${rec.name}`, section: sectionOf(rec), signedIn: true, you: s.you, personal: !!rec.personal }, body);
}

// ------------------------------------------------------------------ danger zone

export async function dangerPage(ctx: Ctx, name: string, err?: { section: "transfer"; error: FormError; owner?: string }, status = 200): Promise<Response> {
  const [s, rec] = await Promise.all([ctx.status(), activeRecord(ctx, name)]);
  if (!rec.can_manage) throw notYours("only the owner or an assigned Maintainer can manage this record");
  if (rec.kind === "user") throw notYours("a user's inbox has no configuration, transfer or removal; it follows its user");
  const group = rec.kind === "group";
  const prefixed = group && /^@[^/]+\//.test(rec.name);
  const transferable = !!rec.can_transfer && rec.name !== "@administrators" && rec.name !== rec.owner && !prefixed;
  const trSt: FormState = { values: { owner: err?.owner ?? "" }, error: err?.section === "transfer" ? err.error : undefined };
  const backHref = group ? `/group?name=${encodeURIComponent(rec.name)}` : recordHref(rec);
  const body = <>
    <PageHead back={{ href: backHref, label: `Back to ${rec.name}` }} icon={<Icon name="flame" />} title={<>Danger Zone · <Name>{rec.name}</Name></>}
      sub="Deactivation, transfer and deletion ask once more before they happen." />
    <ErrorSummary id={err?.section ?? "transfer"} error={err?.error} />
    <div class="stack-gap">
      {!group ? <Card title="Deactivate" icon="power" tone="danger">
        <p>Hide this record from listings and stop its use. Queued work is kept.</p>
        <div class="actions"><a class="btn btn-danger" href={`/record-deactivate?name=${encodeURIComponent(rec.name)}`}><Icon name="power" />Continue to confirmation</a></div>
      </Card> : null}
      {transferable ? <Card title="Transfer ownership" icon="arrow-right-left" tone="danger">
        <form id="form-transfer" method="post" action="/record-confirm">
          <input type="hidden" name="name" value={rec.name} /><input type="hidden" name="action" value="transfer" />
          <TextField name="owner" label="New owner" st={trSt} errId="transfer-error" required placeholder="user@realm" hint="The new Owner takes every right on it, transfer included. Credentials already issued for it keep working." />
          <FieldError id="transfer-error" error={trSt.error} />
          <div class="actions"><Button tone="danger" icon="arrow-right-left">Continue to confirmation</Button></div>
        </form>
      </Card> : prefixed ? <Card title="Transfer ownership" icon="arrow-right-left"><p class="muted">A group named for its owner (<code>{rec.name.split("/")[0]}/…</code>) is never transferred. Its new owner creates their own instead.</p></Card> : null}
      {!group && rec.kind !== "user" ? <Card title="Delete record" icon="trash-2" tone="danger">
        <p>No registration, no access: the name stops routing, its queue and counters go, and anyone holding it loses it.</p>
        <form method="post" action="/record-confirm"><input type="hidden" name="name" value={rec.name} /><input type="hidden" name="action" value="delete" />
          <div class="actions"><Button tone="danger" icon="trash-2">Continue to confirmation</Button></div></form>
      </Card> : group ? <Card title="Removal" icon="trash-2"><p class="muted">A group is retired by emptying its members, never removed: records that name it keep a stable meaning.</p></Card> : null}
    </div>
  </>;
  return respond(ctx, { title: `Danger Zone · ${rec.name}`, section: group ? "groups" : sectionOf(rec), signedIn: true, you: s.you, personal: !!rec.personal }, body, status);
}

async function postConfirm(ctx: Ctx): Promise<Response> {
  const name = ctx.f("name"), action = ctx.f("action");
  const [s, rec] = await Promise.all([ctx.status(), activeRecord(ctx, name)]);
  const back = `/record-danger?name=${encodeURIComponent(name)}`;
  if (action === "transfer") {
    if (!rec.can_transfer || rec.name === rec.owner) throw notYours("only this record's owner or the daemon owner can transfer it");
    const owner = ctx.f("owner").trim();
    if (!owner) return dangerPage(ctx, name, { section: "transfer", error: { message: "New owner is required.", field: "owner", status: 400 } }, 400);
    const body = <>
      <PageHead back={{ href: back, label: "Back to the Danger Zone" }} icon={<Icon name="arrow-right-left" />} title="Confirm ownership transfer" />
      <Card className="confirm-card" tone="danger" title={<>Transfer <code>{rec.name}</code> from <code>{rec.owner}</code> to <code>{owner}</code>?</>} icon="arrow-right-left">
        <ul><li>{owner} becomes its Owner, with every right on it, transfer included.</li><li>You keep only what its allow list and Maintainers give you.</li></ul>
        <form method="post" action="/record" class="actions">
          {[["action", "transfer"], ["name", rec.name], ["owner", owner], ["expected_owner", rec.owner], ["confirmed", "1"]].map(([n, v]) => <input type="hidden" name={n} value={v} />)}
          <Button tone="danger" icon="arrow-right-left">Transfer ownership</Button><a class="btn btn-ghost" href={back}>Cancel</a>
        </form>
      </Card>
    </>;
    return respond(ctx, { title: `Confirm ownership transfer · ${rec.name}`, section: sectionOf(rec), signedIn: true, you: s.you, personal: !!rec.personal }, body);
  }
  if (action === "delete") {
    if (!rec.can_manage || rec.kind === "group" || rec.kind === "user") throw notYours("only the owner or an assigned Maintainer can delete this record");
    const readers = rec.readers == null ? "unavailable" : String(rec.readers);
    const body = <>
      <PageHead back={{ href: back, label: "Back to the Danger Zone" }} icon={<Icon name="trash-2" />} title="Confirm deletion" />
      <Card className="confirm-card" tone="danger" title={<>Delete <code>{rec.name}</code>?</>} icon="trash-2">
        <p>It currently holds {number(rec.queued)} messages and has {readers} outstanding reads.</p>
        <ul><li>The name stops routing and its queue is discarded.</li><li>This cannot be undone; registering the name again starts from nothing.</li></ul>
        <form method="post" action="/record" class="actions">
          {[["action", "delete"], ["name", rec.name], ["expected_owner", rec.owner], ["expected_queued", String(rec.queued ?? 0)], ["expected_readers", readers], ["confirmed", "1"]].map(([n, v]) => <input type="hidden" name={n} value={v} />)}
          <Button tone="danger" icon="trash-2">Delete record</Button><a class="btn btn-ghost" href={back}>Cancel</a>
        </form>
      </Card>
    </>;
    return respond(ctx, { title: `Confirm deletion · ${rec.name}`, section: sectionOf(rec), signedIn: true, you: s.you, personal: !!rec.personal }, body);
  }
  throw new LocalProblem(400, "That confirmation action is not available.");
}

// ------------------------------------------------------------------ POST /record

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

/** A private replacement follows the public settings save, like Secret. */
export async function storeConfiguration(ctx: Ctx, name: string, what: string): Promise<void> {
  const raw = ctx.f("config").trim();
  if (!raw) return;
  try {
    await ctx.bus("POST", "/configure", { bodyJSON: `{"Name":${JSON.stringify(name)},"Config":${formatConfiguration(raw)}}` });
  } catch (e) {
    const reason = e instanceof Refusal ? e.detail : "the daemon did not answer";
    throw new LocalProblem(502, `${what}, but its configuration was not stored: ${reason}. Set it from the edit page.`, true);
  }
}

async function postRecordSettings(ctx: Ctx): Promise<Response> {
  const action = ctx.f("action"), name = ctx.f("name");
  await ctx.status();
  switch (action) {
    case "create": {
      const kind = ctx.f("kind");
      if (!isKind(kind)) throw new LocalProblem(400, "Choose a valid record kind.");
      const values = keep(ctx.form, RECORD_KEEP);
      const bound = ctx.f("bound").trim();
      if (bound && !/^\d+$/.test(bound)) return registerPage(ctx, kind, { values, error: { message: "Queue capacity must be a whole number.", field: "bound", status: 400 } }, 400);
      const body: Record<string, unknown> = { name, kind, descr: ctx.f("descr"), tags: normalizeTags(ctx.f("tags")), allow: terms(ctx.f("allow")), maintainers: terms(ctx.f("maintainers")), personal: ctx.f("personal") === "on", subs: terms(ctx.f("subs")) };
      if (kind === "agent" && ctx.f("roles").trim()) body.roles = terms(ctx.f("roles").replace(/,/g, " "));
      if (kind === "service") { body.addr = ctx.f("addr"); body.protocol = ctx.f("protocol"); }
      if (kind === "agent" || kind === "queue") { body.ttl = ctx.f("ttl"); body.overflow = ctx.f("overflow") || "strict"; body.bound = Number(bound || 0); }
      if (kind === "resource") { body.resource = { uri: ctx.f("uri"), template: ctx.f("template") === "on", source: ctx.f("source"), mimeType: ctx.f("mime") || undefined, title: ctx.f("title") || undefined }; }
      try {
        await ctx.bus("POST", "/register", { body, headers: { "If-None-Match": "*" } });
      } catch (e) {
        const r = formRefusal(e);
        const line = r && r.status !== 412 ? attribute(ctx, r.message, ["subs", "allow", "maintainers"]) : undefined;
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
      if (ctx.f("config").trim()) {
        try { JSON.parse(ctx.f("config")); } catch {
          return settingsPage(ctx, name, { values, error: { message: "Configuration must be valid JSON.", field: "config", status: 400 } }, 400);
        }
      }
      const change: Record<string, unknown> = { name, descr: ctx.f("descr") };
      if (ctx.form!.has("tags")) change.tags = normalizeTags(ctx.f("tags"));
      if (ctx.form!.has("roles")) change.roles = terms(ctx.f("roles").replace(/,/g, " "));
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
        const field = line?.field ?? (/personal|tags?/i.test(r.message) ? "tags" : /maintainer/i.test(r.message) ? "maintainers" : undefined);
        ctx.forget("/lookup"); ctx.forget("/inactive");
        return settingsPage(ctx, name, { values, error: { status: r.status, message: line?.message ?? r.message, field, line: line?.line } }, r.status);
      }
      await storeSecret(ctx, name, "The settings were saved");
      await storeConfiguration(ctx, name, "The settings were saved");
      ctx.forget("/lookup");
      const fresh = await ctx.lookup(name).catch(() => rec);
      const ret = returnTo(ctx.f("return"), [listPath(fresh), "/runner", "/runner-settings"], "");
      return flashRedirect(ctx, recordHref(fresh, ret ? { return: ret } : undefined), "saved");
    }
    case "configure": { // older forms still post here
      const rec = await activeRecord(ctx, name);
      const invalid = (message: string, status: number) => rec.kind === "group"
        ? (() => { throw new LocalProblem(status, message); })()
        : settingsPage(ctx, name, { values: { config: ctx.f("config") }, error: { message, field: "config", status } }, status);
      let config: unknown;
      try { config = JSON.parse(ctx.f("config")); } catch {
        return invalid("Configuration must be valid JSON.", 400);
      }
      try {
        await ctx.bus("POST", "/configure", { bodyJSON: `{"Name":${JSON.stringify(name)},"Config":${formatConfiguration(ctx.f("config"))}}` });
      } catch (e) {
        const r = formRefusal(e);
        if (!r?.preserve) throw e;
        return invalid(r.message, r.status);
      }
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
      const back = `/record-danger?name=${encodeURIComponent(name)}`;
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
      const back = `/record-danger?name=${encodeURIComponent(name)}`;
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

  }
  throw new LocalProblem(400, "That record action is not available.");
}

// ------------------------------------------------------------------ legacy addresses

/** A page that moved: the old address answers 301 to the new one, query kept. */
function moved(to: string) {
  return async (ctx: Ctx) => redirect(to + ctx.url.search, 301);
}

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
    const base = rec.kind === "pubsub" ? "/pubsub/channel" : rec.kind === "queue" ? "/queue" : detailPath(rec.kind);
    return redirect(`${base}${edit ? "/edit" : ""}${ctx.url.search}`, 301);
  };
}

export const handlers = {
  agents: (ctx: Ctx) => list(ctx, "agents"), services: (ctx: Ctx) => list(ctx, "services"), queues: (ctx: Ctx) => list(ctx, "queues"),
  pubsub: (ctx: Ctx) => list(ctx, "pubsub"), resources: (ctx: Ctx) => list(ctx, "resources"),
  newAgent: (ctx: Ctx) => registerPage(ctx, "agent"), newService: (ctx: Ctx) => registerPage(ctx, "service"),
  newQueue: (ctx: Ctx) => registerPage(ctx, "queue"), newPubsub: (ctx: Ctx) => registerPage(ctx, "pubsub"), newResource: (ctx: Ctx) => registerPage(ctx, "resource"),
  agent: (ctx: Ctx) => detail(ctx, "/agent"), service: (ctx: Ctx) => detail(ctx, "/service"), queue: (ctx: Ctx) => detail(ctx, "/queue"), pubsubChannel: (ctx: Ctx) => detail(ctx, "/pubsub/channel"), resource: (ctx: Ctx) => detail(ctx, "/resource"),
  edit: (ctx: Ctx) => settingsPage(ctx, ctx.q("name")),
  deactivate: deactivatePage, danger: (ctx: Ctx) => dangerPage(ctx, ctx.q("name")),
  post: postRecordSettings, confirm: postConfirm,
  moved,
  channels: channelsRedirect({ queue: "/queues", pubsub: "/pubsub" }), channelsNew: channelsRedirect({ queue: "/queues/new", pubsub: "/pubsub/new" }),
  channel: channelRedirect(false), channelEdit: channelRedirect(true),
};
