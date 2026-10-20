import { normalizeTags } from "../mcp/tags.ts";

// Undefined means preserve the record; an explicit empty list clears custom tags.
export function tagOption(args: string[]): { args: string[]; tags?: string[] } {
  const forwarded: string[] = [], values: string[] = [];
  let seen = false, clear = false;
  for (let i = 0; i < args.length; i++) {
    const arg = args[i]!;
    if (arg === "--") { forwarded.push(...args.slice(i)); break; }
    if (arg !== "--tag" && !arg.startsWith("--tag=")) { forwarded.push(arg); continue; }
    const value = arg === "--tag" ? args[++i] : arg.slice("--tag=".length);
    if (value === undefined || (arg === "--tag" && value.startsWith("--"))) throw new Error("--tag needs a tag or comma list");
    if ((value === "" && seen) || clear) throw new Error("--tag '' cannot be combined with other --tag options");
    seen = true;
    if (value === "") { clear = true; continue; }
    for (const word of value.split(",")) {
      const tags = normalizeTags([word]);
      if (!tags.length) throw new Error(`invalid tag ${JSON.stringify(word)}: use letters, underscores or spaces; no-tags is reserved`);
      values.push(...tags);
    }
  }
  return { args: forwarded, tags: seen ? normalizeTags(values) : undefined };
}
