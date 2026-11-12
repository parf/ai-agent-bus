// The schedule pages: every cron (/crons) and at (/ats) on records the
// visitor manages, one schedule (/schedule), and the forms that add, switch,
// fire and delete them. The daemon decides who may; these pages only ask
// (docs/09-daemon-api.md#schedules).
import { h, Fragment, type Child } from "../jsx.ts";
import { Ctx, LocalProblem, type Rec } from "../ctx.ts";
import { respond, flashRedirect, FLASH } from "../ui/frame.tsx";
import { Icon, Help, PageHead, Card, KindIcon, Muted, Empty, Segmented, Tabs, Button, Facts, Pager, recordHref } from "../ui/kit.tsx";
import { TextField, TextAreaField, SelectField, CheckField, FieldError, ErrorSummary, type FormState } from "../ui/forms.tsx";
import { TagBadges } from "../ui/tags.tsx";
import { formRefusal } from "../problem.tsx";
import { entity } from "../glyphs.ts";
import { number } from "../format.ts";
import { recordTags, filterTag } from "../tags.ts";
import { PAGE_SIZE as PAGE } from "../pagination.ts";
import {
  When, StatePill, OutcomePill, STATE_WORD, stateOf, hasProblem, scheduleHref, listHref,
  type Schedule, type ScheduleKind, type ScheduleState, type Outcome,
} from "../ui/schedule.tsx";

const HELP: Record<ScheduleKind, string[]> = {
  recurring: ["A cron sends the same message on a repeating time, such as every weekday at 09:00 UTC.",
    "Any agent, queue or pubsub can hold its own; it fires even while its holder's program is down.",
    "Its Owner, Maintainers and own Agent see and change it; five refused firings in a row disable it."],
  once: ["An at sends one message once: after a delay, at a time of day in UTC, or at an exact instant.",
    "Any agent, queue or pubsub can hold its own; it fires even while its holder's program is down.",
    "Its Owner, Maintainers and own Agent see and change it; outcomes are kept for seven days."],
};
const TITLE: Record<ScheduleKind, string> = { recurring: "Crons", once: "Ats" };
const ICON: Record<ScheduleKind, string> = { recurring: "repeat", once: "alarm-clock" };
const STATES: ScheduleState[] = ["enabled", "hand", "failure", "ended"];
const kindWord = (k: string) => k === "pubsub" ? "PubSub" : `${entity(k)?.word ?? k}s`;
function listUrl(kind: ScheduleKind, p: Record<string, string | undefined>): string {
  const q = new URLSearchParams(Object.entries(p).filter(([, v]) => v) as [string, string][]);
  return q.size ? `${listHref(kind)}?${q}` : listHref(kind);
}

const When_ = ({ s }: { s: Schedule }) => <code>{s.kind === "recurring" ? s.expr : s.at}</code>;

// ------------------------------------------------------------------ /crons, /ats

const HOWS: [string, string][] = [["expr", "Cron expression"], ["in", "In: after a delay"], ["at", "At: a time"]];
const WHEN_HINT: Record<string, string> = {
  expr: "Five cron fields in UTC, or six with a leading second.",
  in: "A delay such as 90s, 30m, 2h or 3d.",
  at: "HH:MM in UTC, or an RFC 3339 instant.",
};

