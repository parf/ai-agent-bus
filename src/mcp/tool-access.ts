// Which of the face's tools touch the local filesystem, and so are never
// pre-approved. The runtime's own prompt is the approval for these: it shows
// the path the model chose. Every other tool is pre-approved by name.
// tool-access.test.ts finds file-touching tools by their schema, so a new
// tool that takes a path fails there until it is listed here.
export const FILE_TOOLS: Readonly<Record<string, "read" | "write">> = {
  ab_bigfile_put: "read",
  ab_bigfile_get: "write",
};

/** Claude Code's name for a tool of the agent-bus plugin's MCP server. */
export const CLAUDE_TOOL_PREFIX = "mcp__plugin_agent-bus_agent-bus__";

/** The `--allowedTools` list: every tool by name except the file-touching ones. */
export function allowedTools(names: readonly string[]): string[] {
  return names.filter((n) => !(n in FILE_TOOLS)).map((n) => CLAUDE_TOOL_PREFIX + n);
}

/** The Codex server config's `tools` table: each file-touching tool asks. */
export function codexPrompted(): string {
  return `tools={${Object.keys(FILE_TOOLS).map((n) => `${n}={approval_mode="prompt"}`).join(",")}}`;
}
