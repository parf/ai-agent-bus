// Keep this grammar aligned with protocol.NormalizeTags through its shared vectors.
const whitespace = "\\t\\n\\v\\f\\r \\u0085\\u00a0\\u1680\\u2000-\\u200a\\u2028\\u2029\\u202f\\u205f\\u3000";
const trim = (value: string) => value.replace(new RegExp(`^[${whitespace}]+|[${whitespace}]+$`, "g"), "");
export function normalizeTags(tags: string[]): string[] {
  const result = new Set<string>();
  for (const raw of tags) for (const part of raw.toLowerCase().split(",")) {
    const word = trim(part);
    if (word === "no-tags") continue;
    const tag = word.replace(/[\t\r\n\f\v]/g, " ").replace(/[^a-z_ ]/g, "").trim().replace(/ +/g, " ");
    if (tag) result.add(tag);
  }
  return [...result].sort();
}