/** The fields an add and an edit share; the daemon checks every one. */
function Fields({ st, hows, how, editing }: { st: FormState; hows: string[]; how: string; editing?: boolean }) {
  const err = "schedule-error";
  return <>
    {hows.length === 1 ? <input type="hidden" name="how" value={hows[0]} />
      : <SelectField name="how" label="When" st={st} errId={err} value={how} options={HOWS.filter(([k]) => hows.includes(k))} />}
    <TextField name="when" label={how === "expr" && hows.length === 1 ? "Expression" : "Expression, delay or time"} st={st} errId={err} required
      placeholder={hows.length === 1 ? "0 9 * * 1-5" : "0 9 * * 1-5, 90s or 14:30"} hint={hows.map(k => WHEN_HINT[k]).join(" ")} />
    <TextField name="target" label="Target" st={st} errId={err} required placeholder="#agent@team or jobs@team"
      hint="Your own inbox, or any agent or queue you may send to." />
    <TextAreaField name="body" label="Message" st={st} errId={err} rows={3} placeholder="nightly report" hint="Sent as written; it may not be empty." />
    <TextField name="until" label="Until" st={st} errId={err} placeholder="2026-12-31T23:59:59Z" hint="Optional: no firing after this RFC 3339 instant." />
    <TextField name="max_runs" label="Max runs" st={st} errId={err} type="number" min="1" placeholder="10" hint="Optional: it ends after this many firings." />
    <TextField name="max_late" label="Max lateness" st={st} errId={err} placeholder="5m" hint="Optional: a firing later than this is skipped." />
    <CheckField name="disabled" label={editing ? "Disabled" : "Start disabled"} st={st} errId={err} checked={st.values.disabled === "on"} hint="Kept, but fires only once enabled." />
    <FieldError id={err} error={st.error} />
  </>;
}

function AddForm({ kind, st }: { kind: ScheduleKind; st: FormState }) {
  const hows = kind === "recurring" ? ["expr"] : ["in", "at"];
  const how = hows.includes(st.values.how ?? "") ? st.values.how! : hows[0]!;
  return <form id="form-schedule-add" method="post" action="/schedule-add" class="form-grid">
    <input type="hidden" name="return" value={kind} />
    <TextField name="record" label="Record" st={st} errId="schedule-error" required placeholder="#agent@team, jobs@team"
      hint="The agent, queue or pubsub that holds it." />
    <Fields st={st} hows={hows} how={how} />
    <div class="actions"><Button tone="primary" icon="plus">Add {kind === "recurring" ? "cron" : "at"}</Button></div>
  </form>;
}

/** The schedule as it stands, for the edit form. */
const current = (s: Schedule): Record<string, string> => ({
  how: s.kind === "recurring" ? "expr" : "at", when: (s.kind === "recurring" ? s.expr : s.at) ?? "", target: s.target, body: s.body,
  until: s.until ?? "", max_runs: s.max_runs ? String(s.max_runs) : "", max_late: s.max_late ?? "", disabled: s.enabled ? "" : "on",
});

function EditForm({ s, st }: { s: Schedule; st: FormState }) {
  const how = st.values.how && ["expr", "in", "at"].includes(st.values.how) ? st.values.how : "expr";
  return <form id="form-schedule-edit" method="post" action="/schedule-edit" class="form-grid">
    <input type="hidden" name="id" value={s.id} /><input type="hidden" name="kind" value={s.kind} />
    <Fields st={st} hows={["expr", "in", "at"]} how={how} editing />
    <div class="actions"><Button tone="primary" icon="check">Save</Button></div>
    <p class="muted small wide">Saving makes you its creator: it then sends on your behalf, to a target you may send to.</p>
  </form>;
}

