import { readFileSync } from 'node:fs';
// The build helper embeds the repository's canonical value in distributable JS.
export const VERSION = process.env.AGENT_BUS_BUILD_VERSION ?? readFileSync(new URL('../../../internal/version/VERSION',import.meta.url),'utf8').trim();
