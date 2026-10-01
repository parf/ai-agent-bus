// Build a Node/Bun ESM package with the repository's shared version.
import { readFileSync, writeFileSync, mkdirSync, copyFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { resolve, dirname } from 'node:path';
import { execFileSync } from 'node:child_process';
const here=dirname(fileURLToPath(import.meta.url));
const root=resolve(here,'../../..');
const version=readFileSync(process.env.AB_CANONICAL_VERSION_FILE ?? resolve(root,'src/internal/version/VERSION'),'utf8').trim();
if (!/^\d+\.\d+\.\d+$/.test(version)) throw new Error('invalid canonical version');
const destination=resolve(process.argv[2] ?? resolve(here,'dist'));
mkdirSync(destination,{recursive:true});
execFileSync('bun',['build',resolve(here,'src/index.ts'),'--target=node','--outdir',destination,'--define',`process.env.AGENT_BUS_BUILD_VERSION=${JSON.stringify(version)}`],{stdio:'inherit',cwd:here});
execFileSync(process.execPath,[resolve(here,'node_modules/typescript/bin/tsc'),'--project',resolve(here,'tsconfig.build.json'),'--outDir',resolve(destination,'types')],{stdio:'inherit',cwd:here});
writeFileSync(resolve(destination,'package.json'),JSON.stringify({name:'@agent-bus/client',version,type:'module',license:'MIT',exports:{'.':{types:'./types/index.d.ts',import:'./index.js'}},files:['types','index.js','LICENSE.md','THIRD-PARTY-LICENSE.txt','README.md']},null,2)+'\n');
for(const file of ['LICENSE.md','README.md','THIRD-PARTY-LICENSE.txt'])copyFileSync(resolve(here,file),resolve(destination,file));