async function listPage(ctx: Ctx, kind: ScheduleKind, st: FormState = { values: {} }, status = 200): Promise<Response> {
  const [you, everything, records] = await Promise.all([ctx.you(), ctx.scheduleList(), ctx.records()]);
  const byName = new Map<string, Rec>(records.map(r => [r.name, r]));
  const all = everything.filter(s => s.kind === kind);
  const recOf = (s: Schedule) => byName.get(s.holder);
  const kindOf = (s: Schedule) => s.holder_kind;
  const present = ["agent", "queue", "pubsub", "service", "resource", "group", "user"].filter(k => all.some(s => kindOf(s) === k));
  const q = ctx.q("q"), rkind = present.includes(ctx.q("kind")) ? ctx.q("kind") : "";
  const state = (STATES as string[]).includes(ctx.q("state")) ? ctx.q("state") as ScheduleState : "";
  const problems = ctx.q("problems") === "1" ? "1" : "", tag = ctx.q("tag") ? filterTag(ctx.q("tag")) : "";
  const needle = q.toLowerCase();
  const params = { q, kind: rkind, state, problems, tag };

  // Each facet counts against the other selected filters.
  const tagCounts = new Map<string, number>();
  const counts = { state: Object.fromEntries(STATES.map(s => [s, 0])) as Record<ScheduleState, number>, all: 0, problems: 0 };
  const rows: Schedule[] = [];
  for (const s of all) {
    if (needle && !s.holder.toLowerCase().includes(needle)) continue;
    if (rkind && kindOf(s) !== rkind) continue;
    const labels = recordTags(recOf(s) ?? {});
    const sm = !state || stateOf(s) === state, pm = !problems || hasProblem(s), tm = !tag || labels.includes(tag);
    if (sm && pm) for (const t of labels) tagCounts.set(t, (tagCounts.get(t) ?? 0) + 1);
    if (!tm) continue;
    if (pm) counts.state[stateOf(s)]++;
    if (sm) { counts.all++; if (hasProblem(s)) counts.problems++; }
    if (sm && pm) rows.push(s);
  }
  const matched = rows.length, pages = Math.max(1, Math.ceil(matched / PAGE));
  const pageNo = Math.min(pages, Math.max(1, Math.floor(Number(ctx.q("page"))) || 1));
  const shown = rows.slice((pageNo - 1) * PAGE, pageNo * PAGE);
  const tagEntries = [...tagCounts].filter(([t]) => t !== "no-tags").sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]));
  if (tag && !tagEntries.some(([t]) => t === tag)) tagEntries.push([tag, 0]);
  const filtered = !!(q || rkind || state || problems || tag);
  const clear = listUrl(kind, {});
  const link = (name: string, value: string) => listUrl(kind, { ...params, [name]: value || undefined });
  const title = TITLE[kind], noun = kind === "recurring" ? "crons" : "ats";
  if (!st.values.record && ctx.q("record")) st = { ...st, values: { ...st.values, record: ctx.q("record") } };

  const body = <>
    <PageHead icon={<Icon name={ICON[kind]} />} title={title} help={<Help id="schedule-help" label={`About ${title}`} title={title} items={HELP[kind]} />} />
    <ErrorSummary id="schedule-add" error={st.error} />
    {tagEntries.length ? <Tabs label="Holder tags" items={[{ href: link("tag", ""), text: "All", current: !tag },
      ...tagEntries.map(([t, count]) => ({ href: link("tag", t), text: t, count, current: tag === t }))]} /> : null}
    <form class="toolbar record-search" method="get" action={listHref(kind)}>
      <div class="search"><Icon name="search" /><label for="schedule-query" class="sr-only">Search {noun}</label>
        <input id="schedule-query" type="search" name="q" value={q} placeholder="Search by holder" /></div>
      {Object.entries(params).filter(([n, v]) => n !== "q" && v).map(([n, v]) => <input type="hidden" name={n} value={v} />)}
    </form>
    {all.length ? <div class="toolbar filters">
      {present.length > 1 ? <Segmented label="Kind" items={[{ href: link("kind", ""), text: "All", current: !rkind }, ...present.map(k => ({ href: link("kind", k), text: kindWord(k), current: rkind === k }))]} /> : null}
      <Segmented label="State" items={[{ href: link("state", ""), text: "All", current: !state },
        ...STATES.map(s => ({ href: link("state", s), text: `${STATE_WORD[s]} (${number(counts.state[s])})`, current: state === s, disabled: counts.state[s] === 0 && state !== s }))]} />
      <Segmented label="Problems" items={[{ href: link("problems", ""), text: `All (${number(counts.all)})`, current: !problems },
        { href: link("problems", "1"), text: `Has problems (${number(counts.problems)})`, current: !!problems, disabled: counts.problems === 0 && !problems }]} />
    </div> : null}
    {all.length === 0 ? <Empty icon={ICON[kind]} title={`No ${noun} yet`}>{kind === "recurring" ? "A cron" : "An at"} appears here once a record you may use holds one; add the first below.</Empty>
      : matched === 0 ? <Empty icon="filter" title={`No ${noun} match these filters`} action={<a class="btn" href={clear}>Clear filters</a>}>Change the search or the filters above, or clear filters.</Empty>
      : <>
        <div class="results-line"><span>Showing {number(shown.length)} of {number(matched)} matching, {number(all.length)} {noun} on records you may use.</span>{filtered ? <a href={clear}>Clear filters</a> : null}</div>
        <div class="table-wrap"><div class="table-scroll"><table class="data stack record-table schedule-table">
          <thead><tr><th>Holder</th><th>{kind === "recurring" ? "Expression" : "At"}</th><th>Target</th><th>Next fire</th><th>Last outcome</th><th class="num">Sent</th><th class="num">Dropped</th><th>State</th><th><span class="sr-only">Actions</span></th></tr></thead>
          <tbody>{shown.map(s => {
            const rec = recOf(s);
            return <tr class={stateOf(s) === "failure" ? "schedule-failed" : stateOf(s) !== "enabled" ? "inactive-record" : ""}>
              <td data-label="Holder"><div class="rec-cell"><KindIcon kind={kindOf(s)} /><div class="rec-title"><a href={recordHref({ name: s.holder, kind: kindOf(s) })}><code>{s.holder}</code></a>
                {rec ? <TagBadges rec={rec} href={t => link("tag", t)} /> : null}</div></div></td>
              <td data-label={kind === "recurring" ? "Expression" : "At"}><When_ s={s} /></td>
              <td data-label="Target"><code>{s.target}</code></td>
              <td data-label="Next fire"><When iso={s.next} /></td>
              <td data-label="Last outcome"><OutcomePill outcome={s.last_outcome} /></td>
              <td class="num" data-label="Sent">{s.sent ? number(s.sent) : ""}</td>
              <td class="num" data-label="Dropped">{s.dropped ? number(s.dropped) : ""}</td>
              <td data-label="State"><StatePill s={s} /></td>
              <td data-label="Actions"><a class="btn btn-sm" href={scheduleHref(s.id)}>Open</a></td>
            </tr>;
          })}</tbody>
        </table></div></div>
        {pages > 1 ? <Pager label={`${title} pages`} prev={pageNo > 1 ? listUrl(kind, { ...params, page: String(pageNo - 1) }) : undefined} next={pageNo < pages ? listUrl(kind, { ...params, page: String(pageNo + 1) }) : undefined}>Page {pageNo} of {pages}</Pager> : null}
      </>}
    <Card title={`Add ${kind === "recurring" ? "a cron" : "an at"}`} icon="plus" id="schedule-add"><AddForm kind={kind} st={st} /></Card>
  </>;
  return respond(ctx, { title, section: kind === "recurring" ? "crons" : "ats", signedIn: true, you }, body, status);
}

