// Activity over a range: Day, Week, Month, with ‹ › to step through what the
// daemon keeps (docs/05-discovery.md#activity-history). One component serves
// /activity and every record's detail page.
import { h, Fragment, type Child, raw, escape } from "../jsx.ts";
import type { Ctx } from "../ctx.ts";
import { Icon } from "./kit.tsx";
import { DayChart, SERIES, total, type Slot } from "./charts.tsx";
import { number, slotLabel } from "../format.ts";

export type RangeKind = "day" | "week" | "month";
export const RANGES: { key: RangeKind; label: string; days: number }[] = [
  { key: "day", label: "Day", days: 1 }, { key: "week", label: "Week", days: 7 }, { key: "month", label: "Month", days: 30 },
];
/** Used only when a daemon states no retention of its own. */
export const KEEP_DAYS = 400;

const MON = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
const DOW = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

/** A local calendar day as yymmdd, the daemon's key. */
export type Ymd = number;
export const ymdOf = (d: Date): Ymd => (d.getFullYear() % 100) * 10000 + (d.getMonth() + 1) * 100 + d.getDate();
export const dateOf = (y: Ymd) => new Date(2000 + Math.floor(y / 10000), Math.floor(y / 100) % 100 - 1, y % 100);
export const addDays = (y: Ymd, n: number): Ymd => { const d = dateOf(y); d.setDate(d.getDate() + n); return ymdOf(d); };
export const validYmd = (y: number) => Number.isInteger(y) && y > 0 && ymdOf(dateOf(y)) === y;
export const ymdLabel = (y: Ymd) => { const d = dateOf(y); return `${MON[d.getMonth()]} ${d.getDate()}`; };

export type Range = {
  kind: RangeKind; days: number; today: Ymd; at: Ymd; from: Ymd; to: Ymd;
  live: boolean;              // Day ending today: the last 24 hours, read live
  prev?: Ymd; next?: Ymd;     // the at of the neighbouring ranges, when they exist
  oldest: Ymd;
};

/** The range the request asks for, clamped to what the daemon keeps. Today
 * and the retention are the daemon's own (`/status`), never this process's clock. */
export function parseRange(ctx: Ctx, today: Ymd = ymdOf(new Date()), kept = KEEP_DAYS): Range {
  const kind = (RANGES.find(r => r.key === ctx.q("range"))?.key ?? "day") as RangeKind;
  const days = RANGES.find(r => r.key === kind)!.days;
  const oldest = addDays(today, -(kept - 1));
  let at = Number(ctx.q("at"));
  if (!validYmd(at) || at > today) at = today;
  if (at < addDays(oldest, days - 1)) at = addDays(oldest, days - 1);
  const from = addDays(at, -(days - 1));
  const prevAt = addDays(at, -days), nextAt = Math.min(addDays(at, days), today);
  return {
    kind, days, today, at, from, to: at, live: kind === "day" && at === today, oldest,
    prev: addDays(prevAt, -(days - 1)) >= oldest ? prevAt : undefined,
    next: at < today ? nextAt : undefined,
  };
}

export type Day = { day: Ymd; slots: Slot[] };

/** The daemon's range for a status answer: its today and its retention. */
export function rangeFor(ctx: Ctx, st: { today?: number; activity_days_kept?: number }): Range {
  return parseRange(ctx, validYmd(st.today ?? 0) ? st.today! : ymdOf(new Date()), st.activity_days_kept || KEEP_DAYS);
}

/** Each visible record's hits over the range, from one daemon answer. */
export async function rangeTotals(ctx: Ctx, r: Range): Promise<Map<string, number>> {
  const q = r.live ? { totals: "1", from: String(r.today), to: String(r.today) } : { totals: "1", from: String(r.from), to: String(r.to) };
  const t = (await ctx.get<Record<string, Slot>>("/activity/days", q)) ?? {};
  return new Map(Object.entries(t).map(([n, s]) => [n, hits([s])]));
}

/** yymmdd of a daemon timestamp, read from its own date, not converted. */
export const ymdOfIso = (iso: string): Ymd => { const m = /^\d{2}(\d{2})-(\d{2})-(\d{2})/.exec(iso); return m ? Number(m[1]! + m[2]! + m[3]!) : 0; };
export type RangeData = { slots: Slot[]; days: Day[] };

