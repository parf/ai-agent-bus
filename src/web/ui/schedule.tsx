// A record's schedules: its recurring crons and one-shot ats, kept and fired
// by the daemon (docs/09-daemon-api.md#schedules). Its Owner, Maintainers and
// own Agent see them; the daemon refuses anybody else, and the card is absent.
import { h, Fragment } from "../jsx.ts";
import { Ctx } from "../ctx.ts";
import { Refusal } from "../daemon.ts";
import { Card, Pill, Muted } from "./kit.tsx";
import { number, validTime } from "../format.ts";

export type ScheduleKind = "recurring" | "once";
export type Outcome = { at: string; slot: string; outcome: string; detail?: string };
export type Schedule = {
  id: string; holder: string; holder_kind: string; kind: ScheduleKind; expr?: string; at?: string; target: string; body: string;
  creator: string; enabled: boolean; disabled?: string; until?: string; max_runs?: number; max_late?: string;
  next?: string; last?: string; last_outcome?: string; runs: number; sent: number; dropped: number; refused_in_a_row?: number;
  created: string; changed: string; outcomes?: Outcome[]; upcoming?: string[];
};

/** A schedule's state as the pages filter it: enabled, or why it is not. */
export type ScheduleState = "enabled" | "hand" | "failure" | "ended";
export const stateOf = (s: Schedule): ScheduleState =>
  s.enabled ? "enabled" : s.disabled === "failure" ? "failure" : s.disabled === "ended" ? "ended" : "hand";
export const STATE_WORD: Record<ScheduleState, string> = {
  enabled: "Enabled", hand: "Disabled by hand", failure: "Disabled by failure", ended: "Ended",
};

/** Disabled by failure, refused on its latest firings, or a last outcome that was not a send. */
export const hasProblem = (s: Schedule) =>
  stateOf(s) === "failure" || (s.refused_in_a_row ?? 0) > 0 || (!!s.last_outcome && s.last_outcome !== "sent");

/** Disabled by failure is red, disabled by hand or ended grey. */
export const StatePill = ({ s }: { s: Schedule }) => {
  const st = stateOf(s);
  return <Pill tone={st === "enabled" ? "ok" : st === "failure" ? "bad" : "muted"} dot>{STATE_WORD[st]}</Pill>;
};

export const OutcomePill = ({ outcome }: { outcome?: string }) =>
  !outcome ? <Muted>—</Muted> : <Pill tone={outcome === "sent" ? "ok" : "bad"}>{outcome.replace(/_/g, " ")}</Pill>;

/** An instant in the viewer's own zone once the page script runs, UTC until then; the raw UTC is its title. */
export const When = ({ iso }: { iso?: string }) => {
  const d = validTime(iso);
  return d ? <time datetime={iso} title={iso} data-local>{d.toISOString().slice(0, 16).replace("T", " ")} UTC</time> : <Muted>—</Muted>;
};

export const scheduleHref = (id: string) => `/schedule?id=${encodeURIComponent(id)}`;
export const listHref = (kind: ScheduleKind) => kind === "once" ? "/ats" : "/crons";

/** The record's schedules, or null when the visitor may not use them. */
export async function recordSchedules(ctx: Ctx, record: string): Promise<Schedule[] | null> {
  return ctx.scheduleList(record).then(l => l, e => { if (e instanceof Refusal) return null; throw e; });
}

/** A compact summary like the KV card: counts per kind and the problems; each schedule opens on its own page. */
export const ScheduleCard = ({ record, list }: { record: string; list: Schedule[] }) => {
  const crons = list.filter(s => s.kind === "recurring").length, ats = list.length - crons;
  const failed = list.filter(s => stateOf(s) === "failure").length;
  const problems = list.filter(s => hasProblem(s)).length;
  const q = (kind: ScheduleKind) => `${listHref(kind)}?q=${encodeURIComponent(record)}`;
  return <Card title="Schedules" icon="calendar-clock" id="schedules" actions={<>
    {problems ? <span class="count tone-bad" title="Disabled by failure, refused on its latest firings, or a last outcome that was not a send">{number(problems)} with problems</span> : null}
    <a class="btn btn-secondary btn-sm" href={`/crons?${new URLSearchParams({ record })}#schedule-add`}>Add</a></>}>
    {list.length ? <p class="kv-counts">
      {crons ? <a href={q("recurring")}><strong>{number(crons)}</strong> {crons === 1 ? "cron" : "crons"}</a> : null}
      {crons && ats ? " · " : null}
      {ats ? <a href={q("once")}><strong>{number(ats)}</strong> {ats === 1 ? "at" : "ats"}</a> : null}
    </p> : <p class="muted small">No crons or ats on this record.</p>}
    {failed ? <p class="warn">{number(failed)} disabled by failure.</p> : null}
    {list.length ? <ul class="schedule-list">{list.slice(0, 10).map(s => <li>
      <a href={scheduleHref(s.id)}><code>{s.kind === "recurring" ? s.expr : s.at}</code></a> → <code>{s.target}</code> <StatePill s={s} />
    </li>)}</ul> : null}
  </Card>;
};
