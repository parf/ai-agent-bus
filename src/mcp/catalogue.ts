import type { Record_ } from "./bus.ts";

// What a caller needs to decide whether to call a record. A service is
// external and has no queue here, so it reports where it is and nothing about
// readers or backlog; for the rest, missing reader data is not a measured
// zero, because it can be an older daemon answering a newer face.
// See docs/03-records.md#record-kinds.
// How long ago, coarse on purpose: a listing is read to tell a session that
// is in use from one left behind, not to audit it.
export function ago(at: string, now = Date.now()): string {
  const s = Math.max(0, Math.round((now - Date.parse(at)) / 1000));
  if (s < 60) return "just now";
  const m = Math.round(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.round(m / 60);
  if (h < 48) return `${h}h ago`;
  return `${Math.round(h / 24)}d ago`;
}

export function catalogue(r: Record_, now = Date.now()): string {
  // A 📚 card has no queue either: it says what it is and who answers it.
  const external = r.kind === "service" || r.kind === "resource";
  const card = r.resource;
  const notes = [
    card ? `${card.template ? "template " : ""}${card.uri}${card.mimeType ? ` (${card.mimeType})` : ""} — read with resources/read${card.source ? `, answered by ${card.source}` : ""}` : undefined,
    r.kind === "service" ? `speaks ${r.protocol}${r.addr ? ` at ${r.addr}` : ""} — call it yourself, not through the bus` : undefined,
    external ? undefined : r.readers === undefined ? "readers: unavailable" : `readers: ${r.readers} outstanding`,
    external || !r.queued ? undefined : `${r.queued} queued`,
    r.last_used ? `last used ${ago(r.last_used, now)}` : undefined,
    r.config_sha ? `configured (${r.config_sha.slice(0, 12)})` : undefined,
  ].filter(Boolean);
  return `${r.name}  [${r.kind}]  ${r.descr ?? ""}`.trimEnd() + `\n    ${notes.join(" · ")}`;
}

// What ab_ls asks the daemon for and keeps. With neither a kind nor `all`, it
// answers the everyday question — which agents can take a message right now —
// so only agents with a reader are listed (docs/05-discovery.md#cli-listing).
export function listing(kind: string | undefined, all: boolean): { kind?: string; live: boolean } {
  return kind || all ? { kind, live: false } : { kind: "agent", live: true };
}

export function beingRead(records: Record_[]): Record_[] {
  return records.filter((r) => (r.readers ?? 0) > 0);
}