/** Every slot of the range: the live day, or the stored days from the daemon. */
export async function loadRange(ctx: Ctx, r: Range, name?: string): Promise<RangeData> {
  if (r.live) {
    const slots = (await ctx.get<Slot[]>("/activity", name ? { name } : undefined)) ?? [];
    return { slots, days: [] };
  }
  const days = (await ctx.get<Day[]>("/activity/days", { ...(name ? { name } : {}), from: String(r.from), to: String(r.to) })) ?? [];
  return { slots: days.flatMap(d => d.slots), days };
}

export const hits = (slots: Slot[]) => slots.reduce((a, x) => a + (x.in ?? 0) + (x.out ?? 0) + (x.dropped ?? 0) + (x.expired ?? 0) + (x.refused ?? 0), 0);

/** Buckets of n slots summed: an hour is 6, a day 144; each starts at its first slot. */
export function bucket(slots: Slot[], n: number): Slot[] {
  const out: Slot[] = [];
  for (let i = 0; i < slots.length; i += n) {
    const part = slots.slice(i, i + n);
    out.push({ at: part[0]!.at, in: 0, out: 0, dropped: 0, expired: 0, refused: 0 });
    for (const s of part) for (const k of SERIES) out.at(-1)![k.key] += s[k.key] ?? 0;
  }
  return out;
}

export function scopeLine(r: Range, data: RangeData): string {
  if (r.live) {
    const s = data.slots;
    return s.length ? `the last 24 hours · ${slotLabel(s[0]!.at)} to now, ten-minute slots` : "the last 24 hours";
  }
  if (r.kind === "day") { const d = dateOf(r.at); return `${DOW[d.getDay()]} ${ymdLabel(r.at)}, ten-minute slots`; }
  return `${ymdLabel(r.from)} – ${ymdLabel(r.to)}, ${r.kind === "week" ? "hourly" : "daily"}`;
}

/** Day · Week · Month tabs, ‹ Prev, Next ›, and Today when away from it. */
export function RangeNav({ r, href, label }: { r: Range; href: (p: { range?: string; at?: number }) => string; label: string }) {
  const q = (kind: RangeKind, at?: number) => href({ range: kind === "day" ? undefined : kind, at: at && at !== r.today ? at : undefined });
  const step = r.kind === "day" ? "day" : r.kind;
  return <nav class="range-nav" aria-label={label}>
    <div class="segmented" role="group" aria-label="Range">
      {RANGES.map(x => <a href={q(x.key, r.at)} aria-current={x.key === r.kind ? "true" : undefined}>{x.label}</a>)}
    </div>
    <div class="range-steps">
      {r.prev ? <a class="btn btn-ghost btn-sm" href={q(r.kind, r.prev)} rel="prev" aria-label={`Previous ${step}`}><Icon name="chevron-left" />Prev</a>
        : <span class="btn btn-ghost btn-sm" aria-disabled="true"><Icon name="chevron-left" />Prev</span>}
      {r.next ? <a class="btn btn-ghost btn-sm" href={q(r.kind, r.next)} rel="next" aria-label={`Next ${step}`}>Next<Icon name="chevron-right" /></a>
        : <span class="btn btn-ghost btn-sm" aria-disabled="true">Next<Icon name="chevron-right" /></span>}
      {r.at !== r.today ? <a class="btn btn-ghost btn-sm" href={q(r.kind)}>Today</a> : null}
    </div>
  </nav>;
}

/** One cell per day of the month, coloured by its hits, each a link to that day. */
function Calendar({ days, href }: { days: Day[]; href: (at: number) => string }) {
  const totals = days.map(d => hits(d.slots));
  const max = Math.max(1, ...totals);
  return <div class="calendar" role="list" aria-label="Hits per day">
    {days.map((d, i) => {
      const n = totals[i]!, lvl = n === 0 ? 0 : Math.min(4, 1 + Math.floor((n / max) * 3.999));
      const date = dateOf(d.day);
      return <a role="listitem" class={`cal-cell r${lvl}`} href={href(d.day)} title={`${ymdLabel(d.day)} · ${number(n)} hits`}>
        <span class="cal-dow">{DOW[date.getDay()]!.slice(0, 2)}</span><span class="cal-day">{date.getDate()}</span>
      </a>;
    })}
  </div>;
}

