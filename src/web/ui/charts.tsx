// Charts. The day chart is drawn in the browser by uPlot from the slots this
// markup carries; the ribbon and sparklines are small SVG drawn here, coloured
// by class so no inline style is ever written.
import { h, Fragment, raw, escape } from "../jsx.ts";
import { number, slotLabel } from "../format.ts";

export type Slot = { at: string; in: number; out: number; dropped: number; expired: number; refused: number };

export const SERIES: { key: keyof Omit<Slot, "at">; label: string }[] = [
  { key: "in", label: "Accepted" },
  { key: "out", label: "Dequeued" },
  { key: "dropped", label: "Dropped" },
  { key: "expired", label: "Expired" },
  { key: "refused", label: "Refused" },
];

export const total = (slots: Slot[], k: keyof Omit<Slot, "at">) => slots.reduce((a, s) => a + (s[k] ?? 0), 0);

/** 144 cells, one per ten minutes; intensity by the slot's traffic against the day's peak. */
export function Ribbon({ slots, label }: { slots: Slot[]; label: string }) {
  const v = slots.map(s => (Number(s.in) || 0) + (Number(s.out) || 0));
  const bad = slots.map(s => (Number(s.dropped) || 0) + (Number(s.expired) || 0) + (Number(s.refused) || 0));
  const max = Math.max(1, ...v);
  const w = 3, gap = 1, n = slots.length || 144;
  let cells = "";
  slots.forEach((s, i) => {
    const lvl = v[i] === 0 ? 0 : Math.min(4, 1 + Math.floor((v[i]! / max) * 3.999));
    cells += `<rect x="${i * (w + gap)}" y="0" width="${w}" height="14" rx="1" class="r${bad[i] ? "x" : lvl}"><title>${escape(slotLabel(String(s.at)))} · ${Number(v[i])} moved${bad[i] ? ` · ${Number(bad[i])} lost or refused` : ""}</title></rect>`;
  });
  return <svg class="ribbon" viewBox={`0 0 ${n * (w + gap) - gap} 14`} preserveAspectRatio="none" role="img" aria-label={label}>{raw(cells)}</svg>;
}

/** A small trend line of one series. */
export function Spark({ values, label }: { values: number[]; label: string }) {
  if (!values.length || values.every(x => x === 0)) return <svg class="spark flat" viewBox="0 0 100 24" aria-hidden="true">{raw(`<path d="M0 22 H100" />`)}</svg>;
  const max = Math.max(...values);
  const pts = values.map((x, i) => `${((i / Math.max(1, values.length - 1)) * 100).toFixed(1)},${(22 - (x / max) * 20).toFixed(1)}`);
  return <svg class="spark" viewBox="0 0 100 24" preserveAspectRatio="none" role="img" aria-label={label}>{raw(`<path class="area" d="M0,24 L${pts.join(" L")} L100,24 Z"/><polyline points="${pts.join(" ")}"/>`)}</svg>;
}

/** The day chart: uPlot draws it from data-slots; the table below holds every value. */
export function DayChart({ slots, id, compact, bars }: { slots: Slot[]; id: string; compact?: boolean; bars?: boolean }) {
  const live = SERIES.filter(s => total(slots, s.key) > 0);
  const payload = JSON.stringify({ bars: !!bars, t: slots.map(s => Math.floor(new Date(s.at).getTime() / 1000)), series: live.map(s => ({ key: s.key, label: s.label, values: slots.map(x => x[s.key] ?? 0) })) });
  const max = Math.max(0, ...live.flatMap(s => slots.map(x => x[s.key] ?? 0)));
  const start = slots[0]?.at, end = slots.at(-1)?.at;
  return <figure class={`daychart ${compact ? "compact" : ""}`}>
    <div class="uplot-host" id={id} data-chart={payload} role="img"
      aria-label={`Activity${start ? ` from ${slotLabel(start)} to ${slotLabel(end!)}` : ""}; shared maximum ${max} over the displayed nonzero series`}></div>
    <figcaption>
      <ul class="legend">{live.map(s => <li class={`lg-${s.key}`}><span class="swatch" aria-hidden="true"></span>{s.label}: <strong>{number(total(slots, s.key))}</strong></li>)}</ul>
    </figcaption>
  </figure>;
}
