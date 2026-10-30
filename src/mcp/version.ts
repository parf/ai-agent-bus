import { readFileSync } from "node:fs";

// The same file Go embeds; no separately maintained MCP release number.
// A compiled release binary has the value built in; a checkout reads the file.
export const version = process.env.AGENT_BUS_BUILD_VERSION ?? readFileSync(new URL("../internal/version/VERSION", import.meta.url), "utf8").trim();