// ------------------------------------------------------------------ /schedule

async function detailPage(ctx: Ctx, st?: FormState, status = 200): Promise<Response> {
  const id = ctx.q("id") || ctx.f("id");
  if (!id) throw new LocalProblem(400, "Name the schedule to open.");
  // A schedule the visitor may not use is the daemon's refusal, on the problem page.
  const [you, s] = await Promise.all([ctx.you(), ctx.schedule(id)]);
  const holderKind = s.holder_kind;
  const edit: FormState = st ?? { values: current(s) };
  const kind = s.kind, back = listHref(kind);
  const outcomes = s.outcomes ?? [];
  const hidden = <><input type="hidden" name="id" value={s.id} /><input type="hidden" name="kind" value={kind} /></>;
  const facts: [Child, Child][] = [
    ["Holder", <><KindIcon kind={holderKind} /> <a href={recordHref({ name: s.holder, kind: holderKind })}><code>{s.holder}</code></a></>],
    [kind === "recurring" ? "Expression" : "At", <When_ s={s} />],
    ["Target", <code>{s.target}</code>],
    ["Creator", <code>{s.creator}</code>],
    ["State", <StatePill s={s} />],
    ["Next fire", <When iso={s.next} />],
    ...(s.upcoming?.length ? [["Upcoming", <ol class="schedule-upcoming">{s.upcoming.map(t => <li><When iso={t} /></li>)}</ol>] as [Child, Child]] : []),
    ["Last fired", <When iso={s.last} />],
    ["Firings", <>{number(s.runs)} · {number(s.sent)} sent · {number(s.dropped)} dropped</>],
  ];
  if (s.refused_in_a_row) facts.push(["Refused in a row", <span class="warn">{number(s.refused_in_a_row)}</span>]);
  if (s.until) facts.push(["Until", <When iso={s.until} />]);
  if (s.max_runs) facts.push(["Max runs", number(s.max_runs)]);
  if (s.max_late) facts.push(["Max lateness", s.max_late]);
  facts.push(["Created", <When iso={s.created} />], ["Changed", <When iso={s.changed} />]);
  const body = <>
    <PageHead back={{ href: back, label: `Back to ${TITLE[kind]}` }} icon={<Icon name={ICON[kind]} />}
      title={<>{kind === "recurring" ? "Cron" : "At"} <code>{s.id}</code></>}
      sub={<>on <a href={recordHref({ name: s.holder, kind: holderKind })}><code>{s.holder}</code></a></>} />
    <ErrorSummary id="schedule-edit" error={edit.error} />
    <div class="detail-grid">
      <div>
        <Card title="Message" icon="mail" id="schedule-message"><pre class="kv-value"><code>{s.body}</code></pre></Card>
        <Card title="Outcomes" icon="history" id="schedule-outcomes" actions={<span class="count">{outcomes.length}</span>}>
          {outcomes.length ? <table class="data stack kv-table">
            <thead><tr><th>Fired</th><th>Slot</th><th>Outcome</th><th>Detail</th></tr></thead>
            <tbody>{outcomes.map(o => <tr>
              <td data-label="Fired"><When iso={o.at} /></td>
              <td data-label="Slot"><When iso={o.slot} /></td>
              <td data-label="Outcome"><OutcomePill outcome={o.outcome} /></td>
              <td data-label="Detail">{o.detail ?? ""}</td>
            </tr>)}</tbody>
          </table> : <p class="muted small">Not fired in the last seven days.</p>}
        </Card>
        <Card title="Edit" icon="pencil" id="schedule-edit"><EditForm s={s} st={edit} /></Card>
      </div>
      <div>
        <Card title="Schedule" icon="calendar-clock" id="schedule-facts"><Facts rows={facts} /></Card>
        <Card title="Actions" icon="settings" id="schedule-actions">
          <form method="post" action="/schedule-switch" class="actions">{hidden}
            {s.enabled ? <Button name="on" value="0" tone="secondary" icon="pause">Disable</Button> : <Button name="on" value="1" tone="secondary" icon="play">Enable</Button>}
            <Button formaction="/schedule-run" tone="secondary" icon="send">Run now</Button>
          </form>
          {!s.enabled ? <p class="muted small">Enabling makes you its creator; it then sends on your behalf.</p> : null}
          <details class="inline-confirm"><summary class="btn btn-sm btn-danger">Delete</summary>
            <form method="post" action="/schedule-delete" class="inline-form">{hidden}
              <button class="btn btn-sm btn-danger">Confirm delete of <code>{s.id}</code></button>
            </form>
          </details>
        </Card>
      </div>
    </div>
  </>;
  return respond(ctx, { title: `${kind === "recurring" ? "Cron" : "At"} ${s.id}`, section: kind === "recurring" ? "crons" : "ats", signedIn: true, you }, body, status);
}

