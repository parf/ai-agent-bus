import { normalizeTags as normalized } from "../mcp/tags.ts";
/** Comma-separated labels; invalid characters are refused, never rewritten. */
export function normalizeTags(input: string | readonly string[]): string[] {
  return normalized(typeof input === "string" ? [input] : input);
}
export function recordTags(r: { tags?: string[]; personal?: boolean }): string[] {
  const tags = normalizeTags(r.tags ?? []);
  if (r.personal && !tags.includes("personal")) tags.push("personal");
  return tags.length ? tags.sort() : ["no-tags"];
}
export const filterTag = (v: string) => v.toLowerCase() === "no-tags" ? "no-tags" : normalizeTags(v)[0] ?? "";
