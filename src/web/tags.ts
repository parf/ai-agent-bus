import { normalizeTags as normalized } from "../mcp/tags.ts";
/** Comma-separated labels; invalid characters are refused, never rewritten. */
export function normalizeTags(input: string | readonly string[]): string[] {
  return normalized(typeof input === "string" ? [input] : input);
}
export function isPersonal(r: { tags?: readonly string[] } | undefined): boolean {
  if (r && Object.hasOwn(r, "personal")) throw new Error("personal is not a record field; use tags");
  return !!r?.tags?.includes("personal");
}
export function recordTags(r: { tags?: readonly string[] }): string[] {
  isPersonal(r); // Refuse retired input rather than reconstructing a classification.
  const tags = normalizeTags(r.tags ?? []);
  return tags.length ? tags : ["no-tags"];
}
export const filterTag = (v: string) => v.toLowerCase() === "no-tags" ? "no-tags" : normalizeTags(v)[0] ?? "";
