// Formatting only: numbers, times and durations as the pages show them.

export const number = (n: number | undefined) => (n ?? 0).toLocaleString("en-US");

const MON = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
const pad = (n: number) => String(n).padStart(2, "0");

export function validTime(iso?: string): Date | undefined {
  if (!iso || iso.startsWith("0001-")) return undefined;
  const d = new Date(iso);
  return isNaN(+d) ? undefined : d;
}

/** now · 5m ago · 3h ago · 12d ago · Jan 2 · Jan 2, 2006 */
export function relative(iso?: string, now = new Date()): string {
  const d = validTime(iso);
  if (!d) return "—";
  const s = Math.max(0, (now.getTime() - d.getTime()) / 1000);
  if (s < 60) return "now";
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  if (s < 30 * 86400) return `${Math.floor(s / 86400)}d ago`;
  return d.getFullYear() === now.getFullYear() ? `${MON[d.getMonth()]} ${d.getDate()}` : `${MON[d.getMonth()]} ${d.getDate()}, ${d.getFullYear()}`;
}

/** 2006-01-02 15:04:05, in the offset the daemon wrote. */
export function stamp(iso?: string): string {
  if (!iso || iso.startsWith("0001-")) return "—";
  const m = /^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2}:\d{2})/.exec(iso);
  return m ? `${m[1]} ${m[2]}` : iso;
}

/** 2006-01-02 15:04 */
export function minute(iso?: string): string {
  const s = stamp(iso);
  return s === "—" ? s : s.slice(0, 16);
}

/** Jan 2 15:04, the node's clock as the daemon wrote it. */
export function slotLabel(iso: string): string {
  const m = /^\d{4}-(\d{2})-(\d{2})T(\d{2}):(\d{2})/.exec(iso);
  return m ? `${MON[Number(m[1]) - 1]} ${Number(m[2])} ${m[3]}:${m[4]}` : iso;
}

/** Local wall-clock of the web process, as the footer states it. */
export function generated(d: Date): string {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

/** Seconds in a Go duration string such as 1h2m3.5s; unparseable is 0. */
export function duration(s?: string): number {
  if (!s) return 0;
  const units: Record<string, number> = { h: 3600, m: 60, s: 1, ms: 1e-3, "µs": 1e-6, us: 1e-6, ns: 1e-9 };
  let total = 0, any = false;
  for (const [, v, u] of s.matchAll(/([\d.]+)(h|ms|m|s|µs|us|ns)/g)) { total += Number(v) * units[u!]!; any = true; }
  return any ? total : 0;
}

export const plural = (n: number, one: string, many = one + "s") => `${number(n)} ${n === 1 ? one : many}`;
