import {expect,test} from "bun:test";
import {readFileSync} from "node:fs";

test("reviewer settings example is valid strict sandbox JSON",()=>{
 const raw=readFileSync(new URL("./review-settings.json.dist",import.meta.url),"utf8");
 expect(()=>JSON.parse(raw)).not.toThrow();
 const settings=JSON.parse(raw);
 expect(settings.sandbox.enabled).toBe(true);
 expect(settings.sandbox.allowUnsandboxedCommands).toBe(false);
 expect(settings.sandbox.failIfUnavailable).toBe(true);
 expect(settings.sandbox.filesystem.denyWrite).toContain("/srv/project");
 expect(settings.sandbox.filesystem.denyWrite).toContain("/home/me");
});
