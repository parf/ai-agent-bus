// One authored description for runner help and bare/self-serving help.
import { readFileSync } from "node:fs";
import manifest from "./agent.json";
import { helpAnswer, type HelpAsk } from "../shared/metadata.ts";
const envDist = readFileSync(new URL("./env.dist", import.meta.url), "utf8");
export const describe = (any: boolean, ask: HelpAsk = "help", json = false) => helpAnswer(manifest, ask, envDist, any ? ["owner"] : [], json);
export const help = describe(true);
