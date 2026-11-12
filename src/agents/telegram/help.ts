// One authored description for runner help and bare/self-serving help.
import { readFileSync } from "node:fs";
import manifest from "./agent.json";
import { fileURLToPath } from "node:url";
import { helpAnswer, imageHandler, type HelpAsk } from "../shared/metadata.ts";
const envDist = readFileSync(new URL("./env.dist", import.meta.url), "utf8");
// The package folder: beside the bundle, as env.dist is.
const dir = fileURLToPath(new URL(".", import.meta.url));
export const describe = (any: boolean, ask: HelpAsk = "help", json = false) => helpAnswer(manifest, ask, envDist, any ? ["owner"] : [], json, process.env.AGENT_BUS_NAME || "", dir);
export const imageAsk = imageHandler(manifest, dir);
export const help = describe(true);