// ------------------------------------------------------------------ posts

const kindOfForm = (ctx: Ctx): ScheduleKind => ctx.f("kind") === "once" || ctx.f("return") === "once" ? "once" : "recurring";

/** The add or edit request the form describes, or the field a value is wrong in. */
function request(ctx: Ctx, hows: string[]): { body: Record<string, unknown> } | { field: string; message: string } {
  const how = hows.includes(ctx.f("how")) ? ctx.f("how") : hows[0]!;
  const when = ctx.f("when").trim(), until = ctx.f("until").trim(), maxRuns = ctx.f("max_runs").trim(), maxLate = ctx.f("max_late").trim();
  if (maxRuns && !/^[1-9]\d*$/.test(maxRuns)) return { field: "max_runs", message: "Max runs is a whole number above 0." };
  // A browser sends every line break in a form as CRLF.
  const body: Record<string, unknown> = { [how]: when, target: ctx.f("target").trim(), body: ctx.f("body").replace(/\r\n/g, "\n") };
  if (until) body.until = until;
  if (maxRuns) body.max_runs = Number(maxRuns);
  if (maxLate) body.max_late = maxLate;
  if (ctx.f("disabled") === "on") body.disabled = true;
  return { body };
}
const FIELDS = ["record", "how", "when", "target", "body", "until", "max_runs", "max_late", "disabled"];
const typed = (ctx: Ctx) => Object.fromEntries(FIELDS.map(n => [n, ctx.f(n)]));
/** A refusal the visitor can correct keeps the form, the values typed in: a
 *  bad field, a cap, or a target the creator may not send to (403). */