/** Seven rows of 144 ten-minute cells, one row per day, each a link to it. */
function WeekGrid({ days, href }: { days: Day[]; href: (at: number) => string }) {
  const all = days.flatMap(d => d.slots.map(s => (s.in ?? 0) + (s.out ?? 0)));
  const max = Math.max(1, ...all);
  return <div class="week-grid">
    {days.map(d => {
      let cells = "";
      d.slots.forEach((s, i) => {
        const v = (Number(s.in) || 0) + (Number(s.out) || 0), bad = (Number(s.dropped) || 0) + (Number(s.expired) || 0) + (Number(s.refused) || 0);
        const lvl = v === 0 ? 0 : Math.min(4, 1 + Math.floor((v / max) * 3.999));
        cells += `<rect x="${i * 4}" y="0" width="3" height="14" rx="1" class="r${bad ? "x" : lvl}"><title>${escape(slotLabel(String(s.at)))} · ${v} moved</title></rect>`;
      });
      const date = dateOf(d.day);
      return <a class="week-row" href={href(d.day)}>
        <span class="week-day">{DOW[date.getDay()]} {date.getDate()}</span>
        <svg class="ribbon" viewBox="0 0 575 14" preserveAspectRatio="none" role="img" aria-label={`${ymdLabel(d.day)}: ${number(hits(d.slots))} hits`}>{raw(cells)}</svg>
        <span class="week-total">{number(hits(d.slots))}</span>
      </a>;
    })}
  </div>;
}

/** The range's chart: slots for a day, hours for a week, day bars for a month. */
export function RangeChart({ r, data, id, compact, dayHref }: { r: Range; data: RangeData; id: string; compact?: boolean; dayHref: (at: number) => string }): JSX.Element {
  const slots = data.slots;
  if (!slots.length) return <p class="muted">The daemon answered no activity.</p>;
  const zero = SERIES.filter(s => total(slots, s.key) === 0);
  if (zero.length === SERIES.length) return <p class="muted">All five series: <strong>0</strong> {r.live ? "in the last day" : "in this range"}.</p>;
  const shown = r.kind === "week" ? bucket(slots, 6) : r.kind === "month" ? bucket(slots, 144) : slots;
  return <>
    <DayChart slots={shown} id={id} compact={compact} bars={r.kind === "month"} />
    {r.kind === "week" && data.days.length ? <WeekGrid days={data.days} href={dayHref} /> : null}
    {r.kind === "month" && data.days.length ? <Calendar days={data.days} href={dayHref} /> : null}
    {zero.length ? <p class="muted small">Zero {r.live ? "all day" : "in this range"}: {zero.map(z => z.label).join(", ")}.</p> : null}
  </>;
}

/** Slot values: per slot for a day, per hour for a week, per day for a month. */
export function RangeTable({ r, data }: { r: Range; data: RangeData }) {
  if (!data.slots.length) return <></>;
  const rows = r.kind === "week" ? bucket(data.slots, 6) : r.kind === "month" ? bucket(data.slots, 144) : data.slots;
  const unit = r.kind === "week" ? "hour" : r.kind === "month" ? "day" : "ten-minute slot";
  return <details class="card slot-table">
    <summary>{r.kind === "month" ? "Day values" : r.kind === "week" ? "Hour values" : "Slot values"}</summary>
    <div class="table-scroll"><table class="data fit" aria-label={`Values, one row per ${unit}`}>
      <thead><tr><th>{r.kind === "month" ? "Day" : r.kind === "week" ? "Hour" : "Slot"}</th>{SERIES.map(s => <th class="num">{s.label}</th>)}</tr></thead>
      <tbody>{rows.map(s => <tr><td>{r.kind === "month" ? ymdLabel(ymdOfIso(s.at)) : slotLabel(s.at)}</td>{SERIES.map(x => <td class="num">{number(s[x.key])}</td>)}</tr>)}</tbody>
    </table></div>
  </details>;
}
