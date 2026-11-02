import { expect, test } from "bun:test";
import { EventsCard, type EventEntry, type EventLog } from "../agent-events.tsx";
import { render } from "../jsx.ts";

const entry = (id: number, level: EventEntry["level"], text = `entry-${id}`): EventEntry => ({ id, level, text, at: "2026-11-02T10:00:00.000Z", last_at: "2026-11-02T10:00:00.000Z", count: 1 });
const log: EventLog = { log: "#one@fixture", rings: { alert: [entry(3, "alert"), entry(1, "alert")], warning: [entry(2, "warning")], notice: [entry(5, "notice"), entry(4, "notice")] }, counts: { alert: 2, warning: 1, notice: 2 } };
const ids = (page: string) => [...page.matchAll(/data-event-id="(\d+)"/g)].map(m => Number(m[1]));

test("preview shows exactly the three greatest ids across levels, not timestamps or severity order", () => {
  const page = render(EventsCard({ events: { log }, name: log.log }));
  expect(ids(page)).toEqual([5, 4, 3]);
  expect(page).toContain("Showing 3 of 5 retained entries.");
  expect(page).toContain('<span class="pill tone-bad">alert</span>');
  expect(page).toContain('events=all#events');
});
test("expand shows every retained entry in severity order and counts entries rather than folded occurrences", () => {
  const folded = structuredClone(log); folded.rings.warning[0]!.count = 8;
  const page = render(EventsCard({ events: { log: folded }, name: log.log, expanded: true }));
  expect(ids(page)).toEqual([3, 1, 2, 5, 4]);
  expect(page).toContain("Showing 5 of 5 retained entries.");
  expect(page).toContain("8 occurrences");
  expect(page).toContain('<span class="pill tone-warn">warning</span>');
  expect(page).not.toContain("events=all");
});
test("refused reads have a not-allowed element, never an empty-log claim", () => {
  const page = render(EventsCard({ events: { problem: "not allowed" }, name: log.log }));
  expect(page).toContain('<p data-events-not-allowed>Not allowed');
  expect(page).not.toContain("No retained events"); expect(ids(page)).toEqual([]);
});
test("event text is escaped and notices keep the plain severity label", () => {
  const escaped = structuredClone(log); escaped.rings.notice[0]!.text = '<script>foreign</script>';
  const page = render(EventsCard({ events: { log: escaped }, name: log.log }));
  expect(page).toContain("&lt;script&gt;foreign&lt;/script&gt;"); expect(page).not.toContain("<script>");
  expect(page).toContain('<span class="pill ">notice</span>');
});

import { agentEvents, eventSummaries, EventMark } from "../agent-events.tsx";
import { Refusal, type Ctx } from "../ctx.ts";
const fake = (answer: unknown) => ({ call: async (to: string, body: string, wait: number) => {
  expect(to).toBe("#bus"); expect(body === `read ${log.log}` || body === "list").toBe(true);
  expect(wait).toBeLessThanOrEqual(3000);
  return { kind: "answer", body: JSON.stringify(answer) };
} }) as unknown as Ctx;

test("read scopes its answer to exactly the requested agent; a different log is never rendered", async () => {
  expect(await agentEvents(fake({ ...log, log: "#other@fixture" }), log.log)).toEqual({ problem: "unavailable" });
  const correct = await agentEvents(fake(log), log.log);
  expect("log" in correct && correct.log.log).toBe(log.log);
});
test("a full empty read is empty; refused and failed reads never masquerade as empty", async () => {
  const empty = { ...log, rings: { alert: [], warning: [], notice: [] }, counts: { alert: 0, warning: 0, notice: 0 } };
  expect(render(EventsCard({ events: await agentEvents(fake(empty), log.log), name: log.log }))).toContain('<p data-events-empty>No retained events.</p>');
  expect(await agentEvents(fake({ error: { code: "forbidden", text: "private" } }), log.log)).toEqual({ problem: "not allowed" });
  expect(await agentEvents(fake({ error: { code: "internal_error" } }), log.log)).toEqual({ problem: "unavailable" });
  for (const kind of ["timeout", "lost", "done"]) {
    const ctx = { call: async () => ({ kind }) } as unknown as Ctx;
    expect(await agentEvents(ctx, log.log)).toEqual({ problem: "unavailable" });
  }
  const ctx = { call: async () => { throw new Refusal(403, "acl"); } } as unknown as Ctx;
  expect(await agentEvents(ctx, log.log)).toEqual({ problem: "not allowed" });
});
test("incomplete, invalid, and oversized retained-log replies cannot claim to be full", async () => {
  for (const answer of [
    { ...log, counts: { ...log.counts, notice: 3 } },
    { ...log, rings: { ...log.rings, notice: null } },
    { ...log, rings: { ...log.rings, notice: [entry(3, "notice")] }, counts: { ...log.counts, notice: 1 } },
    { ...log, rings: { alert: [], warning: [], notice: Array.from({ length: 501 }, (_, i) => entry(i + 1, "notice")) }, counts: { alert: 0, warning: 0, notice: 501 } },
  ]) expect(await agentEvents(fake(answer), log.log)).toEqual({ problem: "unavailable" });
});
test("list uses only summary rows, with recent alerts relative to the daemon clock and exact warning counts", async () => {
  const now = "2026-11-02T10:00:00Z";
  const summaries = await eventSummaries(fake({ now, logs: [
    { log: log.log, counts: log.counts, latest_alert_at: now },
    { log: "#old@fixture", counts: log.counts, latest_alert_at: "2026-10-30T10:00:00Z" },
    { log: "#future@fixture", counts: log.counts, latest_alert_at: "2026-11-03T10:00:00Z" },
  ] }));
  const mark = render(EventMark({ summary: summaries?.get(log.log) }));
  expect(mark).toContain('tone-bad">alert in last day'); expect(mark).toContain('tone-warn">1 retained warning');
  expect(render(EventMark({ summary: summaries?.get("#old@fixture") }))).not.toContain("alert in last day");
  expect(render(EventMark({ summary: summaries?.get("#future@fixture") }))).not.toContain("alert in last day");
  expect(render(EventMark({}))).toBe("");
  expect(await eventSummaries(fake({ error: { code: "internal_error" } }))).toBeUndefined();
});


test("not_found is a distinct no-events-yet state, not an empty success or an outage", async () => {
  const events = await agentEvents(fake({ error: { code: "not_found" } }), log.log);
  expect(events).toEqual({ problem: "not found" });
  const page = render(EventsCard({ events, name: log.log }));
  expect(page).toContain('<p data-events-not-found>No events yet for this agent.</p>');
  expect(page).not.toContain('data-events-empty'); expect(page).not.toContain('data-events-unavailable');
});

test("expand renders the complete 500-entry bound without a hidden page or cut", async () => {
  const full = { ...log, rings: { alert: [], warning: [], notice: Array.from({ length: 500 }, (_, i) => entry(500 - i, "notice")) }, counts: { alert: 0, warning: 0, notice: 500 } };
  const page = render(EventsCard({ events: await agentEvents(fake(full), log.log), name: log.log, expanded: true }));
  expect(ids(page)).toHaveLength(500);
  expect(ids(page)[0]).toBe(500); expect(ids(page).at(-1)).toBe(1);
  expect(page).toContain("Showing 500 of 500 retained entries.");
});
