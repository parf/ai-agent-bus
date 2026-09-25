// Entity and authority marks. The canonical glyphs are src/internal/display
// (shared with the CLI); test/glyphs.test.ts fails when this table drifts from
// it. The web draws the same glyphs the CLI prints, so a kind looks the same
// everywhere (owner, 2026-09-24; Plans/R0.8/web/DECISIONS.md). An icon named
// "kind:<kind>" is one of these glyphs, not a Lucide drawing.

export type Kind = "user" | "agent" | "queue" | "pubsub" | "service" | "group";

export const ENTITY: Record<Kind, { glyph: string; word: string; icon: string }> = {
  user: { glyph: "👤", word: "User", icon: "kind:user" },
  agent: { glyph: "👾", word: "Agent", icon: "kind:agent" },
  queue: { glyph: "📮", word: "Queue", icon: "kind:queue" },
  pubsub: { glyph: "📣", word: "PubSub", icon: "kind:pubsub" },
  service: { glyph: "📡", word: "Service", icon: "kind:service" },
  group: { glyph: "👥", word: "Group", icon: "kind:group" },
};

export const DAEMON_OWNER = { glyph: "🔱", word: "Daemon owner", icon: "kind:owner" };
export const MAINTAINER = { glyph: "👮", word: "Maintainers", icon: "kind:maintainer" };

export function entity(kind: string): { glyph: string; word: string; icon: string } | undefined {
  return ENTITY[(kind === "person" ? "user" : kind) as Kind];
}

/** The label display.Entity gives: glyph and word, or the kind unchanged. */
export function entityLabel(kind: string): string {
  const e = entity(kind);
  return e ? `${e.glyph} ${e.word}` : kind;
}

export function identity(kind: string, daemonOwner: boolean) {
  return daemonOwner ? DAEMON_OWNER : entity(kind);
}

export function authority(daemonOwner?: boolean, administrator?: boolean): string {
  return daemonOwner ? `${DAEMON_OWNER.glyph} ${DAEMON_OWNER.word}` : administrator ? "Daemon administrator" : "User";
}

/** The glyph behind a "kind:" icon name, or undefined for a Lucide one. */
export function kindGlyph(icon: string): { glyph: string; word: string } | undefined {
  if (!icon.startsWith("kind:")) return undefined;
  const k = icon.slice(5);
  if (k === "owner") return DAEMON_OWNER;
  if (k === "maintainer") return { glyph: MAINTAINER.glyph, word: "Maintainer" };
  return entity(k);
}
