// Private per-agent events, read as the current visitor through #bus.
import { h } from "./jsx.ts";
import { Refusal, type Ctx } from "./ctx.ts";
import { Card, Pill } from "./ui/kit.tsx";

export const eventLevels = ["alert", "warning", "notice"] as const;
type Level = typeof eventLevels[number];
export type EventEntry = { id: number; level: Level; text: string; at: string; last_at: string; count: number };
export type EventLog = { log: string; rings: Record<Level, EventEntry[]>; counts: Record<Level, number> };
export type Events = { log: EventLog } | { problem: "not allowed" | "unavailable" };

const MAX_ENTRIES = 500;
const positive = (n: unknown): n is number => Number.isSafeInteger(n) && (n as number) > 0;
const count = (n: unknown): n is number => Number.isSafeInteger(n) && (n as number) >= 0;
const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v === "object" && !Array.isArray(v);

/** Reject incomplete or mismatched replies rather than call a partial log full. */
function readLog(value: unknown, name: string): EventLog | undefined {
  if (!object(value) || value.log !== name || !object(value.rings) || !object(value.counts)) return;
  const rings = {} as EventLog["rings"], counts = {} as EventLog["counts"];
  const ids = new Set<number>();
  for (const level of eventLevels) {
    const entries = value.rings[level], n = value.counts[level];
    if (!Array.isArray(entries) || !count(n) || entries.length !== n) return;
    for (const e of entries) {
      if (!object(e) || !positive(e.id) || ids.has(e.id) || e.level !== level || typeof e.text !== "string" ||
          typeof e.at !== "string" || typeof e.last_at !== "string" || !positive(e.count)) return;
      ids.add(e.id);
    }
    rings[level] = entries as EventEntry[];
    counts[level] = n;
  }
  if (ids.size > MAX_ENTRIES) return;
  return { log: name, rings, counts };
}

async function eventCall(ctx: Ctx, body: string): Promise<unknown> {
  const answer = await ctx.call("#bus", body, 3000);
  if (answer.kind !== "answer" || new TextEncoder().encode(answer.body).length > 1024 * 1024) return;
  try { return JSON.parse(answer.body); } catch { return; }
}

export async function agentEvents(ctx: Ctx, name: string): Promise<Events> {
  try {
    const value = await eventCall(ctx, `read ${name}`);
    if (object(value) && object(value.error) && value.error.code === "forbidden") return { problem: "not allowed" };
    const log = readLog(value, name);
    return log ? { log } : { problem: "unavailable" };
  } catch (e) {
    return { problem: e instanceof Refusal && e.status === 403 ? "not allowed" : "unavailable" };
  }
}

export type EventSummary = { counts: Record<Level, number>; recentAlert: boolean };
/** One summary-only call per listing, as the visitor; never a registry-wide read. */
export async function eventSummaries(ctx: Ctx): Promise<Map<string, EventSummary> | undefined> {
  try {
    const value = await eventCall(ctx, "list");
    if (!object(value) || !Array.isArray(value.logs) || typeof value.now !== "string") return;
    const now = Date.parse(value.now);
    if (!Number.isFinite(now)) return;
    const result = new Map<string, EventSummary>();
    for (const row of value.logs) {
      if (!object(row) || typeof row.log !== "string" || !object(row.counts) || result.has(row.log)) return;
      const counts = {} as Record<Level, number>;
      for (const level of eventLevels) {
        if (!count(row.counts[level])) return;
        counts[level] = row.counts[level];
      }
      const latest = typeof row.latest_alert_at === "string" ? Date.parse(row.latest_alert_at) : NaN;
      result.set(row.log, { counts, recentAlert: counts.alert > 0 && Number.isFinite(latest) && latest <= now && latest >= now - 86400_000 });
    }
    return result;
  } catch { return; }
}

export function EventMark({ summary }: { summary?: EventSummary }) {
  return summary ? <span data-event-summary>
    {summary.recentAlert ? <Pill tone="bad">alert in last day</Pill> : null}
    {summary.counts.warning ? <Pill tone="warn">{summary.counts.warning} retained warnings</Pill> : null}
  </span> : null;
}

export function EventLines({ entries }: { entries: EventEntry[] }) {
  return <ul class="alerts" data-event-lines>{entries.map(e => <li data-event-id={e.id} data-tone={e.level === "alert" ? "bad" : e.level === "warning" ? "warn" : undefined}>
    <Pill tone={e.level === "alert" ? "bad" : e.level === "warning" ? "warn" : undefined}>{e.level}</Pill>{" "}
    <span>{e.text}</span>{" "}<time datetime={e.last_at}>{e.last_at}</time>
    {e.count > 1 ? <span class="muted"> · {e.count} occurrences</span> : null}
  </li>)}</ul>;
}

export function EventsCard({ events, name, expanded = false }: { events: Events; name: string; expanded?: boolean }) {
  if ("problem" in events) return <Card title="Events" icon="list" id="events">
    {events.problem === "not allowed" ? <p data-events-not-allowed>Not allowed to read this agent’s events.</p> : <p class="warn" data-events-unavailable>Events unavailable. Try again.</p>}
  </Card>;
  const all = eventLevels.flatMap(level => events.log.rings[level]);
  const entries = expanded ? all : [...all].sort((a, b) => b.id - a.id).slice(0, 3);
  const q = new URLSearchParams({ name, ...(expanded ? {} : { events: "all" }) });
  return <Card title="Events" icon="list" id="events" actions={<a href={`/agent?${q}#events`}>{expanded ? "Show recent entries" : "Expand log"}</a>}>
    <p data-event-count>Showing {entries.length} of {all.length} retained entries.</p>
    {expanded ? <p class="muted small">Full retained log (up to 500 entries); older expired or evicted entries are not retained.</p> : null}
    {all.length ? <EventLines entries={entries} /> : <p data-events-empty>No retained events.</p>}
  </Card>;
}
