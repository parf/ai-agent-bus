// Entity and authority marks. The canonical glyphs are src/internal/display
// (shared with the CLI); test/glyphs.test.ts fails when this table drifts from
// it. The web draws a Lucide icon for each and keeps the glyph's meaning: the
// label is the same words the CLI prints (Plans/Web/DECISIONS.md, Q121).

export type Kind = "user" | "agent" | "queue" | "pubsub" | "service" | "group";

export const ENTITY: Record<Kind, { glyph: string; word: string; icon: string }> = {
  user: { glyph: "👤", word: "User", icon: "user-round" },
  agent: { glyph: "👾", word: "Agent", icon: "bot" },
  queue: { glyph: "📮", word: "Queue", icon: "inbox" },
  pubsub: { glyph: "📣", word: "PubSub", icon: "megaphone" },
  service: { glyph: "📡", word: "Service", icon: "satellite-dish" },
  group: { glyph: "👥", word: "Group", icon: "users" },
};

export const DAEMON_OWNER = { glyph: "🔱", word: "Daemon owner", icon: "crown" };
export const MAINTAINER = { glyph: "👮", word: "Maintainers", icon: "shield-check" };

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
