// Shared vectors: internal/protocol/testdata/tags.json.
export class InvalidTag extends Error {}
export function normalizeTags(tags: readonly string[]): string[] {
  const result = new Set<string>();
  for (const raw of tags) for (const tag of raw.toLowerCase().split(",")) {
    if (!tag) continue;
    if (tag === "no-tags") throw new InvalidTag("no-tags is a listing filter, not a writable tag");
    if (!/^[a-z0-9_-]+$/.test(tag)) throw new InvalidTag(`invalid tag ${JSON.stringify(tag)}: use a-z, 0-9, - or _`);
    result.add(tag);
  }
  return [...result].sort();
}
