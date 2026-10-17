/** Record labels: commas alone separate, spaces inside a label survive. */
export function normalizeTags(input: string | readonly string[]): string[] {
  const values = typeof input === "string" ? [input] : input;
  return [...new Set(values.flatMap(v => v.split(",")).map(v => v.trim().toLowerCase()).filter(v => v !== "no-tags").map(v => v.replace(/[\t\r\n\f\v]/g, " ").replace(/[^a-z_ ]/g, "").trim().replace(/ +/g, " ")).filter(Boolean))].sort();
}
export function recordTags(r: { tags?: string[]; personal?: boolean }): string[] {
  const tags = normalizeTags(r.tags ?? []);
  if (r.personal && !tags.includes("personal")) tags.push("personal");
  return tags.length ? tags.sort() : ["no-tags"];
}
export const filterTag = (v: string) => v.trim().toLowerCase() === "no-tags" ? "no-tags" : normalizeTags(v)[0] ?? "";