const correctable = (e: unknown) => { const r = formRefusal(e); return r && (r.preserve || r.status === 413 || r.status === 403) ? r : undefined; };

async function postAdd(ctx: Ctx): Promise<Response> {
  const kind = kindOfForm(ctx), record = ctx.f("record").trim();
  const again = (status: number, message: string, field?: string) => listPage(ctx, kind, { values: typed(ctx), error: { status, message, field } }, status);
  if (!record) return again(400, "Name the record that holds it.", "record");
  const req = request(ctx, kind === "recurring" ? ["expr"] : ["in", "at"]);
  if ("field" in req) return again(400, req.message, req.field);
  let answer: { schedule: Schedule; warning?: string };
  try { answer = await ctx.bus("POST", "/schedule/add", { body: { record, ...req.body } }); }
  catch (e) { const r = correctable(e); if (!r) throw e; return again(r.status, r.message); }
  return flashRedirect(ctx, scheduleHref(answer.schedule.id), answer.warning ? "schedule-added-near-cap" : "schedule-added");
}

async function postEdit(ctx: Ctx): Promise<Response> {
  const id = ctx.f("id");
  const again = (status: number, message: string, field?: string) => detailPage(ctx, { values: typed(ctx), error: { status, message, field } }, status);
  const req = request(ctx, ["expr", "in", "at"]);
  if ("field" in req) return again(400, req.message, req.field);
  try { await ctx.bus("POST", "/schedule/edit", { body: { id, ...req.body } }); }
  // A refusal that leaves the schedule readable keeps the form; a missing one is the problem page.
  catch (e) { const r = correctable(e); if (!r || r.status === 404) throw e; return again(r.status, r.message); }
  return flashRedirect(ctx, scheduleHref(id), "schedule-saved");
}

async function postSwitch(ctx: Ctx): Promise<Response> {
  const id = ctx.f("id"), on = ctx.f("on") === "1";
  await ctx.bus("POST", on ? "/schedule/enable" : "/schedule/disable", { body: { id } });
  return flashRedirect(ctx, scheduleHref(id), on ? "schedule-enabled" : "schedule-disabled");
}

async function postRun(ctx: Ctx): Promise<Response> {
  const id = ctx.f("id");
  const { outcome } = await ctx.bus<{ outcome: Outcome }>("POST", "/schedule/run", { body: { id } });
  const key = `schedule-run-${outcome?.outcome}`;
  return flashRedirect(ctx, scheduleHref(id), key in FLASH ? key : "schedule-run-other");
}

async function postDelete(ctx: Ctx): Promise<Response> {
  const id = ctx.f("id");
  await ctx.bus("POST", "/schedule/delete", { body: { id } });
  return flashRedirect(ctx, listHref(kindOfForm(ctx)), "schedule-deleted");
}

export const handlers = {
  crons: (ctx: Ctx) => listPage(ctx, "recurring"), ats: (ctx: Ctx) => listPage(ctx, "once"), schedule: (ctx: Ctx) => detailPage(ctx),
  postAdd, postEdit, postSwitch, postRun, postDelete,
};
