// The plugin cache contains this small entry point; the launcher selects the
// source or bundled face and passes only its own session environment to it.
const entry = process.env.AGENT_BUS_MCP_ENTRY;
if (!entry) {
  console.error("agent-bus channel: start this session with ab-claude");
  process.exit(0);
}
await import(entry);
export {};
