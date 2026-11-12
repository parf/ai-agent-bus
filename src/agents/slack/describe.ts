// The manifest is the only method/settings description, including bare runs.
import { readFileSync } from "node:fs";
import manifest from "./agent.json";
import { fileURLToPath } from "node:url";
import { helpAnswer, imageHandler, type HelpAsk } from "../shared/metadata.ts";
const envDist = readFileSync(new URL("./env.dist", import.meta.url), "utf8");
// The package folder: beside the bundle, as env.dist is.
const dir = fileURLToPath(new URL(".", import.meta.url));
export function describe(_who: string, any: boolean, ask: HelpAsk = "help", json = false): string {
  return helpAnswer(manifest, ask, envDist, any ? ["owner"] : [], json, _who, dir);
}
export const imageAsk = imageHandler(manifest, dir);
