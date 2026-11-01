import {expect,test} from "bun:test";
import {join} from "node:path";
import {version} from "./version.ts";
const root=join(import.meta.dir,"..");
for (const [name,args] of [
 ["MCP",["mcp/server.ts","--version"]],
 ["web",["web/server.ts","--version"]],
 ["launcher",["launchers/launcher.ts","claude","--version"]],
] as const) test(`${name} version credits the author after release/build`,()=>{
 const runArgs=name==="web" ? ["server.ts","--version"] : [...args];
 const cwd=name==="web" ? join(root,"web") : root;
 const p=Bun.spawnSync([process.execPath,...runArgs],{cwd,env:{...process.env,AGENT_BUS_BUILD_INFO:"test build"},stderr:"pipe"});
 expect(p.exitCode).toBe(0);
 expect(p.stdout.toString().trim().split("\n")).toEqual([version,"build_info: test build","© Sergey Porfiriev"]);
});
