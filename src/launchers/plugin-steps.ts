// Which `claude plugin` commands bring Claude's copy of the agent-bus channel
// plugin to this launcher's own tree. Claude keeps its own copy: a release's
// directory changes with every upgrade and rollback, so a release replaces a
// plugin another release installed (an older one's bun entry, or a newer one's
// agent-bus-mcp); a checkout keeps whatever plugin is installed, as before.
export type PluginState = { compiled: boolean; source: string; knownPath?: string; installed: boolean };

export function pluginSteps({ compiled, source, knownPath, installed }: PluginState): string[][] {
  const known = knownPath !== undefined;
  const stale = compiled && known && knownPath !== source;
  if (installed && !stale) return [];
  const steps: string[][] = [];
  if (stale) {
    if (installed) steps.push(["uninstall", "agent-bus@agent-bus-local"]);
    steps.push(["marketplace", "remove", "agent-bus-local"]);
  }
  if (!known || stale) steps.push(["marketplace", "add", source]);
  steps.push(["install", "agent-bus@agent-bus-local", "--scope", "user"]);
  return steps;
}
