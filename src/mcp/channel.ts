// The plugin cache contains this small entry point; the launcher selects the
// source or bundled face and passes only its own session environment to it.
const entry = process.env.AGENT_BUS_MCP_ENTRY;
if (!entry) {
  console.error("agent-bus channel: start this session with ab-claude");
  process.exit(0);
}
if (/\.[cm]?[jt]s$/.test(entry)) {
  await import(entry);
} else {
  // A release's launcher names its compiled binary: run it as the MCP face on
  // this process's stdio, and end as it ends.
  const face = Bun.spawn([entry, "mcp"], { stdio: ["inherit", "inherit", "inherit"] });
  for (const signal of ["SIGINT", "SIGTERM", "SIGHUP"] as const) process.on(signal, () => face.kill(signal));
  process.exit(await face.exited);
}
export {};
