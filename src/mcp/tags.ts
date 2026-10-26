// Shared vectors: internal/protocol/testdata/tags.json.
export class InvalidTag extends Error {}
export function normalizeTags(tags: readonly string[]): string[] {
  const result = new Set<string>();
  for (const raw of tags) for (const tag of raw.toLowerCase().split(",")) {
    if (!tag || tag === "no-tags") continue;
    if (!/^[a-z0-9_-]+$/.test(tag)) throw new InvalidTag(`invalid tag ${JSON.stringify(tag)}: use a-z, 0-9, - or _`);
    result.add(tag);
  }
  return [...result].sort();
}
