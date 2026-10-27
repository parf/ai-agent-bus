// Pure shared dispatcher: safe to bundle in launchers without loading package metadata.
export const asks = ["help", "help-config", "help-secret", "help-json"] as const;
export type HelpAsk = typeof asks[number];
/** No ask alone permits normal work; every unknown ask is a normal reply. */
export function dispatchAsk(ask: string | undefined, help: (ask: HelpAsk) => string, handlers: Record<string, () => string> = {}): string | undefined {
  if (!ask) return undefined;
  if (Object.hasOwn(handlers, ask)) return handlers[ask]!();
  if ((asks as readonly string[]).includes(ask)) return help(ask as HelpAsk);
  if (ask === "mcp") return JSON.stringify({ tools: [] });
  return `unsupported: ${ask}`;
}
