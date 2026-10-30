// One self-contained binary for every TypeScript part of a release archive
// (bun build --compile): the part is chosen by the name it is run under, or by
// the first argument. Built only for release archives; a checkout runs the
// sources with bun.
import { basename } from "node:path";

const parts: Record<string, () => Promise<unknown>> = {
  "agent-bus-mcp": () => import("./mcp/server.ts"),
  "agent-bus-web": async () => (await import("./web/server.ts")).main(),
  "agent-bus-launcher": () => import("./launchers/launcher.ts"),
};

// A compiled binary's argv[1] is its own embedded path; argv0 is the name run.
let part = basename(process.argv0 ?? "");
if (!(part in parts)) part = basename(process.execPath);
if (!(part in parts) && process.argv[2] && `agent-bus-${process.argv[2]}` in parts) {
  part = `agent-bus-${process.argv[2]}`;
  process.argv.splice(2, 1);
}
// The Claude plugin's channel entry: as mcp/channel.ts, it serves only a
// session ab-claude started, and is silent otherwise.
if (part === "agent-bus-mcp" && process.argv[2] === "channel") {
  process.argv.splice(2, 1);
  if (!process.env.AGENT_BUS_MCP_ENTRY) {
    console.error("agent-bus channel: start this session with ab-claude");
    process.exit(0);
  }
}
if (!(part in parts)) {
  console.error(`agent-bus-ts: run as one of ${Object.keys(parts).join(", ")}`);
  process.exit(2);
}
await parts[part]();
