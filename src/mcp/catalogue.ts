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
  const external = r.kind === "service";
  const notes = [
    external ? `speaks ${r.protocol}${r.addr ? ` at ${r.addr}` : ""} — call it yourself, not through the bus` : undefined,
    external ? undefined : r.readers === undefined ? "readers: unavailable" : `readers: ${r.readers} outstanding`,
    external || !r.queued ? undefined : `${r.queued} queued`,
    r.last_used ? `last used ${ago(r.last_used, now)}` : undefined,
    r.config_sha ? `configured (${r.config_sha.slice(0, 12)})` : undefined,
  ].filter(Boolean);
  return `${r.name}  [${r.kind}]  ${r.descr ?? ""}`.trimEnd() + `\n    ${notes.join(" · ")}`;
}
