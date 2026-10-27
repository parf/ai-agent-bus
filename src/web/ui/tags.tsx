import { h } from "../jsx.ts";
import { recordTags } from "../tags.ts";

/** The synthetic empty-tag filter is never a record label. */
export function TagBadges({ rec, href = tag => `/agents?tag=${encodeURIComponent(tag)}` }: {
  rec: { tags?: string[]; personal?: boolean }; href?: (tag: string) => string;
}) {
  const tags = recordTags(rec).filter(tag => tag !== "no-tags");
  return tags.length ? <span class="record-tags" aria-label="Tags">{tags.map(tag =>
    <a class={`record-tag${tag === "personal" ? " tag-personal" : ""}`} href={href(tag)}>{tag}</a>
  )}</span> : null;
}
