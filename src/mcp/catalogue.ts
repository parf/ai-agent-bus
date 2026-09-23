import type { Record_ } from "./bus.ts";

// What a caller needs to decide whether to call a record. A service is
// external and has no queue here, so it reports where it is and nothing about
// readers or backlog; for the rest, missing reader data is not a measured
// zero, because it can be an older daemon answering a newer face.
// See docs/03-records.md#record-kinds.
export function catalogue(r: Record_): string {
  const external = r.kind === "service";
  const notes = [
    external ? `speaks ${r.protocol}${r.addr ? ` at ${r.addr}` : ""} — call it yourself, not through the bus` : undefined,
    external ? undefined : r.readers === undefined ? "readers: unavailable" : `readers: ${r.readers} outstanding`,
    external || !r.queued ? undefined : `${r.queued} queued`,
    r.config_sha ? `configured (${r.config_sha.slice(0, 12)})` : undefined,
  ].filter(Boolean);
  return `${r.name}  [${r.kind}]  ${r.descr ?? ""}`.trimEnd() + `\n    ${notes.join(" · ")}`;
}
