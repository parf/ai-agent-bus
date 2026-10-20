// The manifest is the only method/settings description, including bare runs.
import { readFileSync } from "node:fs";
import manifest from "./agent.json";
import { helpAnswer, type HelpAsk } from "../shared/metadata.ts";
const envDist = readFileSync(new URL("./env.dist", import.meta.url), "utf8");
export function describe(_who: string, any: boolean, ask: HelpAsk = "help", json = false): string {
  return helpAnswer(manifest, ask, envDist, any ? ["owner"] : [], json, _who);
}
