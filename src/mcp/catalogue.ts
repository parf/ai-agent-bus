import type { Record_ } from "./bus.ts";

// What a caller needs to decide whether to call a record. Missing reader data
// is not a measured zero: it can be an older daemon answering a newer face.
export function catalogue(r: Record_): string {
  const notes = [
    r.protocol ? `speaks ${r.protocol}${r.addr ? ` at ${r.addr}` : ""} — call it yourself, not through the bus` : undefined,
    r.readers === undefined ? "readers: unavailable" : `readers: ${r.readers} outstanding`,
    r.queued ? `${r.queued} queued` : undefined,
    r.config_sha ? `configured (${r.config_sha.slice(0, 12)})` : undefined,
  ].filter(Boolean);
  return `${r.name}  [${r.kind}]  ${r.descr ?? ""}`.trimEnd() + `\n    ${notes.join(" · ")}`;
}
