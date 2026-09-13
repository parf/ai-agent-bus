import { readFileSync } from "node:fs";

// The same file Go embeds; no separately maintained MCP release number.
export const version = readFileSync(new URL("../internal/version/VERSION", import.meta.url), "utf8").trim();
