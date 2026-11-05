import { readFileSync } from "node:fs";
// The agent pack's own SemVer, not the bus version; a bundle has it written in.
export const VERSION = process.env.AGENT_BUS_BUILD_AGENTS_VERSION ?? readFileSync(new URL("../VERSION", import.meta.url), "utf8").trim();
