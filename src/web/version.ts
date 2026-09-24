// The shared SemVer, read from its one canonical file at start. An interpreted
// face reports SemVer only (docs/09-setup.md#build-information).
import { readFileSync } from "node:fs";
import { join } from "node:path";

export const VERSION = readFileSync(join(import.meta.dir, "../internal/version/VERSION"), "utf8").trim();
